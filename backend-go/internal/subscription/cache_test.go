package subscription

import (
	"fmt"
	"math/rand"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"gowave/internal/database"
	"gowave/internal/subtpl"
	"gowave/internal/subtpl/ojson"
	"gowave/internal/testdb"
	"gowave/internal/users"
)

// hostsForUserSQL is the query hostsForUser replaced; the index must select
// the same hosts in the same order.
func hostsForUserSQL(db *gorm.DB, userID uint64, withDisabled, withHidden bool) ([]string, error) {
	query := db.Model(&database.Host{}).Where(`EXISTS (
		SELECT 1 FROM internal_squad_inbounds AS isi
		JOIN internal_squad_members AS ism ON ism.internal_squad_uuid = isi.internal_squad_uuid
		WHERE isi.inbound_uuid = hosts.config_profile_inbound_uuid AND ism.user_id = ?
		AND (EXISTS (
			SELECT 1 FROM internal_squad_host_links AS link
			WHERE link.host_uuid = hosts.uuid AND link.squad_uuid = isi.internal_squad_uuid
		)) = (hosts.internal_squads_mode = 'ALLOW_ONLY')
	)`, userID)
	if !withDisabled {
		query = query.Where("hosts.is_disabled = ?", false)
	}
	if !withHidden {
		query = query.Where("hosts.is_hidden = ?", false)
	}
	var rows []database.Host
	if err := query.Order("hosts.view_position ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	var inbounds []string
	if err := db.Model(&database.ConfigProfileInbound{}).Pluck("uuid", &inbounds).Error; err != nil {
		return nil, err
	}
	uuids := []string{}
	for _, row := range rows {
		if row.ConfigProfileInboundUUID != nil && slices.Contains(inbounds, *row.ConfigProfileInboundUUID) {
			uuids = append(uuids, row.UUID)
		}
	}
	return uuids, nil
}

func hostUUIDs(hosts []*subtpl.Host) []string {
	uuids := []string{}
	for _, host := range hosts {
		uuids = append(uuids, host.UUID)
	}
	return uuids
}

// enableCache turns the settle window off for the test, so only write
// tracking keeps the cache fresh.
func enableCache(t *testing.T) {
	t.Helper()
	settle := CacheSettle
	CacheSettle = 0
	t.Cleanup(func() { CacheSettle = settle })
}

func TestHostIndexMatchesQuery(t *testing.T) {
	enableCache(t)
	db := testdb.Open(t)
	random := rand.New(rand.NewSource(7))
	profile := "11111111-1111-4111-8111-000000000000"
	if err := db.Create(&database.ConfigProfile{UUID: profile, Name: "profile", Config: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	var inbounds, squads []string
	for i := 0; i < 4; i++ {
		inbound := fmt.Sprintf("22222222-2222-4222-8222-%012d", i)
		inbounds = append(inbounds, inbound)
		raw := fmt.Sprintf(`{"tag":"in-%d","protocol":"vless","port":443,"settings":{"clients":[],"decryption":"none"},"streamSettings":{"network":"tcp","security":"tls"}}`, i)
		if err := db.Create(&database.ConfigProfileInbound{UUID: inbound, ProfileUUID: profile, Tag: fmt.Sprintf("in-%d", i), Type: "vless", RawInbound: raw}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		squad := fmt.Sprintf("33333333-3333-4333-8333-%012d", i)
		squads = append(squads, squad)
		if err := db.Create(&database.InternalSquad{UUID: squad, Name: fmt.Sprintf("squad-%d", i)}).Error; err != nil {
			t.Fatal(err)
		}
		// The last squad grants nothing.
		for _, inbound := range inbounds {
			if i < 4 && random.Intn(2) == 0 {
				if err := db.Create(&database.InternalSquadInbound{InternalSquadUUID: squad, InboundUUID: inbound}).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for i := 0; i < 40; i++ {
		host := database.Host{UUID: fmt.Sprintf("44444444-4444-4444-8444-%012d", i), Remark: fmt.Sprintf("host-%d", i),
			Address: fmt.Sprintf("host-%d.example.com", i), Port: 443, ViewPosition: 40 - i, ConfigProfileUUID: &profile,
			IsDisabled: random.Intn(5) == 0, IsHidden: random.Intn(5) == 0, InternalSquadsMode: []string{"EXCLUDE", "ALLOW_ONLY"}[random.Intn(2)]}
		if random.Intn(8) > 0 {
			host.ConfigProfileInboundUUID = &inbounds[random.Intn(len(inbounds))]
		}
		if err := db.Create(&host).Error; err != nil {
			t.Fatal(err)
		}
		for _, squad := range squads {
			if random.Intn(3) == 0 {
				if err := db.Create(&database.InternalSquadHostLink{HostUUID: host.UUID, SquadUUID: squad}).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	// Upstream's comparison never matches a NULL mode.
	if err := db.Exec("UPDATE hosts SET internal_squads_mode = NULL WHERE uuid = ?", "44444444-4444-4444-8444-000000000003").Error; err != nil {
		t.Fatal(err)
	}
	h := NewHandler(db, "sub.example")
	service := users.NewService(db)
	for i := 0; i < 12; i++ {
		user, err := service.Create(users.CreateUserDTO{Username: fmt.Sprintf("user_%d", i), ExpireAt: time.Now().UTC().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		for _, squad := range squads {
			if random.Intn(3) == 0 {
				if err := db.Create(&database.InternalSquadMember{InternalSquadUUID: squad, UserID: user.ID}).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, flags := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			want, err := hostsForUserSQL(db, user.ID, flags[0], flags[1])
			if err != nil {
				t.Fatal(err)
			}
			got, err := h.hostsForUser(user.ID, flags[0], flags[1])
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(hostUUIDs(got), want) {
				t.Fatalf("user %d flags %v:\n got %v\nwant %v", i, flags, hostUUIDs(got), want)
			}
		}
	}
	if h.cache.current.Load() == nil {
		t.Fatal("hosts were not cached")
	}
}

// TestCachedRenderMatchesUncached renders every format concurrently from
// the cache and checks the output against an uncached handler and that the
// shared values are left as loaded.
func TestCachedRenderMatchesUncached(t *testing.T) {
	enableCache(t)
	db, user := subscriptionDB(t)
	profile, inbound := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	template := "55555555-5555-4555-8555-555555555555"
	if err := db.Create(&database.SubscriptionTemplate{UUID: template, Name: "host-template", TemplateType: "XRAY_JSON",
		TemplateJson: `{"remnawave":{"injectHosts":[{"selectFrom":"ALL"}]},"outbounds":[{"tag":"direct","protocol":"freedom"}],"dns":{"servers":["1.1.1.1"]}}`}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		host := database.Host{UUID: fmt.Sprintf("66666666-6666-4666-8666-%012d", i), Remark: fmt.Sprintf("shared-%d", i),
			Address: fmt.Sprintf("shared-%d.example.com", i), Port: 443, ViewPosition: i + 2, ConfigProfileUUID: &profile, ConfigProfileInboundUUID: &inbound,
			ShuffleHost: i%3 == 0, Mapper: `{"streamSettings.sockopt.mark":1}`, MuxParams: `{"enabled":true,"concurrency":8}`,
			XhttpExtraParams: `{"xPaddingBytes":"100-1000"}`, SockoptParams: `{"tcpFastOpen":true}`, Tags: database.StringArray{"T"}}
		if i == 1 {
			host.XrayJsonTemplateUUID = &template
		}
		if err := db.Create(&host).Error; err != nil {
			t.Fatal(err)
		}
	}
	setSettings(t, db, map[string]interface{}{"custom_response_headers": `{"X-Shared":"{{USERNAME}}"}`})
	random := subtpl.Random
	subtpl.Random = func(int) int { return 0 }
	t.Cleanup(func() { subtpl.Random = random })

	router := func(h *Handler) *chi.Mux {
		router := chi.NewRouter()
		router.Get("/api/sub/{shortUuid}", h.GetSubscription)
		router.Get("/api/sub/{shortUuid}/{clientType}", h.GetSubscription)
		return router
	}
	cachedHandler := NewHandler(db, "sub.example")
	cachedHandler.SetDisableHistory(true)
	uncachedHandler := NewHandler(db, "sub.example")
	uncachedHandler.SetDisableHistory(true)
	uncachedHandler.cache = nil
	cached, uncached := router(cachedHandler), router(uncachedHandler)
	paths := []string{"", "/json", "/mihomo", "/singbox", "/stash", "/clash"}
	agents := []string{"curl/8.0", "Happ/1.0", "FlClash X/1.0"}
	render := func(router *chi.Mux, path, agent string) string {
		r := httptest.NewRequest("GET", "/api/sub/"+user.ShortUUID+path, nil)
		r.Header.Set("User-Agent", agent)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return fmt.Sprint(w.Code, w.Header().Get("X-Shared"), w.Body.String())
	}
	want := map[string]string{}
	for _, path := range paths {
		for _, agent := range agents {
			want[path+agent] = render(uncached, path, agent)
		}
	}
	if !strings.Contains(want["/json"+"curl/8.0"], "shared-5.example.com") || !strings.Contains(want["/mihomo"+"Happ/1.0"], "shared-5.example.com") {
		t.Fatalf("hosts missing from the reference output: %s", want["/json"+"curl/8.0"])
	}
	for _, path := range paths {
		for _, agent := range agents {
			if got := render(cached, path, agent); got != want[path+agent] {
				t.Fatalf("%s %s:\n got %s\nwant %s", path, agent, got, want[path+agent])
			}
		}
	}
	snap := cachedHandler.cache.current.Load()
	if snap == nil {
		t.Fatal("no snapshot")
	}
	index, _ := snap.hosts.get(nil)
	settings, _ := snap.settings.get(nil)
	dump := func() string {
		out := fmt.Sprintf("%+v|%s|", *settings, ojson.Stringify(settings.CustomResponseHeaders))
		for _, entry := range index.hosts {
			host := *entry.host
			out += fmt.Sprintf("%+v|%s|%s|%s|%s|%s|", host, ojson.Stringify(host.RawInbound), ojson.Stringify(host.Mapper),
				ojson.Stringify(host.XrayJSONTemplate), ojson.Stringify(host.XhttpExtraParams), ojson.Stringify(host.MuxParams))
		}
		snap.templates.Range(func(key, value any) bool {
			result, _ := value.(*lazy[templateResult]).get(nil)
			out += fmt.Sprintf("%v=%s|", key, ojson.Stringify(result.value))
			return true
		})
		return out
	}
	before := dump()
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 5; round++ {
				for _, path := range paths {
					for _, agent := range agents {
						if got := render(cached, path, agent); got != want[path+agent] {
							errs <- fmt.Sprintf("%s %s:\n got %s\nwant %s", path, agent, got, want[path+agent])
							return
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if cachedHandler.cache.current.Load() != snap {
		t.Fatal("snapshot was replaced without a write")
	}
	if after := dump(); after != before {
		t.Fatalf("rendering modified cached values:\nbefore %s\nafter  %s", before, after)
	}
}

func TestCacheTracksWrites(t *testing.T) {
	db, user := subscriptionDB(t)
	tracker := trackerFor(db)
	if trackerFor(db.Session(&gorm.Session{})) != tracker {
		t.Fatal("sessions must share the tracker")
	}
	generation := tracker.generation.Load()
	unchanged := func(name string, write func() error) {
		t.Helper()
		if err := write(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if tracker.generation.Load() != generation {
			t.Fatalf("%s invalidated the cache", name)
		}
	}
	changed := func(name string, write func() error) {
		t.Helper()
		if err := write(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if next := tracker.generation.Load(); next == generation {
			t.Fatalf("%s did not invalidate the cache", name)
		} else {
			generation = next
		}
	}
	unchanged("user update", func() error {
		return db.Model(&database.User{}).Where("id = ?", user.ID).Update("description", "x").Error
	})
	unchanged("traffic update", func() error {
		return db.Model(&database.UserTraffic{}).Where("id = ?", user.ID).Update("used_traffic_bytes", 5).Error
	})
	unchanged("membership", func() error {
		return db.Exec("DELETE FROM internal_squad_members WHERE user_id = ?", user.ID).Error
	})
	unchanged("history", func() error {
		agent := "x"
		return db.Create(&database.UserSubscriptionRequestHistory{UserID: user.ID, UserAgent: &agent, SrrResponseType: "XRAY_JSON", RequestAt: time.Now().UTC()}).Error
	})
	unchanged("node links", func() error {
		return db.Exec("DELETE FROM config_profile_inbounds_to_nodes WHERE node_uuid = ?", "none").Error
	})
	unchanged("read", func() error { var hosts []database.Host; return db.Find(&hosts).Error })

	changed("settings", func() error {
		return db.Model(&database.SubscriptionSetting{}).Where("1 = 1").Update("randomize_hosts", true).Error
	})
	changed("host model", func() error {
		return db.Model(&database.Host{}).Where("1 = 1").Update("remark", "y").Error
	})
	changed("host table", func() error { return db.Table("hosts").Where("1 = 1").Update("port", 444).Error })
	changed("raw quoted", func() error { return db.Exec(`UPDATE "hosts" SET port = 445`).Error })
	changed("transaction", func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			return tx.Create(&database.InternalSquad{UUID: "77777777-7777-4777-8777-777777777777", Name: "tx"}).Error
		})
	})
	changed("squad inbound", func() error {
		return db.Where("internal_squad_uuid = ?", "33333333-3333-4333-8333-333333333333").Delete(&database.InternalSquadInbound{}).Error
	})
	changed("external squad", func() error {
		return db.Create(&database.ExternalSquad{UUID: "88888888-8888-4888-8888-888888888888", Name: "ext"}).Error
	})
	changed("template", func() error {
		return db.Model(&database.SubscriptionTemplate{}).Where("1 = 1").Update("name", gorm.Expr("name")).Error
	})
}

func TestCacheExpiresAndSettles(t *testing.T) {
	db, user := subscriptionDB(t)
	h := NewHandler(db, "sub.example")
	clock := time.Now().Add(time.Hour)
	h.cache.now = func() time.Time { return clock }

	if _, err := h.hostsForUser(user.ID, false, false); err != nil {
		t.Fatal(err)
	}
	first := h.cache.current.Load()
	if first == nil {
		t.Fatal("not cached")
	}
	clock = clock.Add(CacheTTL - time.Millisecond)
	if h.cache.snapshot() != first {
		t.Fatal("snapshot expired early")
	}
	clock = clock.Add(time.Millisecond)
	if h.cache.snapshot() == first {
		t.Fatal("snapshot outlived its TTL")
	}

	// A write disables caching until it settles.
	setSettings(t, db, map[string]interface{}{"randomize_hosts": true})
	clock = time.Now()
	if h.cache.snapshot() != nil {
		t.Fatal("cached while a write settles")
	}
	if settings, err := h.loadSettings(); err != nil || !settings.RandomizeHosts {
		t.Fatalf("settings while settling: %+v %v", settings, err)
	}
	clock = clock.Add(CacheSettle)
	if h.cache.snapshot() == nil {
		t.Fatal("not cached after the write settled")
	}
}
