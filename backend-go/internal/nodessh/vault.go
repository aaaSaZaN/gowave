package nodessh

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/cloudflare/circl/oprf"
	"github.com/google/uuid"
	"golang.org/x/crypto/hkdf"
	"gorm.io/gorm"
	"gowave/internal/database"
	"gowave/internal/middleware"
	"io"
	"net/http"
	"time"
)

func vaultServer(secret string) (oprf.Server, error) {
	seed := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, []byte(secret), nil, []byte("rw-vault-oprf-v1")), seed); err != nil {
		return oprf.Server{}, err
	}
	key, err := oprf.DeriveKey(oprf.SuiteRistretto255, oprf.BaseMode, seed, []byte("rw-vault"))
	if err != nil {
		return oprf.Server{}, err
	}
	return oprf.NewServer(oprf.SuiteRistretto255, key), nil
}
func (h *Handler) EvaluateVault(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.AdminContextKey).(*middleware.TokenClaims)
	if !ok || claims.Role != "ADMIN" {
		http.Error(w, "administrator required", 403)
		return
	}
	var body struct {
		Blinded string `json:"blinded"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body) != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	data, err := base64.StdEncoding.DecodeString(body.Blinded)
	if err != nil || len(data) != 32 {
		http.Error(w, "invalid blinded element", 400)
		return
	}
	now := time.Now().UTC()
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "vault/"+claims.UUID).Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&database.AuthChallenge{}).Where("admin_uuid=? AND purpose='vault-attempt' AND expires_at > ?", claims.UUID, now).Count(&count).Error; err != nil {
			return err
		}
		if count >= 10 {
			return errors.New("vault unlock rate limit reached")
		}
		return tx.Create(&database.AuthChallenge{ID: uuid.NewString(), Purpose: "vault-attempt", AdminUUID: claims.UUID, ExpiresAt: now.Add(15 * time.Minute)}).Error
	})
	if err != nil {
		http.Error(w, "vault unlock rate limit reached", 429)
		return
	}
	element := oprf.SuiteRistretto255.Group().NewElement()
	if err := element.UnmarshalBinary(data); err != nil || element.IsIdentity() {
		http.Error(w, "invalid blinded element", 400)
		return
	}
	server, err := vaultServer(h.secret)
	if err != nil {
		http.Error(w, "vault unavailable", 500)
		return
	}
	evaluation, err := server.Evaluate(&oprf.EvaluationRequest{Elements: []oprf.Blinded{element}})
	if err != nil {
		http.Error(w, "vault evaluation failed", 400)
		return
	}
	evaluated, err := evaluation.Elements[0].MarshalBinaryCompress()
	if err != nil {
		http.Error(w, "vault evaluation failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]string{"evaluated": base64.StdEncoding.EncodeToString(evaluated)}})
}
