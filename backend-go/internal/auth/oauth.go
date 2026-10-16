package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"gowave/internal/database"
	"gowave/internal/httpx"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type OAuthHandler struct {
	db       *gorm.DB
	secret   string
	lifetime int
	client   *http.Client
}

func NewOAuthHandler(db *gorm.DB, secret string, lifetime int) *OAuthHandler {
	return &OAuthHandler{db, secret, lifetime, &http.Client{Timeout: 20 * time.Second}}
}

type providerConfig struct {
	Enabled          bool     `json:"enabled"`
	ClientID         string   `json:"clientId"`
	ClientSecret     string   `json:"clientSecret"`
	AllowedEmails    []string `json:"allowedEmails"`
	AllowedIDs       []string `json:"allowedIds"`
	FrontendDomain   string   `json:"frontendDomain"`
	PlainDomain      string   `json:"plainDomain"`
	KeycloakDomain   string   `json:"keycloakDomain"`
	Realm            string   `json:"realm"`
	WithPKCE         bool     `json:"withPkce"`
	AuthorizationURL string   `json:"authorizationUrl"`
	TokenURL         string   `json:"tokenUrl"`
}
type oauthSession struct {
	Verifier string `json:"verifier"`
	Nonce    string `json:"nonce"`
}

func oauthRandom() string {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
func (h *OAuthHandler) configuration(provider string) (providerConfig, *oauth2.Config, string, error) {
	var settings database.RemnawaveSetting
	if err := h.db.First(&settings).Error; err != nil {
		return providerConfig{}, nil, "", err
	}
	var providers map[string]providerConfig
	if err := json.Unmarshal([]byte(settings.OAuth2Settings), &providers); err != nil {
		return providerConfig{}, nil, "", err
	}
	config, ok := providers[provider]
	if !ok || !config.Enabled || !settings.IsLoginAllowed || config.ClientID == "" || config.ClientSecret == "" {
		return config, nil, "", errors.New("OAuth provider disabled or incomplete")
	}
	endpoint := oauth2.Endpoint{AuthStyle: oauth2.AuthStyleInHeader}
	issuer := ""
	scopes := []string{"openid", "profile", "email"}
	redirect := ""
	if config.FrontendDomain != "" {
		redirect = "https://" + config.FrontendDomain + "/auth/oauth2/callback/" + provider
	}
	switch provider {
	case "github":
		endpoint.AuthURL = "https://github.com/login/oauth/authorize"
		endpoint.TokenURL = "https://github.com/login/oauth/access_token"
		endpoint.AuthStyle = oauth2.AuthStyleInParams
		scopes = []string{"user:email"}
		redirect = ""
	case "yandex":
		endpoint.AuthURL = "https://oauth.yandex.ru/authorize"
		endpoint.TokenURL = "https://oauth.yandex.ru/token"
		endpoint.AuthStyle = oauth2.AuthStyleInParams
		scopes = []string{"login:email"}
		redirect = ""
	case "pocketid":
		issuer = "https://" + config.PlainDomain
		endpoint.AuthURL = issuer + "/authorize"
		endpoint.TokenURL = issuer + "/api/oidc/token"
	case "keycloak":
		issuer = "https://" + config.KeycloakDomain + "/realms/" + url.PathEscape(config.Realm)
		endpoint.AuthURL = issuer + "/protocol/openid-connect/auth"
		endpoint.TokenURL = issuer + "/protocol/openid-connect/token"
		config.WithPKCE = true
	case "generic":
		endpoint.AuthURL = config.AuthorizationURL
		endpoint.TokenURL = config.TokenURL
	case "telegram":
		issuer = "https://oauth.telegram.org"
		endpoint.AuthURL = issuer + "/auth"
		endpoint.TokenURL = issuer + "/token"
		config.WithPKCE = true
		scopes = []string{"openid", "profile", "telegram:bot_access"}
	default:
		return config, nil, "", errors.New("unknown OAuth provider")
	}
	for _, endpointURL := range []string{endpoint.AuthURL, endpoint.TokenURL} {
		u, err := url.Parse(endpointURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return config, nil, "", errors.New("OAuth requires HTTPS endpoints")
		}
	}
	return config, &oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, Endpoint: endpoint, RedirectURL: redirect, Scopes: scopes}, issuer, nil
}
func (h *OAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	config, client, _, err := h.configuration(body.Provider)
	if err != nil {
		http.Error(w, err.Error(), 403)
		return
	}
	state := oauthRandom()
	session := oauthSession{Nonce: oauthRandom()}
	options := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("nonce", session.Nonce)}
	if config.WithPKCE {
		session.Verifier = oauth2.GenerateVerifier()
		options = append(options, oauth2.S256ChallengeOption(session.Verifier))
	}
	data, _ := json.Marshal(session)
	if err := h.db.Create(&database.AuthChallenge{ID: state, Purpose: "oauth/" + body.Provider, Data: string(data), ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}).Error; err != nil {
		http.Error(w, "OAuth state unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]string{"authorizationUrl": client.AuthCodeURL(state, options...)}})
}
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		State    string `json:"state"`
		Code     string `json:"code"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.Code == "" || body.State == "" {
		http.Error(w, "invalid callback", 400)
		return
	}
	config, client, issuer, err := h.configuration(body.Provider)
	if err != nil {
		http.Error(w, "OAuth unavailable", 403)
		return
	}
	record, err := ConsumeChallenge(h.db, body.State, "oauth/"+body.Provider, "")
	if err != nil {
		http.Error(w, "invalid or expired OAuth state", 401)
		return
	}
	var session oauthSession
	if json.Unmarshal([]byte(record.Data), &session) != nil {
		http.Error(w, "invalid session", 401)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, h.client)
	var options []oauth2.AuthCodeOption
	if config.WithPKCE {
		if session.Verifier == "" {
			http.Error(w, "PKCE verifier missing", 401)
			return
		}
		options = append(options, oauth2.VerifierOption(session.Verifier))
	}
	token, err := client.Exchange(ctx, body.Code, options...)
	if err != nil {
		http.Error(w, "OAuth exchange failed", 401)
		return
	}
	identity := ""
	customClaim := false
	switch body.Provider {
	case "github":
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		err = h.fetchIdentity(ctx, "https://api.github.com/user/emails", token.AccessToken, &emails)
		if err == nil {
			for _, email := range emails {
				if email.Primary && email.Verified {
					identity = email.Email
					break
				}
			}
		}
	case "yandex":
		var profile struct {
			Email string `json:"default_email"`
		}
		err = h.fetchIdentity(ctx, "https://login.yandex.ru/info?format=json", token.AccessToken, &profile)
		identity = profile.Email
	default:
		raw, _ := token.Extra("id_token").(string)
		if issuer == "" {
			// Generic settings have no issuer field. Trust only an HTTPS issuer on the
			// configured authorization server, then verify its signed ID token.
			parts := strings.Split(raw, ".")
			if len(parts) != 3 {
				err = errors.New("missing ID token")
				break
			}
			data, e := base64.RawURLEncoding.DecodeString(parts[1])
			if e != nil {
				err = e
				break
			}
			var untrusted struct {
				Issuer string `json:"iss"`
			}
			if json.Unmarshal(data, &untrusted) != nil {
				err = errors.New("invalid ID token")
				break
			}
			parsed, e := url.Parse(untrusted.Issuer)
			expected, _ := url.Parse(client.Endpoint.AuthURL)
			if e != nil || parsed.Scheme != "https" || parsed.Host != expected.Host {
				err = errors.New("unexpected issuer")
				break
			}
			issuer = untrusted.Issuer
		}
		provider, e := oidc.NewProvider(ctx, issuer)
		if e != nil {
			err = e
			break
		}
		verified, e := provider.Verifier(&oidc.Config{ClientID: config.ClientID}).Verify(ctx, raw)
		if e != nil {
			err = e
			break
		}
		if verified.Nonce != session.Nonce {
			err = errors.New("ID token nonce mismatch")
			break
		}
		var claims struct {
			Email    string `json:"email"`
			Verified *bool  `json:"email_verified"`
			ID       string `json:"id"`
			Access   bool   `json:"remnawaveAccess"`
		}
		err = verified.Claims(&claims)
		identity = claims.Email
		customClaim = claims.Access
		if claims.Verified != nil && !*claims.Verified {
			err = errors.New("email not verified")
		}
		if body.Provider == "telegram" {
			identity = claims.ID
			customClaim = false
		}
	}
	allowed := customClaim
	list := config.AllowedEmails
	if body.Provider == "telegram" {
		list = config.AllowedIDs
	}
	for _, item := range list {
		if item == identity {
			allowed = true
		}
	}
	if err != nil || identity == "" || !allowed {
		who := identity
		if who == "" {
			who = "Unknown"
		}
		loginAttempt(h.db, false, who, "–", httpx.ClientIP(r), r.UserAgent(), "OAuth2 "+body.Provider+" identity is not allowed.")
		http.Error(w, "OAuth identity not allowed", 403)
		return
	}
	var admin database.Admin
	if h.db.Where("role='ADMIN'").Order("created_at ASC").First(&admin).Error != nil {
		http.Error(w, "administrator unavailable", 403)
		return
	}
	access, err := GenerateToken(admin.UUID, admin.Username, "ADMIN", h.secret, h.lifetime)
	if err != nil {
		http.Error(w, "login failed", 500)
		return
	}
	loginAttempt(h.db, true, identity, "", httpx.ClientIP(r), r.UserAgent(), "Logged via "+body.Provider+" OAuth2.")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]string{"accessToken": access}})
}
func (h *OAuthHandler) fetchIdentity(ctx context.Context, endpoint, token string, result interface{}) error {
	r, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("User-Agent", "Gowave")
	response, err := h.client.Do(r)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("identity lookup failed")
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result)
}
