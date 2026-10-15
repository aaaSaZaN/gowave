package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"remnawave-go/internal/database"
	"remnawave-go/internal/testdb"
	"remnawave-go/internal/users"
)

func subscriptionDB(t *testing.T) (*gorm.DB, *database.User) {
	t.Helper()
	db := testdb.Open(t)
	limit := 1
	u, err := users.NewService(db).Create(users.CreateUserDTO{Username: "subscriber", ExpireAt: time.Now().UTC().Add(48 * time.Hour), TrafficLimitBytes: 1 << 30, HWIDDeviceLimit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	p, i := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	squad := "33333333-3333-4333-8333-333333333333"
	models := []interface{}{
		&database.ConfigProfile{UUID: p, Name: "profile", Config: `{}`},
		&database.ConfigProfileInbound{UUID: i, ProfileUUID: p, Tag: "tag", Type: "vless", RawInbound: `{"tag":"tag","protocol":"vless","port":443,"settings":{"clients":[],"decryption":"none"},"streamSettings":{"network":"ws","security":"tls","wsSettings":{"path":"/tunnel"}}}`},
		&database.InternalSquad{UUID: squad, Name: "squad"},
		&database.InternalSquadInbound{InternalSquadUUID: squad, InboundUUID: i},
		&database.InternalSquadMember{InternalSquadUUID: squad, UserID: u.ID},
		&database.Host{UUID: "44444444-4444-4444-8444-444444444444", Remark: "visible", Address: "example.com", Port: 443, ConfigProfileUUID: &p, ConfigProfileInboundUUID: &i},
	}
	for _, m := range models {
		if err := db.Create(m).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, u
}

func subscriptionRouter(db *gorm.DB, domain ...string) http.Handler {
	publicDomain := "sub.example"
	if len(domain) > 0 {
		publicDomain = domain[0]
	}
	h := NewHandler(db, publicDomain)
	router := chi.NewRouter()
	router.Get("/api/sub/{shortUuid}", h.GetSubscription)
	router.Get("/api/sub/{shortUuid}/info", h.GetSubscriptionInfo)
	router.Get("/api/sub/{shortUuid}/{clientType}", h.GetSubscription)
	return router
}

func get(router http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func setSettings(t *testing.T, db *gorm.DB, values map[string]interface{}) {
	t.Helper()
	if err := db.Model(&database.SubscriptionSetting{}).Where("1=1").Updates(values).Error; err != nil {
		t.Fatal(err)
	}
}

var curl = map[string]string{"User-Agent": "curl/8.0"}

func TestResponseRulesFollowUpstream(t *testing.T) {
	db, user := subscriptionDB(t)
	router := subscriptionRouter(db)
	base := "/api/sub/" + user.ShortUUID

	if w := get(router, base, nil); w.Code != 403 || w.Body.String() != `{"statusCode":403,"message":"Forbidden"}` {
		t.Fatalf("missing user agent: %d %s", w.Code, w.Body.String())
	}
	// A client type in the path skips the rules.
	setSettings(t, db, map[string]interface{}{"response_rules": `{"version":"1","rules":[{"name":"b","enabled":true,"operator":"AND","conditions":[],"responseType":"BLOCK"}]}`})
	if w := get(router, base, curl); w.Code != 403 {
		t.Fatalf("block rule: %d", w.Code)
	}
	if w := get(router, base+"/json", curl); w.Code != 200 || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("path override: %d %s", w.Code, w.Body.String())
	}
	if w := get(router, base+"/JSON", curl); w.Code != 403 {
		t.Fatalf("client types are case sensitive: %d", w.Code)
	}
	setSettings(t, db, map[string]interface{}{"response_rules": `{"version":"1","settings":{"disableSubscriptionAccessByPath":true},"rules":[{"name":"x","enabled":true,"operator":"AND","conditions":[],"responseType":"XRAY_BASE64"}]}`})
	if w := get(router, base+"/json", curl); w.Code != 403 {
		t.Fatalf("path access disabled: %d", w.Code)
	}
	setSettings(t, db, map[string]interface{}{"response_rules": `{"version":"1","rules":[{"name":"n","enabled":true,"operator":"AND","conditions":[{"headerName":"user-agent","operator":"CONTAINS","value":"gone","caseSensitive":true}],"responseType":"STATUS_CODE_451"}]}`})
	if w := get(router, base, map[string]string{"User-Agent": "gone"}); w.Code != 451 || w.Body.String() != `{"statusCode":451,"message":"Unavailable For Legal Reasons"}` {
		t.Fatalf("451: %d %s", w.Code, w.Body.String())
	}
	if w := get(router, base, curl); w.Code != 403 {
		t.Fatalf("unmatched: %d", w.Code)
	}
	if w := get(router, "/api/sub/missing", map[string]string{"User-Agent": "gone"}); w.Code != 451 {
		t.Fatalf("rules run before the user lookup: %d", w.Code)
	}
}

func TestBaseSubscriptionHeadersAndBody(t *testing.T) {
	db, user := subscriptionDB(t)
	router := subscriptionRouter(db)
	base := "/api/sub/" + user.ShortUUID
	setSettings(t, db, map[string]interface{}{"custom_response_headers": `{"profile-title":"rwEncodeBase64:{{USERNAME}}","x-days":"{{DAYS_LEFT}}"}`})

	w := get(router, base, curl)
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Content-Type"))
	}
	links, _ := base64.StdEncoding.DecodeString(w.Body.String())
	if !strings.HasPrefix(string(links), "vless://"+user.VlessUUID+"@example.com:443?") {
		t.Fatalf("links %q", links)
	}
	if got := w.Header().Get("Content-Disposition"); got != "attachment; filename=subscriber" {
		t.Fatal(got)
	}
	want := fmt.Sprintf("upload=0; download=0; total=%d; expire=%d", 1<<30, user.ExpireAt.Unix())
	if got := w.Header().Get("Subscription-Userinfo"); got != want {
		t.Fatalf("%q != %q", got, want)
	}
	if got := w.Header().Get("Profile-Title"); got != "base64:"+base64.StdEncoding.EncodeToString([]byte("subscriber")) {
		t.Fatal(got)
	}
	if got := w.Header().Get("X-Days"); got != "1" {
		t.Fatal(got)
	}
	etag := w.Header().Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) {
		t.Fatal(etag)
	}
	if w := get(router, base, map[string]string{"User-Agent": "curl/8.0", "If-None-Match": etag}); w.Code != 304 || w.Body.Len() != 0 {
		t.Fatalf("conditional GET: %d", w.Code)
	}
	var history []database.UserSubscriptionRequestHistory
	db.Where("user_id = ?", user.ID).Find(&history)
	if len(history) != 2 || history[0].SrrResponseType != "XRAY_BASE64" || *history[0].SrrRuleName != "Fallback Base64" {
		t.Fatalf("history %+v", history)
	}

	// serveJsonAtBaseSubscription only applies to JSON-capable clients.
	setSettings(t, db, map[string]interface{}{"serve_json_at_base_subscription": true})
	if w := get(router, base, curl); w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatal("curl got JSON")
	}
	if w := get(router, base, map[string]string{"User-Agent": "Happ/2.0"}); w.Header().Get("Content-Type") != "application/json; charset=utf-8" || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("Happ: %s", w.Body.String())
	}
}

