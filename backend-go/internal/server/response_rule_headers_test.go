package server

import (
	"encoding/json"
	"strings"
	"testing"

	"gowave/internal/database"
)

func TestResponseRuleHeadersSettingsAndDebugger(t *testing.T) {
	e := newProbeEnv(t)
	for _, headers := range []string{
		`{"X-Custom":"value"}`,
		`[{"key":"X-Custom","value":"old"},{"key":"x-custom","value":"value"}]`,
	} {
		rules := json.RawMessage(`{"version":"1","rules":[{"name":"headers","enabled":true,"operator":"AND","conditions":[],"responseType":"XRAY_BASE64","responseModifications":{"headers":` + headers + `}}]}`)
		status, body := e.do("PATCH", "/api/subscription-settings", map[string]any{"responseRules": rules})
		if status != 200 || strings.Contains(string(body), `"headers":[`) {
			t.Fatalf("save: %d %s", status, body)
		}
		var saved database.SubscriptionSetting
		if err := e.db.First(&saved).Error; err != nil {
			t.Fatal(err)
		}
		if strings.Contains(saved.ResponseRules, `"headers":[`) || !strings.Contains(saved.ResponseRules, `"value"`) {
			t.Fatalf("legacy or missing persisted headers: %s", saved.ResponseRules)
		}
		status, body = e.do("GET", "/api/subscription-settings", nil)
		if status != 200 || strings.Contains(string(body), `"headers":[`) {
			t.Fatalf("get: %d %s", status, body)
		}
		status, body = e.do("POST", "/api/system/testers/srr-matcher", map[string]any{"responseRules": rules})
		var result struct {
			Response struct {
				Matched       bool              `json:"matched"`
				OutputHeaders map[string]string `json:"outputHeaders"`
			} `json:"response"`
		}
		if err := json.Unmarshal(body, &result); err != nil || status != 200 || !result.Response.Matched {
			t.Fatalf("debugger: %d %s (%v)", status, body, err)
		}
		if len(result.Response.OutputHeaders) != 1 || strings.Contains(string(body), `"headers":[`) {
			t.Fatalf("debugger did not normalize headers: %s", body)
		}
	}
}

func TestResponseRuleHeadersInvalidUpdateIsAtomic(t *testing.T) {
	e := newProbeEnv(t)
	var before database.SubscriptionSetting
	if err := e.db.First(&before).Error; err != nil {
		t.Fatal(err)
	}
	status, body := e.do("PATCH", "/api/subscription-settings", map[string]any{
		"randomizeHosts": !before.RandomizeHosts,
		"responseRules":  json.RawMessage(`{"version":"1","rules":[{"responseModifications":{"headers":{"X-Test":123}}}]}`),
	})
	if status != 400 {
		t.Fatalf("invalid headers: %d %s", status, body)
	}
	var after database.SubscriptionSetting
	if err := e.db.First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.ResponseRules != before.ResponseRules || after.RandomizeHosts != before.RandomizeHosts {
		t.Fatal("invalid update changed persisted settings")
	}
}
