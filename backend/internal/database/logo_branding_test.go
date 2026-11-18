package database_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"gowave/internal/database"
	"gowave/internal/testdb"
)

func TestStockLogoUpgradePreservesCustomBranding(t *testing.T) {
	db := testdb.Open(t)
	var defaults struct {
		Branding struct {
			LogoURL string `json:"logoUrl"`
		} `json:"brandingSettings"`
	}
	if err := json.Unmarshal([]byte(database.DefaultSubpageConfigData), &defaults); err != nil {
		t.Fatal(err)
	}
	payload, ok := strings.CutPrefix(defaults.Branding.LogoURL, "data:image/svg+xml;base64,")
	if !ok {
		t.Fatal("default subscription logo must be self-contained")
	}
	artwork, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || !strings.Contains(string(artwork), "<title>Gowave</title>") {
		t.Fatalf("default logo is not Gowave artwork: %v", err)
	}
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	stock := `{"brandingSettings":{"logoUrl":"https://docs.rw/img/logo.svg","title":"Custom title"},"future":9007199254740993}`
	for index, config := range []string{
		stock,
		` {"brandingSettings":{"logoUrl":"https://example.com/custom.svg"},"future":9007199254740993} `,
		`{"brandingSettings":{"logoUrl":"https://docs.rw/img/logo.svg?custom=1"}}`,
		`{"brandingSettings":{"logoUrl":null}}`,
		`{"brandingSettings":{}}`,
		`null`,
		`invalid`,
	} {
		page := database.SubscriptionPageConfig{
			UUID: uuid.NewString(), Name: uuid.NewString(), Config: config, UpdatedAt: stamp,
		}
		if err := db.Create(&page).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("DELETE FROM gowave_migrations WHERE name = ?", "010_gowave_logos.sql").Error; err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := database.AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
		}
		var saved database.SubscriptionPageConfig
		if err := db.First(&saved, "uuid = ?", page.UUID).Error; err != nil {
			t.Fatal(err)
		}
		if !saved.UpdatedAt.Equal(stamp) {
			t.Fatal("logo upgrade changed the page timestamp")
		}
		if index == 0 {
			want := strings.Replace(stock, "https://docs.rw/img/logo.svg", defaults.Branding.LogoURL, 1)
			var gotJSON, wantJSON map[string]json.RawMessage
			if err := json.Unmarshal([]byte(saved.Config), &gotJSON); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
				t.Fatal(err)
			}
			gotBytes, _ := json.Marshal(gotJSON)
			wantBytes, _ := json.Marshal(wantJSON)
			if string(gotBytes) != string(wantBytes) {
				t.Fatal("stock logo replacement changed other page settings")
			}
		} else if saved.Config != config {
			t.Fatalf("custom or incomplete configuration was changed: %q", config)
		}
	}

	var panel database.PanelSetting
	if err := db.First(&panel).Error; err != nil {
		t.Fatal(err)
	}
	for _, logo := range []string{"https://docs.rw/img/logo.svg", "https://example.com/custom.svg"} {
		branding := `{"logoUrl":"` + logo + `","title":"Custom panel"}`
		if err := db.Model(&panel).Updates(map[string]interface{}{
			"logo_url": logo, "branding_settings": branding,
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("DELETE FROM gowave_migrations WHERE name = ?", "010_gowave_logos.sql").Error; err != nil {
			t.Fatal(err)
		}
		if err := database.AutoMigrate(db); err != nil {
			t.Fatal(err)
		}
		if err := db.First(&panel, "id = ?", panel.ID).Error; err != nil {
			t.Fatal(err)
		}
		want := logo
		if logo == "https://docs.rw/img/logo.svg" {
			want = ""
		}
		if panel.LogoURL != want || panel.BrandingSettings != strings.Replace(branding, logo, want, 1) {
			t.Fatal("panel logo replacement changed custom branding")
		}
	}
}