func TestHiddenHostsReachXrayJSONInjector(t *testing.T) {
	db, user := subscriptionDB(t)
	router := subscriptionRouter(db)
	p, i := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	if err := db.Create(&database.Host{UUID: "55555555-5555-4555-8555-555555555555", ViewPosition: 2, Remark: "hidden", Address: "hidden.example", Port: 443, IsHidden: true, ConfigProfileUUID: &p, ConfigProfileInboundUUID: &i}).Error; err != nil {
		t.Fatal(err)
	}
	db.Model(&database.SubscriptionTemplate{}).Where("template_type='XRAY_JSON'").Update("template_json", `{"remnawave":{"injectHosts":[{"selector":{"type":"remarkRegex","pattern":"hidden"},"selectFrom":"HIDDEN"}]},"outbounds":[]}`)
	if w := get(router, "/api/sub/"+user.ShortUUID+"/json", curl); w.Code != 200 || !strings.Contains(w.Body.String(), "hidden.example") {
		t.Fatalf("hidden injection %d %s", w.Code, w.Body.String())
	}
	w := get(router, "/api/sub/"+user.ShortUUID, curl)
	links, _ := base64.StdEncoding.DecodeString(w.Body.String())
	if strings.Contains(string(links), "hidden.example") {
		t.Fatal("hidden host in base64")
	}
}

