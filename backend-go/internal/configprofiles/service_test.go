package configprofiles

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"gowave/internal/database"
	"gowave/internal/testdb"
)

func TestPatchHostCorrectsStoredModeAndRollsBackInvalidRelations(t *testing.T) {
	db := testdb.Open(t)
	host := database.Host{UUID: "host", Remark: "original", Address: "example.com", Port: 443, InternalSquadsMode: "INVALID"}
	if err := db.Create(&host).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	updated, err := service.PatchHost(host.UUID, map[string]json.RawMessage{
		"internalSquads": json.RawMessage(`{"mode":"ALLOW_ONLY","squads":[]}`),
	})
	if err != nil || updated.InternalSquadsMode != "ALLOW_ONLY" {
		t.Fatalf("mode correction failed: %+v, %v", updated, err)
	}
	if _, err := service.PatchHost(host.UUID, map[string]json.RawMessage{
		"remark": json.RawMessage(`"changed"`), "nodes": json.RawMessage(`123`),
	}); err == nil {
		t.Fatal("invalid nodes accepted")
	}
	saved, err := service.GetHostByUUID(host.UUID)
	if err != nil || saved.Remark != "original" || saved.InternalSquadsMode != "ALLOW_ONLY" {
		t.Fatalf("unexpected persisted host: %+v, %v", saved, err)
	}
}

func seedShadowsocksHost(t *testing.T) *Service {
	t.Helper()
	db := testdb.Open(t)
	service := NewService(db)
	profile := database.ConfigProfile{UUID: "profile-ss", Name: "ss profile", Config: "{}"}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	inbound := database.ConfigProfileInbound{
		UUID:        "inbound-ss",
		ProfileUUID: profile.UUID,
		Tag:         "SS",
		Type:        "shadowsocks",
		RawInbound:  `{"settings":{"method":"2022-blake3-aes-128-gcm"}}`,
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	inboundRef := inbound.UUID
	host := database.Host{
		UUID:                     "host-ss",
		Remark:                   "ok",
		Address:                  "example.com",
		Port:                     1234,
		InternalSquadsMode:       "EXCLUDE",
		SecurityLayer:            "DEFAULT",
		ConfigProfileUUID:        &profile.UUID,
		ConfigProfileInboundUUID: &inboundRef,
	}
	if err := db.Create(&host).Error; err != nil {
		t.Fatal(err)
	}
	return service
}

func TestGenerateHostPreviewBuildsLinkFromStoredInbound(t *testing.T) {
	service := seedShadowsocksHost(t)

	// The stored shadowsocks inbound wins over the requested vless protocol.
	link, err := service.GenerateHostPreview(PreviewRequest{
		HostUUID:  "host-ss",
		UserAgent: "share",
		Protocol:  "vless",
	})
	if err != nil {
		t.Fatal(err)
	}
	credentials := base64.StdEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:11111111222233334444555555555555"))
	if link.ClientType != "SS Key" || link.PreviewContent != "ss://"+credentials+"@example.com:1234#ok" {
		t.Fatalf("unexpected ss preview: %q (%s)", link.PreviewContent, link.ClientType)
	}
}

func TestGenerateHostPreviewPayloadUsesModuleOfReferencedInbound(t *testing.T) {
	service := seedShadowsocksHost(t)

	preview, err := service.GenerateHostPreview(PreviewRequest{
		Host: &CreateHostDTO{
			Remark:                   "ok",
			Address:                  "example.com",
			Port:                     1234,
			ConfigProfileInboundUUID: "inbound-ss",
		},
		UserAgent: "ClashMeta",
		Protocol:  "vless",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.PreviewContent, "cipher: 2022-blake3-aes-128-gcm") {
		t.Fatalf("clash preview lost the inbound cipher: %q", preview.PreviewContent)
	}
	if strings.Contains(preview.PreviewContent, "type: vless") {
		t.Fatalf("clash preview ignored the inbound protocol: %q", preview.PreviewContent)
	}
}

func TestGenerateHostPreviewFallsBackToVlessWithoutInbound(t *testing.T) {
	db := testdb.Open(t)
	service := NewService(db)
	host := database.Host{UUID: "host-bare", Remark: "ok", Address: "example.com", Port: 443, InternalSquadsMode: "EXCLUDE"}
	if err := db.Create(&host).Error; err != nil {
		t.Fatal(err)
	}

	preview, err := service.GenerateHostPreview(PreviewRequest{HostUUID: "host-bare", UserAgent: "share"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(preview.PreviewContent, "vless://11111111-2222-3333-4444-555555555555@example.com:443?") {
		t.Fatalf("unexpected fallback preview: %q", preview.PreviewContent)
	}
}
