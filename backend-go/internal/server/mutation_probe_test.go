package server

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gowave/internal/contracts"
	"gowave/internal/database"
)

type probeResult struct {
	Name   string          `json:"name"`
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Status int             `json:"status"`
	Expect int             `json:"expectedStatus,omitempty"`
	Body   json.RawMessage `json:"body"`
}

// step runs one contract command and checks the upstream success status.
func (e *probeEnv) step(results *[]probeResult, failures *[]string, method, path string, body interface{}) json.RawMessage {
	e.t.Helper()
	command, ok := contracts.FindCommand(method, strings.SplitN(path, "?", 2)[0])
	if !ok {
		e.t.Fatalf("%s %s is not a contract command", method, path)
	}
	status, data := e.do(method, path, body)
	if status != command.Status {
		*failures = append(*failures, fmt.Sprintf("%s %s (%s): status %d, want %d: %s", method, path, command.Name, status, command.Status, data))
	}
	raw := json.RawMessage(data)
	if len(data) == 0 {
		raw = json.RawMessage("null")
	} else if !json.Valid(data) {
		raw = mustJSON(string(data))
	}
	*results = append(*results, probeResult{Name: command.Name, Method: method, Path: path, Status: status, Body: raw})
	return raw
}

func responseField(raw json.RawMessage, path ...string) string {
	var value interface{}
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	object, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}
	value = object["response"]
	for _, key := range path {
		object, ok := value.(map[string]interface{})
		if !ok {
			return ""
		}
		value = object[key]
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprint(int64(typed))
	}
	return ""
}