func TestHwidLimitIsAtomic(t *testing.T) {
	db, user := subscriptionDB(t)
	router := subscriptionRouter(db)
	setSettings(t, db, map[string]interface{}{"hwid_settings": `{"enabled":true,"fallbackDeviceLimit":1,"maxDevicesAnnounce":"Too many"}`})
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- get(router, "/api/sub/"+user.ShortUUID, map[string]string{"User-Agent": "curl/8.0", "X-HWID": fmt.Sprintf("device-number-%02d", i)})
		}(i)
	}
	wg.Wait()
	close(results)
	admitted := 0
	for w := range results {
		if w.Code != 200 {
			t.Fatalf("status %d", w.Code)
		}
		if w.Header().Get("X-Hwid-Limit") == "true" {
			if w.Header().Get("X-Hwid-Max-Devices-Reached") != "true" || w.Header().Get("Announce") != "base64:"+base64.StdEncoding.EncodeToString([]byte("Too many")) {
				t.Fatalf("limit headers %v", w.Header())
			}
			links, _ := base64.StdEncoding.DecodeString(w.Body.String())
			if strings.Contains(string(links), user.VlessUUID) {
				t.Fatal("credentials leaked")
			}
			continue
		}
		if w.Header().Get("X-Hwid-Active") != "true" {
			t.Fatal("admitted without x-hwid-active")
		}
		admitted++
	}
	if admitted != 1 {
		t.Fatalf("admitted %d devices", admitted)
	}
	var count int64
	db.Model(&database.HwidDevice{}).Where("user_id=?", user.ID).Count(&count)
	if count != 1 {
		t.Fatalf("registered %d", count)
	}
	w := get(router, "/api/sub/"+user.ShortUUID, curl)
	if w.Header().Get("X-Hwid-Not-Supported") != "true" || w.Header().Get("X-Hwid-Limit") != "true" {
		t.Fatalf("missing HWID: %v", w.Header())
	}
}

func TestEncryptedResponse(t *testing.T) {
	db, user := subscriptionDB(t)
	router := subscriptionRouter(db)
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	rules := fmt.Sprintf(`{"version":"1","rules":[{"name":"e","enabled":true,"operator":"AND","conditions":[],"responseType":"XRAY_BASE64","responseModifications":{"encryption":{"method":"age1","key":" %s "},"headers":[{"key":"x-early","value":"1"}]}}]}`, identity.Recipient())
	setSettings(t, db, map[string]interface{}{"response_rules": rules})
	w := get(router, "/api/sub/"+user.ShortUUID, curl)
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" || w.Header().Get("X-Early") != "1" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	reader, err := age.Decrypt(armor.NewReader(strings.NewReader(w.Body.String())), identity)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(reader)
	links, _ := base64.StdEncoding.DecodeString(string(plain))
	if !strings.Contains(string(links), user.VlessUUID) {
		t.Fatalf("decrypted %q", plain)
	}
}

func TestBrowserAndInfo(t *testing.T) {
	db, user := subscriptionDB(t)
	router := subscriptionRouter(db)
	w := get(router, "/api/sub/"+user.ShortUUID, map[string]string{"User-Agent": "Mozilla/5.0", "Accept": "text/html"})
	var body map[string]interface{}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || body["isFound"] != true || body["subscriptionUrl"] != "https://sub.example/"+user.ShortUUID {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if w := get(router, "/api/sub/missing", map[string]string{"User-Agent": "Mozilla/5.0", "Accept": "text/html"}); w.Code != 404 || w.Body.String() != `{"isFound":false,"statusCode":404,"message":"Resource not found"}` {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = get(router, "/api/sub/"+user.ShortUUID+"/info", nil)
	var info struct {
		Response struct {
			Links []string
			User  struct{ DaysLeft int }
		}
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &info) != nil || len(info.Response.Links) != 1 || info.Response.User.DaysLeft != 1 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
