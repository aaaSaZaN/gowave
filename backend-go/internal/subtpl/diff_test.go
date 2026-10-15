package subtpl

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"remnawave-go/internal/subtpl/js"
	"remnawave-go/internal/subtpl/ojson"
)

// The fixtures come from scripts/subscription-diff: matrix.json lists hosts
// and cases, golden.json holds upstream 3.4.5's output for each case.
type matrixFile struct {
	Now             string            `json:"now"`
	SubPublicDomain string            `json:"subPublicDomain"`
	User            json.RawMessage   `json:"user"`
	Settings        json.RawMessage   `json:"settings"`
	Hosts           []json.RawMessage `json:"hosts"`
	Templates       map[string]json.RawMessage
	Cases           []struct {
		Name                       string                     `json:"name"`
		Types                      []string                   `json:"types"`
		Extended                   []bool                     `json:"extended"`
		IgnoreHostXrayJSONTemplate []bool                     `json:"ignoreHostXrayJsonTemplate"`
		UserPatch                  map[string]json.RawMessage `json:"userPatch"`
		SettingsPatch              map[string]json.RawMessage `json:"settingsPatch"`
		HostIndexes                *[]int                     `json:"hostIndexes"`
		HostPatch                  map[string]json.RawMessage `json:"hostPatch"`
		Fallback                   *struct {
			ShowHwidMaxDeviceRemarks    bool     `json:"showHwidMaxDeviceRemarks"`
			ShowHwidNotSupportedRemarks bool     `json:"showHwidNotSupportedRemarks"`
			RespondWithRemarks          []string `json:"respondWithRemarks"`
		} `json:"fallback"`
		HostOverrides      map[string]json.RawMessage `json:"hostOverrides"`
		ExcludeHostsByTags []string                   `json:"excludeHostsByTags"`
		Templates          map[string]json.RawMessage `json:"templates"`
	} `json:"cases"`
}

func patched(base json.RawMessage, patch map[string]json.RawMessage) json.RawMessage {
	if len(patch) == 0 {
		return base
	}
	var m map[string]json.RawMessage
	json.Unmarshal(base, &m)
	for k, v := range patch {
		m[k] = v
	}
	out, _ := json.Marshal(m)
	return out
}

func parseUser(raw json.RawMessage) *User {
	var u struct {
		ID                       int64
		ShortUUID                string `json:"shortUuid"`
		Username, Status         string
		TrafficLimitBytes        int64
		TrafficLimitStrategy     string
		UsedTrafficBytes         int64
		LifetimeUsedTrafficBytes int64
		ExpireAt, CreatedAt      time.Time
		LastTrafficResetAt       *time.Time
		Email                    *string
		TelegramID               *int64 `json:"telegramId"`
		Tag, Description         *string
		HwidDeviceLimit          *int
		VlessUUID                string `json:"vlessUuid"`
		TrojanPassword           string
		SsPassword               string
	}
	if err := json.Unmarshal(raw, &u); err != nil {
		panic(err)
	}
	return &User{ID: u.ID, ShortUUID: u.ShortUUID, Username: u.Username, Status: u.Status, TrafficLimitBytes: u.TrafficLimitBytes,
		TrafficLimitStrategy: u.TrafficLimitStrategy, UsedTrafficBytes: u.UsedTrafficBytes, LifetimeUsedTrafficBytes: u.LifetimeUsedTrafficBytes,
		ExpireAt: u.ExpireAt, CreatedAt: u.CreatedAt, LastTrafficResetAt: u.LastTrafficResetAt, Email: u.Email, TelegramID: u.TelegramID,
		Tag: u.Tag, Description: u.Description, HwidDeviceLimit: u.HwidDeviceLimit, VlessUUID: u.VlessUUID, TrojanPassword: u.TrojanPassword, SsPassword: u.SsPassword}
}

func parseSettings(raw json.RawMessage) *Settings {
	var s struct {
		IsShowCustomRemarks bool
		CustomRemarks       CustomRemarks
		HwidSettings        struct{ FallbackDeviceLimit *int }
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		panic(err)
	}
	return &Settings{IsShowCustomRemarks: s.IsShowCustomRemarks, CustomRemarks: s.CustomRemarks, HwidFallbackDeviceLimit: s.HwidSettings.FallbackDeviceLimit}
}

