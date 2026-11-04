package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gowave/internal/database"
	"gowave/internal/subscription"
)

// TestSubscriptionCacheFollowsAdminChanges warms the public subscription
// cache before every admin change and expects the next request to see it.
// The settle window is off, so only the write tracking keeps it fresh.
func TestSubscriptionCacheFollowsAdminChanges(t *testing.T) {
	settle := subscription.CacheSettle
	subscription.CacheSettle = 0
	t.Cleanup(func() { subscription.CacheSettle = settle })

	e := newProbeEnv(t)
	mustDo := func(method, path string, body interface{}) json.RawMessage {
		t.Helper()
		status, data := e.do(method, path, body)
		if status >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, status, data)
		}
		return data
	}
	var config interface{}
	_ = json.Unmarshal([]byte(probeXrayConfig), &config)
	profile := e.mustCreate("/api/config-profiles", map[string]interface{}{"name": "Cache", "config": config}, "uuid")
	var inbound database.ConfigProfileInbound
	if err := e.db.Where("profile_uuid = ?", profile).First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	squad := e.mustCreate("/api/internal-squads", map[string]interface{}{"name": "Main", "inbounds": []string{inbound.UUID}}, "uuid")
	other := e.mustCreate("/api/internal-squads", map[string]interface{}{"name": "Other", "inbounds": []string{inbound.UUID}}, "uuid")
	newHost := func(remark, address string) string {
		return e.mustCreate("/api/hosts", map[string]interface{}{
			"inbound": map[string]interface{}{"configProfileUuid": profile, "configProfileInboundUuid": inbound.UUID},
			"remark":  remark, "address": address, "port": 443,
		}, "uuid")
	}
	first := newHost("first", "first.example.com")
	expire := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	userID := responseField(mustDo("POST", "/api/users", map[string]interface{}{"username": "cache_user", "expireAt": expire, "activeInternalSquads": []string{squad}}), "id")
	var user database.User
	if err := e.db.Where("username = ?", "cache_user").First(&user).Error; err != nil {
		t.Fatal(err)
	}

	sub := func(kind string) (http.Header, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", e.server.URL+"/api/sub/"+user.ShortUUID+"/"+kind, nil)
		req.Header.Set("User-Agent", "cache-test/1.0")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: %d %s", kind, res.StatusCode, body)
		}
		return res.Header, string(body)
	}
	// change warms the cache with every format, applies the change, then
	// checks the JSON subscription.
	change := func(name string, apply func(), check func(header http.Header, body string) bool) {
		t.Helper()
		for _, kind := range []string{"json", "mihomo", "singbox"} {
			sub(kind)
		}
		apply()
		if header, body := sub("json"); !check(header, body) {
			t.Fatalf("%s: stale subscription: %v\n%s", name, header, body)
		}
	}
	has := func(text string) func(http.Header, string) bool {
		return func(_ http.Header, body string) bool { return strings.Contains(body, text) }
	}
	lacks := func(text string) func(http.Header, string) bool {
		return func(_ http.Header, body string) bool { return !strings.Contains(body, text) }
	}

	if _, body := sub("json"); !strings.Contains(body, "first.example.com") {
		t.Fatalf("initial subscription: %s", body)
	}

	// Hosts.
	change("host address", func() {
		mustDo("PATCH", "/api/hosts", map[string]interface{}{"uuid": first, "address": "renamed.example.com"})
	}, has("renamed.example.com"))
	var second string
	change("new host", func() { second = newHost("second", "second.example.com") }, has("second.example.com"))
	change("host allow-only other squad", func() {
		mustDo("PATCH", "/api/hosts", map[string]interface{}{"uuid": second, "internalSquads": map[string]interface{}{"mode": "ALLOW_ONLY", "squads": []string{other}}})
	}, lacks("second.example.com"))
	change("host allow-only user squad", func() {
		mustDo("PATCH", "/api/hosts", map[string]interface{}{"uuid": second, "internalSquads": map[string]interface{}{"mode": "ALLOW_ONLY", "squads": []string{squad}}})
	}, has("second.example.com"))
	change("host excludes user squad", func() {
		mustDo("PATCH", "/api/hosts", map[string]interface{}{"uuid": second, "internalSquads": map[string]interface{}{"mode": "EXCLUDE", "squads": []string{squad}}})
	}, lacks("second.example.com"))
	change("host excludes other squad", func() {
		mustDo("PATCH", "/api/hosts", map[string]interface{}{"uuid": second, "internalSquads": map[string]interface{}{"mode": "EXCLUDE", "squads": []string{other}}})
	}, has("second.example.com"))
	change("bulk disable", func() {
		mustDo("POST", "/api/hosts/bulk/disable", map[string]interface{}{"uuids": []string{second}})
	}, lacks("second.example.com"))
	change("bulk enable", func() {
		mustDo("POST", "/api/hosts/bulk/enable", map[string]interface{}{"uuids": []string{second}})
	}, has("second.example.com"))
	change("bulk update", func() {
		mustDo("PATCH", "/api/hosts/bulk/update", map[string]interface{}{"uuids": []string{second}, "port": 8443})
	}, has("8443"))
	change("delete host", func() { mustDo("DELETE", "/api/hosts/"+second, nil) }, lacks("second.example.com"))

	// Config profile inbounds.
	change("inbound transport", func() {
		changed := strings.Replace(probeXrayConfig, `"network":"tcp"`, `"network":"ws","wsSettings":{"path":"/cache-path"}`, 1)
		var value interface{}
		_ = json.Unmarshal([]byte(changed), &value)
		mustDo("PATCH", "/api/config-profiles", map[string]interface{}{"uuid": profile, "config": value})
	}, has("/cache-path"))

	// Internal squads and membership.
	change("squad loses inbound", func() {
		mustDo("PATCH", "/api/internal-squads", map[string]interface{}{"uuid": squad, "inbounds": []string{}})
	}, lacks("renamed.example.com"))
	change("squad regains inbound", func() {
		mustDo("PATCH", "/api/internal-squads", map[string]interface{}{"uuid": squad, "inbounds": []string{inbound.UUID}})
	}, has("renamed.example.com"))
	change("user leaves squads", func() {
		mustDo("POST", "/api/users/bulk/update-squads", map[string]interface{}{"userIds": []json.Number{json.Number(userID)}, "activeInternalSquads": []string{}})
	}, lacks("renamed.example.com"))
	change("user joins other squad", func() {
		mustDo("POST", "/api/users/bulk/update-squads", map[string]interface{}{"userIds": []json.Number{json.Number(userID)}, "activeInternalSquads": []string{other}})
	}, has("renamed.example.com"))
	change("delete squad", func() { mustDo("DELETE", "/api/internal-squads/"+other, nil) }, lacks("renamed.example.com"))
	mustDo("POST", "/api/users/bulk/update-squads", map[string]interface{}{"userIds": []json.Number{json.Number(userID)}, "activeInternalSquads": []string{squad}})

	// Subscription settings.
	settingsUUID := responseField(mustDo("GET", "/api/subscription-settings", nil), "uuid")
	change("settings headers", func() {
		mustDo("PATCH", "/api/subscription-settings", map[string]interface{}{"uuid": settingsUUID, "customResponseHeaders": map[string]string{"X-Cache-Test": "settings"}})
	}, func(header http.Header, _ string) bool { return header.Get("X-Cache-Test") == "settings" })

	// Templates.
	var templates struct {
		Response struct {
			Templates []struct {
				UUID         string `json:"uuid"`
				Name         string `json:"name"`
				TemplateType string `json:"templateType"`
			} `json:"templates"`
		} `json:"response"`
	}
	_ = json.Unmarshal(mustDo("GET", "/api/subscription-templates", nil), &templates)
	defaultJSON := ""
	for _, template := range templates.Response.Templates {
		if template.Name == "Default" && template.TemplateType == "XRAY_JSON" {
			defaultJSON = template.UUID
		}
	}
	if defaultJSON == "" {
		t.Fatalf("no Default XRAY_JSON template: %+v", templates)
	}
	change("default template", func() {
		mustDo("PATCH", "/api/subscription-templates", map[string]interface{}{"uuid": defaultJSON, "templateJson": map[string]interface{}{"dns": map[string]interface{}{"tag": "default-marker"}}})
	}, has("default-marker"))
	custom := e.mustCreate("/api/subscription-templates", map[string]interface{}{"name": "squad-template", "templateType": "XRAY_JSON"}, "uuid")
	mustDo("PATCH", "/api/subscription-templates", map[string]interface{}{"uuid": custom, "templateJson": map[string]interface{}{"dns": map[string]interface{}{"tag": "squad-marker"}}})

	// External squads.
	external := e.mustCreate("/api/external-squads", map[string]interface{}{"name": "ext"}, "uuid")
	change("join external squad", func() {
		mustDo("POST", "/api/external-squads/"+external+"/bulk-actions/add-users", nil)
		mustDo("PATCH", "/api/external-squads", map[string]interface{}{"uuid": external, "responseHeadersAdd": map[string]string{"X-Squad": "added"}})
	}, func(header http.Header, _ string) bool { return header.Get("X-Squad") == "added" })
	change("external squad headers", func() {
		mustDo("PATCH", "/api/external-squads", map[string]interface{}{"uuid": external, "responseHeadersAdd": map[string]string{"X-Squad": "changed"}})
	}, func(header http.Header, _ string) bool { return header.Get("X-Squad") == "changed" })
	change("external squad template", func() {
		mustDo("PATCH", "/api/external-squads", map[string]interface{}{"uuid": external, "templates": []map[string]string{{"templateUuid": custom, "templateType": "XRAY_JSON"}}})
	}, has("squad-marker"))
	change("squad template content", func() {
		mustDo("PATCH", "/api/subscription-templates", map[string]interface{}{"uuid": custom, "templateJson": map[string]interface{}{"dns": map[string]interface{}{"tag": "squad-marker-2"}}})
	}, has("squad-marker-2"))
	change("delete external squad", func() { mustDo("DELETE", "/api/external-squads/"+external, nil) },
		func(header http.Header, body string) bool {
			return header.Get("X-Squad") == "" && strings.Contains(body, "default-marker")
		})

	// Writes outside the API are tracked as well.
	change("raw host update", func() {
		if err := e.db.Exec("UPDATE hosts SET address = ? WHERE uuid = ?", "raw.example.com", first).Error; err != nil {
			t.Fatal(err)
		}
	}, has("raw.example.com"))
	change("model delete", func() {
		if err := e.db.Where("1 = 1").Delete(&database.Host{}).Error; err != nil {
			t.Fatal(err)
		}
	}, lacks("raw.example.com"))
}
