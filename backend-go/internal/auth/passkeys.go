package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/middleware"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

type PasskeyHandler struct {
	db       *gorm.DB
	secret   string
	lifetime int
}

func NewPasskeyHandler(db *gorm.DB, secret string, lifetime int) *PasskeyHandler {
	return &PasskeyHandler{db, secret, lifetime}
}

type passkeyUser struct {
	admin       database.Admin
	credentials []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return []byte(u.admin.UUID) }
func (u passkeyUser) WebAuthnName() string                       { return u.admin.Username }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.admin.Username }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func (h *PasskeyHandler) configured() (*webauthn.WebAuthn, error) {
	var setting database.RemnawaveSetting
	if err := h.db.First(&setting, 1).Error; err != nil {
		return nil, err
	}
	var config struct {
		Enabled bool   `json:"enabled"`
		RPID    string `json:"rpId"`
		Origin  string `json:"origin"`
	}
	if err := json.Unmarshal([]byte(setting.PasskeySettings), &config); err != nil {
		return nil, err
	}
	if !config.Enabled || config.RPID == "" || config.Origin == "" {
		return nil, errors.New("passkey authentication is disabled or unconfigured")
	}
	return webauthn.New(&webauthn.Config{RPID: config.RPID, RPDisplayName: "Remnawave", RPOrigins: []string{config.Origin, "https://" + config.RPID}, AuthenticatorSelection: protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementPreferred, UserVerification: protocol.VerificationRequired}})
}

func (h *PasskeyHandler) loadUser(id string) (*passkeyUser, error) {
	u := &passkeyUser{}
	query := h.db.Where("role = ?", "ADMIN")
	if id != "" {
		query = query.Where("uuid = ?", id)
	}
	if err := query.First(&u.admin).Error; err != nil {
		return nil, err
	}
	var records []database.Passkey
	if err := h.db.Where("admin_uuid = ?", u.admin.UUID).Find(&records).Error; err != nil {
		return nil, err
	}
	for _, record := range records {
		var credential webauthn.Credential
		if record.CredentialData != "" {
			if err := json.Unmarshal([]byte(record.CredentialData), &credential); err != nil {
				return nil, err
			}
		} else {
			id, err := base64.RawURLEncoding.DecodeString(record.ID)
			if err != nil || len(record.PublicKey) == 0 {
				continue
			}
			credential.ID, credential.PublicKey = id, record.PublicKey
			credential.Authenticator.SignCount = uint32(record.Counter)
			credential.Flags.BackupEligible = record.DeviceType == "multiDevice"
			credential.Flags.BackupState = record.BackedUp
			for _, transport := range strings.Split(record.Transports, ",") {
				if transport != "" {
					credential.Transport = append(credential.Transport, protocol.AuthenticatorTransport(transport))
				}
			}
		}
		u.credentials = append(u.credentials, credential)
	}
	return u, nil
}

func (h *PasskeyHandler) storeSession(session *webauthn.SessionData, purpose, adminUUID string) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return h.db.Create(&database.AuthChallenge{ID: session.Challenge, Purpose: purpose, AdminUUID: adminUUID, Data: string(data), ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}).Error
}

func passkeyJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": value})
}

