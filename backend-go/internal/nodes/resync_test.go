package nodes

import (
	"slices"
	"testing"
	"time"

	"gowave/internal/database"
	"gowave/internal/testdb"

	"github.com/google/uuid"
)

func TestResyncPayloadsMatchPerUserSync(t *testing.T) {
	db := testdb.Open(t)
	s := &Service{db: db}
	tcp, reality := "tcp", "reality"
	profile := database.ConfigProfile{UUID: uuid.NewString(), Name: "p1", Config: "{}"}
	other := database.ConfigProfile{UUID: uuid.NewString(), Name: "p2", Config: "{}"}
	vless := database.ConfigProfileInbound{UUID: uuid.NewString(), ProfileUUID: profile.UUID, Tag: "VLESS", Type: "vless", Network: &tcp, Security: &reality, RawInbound: `{"settings":{}}`}
	trojan := database.ConfigProfileInbound{UUID: uuid.NewString(), ProfileUUID: profile.UUID, Tag: "TROJAN", Type: "trojan"}
	ss := database.ConfigProfileInbound{UUID: uuid.NewString(), ProfileUUID: profile.UUID, Tag: "SS", Type: "shadowsocks", RawInbound: `{"settings":{"method":"2022-blake3-aes-128-gcm"}}`}
	elsewhere := database.ConfigProfileInbound{UUID: uuid.NewString(), ProfileUUID: other.UUID, Tag: "ELSEWHERE", Type: "vless"}
	for _, record := range []interface{}{&profile, &other, &vless, &trojan, &ss, &elsewhere} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	squadA := database.InternalSquad{UUID: uuid.NewString(), Name: "a"}
	squadB := database.InternalSquad{UUID: uuid.NewString(), Name: "b"}
	db.Create(&squadA)
	db.Create(&squadB)
	for _, link := range []database.InternalSquadInbound{
		{InternalSquadUUID: squadA.UUID, InboundUUID: vless.UUID},
		{InternalSquadUUID: squadA.UUID, InboundUUID: trojan.UUID},
		{InternalSquadUUID: squadB.UUID, InboundUUID: vless.UUID},
		{InternalSquadUUID: squadB.UUID, InboundUUID: ss.UUID},
		{InternalSquadUUID: squadB.UUID, InboundUUID: elsewhere.UUID},
	} {
		if err := db.Create(&link).Error; err != nil {
			t.Fatal(err)
		}
	}
	newUser := func(name, status string, squads ...string) database.User {
		user := database.User{ShortUUID: uuid.NewString()[:16], Username: name, Status: status, ExpireAt: time.Now().Add(time.Hour), VlessUUID: uuid.NewString(), TrojanPassword: "t-" + name, SsPassword: "s-" + name}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		for _, squad := range squads {
			db.Create(&database.InternalSquadMember{InternalSquadUUID: squad, UserID: user.ID})
		}
		return user
	}
	both := newUser("both", "ACTIVE", squadA.UUID, squadB.UUID)
	orphan := newUser("orphan", "ACTIVE")
	newUser("disabled", "DISABLED", squadA.UUID)

	node := database.Node{UUID: uuid.NewString(), Name: "n1", Address: "192.0.2.10", ActiveConfigProfileUUID: &profile.UUID}
	db.Create(&node)
	tags := s.GetNodeActiveInboundTags(&node)
	slices.Sort(tags)
	if !slices.Equal(tags, []string{"SS", "TROJAN", "VLESS"}) {
		t.Fatalf("active tags = %v", tags)
	}
	var active []database.User
	db.Where("status = ?", "ACTIVE").Order("id").Find(&active)
	add, remove, err := s.resyncPayloads(active, tags)
	if err != nil {
		t.Fatal(err)
	}
	if len(add.Users) != 1 || add.Users[0].UserData.UserID != itoa(both.ID) {
		t.Fatalf("add users = %+v", add.Users)
	}
	data := add.Users[0].UserData
	if data.HashUUID != both.VlessUUID || data.TrojanPassword != "t-both" || data.SSPassword != "s-both" {
		t.Fatalf("user data = %+v", data)
	}
	got := map[string]BulkUserInbound{}
	for _, inbound := range add.Users[0].InboundData {
		if _, dup := got[inbound.Tag]; dup {
			t.Fatalf("tag %s sent twice", inbound.Tag)
		}
		got[inbound.Tag] = inbound
	}
	if len(got) != 3 || got["VLESS"].Flow != "xtls-rprx-vision" || got["TROJAN"].Type != "trojan" || got["SS"].Type != "shadowsocks22" {
		t.Fatalf("inbounds = %+v", got)
	}
	if len(remove.Users) != 1 || remove.Users[0].UserID != itoa(orphan.ID) || remove.Users[0].HashUUID != orphan.VlessUUID {
		t.Fatalf("remove users = %+v", remove.Users)
	}
}

func TestSyncAllUsersToNodeQueuesOneBulkJob(t *testing.T) {
	db := testdb.Open(t)
	s := &Service{db: db}
	node := database.Node{UUID: uuid.NewString(), Name: "n1", Address: "192.0.2.11"}
	db.Create(&node)
	for i := 0; i < 50; i++ {
		db.Create(&database.User{ShortUUID: uuid.NewString()[:16], Username: "u" + uuid.NewString()[:8], Status: "ACTIVE", ExpireAt: time.Now().Add(time.Hour), VlessUUID: uuid.NewString()})
	}
	failed := database.Job{ID: uuid.NewString(), Kind: "user-sync", EntityID: "1", Target: node.UUID, Payload: "{}", State: "failed", Attempts: 10, AvailableAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	db.Create(&failed)

	s.SyncAllUsersToNode(&node)
	s.SyncAllUsersToNode(&node)

	var resync, perUser int64
	db.Model(&database.Job{}).Where("kind = ? AND target = ? AND state = ?", "user-resync", node.UUID, "pending").Count(&resync)
	db.Model(&database.Job{}).Where("kind = ? AND target = ? AND state = ?", "user-sync", node.UUID, "pending").Count(&perUser)
	if resync != 1 {
		t.Fatalf("pending resync jobs = %d, want 1", resync)
	}
	if perUser != 1 {
		t.Fatalf("pending user-sync jobs = %d, want only the revived failed one", perUser)
	}
}
