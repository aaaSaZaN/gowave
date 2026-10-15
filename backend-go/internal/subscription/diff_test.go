package subscription

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"remnawave-go/internal/database"
	"remnawave-go/internal/subtpl"
	"remnawave-go/internal/testdb"
)

// testdata/http.json.gz comes from scripts/subscription-diff/run-upstream-http.mjs:
// upstream 3.4.5's middleware, controller and service answering each
// scenario on Express 5.
type httpFixture struct {
	Now             string            `json:"now"`
	SubPublicDomain string            `json:"subPublicDomain"`
	Hosts           []json.RawMessage `json:"hosts"`
	Results         []struct {
		Name      string `json:"name"`
		BodyCheck string `json:"bodyCheck"`
		State     struct {
			User      json.RawMessage `json:"user"`
			Settings  map[string]json.RawMessage
			Templates []struct {
				Name         string          `json:"name"`
				TemplateType string          `json:"templateType"`
				Content      json.RawMessage `json:"content"`
			} `json:"templates"`
			Squad          map[string]json.RawMessage `json:"squad"`
			SquadTemplates map[string]string          `json:"squadTemplates"`
			Devices        []string                   `json:"devices"`
		} `json:"state"`
		Request struct {
			ShortUUID string            `json:"shortUuid"`
			Path      string            `json:"path"`
			Headers   map[string]string `json:"headers"`
		} `json:"request"`
		Response struct {
			Status  int               `json:"status"`
			Headers map[string]string `json:"headers"`
			Body    string            `json:"body"`
		} `json:"response"`
	} `json:"results"`
}

func readHTTPFixture(t *testing.T) *httpFixture {
	t.Helper()
	file, err := os.Open("testdata/http.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var fixture httpFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return &fixture
}

// column stores a JSON value the way the Go schema keeps JSON columns.
func column(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "null"
	}
	return string(raw)
}

func optionalColumn(raw json.RawMessage) string {
	if value := column(raw); value != "null" {
		return value
	}
	return ""
}

func mustCreate(t *testing.T, db *gorm.DB, values ...interface{}) {
	t.Helper()
	for _, value := range values {
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("%T: %v", value, err)
		}
	}
}