// TestContractMutationProbe drives create/update/action/bulk/delete commands
// in dependency order, checking statuses here and response bodies through
// scripts/validate-contract.cjs (GOWAVE_PROBE_OUT + "-mutations.json").
func TestContractMutationProbe(t *testing.T) {
	e := newProbeEnv(t)
	if err := e.db.Model(&database.PanelSetting{}).Where("id = 1").
		Update("passkey_settings", `{"enabled":true,"rpId":"panel.example.com","origin":"https://panel.example.com"}`).Error; err != nil {
		t.Fatal(err)
	}
	var results []probeResult
	var failures []string
	step := func(method, path string, body interface{}) json.RawMessage {
		return e.step(&results, &failures, method, path, body)
	}
	settingsUUID := responseField(step("GET", "/api/subscription-settings", nil), "uuid")
	for _, headers := range []string{`{"X-Probe":"value"}`, `[{"key":"X-Probe","value":"value"}]`} {
		rules := json.RawMessage(`{"version":"1","rules":[{"name":"headers","enabled":true,"operator":"AND","conditions":[],"responseType":"XRAY_BASE64","responseModifications":{"headers":` + headers + `}}]}`)
		step("PATCH", "/api/subscription-settings", map[string]interface{}{"uuid": settingsUUID, "responseRules": rules})
		step("POST", "/api/system/testers/srr-matcher", map[string]interface{}{"responseRules": rules})
	}

	var config interface{}
	_ = json.Unmarshal([]byte(probeXrayConfig), &config)
	profile := responseField(step("POST", "/api/config-profiles", map[string]interface{}{"name": "Mutations", "config": config}), "uuid")
	step("PATCH", "/api/config-profiles", map[string]interface{}{"uuid": profile, "name": "Mutations2"})
	step("PATCH", "/api/config-profiles/tags", map[string]interface{}{"uuid": profile, "tags": []string{"EU"}})
	step("POST", "/api/config-profiles/actions/reorder", map[string]interface{}{"items": []map[string]interface{}{{"uuid": profile, "viewPosition": 1}}})

	var inbound database.ConfigProfileInbound
	if err := e.db.Where("profile_uuid = ?", profile).First(&inbound).Error; err != nil {
		t.Fatalf("profile inbound: %v", err)
	}
	squad := responseField(step("POST", "/api/internal-squads", map[string]interface{}{"name": "Squad", "inbounds": []string{inbound.UUID}}), "uuid")
	step("PATCH", "/api/internal-squads", map[string]interface{}{"uuid": squad, "name": "Squad2"})
	step("PATCH", "/api/internal-squads/tags", map[string]interface{}{"uuid": squad, "tags": []string{"MAIN"}})
	step("POST", "/api/internal-squads/actions/reorder", map[string]interface{}{"items": []map[string]interface{}{{"uuid": squad, "viewPosition": 1}}})

	node := responseField(step("POST", "/api/nodes", map[string]interface{}{"name": "mut-node", "address": "198.51.100.7", "port": 2222, "configProfile": map[string]interface{}{"activeConfigProfileUuid": profile, "activeInbounds": []string{inbound.UUID}}}), "uuid")
	step("PATCH", "/api/nodes", map[string]interface{}{"uuid": node, "name": "mut-node-2", "countryCode": "DE"})
	step("POST", "/api/nodes/"+node+"/actions/disable", nil)
	step("POST", "/api/nodes/"+node+"/actions/enable", nil)
	step("POST", "/api/nodes/"+node+"/actions/reset-traffic", nil)
	step("POST", "/api/nodes/actions/reorder", map[string]interface{}{"nodes": []map[string]interface{}{{"uuid": node, "viewPosition": 1}}})
	step("POST", "/api/nodes/bulk-actions/update", map[string]interface{}{"uuids": []string{node}, "fields": map[string]interface{}{"countryCode": "NL"}})
	step("PUT", "/api/metadata/node/"+node, map[string]interface{}{"metadata": map[string]interface{}{"rack": "a1"}})

	host := responseField(step("POST", "/api/hosts", map[string]interface{}{"inbound": map[string]interface{}{"configProfileUuid": profile, "configProfileInboundUuid": inbound.UUID}, "remark": "mut-host", "address": "mut.example.com", "port": 443}), "uuid")
	step("PATCH", "/api/hosts", map[string]interface{}{"uuid": host, "remark": "mut-host-2", "sni": nil})
	clone := responseField(step("POST", "/api/hosts/actions/clone", map[string]interface{}{"cloneFromUuid": host}), "uuid")
	step("POST", "/api/hosts/actions/reorder", map[string]interface{}{"hosts": []map[string]interface{}{{"uuid": host, "viewPosition": 1}, {"uuid": clone, "viewPosition": 2}}})
	step("POST", "/api/hosts/bulk/disable", map[string]interface{}{"uuids": []string{host, clone}})
	var disabled int64
	e.db.Model(&database.Host{}).Where("uuid IN ? AND is_disabled = ?", []string{host, clone}, true).Count(&disabled)
	if disabled != 2 {
		failures = append(failures, fmt.Sprintf("bulk disable changed %d of 2 hosts", disabled))
	}
	step("POST", "/api/hosts/bulk/enable", map[string]interface{}{"uuids": []string{host, clone}})
	step("PATCH", "/api/hosts/bulk/update", map[string]interface{}{"uuids": []string{host, clone}, "tags": []string{"BULK"}})
	step("POST", "/api/hosts/bulk/delete", map[string]interface{}{"uuids": []string{clone}})

	external := responseField(step("POST", "/api/external-squads", map[string]interface{}{"name": "ext"}), "uuid")
	step("PATCH", "/api/external-squads", map[string]interface{}{"uuid": external, "hwidSettings": map[string]interface{}{"enabled": true, "fallbackDeviceLimit": 3, "maxDevicesAnnounce": nil}})
	step("PATCH", "/api/external-squads/tags", map[string]interface{}{"uuid": external, "tags": []string{"EXT"}})

	expire := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	user := responseField(step("POST", "/api/users", map[string]interface{}{"username": "mut_user", "expireAt": expire, "activeInternalSquads": []string{squad}}), "id")
	second := responseField(step("POST", "/api/users", map[string]interface{}{"username": "mut_user_2", "expireAt": expire}), "id")
	step("PATCH", "/api/users", map[string]interface{}{"id": json.Number(user), "description": "probe", "trafficLimitBytes": 1 << 30})
	step("POST", "/api/users/resolve", map[string]interface{}{"username": "mut_user"})
	step("POST", "/api/users/"+user+"/actions/disable", nil)
	step("POST", "/api/users/"+user+"/actions/enable", nil)
	step("POST", "/api/users/"+user+"/actions/extend", map[string]interface{}{"days": 5})
	step("POST", "/api/users/"+user+"/actions/reset-traffic", nil)
	step("POST", "/api/users/"+user+"/actions/revoke", map[string]interface{}{})
	step("PUT", "/api/metadata/user/"+user, map[string]interface{}{"metadata": map[string]interface{}{"plan": "pro"}})
	step("POST", "/api/external-squads/"+external+"/bulk-actions/add-users", nil)
	step("DELETE", "/api/external-squads/"+external+"/bulk-actions/remove-users", nil)
	step("POST", "/api/internal-squads/"+squad+"/bulk-actions/add-many-users", map[string]interface{}{"userIds": []json.Number{json.Number(second)}})
	step("DELETE", "/api/internal-squads/"+squad+"/bulk-actions/remove-many-users", map[string]interface{}{"userIds": []json.Number{json.Number(second)}})
	ids := []json.Number{json.Number(user), json.Number(second)}
	step("POST", "/api/users/bulk/extend-expiration-date", map[string]interface{}{"userIds": ids, "extendDays": 1})
	step("POST", "/api/users/bulk/reset-traffic", map[string]interface{}{"userIds": ids})
	step("POST", "/api/users/bulk/update", map[string]interface{}{"userIds": ids, "fields": map[string]interface{}{"tag": "BULK"}})
	step("POST", "/api/users/bulk/update-squads", map[string]interface{}{"userIds": ids, "activeInternalSquads": []string{squad}})
	step("POST", "/api/users/bulk/revoke-subscription", map[string]interface{}{"userIds": ids})
	step("POST", "/api/users/bulk/all/extend-expiration-date", map[string]interface{}{"extendDays": 1})
	step("POST", "/api/users/bulk/all/reset-traffic", nil)
	step("POST", "/api/users/bulk/all/update", map[string]interface{}{"description": "all"})
	step("POST", "/api/hwid/devices", map[string]interface{}{"hwid": "probe-hwid", "userId": json.Number(user)})
	step("POST", "/api/hwid/devices/delete", map[string]interface{}{"hwid": "probe-hwid", "userId": json.Number(user)})
	step("POST", "/api/hwid/devices/delete-all", map[string]interface{}{"userId": json.Number(user)})

	template := responseField(step("POST", "/api/subscription-templates", map[string]interface{}{"name": "mut-template", "templateType": "XRAY_JSON"}), "uuid")
	step("PATCH", "/api/subscription-templates", map[string]interface{}{"uuid": template, "name": "mut-template-2"})
	step("PATCH", "/api/subscription-templates/tags", map[string]interface{}{"uuid": template, "tags": []string{"T"}})
	subpage := responseField(step("POST", "/api/subscription-page-configs", map[string]interface{}{"name": "mut-subpage"}), "uuid")
	subpageClone := responseField(step("POST", "/api/subscription-page-configs/actions/clone", map[string]interface{}{"cloneFromUuid": subpage}), "uuid")
	step("PATCH", "/api/subscription-page-configs/tags", map[string]interface{}{"uuid": subpage, "tags": []string{"S"}})

	plugin := responseField(step("POST", "/api/node-plugins", map[string]interface{}{"name": "mut-plugin"}), "uuid")
	pluginClone := responseField(step("POST", "/api/node-plugins/actions/clone", map[string]interface{}{"cloneFromUuid": plugin}), "uuid")
	step("PATCH", "/api/node-plugins/tags", map[string]interface{}{"uuid": plugin, "tags": []string{"P"}})
	step("POST", "/api/node-plugins/shared-lists", map[string]interface{}{"name": "mut-list", "config": map[string]interface{}{"type": "ipList", "items": []string{"10.0.0.1"}}})
	step("POST", "/api/snippets", map[string]interface{}{"name": "mut-snippet", "snippet": []interface{}{map[string]interface{}{"tag": "x"}}})
	step("PATCH", "/api/snippets", map[string]interface{}{"name": "mut-snippet", "snippet": []interface{}{map[string]interface{}{"tag": "y"}}})
	integration := responseField(step("POST", "/api/node-integrations", map[string]interface{}{"name": "mut-int", "config": map[string]interface{}{}}), "uuid")
	provider := responseField(step("POST", "/api/infra-billing/providers", map[string]interface{}{"name": "mut-provider"}), "uuid")
	step("PATCH", "/api/infra-billing/providers", map[string]interface{}{"uuid": provider, "loginUrl": "https://provider.example.com"})
	step("POST", "/api/infra-billing/history", map[string]interface{}{"providerUuid": provider, "amount": 5, "billedAt": expire})
	step("POST", "/api/infra-billing/nodes", map[string]interface{}{"providerUuid": provider, "nodeUuid": node, "name": "bill", "nextBillingAt": expire})
	token := responseField(step("POST", "/api/tokens", map[string]interface{}{"name": "probe-token", "expiresInDays": 1, "scopes": []string{"users:read"}}), "uuid")
	step("POST", "/api/tokens/ott", nil)

	// Seed a torrent-blocker report as an agent would deliver it.
	var nodeRow database.Node
	e.db.Where("uuid = ?", node).First(&nodeRow)
	report := `{"actionReport":{"blocked":true,"ip":"198.51.100.20","blockDuration":3600,"willUnblockAt":"2026-03-04T13:00:00.000Z","userId":"` + user + `","processedAt":"2026-03-04T12:00:00.000Z"},"xrayReport":{"email":"` + user + `","level":0,"protocol":"bittorrent","network":"tcp","source":"198.51.100.20:5000","destination":"203.0.113.9:6881","routeTarget":null,"originalTarget":null,"inboundTag":"VLESS","inboundName":null,"inboundLocal":null,"outboundTag":"BLOCK","ts":1772625600}}`
	userID, _ := strconv.ParseUint(user, 10, 64)
	if err := e.db.Create(&database.TorrentBlockerReport{UserID: userID, NodeID: nodeRow.ID, Report: report, CreatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}

	filtered := step("GET", "/api/node-plugins/torrent-blocker?start=0&size=10&filters="+url.QueryEscape(`[{"id":"report.actionReport.ip","value":"198.51.100"},{"id":"user.username","value":"mut_user"}]`)+"&sorting="+url.QueryEscape(`[{"id":"createdAt","desc":true}]`), nil)
	if responseField(filtered, "total") != "1" {
		failures = append(failures, "torrent-blocker JSON filters did not match the seeded report: "+string(filtered))
	}

	// Read every GET command against the populated installation.
	gets, getFailures := e.sweepGets(probeIDs{
		User: user, ShortUUID: responseField(step("POST", "/api/users/resolve", map[string]interface{}{"id": json.Number(user)}), "shortUuid"),
		Username: "mut_user", Profile: profile, Squad: squad, External: external, Host: host, Node: node, Plugin: plugin,
		Integration: integration, Provider: provider, Template: template, Subpage: subpage, SharedList: "mut-list",
	}, map[string]int{"GetPasskeyAuthenticationOptionsCommand": 403})
	results = append(results, gets...)
	failures = append(failures, getFailures...)

	// Teardown exercises deletes and status-filtered bulk deletion.
	step("DELETE", "/api/tokens/"+token, nil)
	step("DELETE", "/api/node-integrations/"+integration, nil)
	step("DELETE", "/api/snippets", map[string]interface{}{"name": "mut-snippet"})
	step("DELETE", "/api/node-plugins/shared-lists", map[string]interface{}{"name": "mut-list"})
	step("DELETE", "/api/node-plugins/"+pluginClone, nil)
	step("DELETE", "/api/subscription-page-configs/"+subpageClone, nil)
	step("DELETE", "/api/subscription-templates/"+template, nil)
	step("POST", "/api/users/bulk/delete", map[string]interface{}{"userIds": []json.Number{json.Number(second)}})
	step("POST", "/api/users/bulk/delete-by-status", map[string]interface{}{"status": "LIMITED"})
	step("DELETE", "/api/users/"+user, nil)
	step("DELETE", "/api/hosts/"+host, nil)
	step("DELETE", "/api/external-squads/"+external, nil)
	step("DELETE", "/api/infra-billing/providers/"+provider, nil)
	step("DELETE", "/api/nodes/"+node, nil)
	step("DELETE", "/api/internal-squads/"+squad, nil)
	step("DELETE", "/api/config-profiles/"+profile, nil)

	if out := os.Getenv("GOWAVE_PROBE_OUT"); out != "" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(strings.TrimSuffix(out, ".json")+"-mutations.json", data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(failures) > 0 {
		t.Fatalf("%d of %d steps diverge from the contract:\n%s", len(failures), len(results), strings.Join(failures, "\n"))
	}
}