// HostFromJSON builds a Host from the upstream entity JSON shape.
func HostFromJSON(raw json.RawMessage) *Host {
	value, err := ojson.Parse(raw)
	if err != nil {
		panic(err)
	}
	o := value.(*ojson.Object)
	sp := func(key string) *string { return asStringPtr(o.Value(key)) }
	b := func(key string) bool { return o.Value(key) == true }
	var route *int
	if v, ok := o.Value("vlessRouteId").(float64); ok {
		n := int(v)
		route = &n
	}
	mapper, _ := o.Value("mapper").(*ojson.Object)
	return &Host{
		UUID: asString(o.Value("uuid")), ViewPosition: int(num(o, "viewPosition")), Remark: asString(o.Value("remark")),
		Address: asString(o.Value("address")), Port: int(num(o, "port")), Path: sp("path"), SNI: sp("sni"), Host: sp("host"),
		ALPN: sp("alpn"), Fingerprint: sp("fingerprint"), SecurityLayer: asString(o.Value("securityLayer")),
		XhttpExtraParams: o.Value("xhttpExtraParams"), MuxParams: o.Value("muxParams"), SockoptParams: o.Value("sockoptParams"),
		FinalMask: o.Value("finalMask"), IsDisabled: b("isDisabled"), ServerDescription: sp("serverDescription"),
		PinnedPeerCertSha256: sp("pinnedPeerCertSha256"), VerifyPeerCertByName: sp("verifyPeerCertByName"),
		MihomoIPVersion: sp("mihomoIpVersion"), Tags: strList(o.Value("tags")), IsHidden: b("isHidden"),
		OverrideSniFromAddress: b("overrideSniFromAddress"), KeepSniBlank: b("keepSniBlank"), VlessRouteID: route,
		ShuffleHost: b("shuffleHost"), MihomoX25519: b("mihomoX25519"), ConfigProfileUUID: sp("configProfileUuid"),
		ConfigProfileInboundUUID: sp("configProfileInboundUuid"), ExcludeFromSubscriptionTypes: strList(o.Value("excludeFromSubscriptionTypes")),
		Mapper: mapper, RawInbound: o.Value("rawInbound"), InboundTag: asString(o.Value("inboundTag")), XrayJSONTemplate: o.Value("xrayJsonTemplate"),
	}
}

func templateFrom(raw json.RawMessage) (*ojson.Object, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		value, err := LoadYAML(text)
		if err != nil {
			return nil, err
		}
		object, _ := value.(*ojson.Object)
		return object, nil
	}
	value, err := ojson.Parse(raw)
	if err != nil {
		return nil, err
	}
	object, _ := value.(*ojson.Object)
	return object, nil
}