func seedHTTPScenario(t *testing.T, db *gorm.DB, fixture *httpFixture, index int) {
	t.Helper()
	state := fixture.Results[index].State
	settings := state.Settings
	var flags struct {
		ServeJSON bool `json:"serveJsonAtBaseSubscription"`
		Remarks   bool `json:"isShowCustomRemarks"`
		Randomize bool `json:"randomizeHosts"`
	}
	for key, target := range map[string]*bool{"serveJsonAtBaseSubscription": &flags.ServeJSON, "isShowCustomRemarks": &flags.Remarks, "randomizeHosts": &flags.Randomize} {
		_ = json.Unmarshal(settings[key], target)
	}
	setSettings(t, db, map[string]interface{}{
		"serve_json_at_base_subscription": flags.ServeJSON, "is_show_custom_remarks": flags.Remarks, "randomize_hosts": flags.Randomize,
		"custom_remarks": column(settings["customRemarks"]), "custom_response_headers": column(settings["customResponseHeaders"]),
		"hwid_settings": column(settings["hwidSettings"]), "response_rules": column(settings["responseRules"]),
	})

	if err := db.Where("1 = 1").Delete(&database.SubscriptionTemplate{}).Error; err != nil {
		t.Fatal(err)
	}
	templateUUIDs := map[string]string{}
	for i, template := range state.Templates {
		row := &database.SubscriptionTemplate{UUID: fmt.Sprintf("77777777-0000-4000-8000-%012d", i), Name: template.Name, TemplateType: template.TemplateType}
		var text string
		if json.Unmarshal(template.Content, &text) == nil {
			row.TemplateYaml = text
		} else {
			row.TemplateJson = string(template.Content)
		}
		mustCreate(t, db, row)
		templateUUIDs[template.TemplateType+"/"+template.Name] = row.UUID
	}

	var user struct {
		ID                                     uint64
		ShortUUID                              string `json:"shortUuid"`
		Username, Status, TrafficLimitStrategy string
		TrafficLimitBytes, UsedTrafficBytes    uint64
		LifetimeUsedTrafficBytes               uint64
		ExpireAt, CreatedAt                    time.Time
		LastTrafficResetAt                     *time.Time
		Email, Tag, Description                *string
		TelegramID                             *int64 `json:"telegramId"`
		HwidDeviceLimit                        *int
		VlessUUID                              string `json:"vlessUuid"`
		TrojanPassword, SsPassword             string
		ExternalSquadUUID                      *string `json:"externalSquadUuid"`
	}
	if err := json.Unmarshal(state.User, &user); err != nil {
		t.Fatal(err)
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	if state.Squad != nil {
		var uuid string
		var remove []string
		_ = json.Unmarshal(state.Squad["uuid"], &uuid)
		_ = json.Unmarshal(state.Squad["responseHeadersRemove"], &remove)
		mustCreate(t, db, &database.ExternalSquad{UUID: uuid, Name: "squad", SubscriptionSettings: column(state.Squad["subscriptionSettings"]),
			HostOverrides: column(state.Squad["hostOverrides"]), ResponseHeadersAdd: column(state.Squad["responseHeadersAdd"]),
			ResponseHeadersRemove: remove, HwidSettings: column(state.Squad["hwidSettings"]), CustomRemarks: column(state.Squad["customRemarks"])})
		for templateType, name := range state.SquadTemplates {
			mustCreate(t, db, &database.ExternalSquadTemplate{ExternalSquadUUID: uuid, TemplateType: templateType, TemplateUUID: templateUUIDs[templateType+"/"+name]})
		}
	}
	mustCreate(t, db,
		&database.User{ID: user.ID, ShortUUID: user.ShortUUID, Username: user.Username, Status: user.Status,
			TrafficLimitBytes: user.TrafficLimitBytes, TrafficLimitStrategy: user.TrafficLimitStrategy, ExpireAt: user.ExpireAt,
			LastTrafficResetAt: user.LastTrafficResetAt, TrojanPassword: user.TrojanPassword, VlessUUID: user.VlessUUID, SsPassword: user.SsPassword,
			Description: deref(user.Description), Tag: deref(user.Tag), TelegramID: user.TelegramID, Email: deref(user.Email),
			HWIDDeviceLimit: user.HwidDeviceLimit, ExternalSquadUUID: user.ExternalSquadUUID, CreatedAt: user.CreatedAt, UpdatedAt: user.CreatedAt},
		&database.UserTraffic{ID: user.ID, UsedTrafficBytes: user.UsedTrafficBytes, LifetimeUsedTrafficBytes: user.LifetimeUsedTrafficBytes},
	)
	for _, hwid := range state.Devices {
		mustCreate(t, db, &database.HwidDevice{HWID: hwid, UserID: user.ID, CreatedAt: user.CreatedAt, UpdatedAt: user.CreatedAt})
	}

	profile, squad := "11111111-1111-4111-8111-111111111111", "88888888-8888-4888-8888-888888888888"
	mustCreate(t, db, &database.ConfigProfile{UUID: profile, Name: "profile", Config: `{}`}, &database.InternalSquad{UUID: squad, Name: "internal"},
		&database.InternalSquadMember{InternalSquadUUID: squad, UserID: user.ID})
	for i, raw := range fixture.Hosts {
		var host struct {
			UUID, Remark, Address, SecurityLayer, InternalSquadsMode string
			ViewPosition, Port                                       int
			Path, Sni, Host, Alpn, Fingerprint, ServerDescription    *string
			PinnedPeerCertSha256, VerifyPeerCertByName               *string
			MihomoIPVersion                                          *string `json:"mihomoIpVersion"`
			IsDisabled, IsHidden, OverrideSniFromAddress             bool
			KeepSniBlank, ShuffleHost, MihomoX25519                  bool
			VlessRouteID                                             *int `json:"vlessRouteId"`
			Tags, ExcludeFromSubscriptionTypes                       []string
			XhttpExtraParams, MuxParams, SockoptParams, FinalMask    json.RawMessage
			Mapper, RawInbound, XrayJSONTemplate                     json.RawMessage
			InboundTag                                               string
		}
		if err := json.Unmarshal(raw, &host); err != nil {
			t.Fatal(err)
		}
		var templateFields struct {
			XrayJSONTemplate json.RawMessage `json:"xrayJsonTemplate"`
		}
		_ = json.Unmarshal(raw, &templateFields)
		inbound := fmt.Sprintf("99999999-0000-4000-8000-%012d", i)
		mustCreate(t, db, &database.ConfigProfileInbound{UUID: inbound, ProfileUUID: profile, Tag: host.InboundTag, Type: "vless", RawInbound: string(host.RawInbound)},
			&database.InternalSquadInbound{InternalSquadUUID: squad, InboundUUID: inbound})
		row := &database.Host{UUID: host.UUID, ViewPosition: host.ViewPosition, Remark: host.Remark, Address: host.Address, Port: host.Port,
			Path: deref(host.Path), Sni: deref(host.Sni), Host: deref(host.Host), Alpn: deref(host.Alpn), Fingerprint: deref(host.Fingerprint),
			SecurityLayer: host.SecurityLayer, XhttpExtraParams: optionalColumn(host.XhttpExtraParams), MuxParams: optionalColumn(host.MuxParams),
			SockoptParams: optionalColumn(host.SockoptParams), FinalMask: optionalColumn(host.FinalMask), IsDisabled: host.IsDisabled,
			ServerDescription: deref(host.ServerDescription), VlessRouteId: host.VlessRouteID, PinnedPeerCertSha256: deref(host.PinnedPeerCertSha256),
			VerifyPeerCertByName: deref(host.VerifyPeerCertByName), ShuffleHost: host.ShuffleHost, MihomoX25519: host.MihomoX25519,
			MihomoIpVersion: deref(host.MihomoIPVersion), KeepSniBlank: host.KeepSniBlank, ExcludeFromSubscriptionTypes: host.ExcludeFromSubscriptionTypes,
			Mapper: column(host.Mapper), InternalSquadsMode: "EXCLUDE", Tags: host.Tags, IsHidden: host.IsHidden,
			OverrideSniFromAddress: host.OverrideSniFromAddress, ConfigProfileUUID: &profile, ConfigProfileInboundUUID: &inbound}
		if column(templateFields.XrayJSONTemplate) != "null" {
			uuid := fmt.Sprintf("77777777-1111-4000-8000-%012d", i)
			mustCreate(t, db, &database.SubscriptionTemplate{UUID: uuid, Name: "host-" + host.UUID, TemplateType: "XRAY_JSON", TemplateJson: string(templateFields.XrayJSONTemplate)})
			row.XrayJsonTemplateUUID = &uuid
		}
		mustCreate(t, db, row)
	}
}

// Headers whose value depends on a random body.
var randomBodyHeaders = map[string]bool{"content-length": true, "etag": true}

func TestHTTPMatchesUpstream(t *testing.T) {
	fixture := readHTTPFixture(t)
	now, err := time.Parse(time.RFC3339Nano, fixture.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func(n func() time.Time, r func(int) int) { subtpl.Now, subtpl.Random = n, r }(subtpl.Now, subtpl.Random)
	subtpl.Now = func() time.Time { return now }
	subtpl.Random = func(int) int { return 0 }

	for index, result := range fixture.Results {
		t.Run(result.Name, func(t *testing.T) {
			db := testdb.Open(t)
			seedHTTPScenario(t, db, fixture, index)
			router := subscriptionRouter(db, fixture.SubPublicDomain)
			request := httptest.NewRequest("GET", "/api/sub/"+result.Request.ShortUUID+result.Request.Path, nil)
			request.Header = map[string][]string{}
			for name, value := range result.Request.Headers {
				if name == "host" {
					request.Host = value
					continue
				}
				request.Header.Set(name, value)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			want := result.Response
			if recorder.Code != want.Status {
				t.Errorf("status %d, want %d", recorder.Code, want.Status)
			}
			got := map[string]string{}
			for name, values := range recorder.Header() {
				got[strings.ToLower(name)] = strings.Join(values, ", ")
			}
			for name, value := range want.Headers {
				if result.BodyCheck == "armor" && randomBodyHeaders[name] {
					continue
				}
				if got[name] != value {
					t.Errorf("header %s = %q, want %q", name, got[name], value)
				}
				delete(got, name)
			}
			for name, value := range got {
				if !(result.BodyCheck == "armor" && randomBodyHeaders[name]) {
					t.Errorf("extra header %s = %q", name, value)
				}
			}
			body := recorder.Body.String()
			if result.BodyCheck == "armor" {
				if !strings.HasPrefix(body, "-----BEGIN AGE ENCRYPTED FILE-----\n") || !strings.HasSuffix(body, "-----END AGE ENCRYPTED FILE-----\n") {
					t.Errorf("not an armored age file: %.80q", body)
				}
				return
			}
			if body != want.Body {
				compareBodies(t, body, want.Body)
			}
		})
	}
}

func compareBodies(t *testing.T, got, want string) {
	t.Helper()
	limit := len(got)
	if len(want) < limit {
		limit = len(want)
	}
	at := limit
	for i := 0; i < limit; i++ {
		if got[i] != want[i] {
			at = i
			break
		}
	}
	from := at - 120
	if from < 0 {
		from = 0
	}
	clip := func(s string) string {
		end := at + 200
		if end > len(s) {
			end = len(s)
		}
		if from > len(s) {
			return ""
		}
		return s[from:end]
	}
	t.Errorf("body differs at byte %d (len %d, want %d)\n got: %q\nwant: %q", at, len(got), len(want), clip(got), clip(want))
}
