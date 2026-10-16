package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gowave/config"
	"gowave/internal/contracts"
	"gowave/internal/database"
	"gowave/internal/middleware"
	"gowave/internal/testdb"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type probeEnv struct {
	t      *testing.T
	db     *gorm.DB
	server *httptest.Server
	token  string
}

func newProbeEnv(t *testing.T) *probeEnv {
	t.Helper()
	db := testdb.Open(t)
	secret := strings.Repeat("s", 32)
	app := NewApp(&config.Config{AppSecret: secret, DBDriver: "sqlite", JWTLifetime: 12}, db)
	server := httptest.NewServer(app.Router)
	t.Cleanup(server.Close)

	admin := database.Admin{UUID: uuid.NewString(), Username: "admin", PasswordHash: "x", Role: "ADMIN"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	claims := middleware.TokenClaims{UUID: admin.UUID, Username: admin.Username, Role: "ADMIN", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return &probeEnv{t: t, db: db, server: server, token: token}
}

func (e *probeEnv) do(method, path string, body interface{}) (int, []byte) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, e.server.URL+path, reader)
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data
}

func (e *probeEnv) mustCreate(path string, body interface{}, field string) string {
	e.t.Helper()
	status, data := e.do("POST", path, body)
	if status >= 300 {
		e.t.Fatalf("POST %s: %d %s", path, status, data)
	}
	var envelope struct {
		Response map[string]interface{} `json:"response"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		e.t.Fatalf("POST %s: %v %s", path, err, data)
	}
	value, _ := envelope.Response[field].(string)
	if value == "" {
		e.t.Fatalf("POST %s: response has no %s: %s", path, field, data)
	}
	return value
}

func mustJSON(value interface{}) []byte {
	data, _ := json.Marshal(value)
	return data
}

const probeXrayConfig = `{"log":{"loglevel":"warning"},"inbounds":[{"tag":"VLESS_TCP","port":443,"protocol":"vless","settings":{"clients":[],"decryption":"none"},"streamSettings":{"network":"tcp","security":"reality","realitySettings":{"show":false,"target":"example.com:443","serverNames":["example.com"],"privateKey":"ADcjKOQ2Vs2PMg4TfnqaaKw5H0RkVSO-a-6QJXg3FUc","shortIds":[""]}}}],"outbounds":[{"tag":"DIRECT","protocol":"freedom"}]}`

type probeIDs struct {
	User, ShortUUID, Username                                 string
	Profile, Squad, External, Host, Node, Plugin, Integration string
	Provider, Template, Subpage, SharedList                   string
}

// sweepGets calls every parameterised GET command of the contract, resolving
// path parameters from ids; it reports 5xx and unexpected statuses.
func (e *probeEnv) sweepGets(ids probeIDs, expectedStatus map[string]int) ([]probeResult, []string) {
	replace := strings.NewReplacer(":userId", ids.User, ":shortUuid", ids.ShortUUID, ":username", ids.Username,
		":squadUuid", ids.Squad, ":nodeUuid", ids.Node)
	// :uuid is resource-specific; resolve it per path prefix.
	uuidFor := func(path string) string {
		switch {
		case strings.HasPrefix(path, "/api/config-profiles"):
			return ids.Profile
		case strings.HasPrefix(path, "/api/internal-squads"), strings.HasPrefix(path, "/api/bandwidth-stats/internal-squads"):
			return ids.Squad
		case strings.HasPrefix(path, "/api/external-squads"):
			return ids.External
		case strings.HasPrefix(path, "/api/hosts"):
			return ids.Host
		case strings.HasPrefix(path, "/api/nodes"), strings.HasPrefix(path, "/api/bandwidth-stats/nodes"), strings.HasPrefix(path, "/api/metadata/node"):
			return ids.Node
		case strings.HasPrefix(path, "/api/node-plugins"):
			return ids.Plugin
		case strings.HasPrefix(path, "/api/node-integrations"):
			return ids.Integration
		case strings.HasPrefix(path, "/api/infra-billing/providers"):
			return ids.Provider
		case strings.HasPrefix(path, "/api/subscription-templates"):
			return ids.Template
		case strings.HasPrefix(path, "/api/subscription-page-configs"):
			return ids.Subpage
		}
		return ""
	}
	query := map[string]string{
		"/api/bandwidth-stats/nodes":                                          "?start=2026-01-01&end=2026-12-31",
		"/api/bandwidth-stats/nodes/:uuid/users":                              "?start=2026-01-01&end=2026-12-31&topUsersLimit=10",
		"/api/bandwidth-stats/users/:userId":                                  "?start=2026-01-01&end=2026-12-31&topNodesLimit=10",
		"/api/bandwidth-stats/internal-squads/:uuid/usage":                    "?start=2026-01-01&end=2026-12-31",
		"/api/bandwidth-stats/internal-squads/:squadUuid/users/:userId/usage": "?start=2026-01-01&end=2026-12-31",
		"/api/node-plugins/shared-lists/by-name":                              "?name=" + ids.SharedList,
		"/api/users/stream":                                                   "?size=10",
		"/api/system/stats/digest":                                            "?start=2026-01-01T00:00:00Z&end=2026-12-31T00:00:00Z",
	}
	var results []probeResult
	var failures []string
	for _, command := range contracts.Commands {
		if command.Method != "GET" || strings.Contains(command.Path, ":jobId") {
			continue
		}
		path := strings.ReplaceAll(command.Path, ":uuid", uuidFor(command.Path))
		path = replace.Replace(path) + query[command.Path]
		status, data := e.do("GET", path, nil)
		want := expectedStatus[command.Name]
		if status >= 500 || (want != 0 && status != want) {
			failures = append(failures, fmt.Sprintf("%s %s: status %d (expected %d): %s", command.Name, path, status, want, data))
		}
		body := json.RawMessage(data)
		if !json.Valid(data) {
			body = mustJSON(string(data))
		}
		results = append(results, probeResult{command.Name, command.Method, path, status, want, body})
	}
	return results, failures
}

// TestContractResponseProbe seeds a small installation through the public API,
// calls each parameterised GET endpoint, and, when GOWAVE_PROBE_OUT is set,
// writes the responses for scripts/validate-contract.cjs to check against the
// pinned zod response schemas.
func TestContractResponseProbe(t *testing.T) {
	e := newProbeEnv(t)
	if err := e.db.Model(&database.RemnawaveSetting{}).Where("id = 1").
		Update("passkey_settings", `{"enabled":true,"rpId":"panel.example.com","origin":"https://panel.example.com"}`).Error; err != nil {
		t.Fatal(err)
	}
	var profileConfig interface{}
	_ = json.Unmarshal([]byte(probeXrayConfig), &profileConfig)
	profile := e.mustCreate("/api/config-profiles", map[string]interface{}{"name": "Probe", "config": profileConfig}, "uuid")

	status, data := e.do("GET", "/api/config-profiles/"+profile+"/inbounds", nil)
	var inbounds struct {
		Response struct {
			Inbounds []struct {
				UUID string `json:"uuid"`
			} `json:"inbounds"`
		} `json:"response"`
	}
	if status != 200 || json.Unmarshal(data, &inbounds) != nil || len(inbounds.Response.Inbounds) == 0 {
		t.Fatalf("profile inbounds: %d %s", status, data)
	}
	inbound := inbounds.Response.Inbounds[0].UUID

	squad := e.mustCreate("/api/internal-squads", map[string]interface{}{"name": "Probe", "inbounds": []string{inbound}}, "uuid")
	node := e.mustCreate("/api/nodes", map[string]interface{}{"name": "probe-node", "address": "203.0.113.10", "port": 2222, "configProfile": map[string]interface{}{"activeConfigProfileUuid": profile, "activeInbounds": []string{inbound}}}, "uuid")
	host := e.mustCreate("/api/hosts", map[string]interface{}{"inbound": map[string]interface{}{"configProfileUuid": profile, "configProfileInboundUuid": inbound}, "remark": "probe", "address": "probe.example.com", "port": 443}, "uuid")
	expire := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	status, data = e.do("POST", "/api/users", map[string]interface{}{"username": "probe_user", "expireAt": expire, "activeInternalSquads": []string{squad}})
	if status >= 300 {
		t.Fatalf("create user: %d %s", status, data)
	}
	var user struct {
		Response struct {
			ID        int64  `json:"id"`
			UUID      string `json:"uuid"`
			ShortUUID string `json:"shortUuid"`
			Username  string `json:"username"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &user); err != nil {
		t.Fatal(err)
	}
	userRef := user.Response.UUID
	if userRef == "" {
		userRef = string(mustJSON(user.Response.ID))
	}
	external := e.mustCreate("/api/external-squads", map[string]interface{}{"name": "probe-ext"}, "uuid")
	plugin := e.mustCreate("/api/node-plugins", map[string]interface{}{"name": "probe-plugin"}, "uuid")
	integration := e.mustCreate("/api/node-integrations", map[string]interface{}{"name": "probe-int", "config": map[string]interface{}{}}, "uuid")
	provider := e.mustCreate("/api/infra-billing/providers", map[string]interface{}{"name": "probe-provider"}, "uuid")
	template := e.mustCreate("/api/subscription-templates", map[string]interface{}{"name": "probe-template", "templateType": "XRAY_JSON"}, "uuid")
	const defaultSubpage = "00000000-0000-0000-0000-000000000000"

	results, serverErrors := e.sweepGets(probeIDs{
		User: userRef, ShortUUID: user.Response.ShortUUID, Username: user.Response.Username,
		Profile: profile, Squad: squad, External: external, Host: host, Node: node, Plugin: plugin,
		Integration: integration, Provider: provider, Template: template, Subpage: defaultSubpage, SharedList: "missing",
	}, map[string]int{
		// Upstream answers 403 when passkeys are enabled but none is registered,
		// and 404 for an unknown shared list.
		"GetPasskeyAuthenticationOptionsCommand": 403,
		"GetSharedListCommand":                   404,
	})
	if out := os.Getenv("GOWAVE_PROBE_OUT"); out != "" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(serverErrors) > 0 {
		t.Fatalf("server errors:\n%s", strings.Join(serverErrors, "\n"))
	}
}