func TestRenderMatchesUpstream(t *testing.T) {
	data, err := os.ReadFile("testdata/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix matrixFile
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatal(err)
	}
	goldenFile, err := os.Open("testdata/golden.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer goldenFile.Close()
	reader, err := gzip.NewReader(goldenFile)
	if err != nil {
		t.Fatal(err)
	}
	goldenData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]struct{ Body, ContentType string }
	if err := json.Unmarshal(goldenData, &golden); err != nil {
		t.Fatal(err)
	}
	now, _ := time.Parse(time.RFC3339Nano, matrix.Now)
	defer func(n func() time.Time, r func(int) int) { Now, Random = n, r }(Now, Random)
	Now = func() time.Time { return now }
	Random = func(int) int { return 0 }

	for _, c := range matrix.Cases {
		templates := map[string]json.RawMessage{}
		for k, v := range matrix.Templates {
			templates[k] = v
		}
		for k, v := range c.Templates {
			templates[k] = v
		}
		lookup := func(kind, _ string) (*ojson.Object, error) { return templateFrom(templates[kind]) }
		extendedValues := c.Extended
		if extendedValues == nil {
			extendedValues = []bool{false}
		}
		ignoreValues := c.IgnoreHostXrayJSONTemplate
		if ignoreValues == nil {
			ignoreValues = []bool{false}
		}
		for _, kind := range c.Types {
			for _, extended := range extendedValues {
				for _, ignore := range ignoreValues {
					key := fmt.Sprintf("%s/%s/%v/%v", c.Name, kind, extended, ignore)
					t.Run(key, func(t *testing.T) {
						var hosts []*Host
						indexes := make([]int, len(matrix.Hosts))
						for i := range indexes {
							indexes[i] = i
						}
						if c.HostIndexes != nil {
							indexes = *c.HostIndexes
						}
						for _, i := range indexes {
							raw := matrix.Hosts[i]
							if patch, ok := c.HostPatch[fmt.Sprint(i)]; ok {
								var p map[string]json.RawMessage
								json.Unmarshal(patch, &p)
								raw = patched(raw, p)
							}
							hosts = append(hosts, HostFromJSON(raw))
						}
						options := ResolveOptions{
							Settings: parseSettings(patched(matrix.Settings, c.SettingsPatch)), Hosts: hosts,
							User: parseUser(patched(matrix.User, c.UserPatch)), SubPublicDomain: matrix.SubPublicDomain,
						}
						if c.Fallback != nil {
							options.Fallback = &FallbackOptions{ShowHwidMaxDeviceRemarks: c.Fallback.ShowHwidMaxDeviceRemarks, ShowHwidNotSupportedRemarks: c.Fallback.ShowHwidNotSupportedRemarks, RespondWithRemarks: c.Fallback.RespondWithRemarks}
						}
						if c.HostOverrides != nil {
							overrides := &HostOverrides{}
							if raw, ok := c.HostOverrides["vlessRouteId"]; ok {
								var v *int
								json.Unmarshal(raw, &v)
								overrides.VlessRouteID = &v
							}
							if raw, ok := c.HostOverrides["serverDescription"]; ok {
								var v *string
								json.Unmarshal(raw, &v)
								overrides.ServerDescription = &v
							}
							options.HostOverrides = overrides
						}
						if len(c.ExcludeHostsByTags) > 0 {
							options.ExcludeHostsByTags = map[string]bool{}
							for _, tag := range c.ExcludeHostsByTags {
								options.ExcludeHostsByTags[tag] = true
							}
						}
						body, contentType := Render(RenderRequest{ResponseType: kind, IsExtendedClient: extended, IgnoreHostXrayJSONTemplate: ignore, Resolve: options}, lookup)
						want := golden[key]
						if contentType != want.ContentType {
							t.Errorf("content type %q, want %q", contentType, want.ContentType)
						}
						compareBodies(t, kind, body, want.Body)
					})
				}
			}
		}
	}
}

func compareBodies(t *testing.T, kind, got, want string) {
	t.Helper()
	switch kind {
	case "XRAY_BASE64":
		gotLines := strings.Split(js.DecodeBase64String(got), "\n")
		wantLines := strings.Split(js.DecodeBase64String(want), "\n")
		for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				t.Errorf("link %d:\n got %s\nwant %s", i, g, w)
			}
		}
		if got != want && !t.Failed() {
			t.Errorf("base64 encoding differs")
		}
	case "MIHOMO", "STASH", "CLASH":
		g, err1 := LoadYAML(got)
		w, err2 := LoadYAML(want)
		if err1 != nil || err2 != nil {
			t.Fatalf("yaml parse: %v / %v", err1, err2)
		}
		reportDiff(t, ojson.StringifyIndent(g, 1), ojson.StringifyIndent(w, 1))
		if got != want && !t.Failed() {
			reportDiff(t, got, want)
		}
	default:
		reportDiff(t, prettyJSON(got), prettyJSON(want))
		if got != want && !t.Failed() {
			t.Errorf("JSON differs only in formatting")
		}
	}
}

func prettyJSON(s string) string {
	value, err := ojson.Parse([]byte(s))
	if err != nil {
		return s
	}
	return ojson.StringifyIndent(value, 1)
}

func reportDiff(t *testing.T, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	shown := 0
	for i := 0; i < len(g) || i < len(w); i++ {
		var a, b string
		if i < len(g) {
			a = g[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			t.Errorf("line %d:\n got %s\nwant %s", i+1, a, b)
			if shown++; shown >= 5 {
				return
			}
		}
	}
}
