package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"remnawave-go/internal/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	db          *gorm.DB
	appSecret   string
	jwtLifetime int
}

func NewHandler(db *gorm.DB, appSecret string, jwtLifetime int) *Handler {
	return &Handler{
		db:          db,
		appSecret:   appSecret,
		jwtLifetime: jwtLifetime,
	}
}

type AuthStatusResponse struct {
	Response struct {
		IsLoginAllowed    bool `json:"isLoginAllowed"`
		IsRegisterAllowed bool `json:"isRegisterAllowed"`
		Authentication    struct {
			Password struct {
				Enabled bool `json:"enabled"`
			} `json:"password"`
			Passkey struct {
				Enabled bool `json:"enabled"`
			} `json:"passkey"`
			OAuth2 struct {
				Providers map[string]bool `json:"providers"`
			} `json:"oauth2"`
		} `json:"authentication"`
		Branding struct {
			Title   *string `json:"title"`
			LogoURL *string `json:"logoUrl"`
		} `json:"branding"`
	} `json:"response"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Response struct {
		AccessToken string `json:"accessToken"`
	} `json:"response"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	Response struct {
		AccessToken string `json:"accessToken"`
	} `json:"response"`
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var adminCount int64
	h.db.Model(&database.Admin{}).Count(&adminCount)

	var setting database.RemnawaveSetting
	if err := h.db.First(&setting, 1).Error; err != nil {
		setting = database.RemnawaveSetting{
			Title:               "Remnawave",
			IsLoginAllowed:      true,
			IsRegisterAllowed:   false,
			PasswordAuthEnabled: false,
		}
	} else {
		if setting.PasswordSettings != "" && setting.PasswordSettings != "{}" {
			var pw map[string]interface{}
			if err := json.Unmarshal([]byte(setting.PasswordSettings), &pw); err == nil {
				if en, ok := pw["enabled"].(bool); ok {
					setting.PasswordAuthEnabled = en
				}
			}
		}
	}

	var resp AuthStatusResponse
	if adminCount == 0 {
		resp.Response.IsLoginAllowed = false
		resp.Response.IsRegisterAllowed = true
	} else {
		resp.Response.IsLoginAllowed = setting.IsLoginAllowed
		resp.Response.IsRegisterAllowed = false
	}

	resp.Response.Authentication.Password.Enabled = setting.PasswordAuthEnabled
	resp.Response.Authentication.Passkey.Enabled = false // Disabled until secure WebAuthn is implemented
	resp.Response.Authentication.OAuth2.Providers = map[string]bool{
		"github":   false,
		"pocketid": false,
		"yandex":   false,
		"keycloak": false,
		"generic":  false,
		"telegram": false,
	}
	if setting.OAuth2Settings != "" && setting.OAuth2Settings != "{}" {
		var oauth2 map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(setting.OAuth2Settings), &oauth2); err == nil {
			for provider, data := range oauth2 {
				if en, ok := data["enabled"].(bool); ok {
					resp.Response.Authentication.OAuth2.Providers[provider] = en
				}
			}
		}
	}
	title := setting.Title
	if title == "" {
		title = "Remnawave"
	}
	resp.Response.Branding.Title = &title
	if setting.LogoURL != "" {
		resp.Response.Branding.LogoURL = &setting.LogoURL
	} else {
		resp.Response.Branding.LogoURL = nil
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var adminCount int64
	h.db.Model(&database.Admin{}).Count(&adminCount)
	if adminCount > 0 {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Registration is disabled because an admin already exists"}`))
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"Bad request"}`))
		return
	}

	if req.Username == "" || len(req.Password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"Username is required and password must be at least 8 characters"}`))
		return
	}

	hash, err := HashPassword(req.Password, h.appSecret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Failed to hash password"}`))
		return
	}

	admin := database.Admin{
		UUID:         uuid.NewString(),
		Username:     req.Username,
		PasswordHash: hash,
		Role:         "ADMIN",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.Create(&admin).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Failed to create admin"}`))
		return
	}

	token, err := GenerateToken(admin.UUID, admin.Username, admin.Role, h.appSecret, h.jwtLifetime)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"Internal server error"}`))
		return
	}

	var resp RegisterResponse
	resp.Response.AccessToken = token
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"message":"Bad request"}`, http.StatusBadRequest)
		return
	}

	var admin database.Admin
	if err := h.db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Invalid credentials"}`))
		return
	}

	if !VerifyPassword(req.Password, admin.PasswordHash, h.appSecret) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Invalid credentials"}`))
		return
	}

	token, err := GenerateToken(admin.UUID, admin.Username, admin.Role, h.appSecret, h.jwtLifetime)
	if err != nil {
		http.Error(w, `{"message":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	var resp LoginResponse
	resp.Response.AccessToken = token
	json.NewEncoder(w).Encode(resp)
}
