package nodessh

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"gowave/internal/auth"
	"gowave/internal/database"
	"gowave/internal/middleware"
	"net"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	db       *gorm.DB
	secret   string
	sessions chan struct{}
}

func NewHandler(db *gorm.DB, secret string) *Handler {
	return &Handler{db, secret, make(chan struct{}, 20)}
}

type ticketData struct {
	NodeUUID string `json:"nodeUuid"`
	IP       string `json:"clientIp"`
}

func clientIP(r *http.Request) string {
	if value := r.Header.Get("X-Remnawave-Real-IP"); value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func (h *Handler) Ticket(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.AdminContextKey).(*middleware.TokenClaims)
	if !ok || claims.Role != "ADMIN" {
		http.Error(w, "administrator required", 403)
		return
	}
	id := chi.URLParam(r, "uuid")
	var node database.Node
	if h.db.Where("uuid=?", id).First(&node).Error != nil {
		http.Error(w, "node not found", 404)
		return
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		http.Error(w, "ticket unavailable", 500)
		return
	}
	ticket := base64.RawURLEncoding.EncodeToString(random)
	data, _ := json.Marshal(ticketData{node.UUID, clientIP(r)})
	if err := h.db.Create(&database.AuthChallenge{ID: ticket, Purpose: "ssh-ticket", AdminUUID: claims.UUID, Data: string(data), ExpiresAt: time.Now().UTC().Add(15 * time.Second)}).Error; err != nil {
		http.Error(w, "ticket unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"ticket": ticket, "path": "/api/node-ssh/ws", "expiresInSeconds": 15}})
}
func (h *Handler) Socket(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",")
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "rw" {
		http.Error(w, "invalid credentials", 401)
		return
	}
	copy := r.Clone(r.Context())
	copy.Header.Set("Authorization", "Bearer "+strings.TrimSpace(parts[2]))
	middleware.Auth(h.secret, h.db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(middleware.AdminContextKey).(*middleware.TokenClaims)
		if claims.Role != "ADMIN" {
			http.Error(w, "administrator required", 403)
			return
		}
		record, err := auth.ConsumeChallenge(h.db, strings.TrimSpace(parts[1]), "ssh-ticket", claims.UUID)
		if err != nil {
			http.Error(w, "invalid ticket", 401)
			return
		}
		var data ticketData
		if json.Unmarshal([]byte(record.Data), &data) != nil || data.IP != clientIP(r) {
			http.Error(w, "ticket mismatch", 401)
			return
		}
		var node database.Node
		if h.db.Where("uuid=?", data.NodeUUID).First(&node).Error != nil {
			http.Error(w, "node not found", 404)
			return
		}
		select {
		case h.sessions <- struct{}{}:
			defer func() { <-h.sessions }()
		default:
			http.Error(w, "session limit reached", 503)
			return
		}
		h.terminal(w, r, &node)
	})).ServeHTTP(w, copy)
}
