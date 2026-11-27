package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gowave/internal/database"
)

func TestHostPreviewUsesSubscriptionRules(t *testing.T) {
	e := newProbeEnv(t)
	profile := e.mustCreate("/api/config-profiles", map[string]any{
		"name": "Host preview", "config": json.RawMessage(probeXrayConfig),
	}, "uuid")
	var inbound database.ConfigProfileInbound
	if err := e.db.Where("profile_uuid = ?", profile).First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	host := map[string]any{
		"remark": "Preview host", "address": "saved.example.com", "port": 8443, "isDisabled": true,
		"inbound": map[string]string{"configProfileUuid": profile, "configProfileInboundUuid": inbound.UUID},
	}
	id := e.mustCreate("/api/hosts", host, "uuid")
	const previewPath = "/api/hosts/preview"
	type previewResult struct {
		Type    string            `json:"clientType"`
		Rule    string            `json:"ruleName"`
		Status  *int              `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"previewContent"`
	}
	preview := func(input map[string]any) previewResult {
		t.Helper()
		status, body := e.do(http.MethodPost, previewPath, input)
		var envelope struct {
			Response previewResult `json:"response"`
		}
		if err := json.Unmarshal(body, &envelope); status != http.StatusOK || err != nil {
			t.Fatalf("preview returned %d: %s (%v)", status, body, err)
		}
		return envelope.Response
	}
	input := map[string]any{"hostUuid": id, "userAgent": "ClashMeta"}
	if result := preview(input); result.Type != "MIHOMO" || strings.Contains(result.Body, "saved.example.com") {
		t.Fatalf("disabled host appeared in preview or format was wrong: %#v", result)
	}
	input["previewAsEnabled"] = true
	if result := preview(input); !strings.Contains(result.Body, "saved.example.com") || !strings.Contains(result.Body, "port: 8443") {
		t.Fatalf("preview did not use the saved host and inbound: %#v", result)
	}
	input["userAgent"] = "generic-client"
	if result := preview(input); result.Type != "XRAY_BASE64" {
		t.Fatalf("changing User-Agent did not change the format: %#v", result)
	} else if body, err := base64.StdEncoding.DecodeString(result.Body); err != nil || !strings.Contains(string(body), "@saved.example.com:8443") {
		t.Fatalf("unexpected Base64 response: %q (%v)", body, err)
	}

	status, body := e.do(http.MethodPatch, "/api/subscription-settings", json.RawMessage(`{
		"customResponseHeaders": {"x-preview-source": "global", "x-sample-user": "{{USERNAME}}"},
		"responseRules": {
			"version": "1",
			"rules": [
				{"name":"Custom YAML","enabled":true,"responseType":"MIHOMO",
				 "conditions":[{"headerName":"x-preview-mode","operator":"EQUALS","value":"yaml"}],
				 "responseModifications":{"headers":{"X-Preview-Source":"rule"},"applyHeadersToEnd":true}},
				{"name":"Close socket","enabled":true,"responseType":"SOCKET_DROP",
				 "conditions":[{"headerName":"x-preview-mode","operator":"EQUALS","value":"drop"}]},
				{"name":"Unavailable","enabled":true,"responseType":"STATUS_CODE_451","conditions":[]}
			]
		}
	}`))
	if status != http.StatusOK {
		t.Fatalf("settings update returned %d: %s", status, body)
	}

	result := preview(input)
	if result.Status == nil || *result.Status != http.StatusUnavailableForLegalReasons || result.Rule != "Unavailable" {
		t.Fatalf("unexpected blocked preview: %#v", result)
	}
	input["requestHeaders"] = []map[string]string{{"name": "X-Preview-Mode", "value": "drop"}}
	if result := preview(input); result.Type != "SOCKET_DROP" || result.Status != nil || len(result.Headers) != 0 {
		t.Fatalf("unexpected socket drop preview: %#v", result)
	}

	host["address"] = "draft.example.com"
	result = preview(map[string]any{
		"host": host, "userAgent": "generic-client", "previewAsEnabled": true,
		"requestHeaders": []map[string]string{{"name": "x-Preview-mode", "value": "yaml"}},
	})
	if result.Status == nil || *result.Status != http.StatusOK || result.Rule != "Custom YAML" || result.Type != "MIHOMO" ||
		result.Headers["X-Preview-Source"] != "rule" || result.Headers["X-Sample-User"] != "preview" ||
		!strings.Contains(result.Body, "draft.example.com") {
		t.Fatalf("unexpected draft preview: %#v", result)
	}

	var saved database.Host
	if err := e.db.Where("uuid = ?", id).Take(&saved).Error; err != nil || !saved.IsDisabled || saved.Address != "saved.example.com" {
		t.Fatalf("preview changed the saved host: %#v (%v)", saved, err)
	}
	for _, model := range []any{&database.User{}, &database.UserSubscriptionRequestHistory{}, &database.HwidDevice{}} {
		var count int64
		if err := e.db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("preview stored %T rows: %d (%v)", model, count, err)
		}
	}
}