func (h *PasskeyHandler) RegistrationOptions(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.AdminContextKey).(*middleware.TokenClaims)
	if !ok || claims.Role != "ADMIN" {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
		return
	}
	wa, err := h.configured()
	if err != nil {
		http.Error(w, `{"message":"Passkey authentication is not configured"}`, http.StatusForbidden)
		return
	}
	u, err := h.loadUser(claims.UUID)
	if err != nil {
		http.Error(w, `{"message":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}
	options, session, err := wa.BeginRegistration(u)
	if err != nil || h.storeSession(session, "passkey-register", u.admin.UUID) != nil {
		http.Error(w, `{"message":"Cannot create challenge"}`, http.StatusInternalServerError)
		return
	}
	passkeyJSON(w, options.Response)
}

func (h *PasskeyHandler) AuthenticationOptions(w http.ResponseWriter, r *http.Request) {
	wa, err := h.configured()
	if err != nil {
		http.Error(w, `{"message":"Passkey authentication is disabled"}`, http.StatusForbidden)
		return
	}
	u, err := h.loadUser("")
	if err != nil || len(u.credentials) == 0 {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
		return
	}
	options, session, err := wa.BeginLogin(u, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil || h.storeSession(session, "passkey-login", u.admin.UUID) != nil {
		http.Error(w, `{"message":"Cannot create challenge"}`, http.StatusInternalServerError)
		return
	}
	passkeyJSON(w, options.Response)
}

func passkeyResponse(r *http.Request) (json.RawMessage, error) {
	var body struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&body); err != nil {
		return nil, err
	}
	if len(body.Response) == 0 {
		return nil, errors.New("missing response")
	}
	return body.Response, nil
}

func (h *PasskeyHandler) session(challenge, purpose, adminUUID string) (*passkeyUser, *webauthn.SessionData, error) {
	record, err := ConsumeChallenge(h.db, challenge, purpose, adminUUID)
	if err != nil {
		return nil, nil, err
	}
	u, err := h.loadUser(record.AdminUUID)
	if err != nil {
		return nil, nil, err
	}
	var session webauthn.SessionData
	if err = json.Unmarshal([]byte(record.Data), &session); err != nil {
		return nil, nil, err
	}
	return u, &session, nil
}

func (h *PasskeyHandler) VerifyRegistration(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.AdminContextKey).(*middleware.TokenClaims)
	if !ok || claims.Role != "ADMIN" {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
		return
	}
	wa, err := h.configured()
	if err != nil {
		http.Error(w, `{"message":"Passkeys are disabled"}`, http.StatusForbidden)
		return
	}
	body, err := passkeyResponse(r)
	if err != nil {
		http.Error(w, `{"message":"Invalid passkey response"}`, http.StatusBadRequest)
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(body)
	if err != nil {
		http.Error(w, `{"message":"Invalid passkey response"}`, http.StatusBadRequest)
		return
	}
	u, session, err := h.session(parsed.Response.CollectedClientData.Challenge, "passkey-register", claims.UUID)
	if err != nil {
		http.Error(w, `{"message":"Invalid or expired challenge"}`, http.StatusUnauthorized)
		return
	}
	credential, err := wa.CreateCredential(u, *session, parsed)
	if err != nil {
		http.Error(w, `{"message":"Passkey verification failed"}`, http.StatusUnauthorized)
		return
	}
	data, err := json.Marshal(credential)
	if err != nil {
		http.Error(w, `{"message":"Cannot save credential"}`, http.StatusInternalServerError)
		return
	}
	transports := make([]string, len(credential.Transport))
	for i, transport := range credential.Transport {
		transports[i] = string(transport)
	}
	device := "singleDevice"
	if credential.Flags.BackupEligible {
		device = "multiDevice"
	}
	pk := database.Passkey{ID: base64.RawURLEncoding.EncodeToString(credential.ID), AdminUUID: u.admin.UUID, PublicKey: credential.PublicKey, CredentialData: string(data), Counter: int64(credential.Authenticator.SignCount), DeviceType: device, BackedUp: credential.Flags.BackupState, Transports: strings.Join(transports, ","), PasskeyProvider: "Passkey"}
	if err := h.db.Create(&pk).Error; err != nil {
		http.Error(w, `{"message":"Cannot save credential"}`, http.StatusConflict)
		return
	}
	passkeyJSON(w, map[string]interface{}{"verified": true})
}

func (h *PasskeyHandler) VerifyAuthentication(w http.ResponseWriter, r *http.Request) {
	wa, err := h.configured()
	if err != nil {
		http.Error(w, `{"message":"Passkeys are disabled"}`, http.StatusForbidden)
		return
	}
	body, err := passkeyResponse(r)
	if err != nil {
		http.Error(w, `{"message":"Invalid passkey response"}`, http.StatusUnauthorized)
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(body)
	if err != nil {
		http.Error(w, `{"message":"Invalid passkey response"}`, http.StatusUnauthorized)
		return
	}
	u, session, err := h.session(parsed.Response.CollectedClientData.Challenge, "passkey-login", "")
	if err != nil {
		http.Error(w, `{"message":"Invalid or expired challenge"}`, http.StatusUnauthorized)
		return
	}
	counters := map[string]uint32{}
	for _, existing := range u.credentials {
		counters[base64.RawURLEncoding.EncodeToString(existing.ID)] = existing.Authenticator.SignCount
	}
	credential, err := wa.ValidateLogin(u, *session, parsed)
	if err != nil || credential.Authenticator.CloneWarning {
		loginAttempt(h.db, false, "Unknown", "–", httpx.ClientIP(r), r.UserAgent(), "Passkey verification failed.")
		http.Error(w, `{"message":"Passkey verification failed"}`, http.StatusUnauthorized)
		return
	}
	data, err := json.Marshal(credential)
	if err != nil {
		http.Error(w, `{"message":"Cannot update credential"}`, http.StatusInternalServerError)
		return
	}
	result := h.db.Model(&database.Passkey{}).Where("id = ? AND admin_uuid = ? AND counter = ?", base64.RawURLEncoding.EncodeToString(credential.ID), u.admin.UUID, counters[base64.RawURLEncoding.EncodeToString(credential.ID)]).Updates(map[string]interface{}{"counter": int64(credential.Authenticator.SignCount), "backed_up": credential.Flags.BackupState, "credential_data": string(data), "updated_at": time.Now().UTC()})
	if result.Error != nil || result.RowsAffected != 1 {
		http.Error(w, `{"message":"Cannot update credential"}`, http.StatusInternalServerError)
		return
	}
	token, err := GenerateToken(u.admin.UUID, u.admin.Username, u.admin.Role, h.secret, h.lifetime)
	if err != nil {
		http.Error(w, `{"message":"Cannot issue session"}`, http.StatusInternalServerError)
		return
	}
	loginAttempt(h.db, true, u.admin.Username, "", httpx.ClientIP(r), r.UserAgent(), "Passkey authentication successful.")
	passkeyJSON(w, map[string]interface{}{"accessToken": token})
}
