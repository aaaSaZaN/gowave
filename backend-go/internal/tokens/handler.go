package tokens

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"remnawave-go/internal/contracts"
	"remnawave-go/internal/database"
	"remnawave-go/internal/httpx"
	"remnawave-go/internal/middleware"
	"remnawave-go/internal/notify"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	db        *gorm.DB
	appSecret string
}

func NewHandler(db *gorm.DB, appSecret string) *Handler {
	return &Handler{
		db:        db,
		appSecret: appSecret,
	}
}

type ApiTokenItem struct {
	UUID      string   `json:"uuid"`
	Name      string   `json:"name"`
	ExpireAt  string   `json:"expireAt"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

type CreateApiTokenResponse struct {
	UUID      string   `json:"uuid"`
	Name      string   `json:"name"`
	ExpireAt  string   `json:"expireAt"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	Token     string   `json:"token"`
}

func (h *Handler) GetScopes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"response":` + string(ScopesData) + `}`))
}

func (h *Handler) GetApiTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var tokens []database.ApiToken
	if err := h.db.Order("created_at desc").Find(&tokens).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	result := make([]ApiTokenItem, 0, len(tokens))
	for _, t := range tokens {
		scopes := []string(t.Scopes)
		if len(scopes) == 0 {
			scopes = []string{"*"}
		}
		result = append(result, ApiTokenItem{
			UUID:      t.UUID,
			Name:      t.Name,
			ExpireAt:  t.ExpireAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			Scopes:    scopes,
			CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			UpdatedAt: t.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tokens": result,
		},
	})
}

func tokenEvent(name string, token *database.ApiToken) notify.Event {
	return notify.Event{Name: name, Data: map[string]interface{}{"apiToken": map[string]interface{}{
		"name": token.Name, "uuid": token.UUID, "scopes": []string(token.Scopes),
		"expireAt": token.ExpireAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}}}
}

func (h *Handler) CreateApiToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string    `json:"name"`
		ExpiresInDays *float64  `json:"expiresInDays"`
		Scopes        *[]string `json:"scopes"`
	}
	if err := httpx.Decode(r, 64<<10, &body); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "Invalid request body")
		return
	}
	if len(body.Name) < 2 || len(body.Name) > 30 {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "name must be 2-30 characters")
		return
	}
	if body.ExpiresInDays == nil || *body.ExpiresInDays < 1 {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "expiresInDays must be at least 1")
		return
	}
	scopes := []string{"*"}
	if body.Scopes != nil {
		scopes = *body.Scopes
	}
	for _, scope := range scopes {
		if !contracts.ValidScope(scope) {
			httpx.Error(w, r, http.StatusBadRequest, "A229", "Invalid API token scope: "+scope)
			return
		}
	}

	tokenUUID := uuid.New().String()
	now := time.Now().UTC()
	expireAt := now.Add(time.Duration(*body.ExpiresInDays * float64(24*time.Hour)))
	claims := &middleware.TokenClaims{
		UUID: tokenUUID,
		Role: "API",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expireAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.appSecret))
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A005", "Create API token error")
		return
	}
	dbToken := database.ApiToken{UUID: tokenUUID, Name: body.Name, ExpireAt: expireAt, Scopes: scopes, CreatedAt: now, UpdatedAt: now}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&dbToken).Error; err != nil {
			return err
		}
		return notify.Emit(tx, tokenEvent("service.api_token_created", &dbToken))
	}); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A005", "Create API token error")
		return
	}
	httpx.Response(w, CreateApiTokenResponse{
		UUID:      tokenUUID,
		Name:      body.Name,
		ExpireAt:  expireAt.Format("2006-01-02T15:04:05.000Z"),
		Scopes:    scopes,
		CreatedAt: now.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt: now.Format("2006-01-02T15:04:05.000Z"),
		Token:     tokenString,
	})
}

func (h *Handler) DeleteApiToken(w http.ResponseWriter, r *http.Request) {
	var token database.ApiToken
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("uuid = ?", chi.URLParam(r, "uuid")).First(&token).Error; err != nil {
			return err
		}
		if err := tx.Delete(&token).Error; err != nil {
			return err
		}
		return notify.Emit(tx, tokenEvent("service.api_token_deleted", &token))
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "A006", "Delete API token error")
		return
	}
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A006", "Delete API token error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetOtt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	now := time.Now().UTC()
	admin, ok := r.Context().Value(middleware.AdminContextKey).(*middleware.TokenClaims)
	if !ok || admin.Role != "ADMIN" {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
		return
	}
	id := uuid.NewString()
	claims := jwt.MapClaims{"jti": id, "scope": "ott", "iss": "backend-tools", "sub": admin.UUID, "exp": now.Add(30 * time.Second).Unix(), "iat": now.Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.appSecret))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to create OTT"})
		return
	}
	if err := h.db.Create(&database.AuthChallenge{ID: id, Purpose: "ott", AdminUUID: admin.UUID, ExpiresAt: now.Add(30 * time.Second)}).Error; err != nil {
		http.Error(w, `{"message":"Failed to create OTT"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"ott": tokenString,
		},
	})
}