func TestHostPreviewInjectsXrayOutbounds(t *testing.T) {
	e := newProbeEnv(t)
	profile := e.mustCreate("/api/config-profiles", map[string]any{
		"name": "Injector preview", "config": json.RawMessage(probeXrayConfig),
	}, "uuid")
	var inbound database.ConfigProfileInbound
	if err := e.db.Where("profile_uuid = ?", profile).First(&inbound).Error; err != nil {
		t.Fatal(err)
	}

	status, body := e.do(http.MethodPatch, "/api/subscription-settings", json.RawMessage(`{
		"responseRules": {
			"version": "1",
			"rules": [{"name":"Xray preview","enabled":true,"responseType":"XRAY_JSON","conditions":[]}]
		}
	}`))
	if status != http.StatusOK {
		t.Fatalf("settings update returned %d: %s", status, body)
	}

	if err := e.db.Model(&database.SubscriptionTemplate{}).Where("template_type = ?", "XRAY_JSON").
		Update("template_json", `{
			"remnawave":{"injectHosts":[
				{"selectFrom":"HIDDEN","selector":{"type":"remarkRegex","pattern":"^inject "}}
			]},
			"outbounds":[{"tag":"direct","protocol":"freedom"}]
		}`).Error; err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []struct {
		remark, address  string
		hidden, disabled bool
	}{
		{"inject hidden", "hidden.example.com", true, false},
		{"inject visible", "visible.example.com", false, false},
		{"inject disabled", "disabled.example.com", true, true},
		{"other hidden", "other.example.com", true, false},
	} {
		e.mustCreate("/api/hosts", map[string]any{
			"remark": candidate.remark, "address": candidate.address, "port": 443,
			"isHidden": candidate.hidden, "isDisabled": candidate.disabled,
			"inbound": map[string]string{"configProfileUuid": profile, "configProfileInboundUuid": inbound.UUID},
		}, "uuid")
	}

	target := map[string]any{
		"remark": "Autoselect", "address": "virtual.example.com", "port": 443,
		"inbound": map[string]string{"configProfileUuid": profile, "configProfileInboundUuid": inbound.UUID},
	}
	id := e.mustCreate("/api/hosts", target, "uuid")
	for name, input := range map[string]map[string]any{
		"saved": {"hostUuid": id, "userAgent": "Happ/2.0"},
		"draft": {"host": target, "userAgent": "Happ/2.0"},
	} {
		t.Run(name, func(t *testing.T) {
			var envelope struct {
				Response struct {
					Type   string `json:"clientType"`
					Status int    `json:"status"`
					Body   string `json:"previewContent"`
				} `json:"response"`
			}
			status, body := e.do(http.MethodPost, "/api/hosts/preview", input)
			if err := json.Unmarshal(body, &envelope); err != nil || status != http.StatusOK {
				t.Fatalf("preview returned %d: %s (%v)", status, body, err)
			}
			result := envelope.Response
			var configs []struct {
				Remarks   string            `json:"remarks"`
				Outbounds []json.RawMessage `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(result.Body), &configs); err != nil ||
				result.Type != "XRAY_JSON" || result.Status != http.StatusOK ||
				len(configs) != 1 || configs[0].Remarks != "Autoselect" || len(configs[0].Outbounds) != 2 {
				t.Fatalf("unexpected Xray preview: %#v (%v)", result, err)
			}
			if !strings.Contains(string(configs[0].Outbounds[0]), "hidden.example.com") ||
				!strings.Contains(string(configs[0].Outbounds[1]), `"tag":"direct"`) {
				t.Fatalf("unexpected outbounds: %s", result.Body)
			}
		})
	}
}
