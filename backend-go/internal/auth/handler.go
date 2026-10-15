package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"remnawave-go/internal/database"
	"remnawave-go/internal/httpx"
	"remnawave-go/internal/notify"

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
	var isPasskeyEnabled bool
	if setting.PasskeySettings != "" && setting.PasskeySettings != "{}" {
		var ps map[string]interface{}
		if err := json.Unmarshal([]byte(setting.PasskeySettings), &ps); err == nil {
			if en, ok := ps["enabled"].(bool); ok {
				isPasskeyEnabled = en
			}
		}
	}
	resp.Response.Authentication.Passkey.Enabled = isPasskeyEnabled
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

// loginAttempt emits service.login_attempt_* without submitted credentials.
func loginAttempt(db *gorm.DB, success bool, username, password, ip, userAgent, description string) {
	if description == "" {
		description = "–"
	}
	attempt := map[string]string{"username": username, "ip": ip, "userAgent": userAgent, "description": description}
	name := "service.login_attempt_success"
	if !success {
		name = "service.login_attempt_failed"
	}
	if err := notify.Emit(db, notify.Event{Name: name, Data: map[string]interface{}{"loginAttempt": attempt}}); err != nil {
		log.Printf("[AUTH] %s notification: %v", name, err)
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.Decode(r, 4096, &req); err != nil || req.Username == "" || req.Password == "" {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "username and password are required")
		return
	}
	ip, agent := httpx.ClientIP(r), r.UserAgent()
	fail := func(description string) {
		loginAttempt(h.db, false, req.Username, req.Password, ip, agent, description)
		httpx.Error(w, r, http.StatusForbidden, "A068", "Forbidden")
	}

	var setting database.RemnawaveSetting
	if err := h.db.First(&setting, 1).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A067", "Get auth status error")
		return
	}
	if !setting.IsLoginAllowed {
		fail("Login is not allowed.")
		return
	}
	enabled := setting.PasswordAuthEnabled
	if setting.PasswordSettings != "" && setting.PasswordSettings != "{}" {
		var password struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(setting.PasswordSettings), &password); err != nil {
			httpx.Error(w, r, http.StatusInternalServerError, "A067", "Get auth status error")
			return
		}
		if password.Enabled != nil {
			enabled = *password.Enabled
		}
	}
	if !enabled {
		fail("Someone tried to login with password authentication, but it is disabled.")
		return
	}

	var admin database.Admin
	if err := h.db.Where("username = ? AND role = ?", req.Username, "ADMIN").First(&admin).Error; err != nil {
		fail("Admin is not found in database.")
		return
	}
	if !VerifyPassword(req.Password, admin.PasswordHash, h.appSecret) {
		fail("Invalid password.")
		return
	}

	token, err := GenerateToken(admin.UUID, admin.Username, admin.Role, h.appSecret, h.jwtLifetime)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A002", "Login error")
		return
	}
	loginAttempt(h.db, true, admin.Username, "", ip, agent, "")
	httpx.Response(w, map[string]string{"accessToken": token})
}
