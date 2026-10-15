package stubs

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sync"

	"remnawave-go/config"
	"remnawave-go/internal/auth"
	"remnawave-go/internal/database"
	"remnawave-go/internal/keygen"
	"remnawave-go/internal/nodes"
	"remnawave-go/internal/nodessh"
	"remnawave-go/internal/subscription"
	"remnawave-go/internal/tokens"
)

type JobResult struct {
	IsCompleted bool                   `json:"isCompleted"`
	IsFailed    bool                   `json:"isFailed"`
	Progress    map[string]interface{} `json:"progress,omitempty"`
	Result      interface{}            `json:"result"`
	CreatedAt   time.Time              `json:"-"`
}

type StubHandler struct {
	db              *gorm.DB
	keygenSvc       *keygen.Service
	appSecret       string
	jwtLifetime     int
	nodeClient      *nodes.Client
	jobs            sync.Map
	subPublicDomain string
	cfg             *config.Config
	clientSource    func() *nodes.Client
}

func NewHandler(db *gorm.DB, appSecret string, jwtLifetime int, nodeClient ...*nodes.Client) *StubHandler {
	var nc *nodes.Client
	if len(nodeClient) > 0 {
		nc = nodeClient[0]
	}
	return &StubHandler{
		db:          db,
		keygenSvc:   keygen.NewService(db),
		appSecret:   appSecret,
		jwtLifetime: jwtLifetime,
		nodeClient:  nc,
	}
}

func (s *StubHandler) SetSubPublicDomain(domain string) { s.subPublicDomain = domain }

// SetClientSource makes the handler use the node service's client, which is
// rebuilt after a restore, instead of a private copy.
func (s *StubHandler) SetClientSource(source func() *nodes.Client) { s.clientSource = source }

func (s *StubHandler) getNodeClient() *nodes.Client {
	if s.clientSource != nil {
		return s.clientSource()
	}
	if s.nodeClient != nil {
		return s.nodeClient
	}
	km, err := s.keygenSvc.GetMasterKeygen()
	if err != nil {
		return nil
	}
	client, err := nodes.NewNodeClient([]byte(km.CACert), []byte(km.ClientCert), []byte(km.ClientKey), []byte(km.PrivKey), []byte(km.PubKey))
	if err != nil {
		return nil
	}
	s.nodeClient = client
	return client
}

func formatLastSeen(v interface{}) string {
	if v == nil {
		return time.Now().UTC().Format(time.RFC3339)
	}
	switch val := v.(type) {
	case string:
		if val != "" {
			t, err := time.Parse(time.RFC3339, val)
			if err == nil {
				return t.UTC().Format(time.RFC3339)
			}
			t, err = time.Parse("2006-01-02T15:04:05.000Z", val)
			if err == nil {
				return t.UTC().Format(time.RFC3339)
			}
			return val
		}
	case float64:
		t := time.Unix(int64(val), 0)
		if val > 1e11 {
			t = time.UnixMilli(int64(val))
		}
		return t.UTC().Format(time.RFC3339)
	case int64:
		t := time.Unix(val, 0)
		if val > 1e11 {
			t = time.UnixMilli(val)
		}
		return t.UTC().Format(time.RFC3339)
	case time.Time:
		return val.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func (s *StubHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/node-plugins/torrent-blocker/stats", s.GetTorrentBlockerStats)
	r.Get("/api/node-plugins/torrent-blocker", s.GetTorrentBlockerReports)
	r.Delete("/api/node-plugins/torrent-blocker/truncate", s.TruncateTorrentBlockerReports)

	r.Get("/api/node-plugins/shared-lists/by-name", s.GetSharedListByName)
	r.Get("/api/node-plugins/shared-lists", s.GetSharedLists)
	r.Post("/api/node-plugins/shared-lists", s.CreateSharedList)
	r.Patch("/api/node-plugins/shared-lists", s.UpdateSharedList)
	r.Delete("/api/node-plugins/shared-lists", s.DeleteSharedList)
	r.Post("/api/node-plugins/shared-lists/actions/sync", s.SyncSharedLists)

	r.Get("/api/node-plugins/tags", s.GetNodePluginsTags)
	r.Patch("/api/node-plugins/tags", s.SetNodePluginsTags)
	r.Get("/api/node-plugins", s.GetNodePlugins)
	r.Post("/api/node-plugins", s.CreateNodePlugin)
	r.Get("/api/node-plugins/{uuid}", s.GetNodePluginByUuid)
	r.Patch("/api/node-plugins", s.UpdateNodePlugin)
	r.Delete("/api/node-plugins/{uuid}", s.DeleteNodePlugin)
	r.Post("/api/node-plugins/actions/clone", s.CloneNodePlugin)
	r.Post("/api/node-plugins/actions/reorder", s.ReorderNodePlugins)
	r.Post("/api/node-plugins/actions/sync", s.SyncNodePlugin)
	r.Post("/api/node-plugins/executor", s.ExecuteNodePlugin)

	r.Get("/api/node-integrations", s.GetNodeIntegrations)
	r.Post("/api/node-integrations", s.CreateNodeIntegration)
	r.Get("/api/node-integrations/{uuid}", s.GetNodeIntegration)
	r.Patch("/api/node-integrations", s.UpdateNodeIntegration)
	r.Delete("/api/node-integrations/{uuid}", s.DeleteNodeIntegration)

	r.Get("/api/keygen", s.GetKeygen)

	r.Get("/api/snippets", s.GetSnippets)
	r.Post("/api/snippets", s.CreateSnippet)
	r.Patch("/api/snippets", s.UpdateSnippet)
	r.Delete("/api/snippets", s.DeleteSnippet)
	r.Post("/api/snippets/actions/sync", s.SyncSnippets)

	r.Get("/api/passkeys", s.GetPasskeys)
	r.Patch("/api/passkeys", s.UpdatePasskey)
	r.Delete("/api/passkeys", s.DeletePasskey)
	r.Get("/api/passkeys/registration/options", s.GetPasskeyRegOptions)
	r.Post("/api/passkeys/registration/verify", s.VerifyPasskeyReg)

	r.Post("/api/node-ssh/{uuid}/ticket", s.CreateSshTicket)
	r.Post("/api/node-ssh/vault/evaluate", s.EvaluateVault)

	r.Get("/api/hwid/devices/stats", s.GetHwidStats)
	r.Get("/api/hwid/devices/top-users", s.GetHwidTopUsers)
	r.Get("/api/hwid/devices", s.GetHwidDevices)
	r.Post("/api/hwid/devices", s.CreateHwidDevice)
	r.Post("/api/hwid/devices/delete", s.DeleteHwidDevice)
	r.Post("/api/hwid/devices/delete-all", s.DeleteAllHwidDevices)
	r.Get("/api/hwid/devices/{userId}", s.GetUserHwidDevices)

	r.Post("/api/connections/by-user/{userId}", s.ConnectionsByUser)
	r.Get("/api/connections/by-user/{jobId}", s.ConnectionsByUserResult)
	r.Post("/api/connections/by-node/{nodeUuid}", s.ConnectionsByNode)
	r.Get("/api/connections/by-node/{jobId}", s.ConnectionsByNodeResult)
	r.Post("/api/connections/geocheck/{nodeUuid}", s.GeocheckByNode)
	r.Get("/api/connections/geocheck/{jobId}", s.GeocheckByNodeResult)
	r.Post("/api/connections/drop", s.DropConnections)

	r.Get("/api/metadata/node/{uuid}", s.GetNodeMetadata)
	r.Put("/api/metadata/node/{uuid}", s.UpsertNodeMetadata)
	r.Get("/api/metadata/user/{userId}", s.GetUserMetadata)
	r.Put("/api/metadata/user/{userId}", s.UpsertUserMetadata)

	r.Get("/api/subscriptions", s.GetAllSubscriptions)
	r.Get("/api/subscriptions/by-id/{userId}", s.GetSubscriptionById)
	r.Get("/api/subscriptions/by-short-uuid/{shortUuid}", s.GetSubscriptionByShortUuid)
	r.Get("/api/subscriptions/by-short-uuid/{shortUuid}/raw", s.GetRawSubscriptionByShortUuid)
	r.Get("/api/subscriptions/by-username/{username}", s.GetSubscriptionByUsername)
	r.Get("/api/subscriptions/connection-keys/{userId}", s.GetConnectionKeysByUserId)
	r.Get("/api/subscriptions/subpage-config/{shortUuid}", s.GetSubpageConfigByShortUuid)

	r.Get("/api/subscription-request-history/stats", s.GetSubHistoryStats)
	r.Get("/api/subscription-request-history", s.GetSubHistory)

	r.Post("/api/bandwidth-stats/nodes/usage", s.GetBandwidthNodesUsage)
	r.Post("/api/bandwidth-stats/nodes/users", s.GetBandwidthStatsNodeUsers)
	r.Get("/api/bandwidth-stats/nodes/{uuid}/users", s.GetBandwidthStatsNodeUsers)
	r.Get("/api/bandwidth-stats/users/{userId}", s.GetBandwidthStatsUsers)
	r.Get("/api/bandwidth-stats/nodes", s.GetBandwidthStatsNodes)
	r.Get("/api/bandwidth-stats/internal-squads/{uuid}/usage", s.GetBandwidthSquadUsage)
	r.Get("/api/bandwidth-stats/internal-squads/{squadUuid}/users/{userId}/usage", s.GetBandwidthSquadUserUsage)

	r.Get("/api/system/configuration", s.GetSystemConfiguration)
	r.Get("/api/system/stats/digest", s.GetStatsDigest)
	r.Post("/api/system/testers/srr-matcher", s.DebugSrrMatcher)
	r.Get("/api/system/tools/x25519/generate", s.GenerateX25519)
}

func formatPlugin(p *database.NodePlugin) map[string]interface{} {
	tags := []string(p.Tags)
	if tags == nil {
		tags = []string{}
	}
	var cfg interface{}
	if p.PluginConfig != "" && p.PluginConfig != "null" {
		_ = json.Unmarshal([]byte(p.PluginConfig), &cfg)
	}
	return map[string]interface{}{
		"uuid":         p.UUID,
		"viewPosition": p.ViewPosition,
		"name":         p.Name,
		"tags":         tags,
		"pluginConfig": cfg,
	}
}

func (s *StubHandler) GetNodePlugins(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var plugins []database.NodePlugin
	s.db.Order("view_position asc, created_at asc").Find(&plugins)
	res := make([]map[string]interface{}, 0, len(plugins))
	for _, p := range plugins {
		res = append(res, formatPlugin(&p))
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":       len(res),
			"nodePlugins": res,
		},
	})
}

func (s *StubHandler) GetNodePluginByUuid(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	var p database.NodePlugin
	if err := s.db.Where("uuid = ?", uuidParam).First(&p).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Plugin not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatPlugin(&p),
	})
}

func (s *StubHandler) CreateNodePlugin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		Name         string      `json:"name"`
		PluginConfig interface{} `json:"pluginConfig"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	var count int64
	s.db.Model(&database.NodePlugin{}).Count(&count)
	now := time.Now().UTC()
	cfgBytes, _ := json.Marshal(body.PluginConfig)

	p := database.NodePlugin{
		UUID:         uuid.New().String(),
		ViewPosition: int(count) + 1,
		Name:         strings.TrimSpace(body.Name),
		Tags:         database.StringArray{},
		PluginConfig: string(cfgBytes),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.db.Create(&p)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatPlugin(&p),
	})
}

func (s *StubHandler) UpdateNodePlugin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		UUID         string      `json:"uuid"`
		Name         *string     `json:"name"`
		PluginConfig interface{} `json:"pluginConfig"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	var p database.NodePlugin
	if err := s.db.Where("uuid = ?", body.UUID).First(&p).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Plugin not found"})
		return
	}
	if body.Name != nil {
		p.Name = strings.TrimSpace(*body.Name)
	}
	if body.PluginConfig != nil {
		b, _ := json.Marshal(body.PluginConfig)
		p.PluginConfig = string(b)
	}
	p.UpdatedAt = time.Now().UTC()
	s.db.Save(&p)
	s.syncPluginToNodes(p.UUID)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatPlugin(&p),
	})
}

func (s *StubHandler) DeleteNodePlugin(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	s.db.Where("uuid = ?", uuidParam).Delete(&database.NodePlugin{})
	w.WriteHeader(http.StatusNoContent)
}

func (s *StubHandler) CloneNodePlugin(w http.ResponseWriter, r *http.Request) { s.pluginClone(w, r) }

func (s *StubHandler) ReorderNodePlugins(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"items"`
		NodePlugins []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"nodePlugins"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	items := body.Items
	if len(items) == 0 {
		items = body.NodePlugins
	}

	for _, item := range items {
		if item.UUID != "" {
			s.db.Model(&database.NodePlugin{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
		}
	}

	s.GetNodePlugins(w, r)
}

func (s *StubHandler) syncPluginToNodes(id string) {
	if err := enqueuePlugin(s.db, id); err != nil {
		log.Printf("[PLUGINS] %v", err)
	}
}

func (s *StubHandler) SyncNodePlugin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		PluginUUID string `json:"pluginUuid"`
		UUID       string `json:"uuid"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	targetUUID := body.PluginUUID
	if targetUUID == "" {
		targetUUID = body.UUID
	}
	if targetUUID != "" {
		s.syncPluginToNodes(targetUUID)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"success": true}})
}

func (s *StubHandler) ExecuteNodePlugin(w http.ResponseWriter, r *http.Request) {
	s.pluginExecute(w, r)
}

func (s *StubHandler) GetNodePluginsTags(w http.ResponseWriter, r *http.Request) { s.pluginTags(w, r) }

func (s *StubHandler) SetNodePluginsTags(w http.ResponseWriter, r *http.Request) {
	s.pluginSetTags(w, r)
}

func (s *StubHandler) SyncSharedLists(w http.ResponseWriter, r *http.Request) { s.sharedListSync(w, r) }

func (s *StubHandler) GetNodeIntegrations(w http.ResponseWriter, r *http.Request) {
	s.integrationList(w, r)
}

func (s *StubHandler) GetNodeIntegration(w http.ResponseWriter, r *http.Request) {
	s.integrationGet(w, r)
}

func (s *StubHandler) CreateNodeIntegration(w http.ResponseWriter, r *http.Request) {
	s.integrationWrite(w, r)
}

func (s *StubHandler) UpdateNodeIntegration(w http.ResponseWriter, r *http.Request) {
	s.integrationWrite(w, r)
}

func (s *StubHandler) DeleteNodeIntegration(w http.ResponseWriter, r *http.Request) {
	s.integrationDelete(w, r)
}

func (s *StubHandler) GetKeygen(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	secretKey, err := s.keygenSvc.GenerateNodeSecretKey()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"message":"Failed generating node secret key: %v"}`, err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"secretKey": secretKey,
		},
	})
}

func (s *StubHandler) GetSnippets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var snippets []database.ConfigProfileSnippet
	s.db.Find(&snippets)
	res := make([]map[string]interface{}, 0, len(snippets))
	for _, sn := range snippets {
		var content interface{}
		_ = json.Unmarshal([]byte(sn.Snippet), &content)
		res = append(res, map[string]interface{}{
			"name":    sn.Name,
			"snippet": content,
		})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":    len(res),
			"snippets": res,
		},
	})
}

func (s *StubHandler) CreateSnippet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		Name    string      `json:"name"`
		Snippet interface{} `json:"snippet"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	b, _ := json.Marshal(body.Snippet)
	sn := database.ConfigProfileSnippet{
		Name:      strings.TrimSpace(body.Name),
		Snippet:   string(b),
		CreatedAt: time.Now().UTC(),
	}
	s.db.Save(&sn)
	w.WriteHeader(http.StatusCreated)
	s.GetSnippets(w, r)
}

func (s *StubHandler) UpdateSnippet(w http.ResponseWriter, r *http.Request) {
	s.CreateSnippet(w, r)
}

func (s *StubHandler) DeleteSnippet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.db.Where("name = ?", body.Name).Delete(&database.ConfigProfileSnippet{})
	w.WriteHeader(http.StatusNoContent)
}

func (s *StubHandler) SyncSnippets(w http.ResponseWriter, r *http.Request) { s.snippetSync(w, r) }

func (s *StubHandler) GetApiTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var tokens []database.ApiToken
	s.db.Find(&tokens)
	res := make([]map[string]interface{}, 0, len(tokens))
	for _, t := range tokens {
		scopes := []string(t.Scopes)
		if len(scopes) == 0 {
			scopes = []string{"*"}
		}
		res = append(res, map[string]interface{}{
			"uuid":      t.UUID,
			"name":      t.Name,
			"expireAt":  t.ExpireAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"scopes":    scopes,
			"createdAt": t.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"updatedAt": t.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tokens": res,
		},
	})
}

func (s *StubHandler) CreateApiToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		Name     string    `json:"name"`
		ExpireAt time.Time `json:"expireAt"`
		Scopes   []string  `json:"scopes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	tokBytes := make([]byte, 32)
	rand.Read(tokBytes)
	tokenStr := fmt.Sprintf("rw_%s", base64.RawURLEncoding.EncodeToString(tokBytes))
	now := time.Now().UTC()

	scopes := body.Scopes
	if len(scopes) == 0 {
		scopes = []string{"*"}
	}

	t := database.ApiToken{
		UUID:      uuid.New().String(),
		Name:      strings.TrimSpace(body.Name),
		ExpireAt:  body.ExpireAt,
		Scopes:    scopes,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.Create(&t).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"uuid":      t.UUID,
			"name":      t.Name,
			"expireAt":  t.ExpireAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"scopes":    t.Scopes,
			"createdAt": t.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"updatedAt": t.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"token":     tokenStr,
		},
	})
}

func (s *StubHandler) DeleteApiToken(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	s.db.Where("uuid = ?", uuidParam).Delete(&database.ApiToken{})
	w.WriteHeader(http.StatusNoContent)
}

func (s *StubHandler) GetPasskeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var passkeys []database.Passkey
	s.db.Order("created_at desc").Find(&passkeys)
	list := make([]map[string]interface{}, len(passkeys))
	for i, pk := range passkeys {
		name := pk.PasskeyProvider
		if name == "" {
			name = "Passkey"
		}
		list[i] = map[string]interface{}{
			"id":         pk.ID,
			"name":       name,
			"createdAt":  pk.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			"lastUsedAt": pk.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"passkeys": list,
		},
	})
}

func (s *StubHandler) UpdatePasskey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.ID != "" && body.Name != "" {
		s.db.Model(&database.Passkey{}).Where("id = ?", body.ID).Updates(map[string]interface{}{
			"passkey_provider": body.Name,
			"updated_at":       time.Now(),
		})
	}
	s.GetPasskeys(w, r)
}

func (s *StubHandler) DeletePasskey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.ID != "" {
		s.db.Where("id = ?", body.ID).Delete(&database.Passkey{})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *StubHandler) GetPasskeyRegOptions(w http.ResponseWriter, r *http.Request) {
	auth.NewPasskeyHandler(s.db, s.appSecret, s.jwtLifetime).RegistrationOptions(w, r)
}

func (s *StubHandler) VerifyPasskeyReg(w http.ResponseWriter, r *http.Request) {
	auth.NewPasskeyHandler(s.db, s.appSecret, s.jwtLifetime).VerifyRegistration(w, r)
}

func (s *StubHandler) GetPasskeyAuthOptions(w http.ResponseWriter, r *http.Request) {
	auth.NewPasskeyHandler(s.db, s.appSecret, s.jwtLifetime).AuthenticationOptions(w, r)
}

func (s *StubHandler) VerifyPasskeyAuth(w http.ResponseWriter, r *http.Request) {
	auth.NewPasskeyHandler(s.db, s.appSecret, s.jwtLifetime).VerifyAuthentication(w, r)
}

func (s *StubHandler) GetOtt(w http.ResponseWriter, r *http.Request) {
	tokens.NewHandler(s.db, s.appSecret).GetOtt(w, r)
}

func (s *StubHandler) CreateSshTicket(w http.ResponseWriter, r *http.Request) {
	nodessh.NewHandler(s.db, s.appSecret).Ticket(w, r)
}

func (s *StubHandler) EvaluateVault(w http.ResponseWriter, r *http.Request) {
	nodessh.NewHandler(s.db, s.appSecret).EvaluateVault(w, r)
}

func (s *StubHandler) OAuth2TelegramCallback(w http.ResponseWriter, r *http.Request) {
	auth.NewOAuthHandler(s.db, s.appSecret, s.jwtLifetime).Callback(w, r)
}

func (s *StubHandler) OAuth2Authorize(w http.ResponseWriter, r *http.Request) {
	auth.NewOAuthHandler(s.db, s.appSecret, s.jwtLifetime).Authorize(w, r)
}

func (s *StubHandler) OAuth2Callback(w http.ResponseWriter, r *http.Request) {
	auth.NewOAuthHandler(s.db, s.appSecret, s.jwtLifetime).Callback(w, r)
}

func (s *StubHandler) GetHwidStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var totalHwidDevices int64
	var totalUniqueDevices int64
	var totalUsers int64
	s.db.Model(&database.HwidDevice{}).Count(&totalHwidDevices)
	s.db.Model(&database.HwidDevice{}).Distinct("hwid").Count(&totalUniqueDevices)
	s.db.Model(&database.HwidDevice{}).Distinct("user_id").Count(&totalUsers)

	avg := 0.0
	if totalUsers > 0 {
		avg = math.Round((float64(totalHwidDevices)/float64(totalUsers))*100) / 100
	}

	type AppCount struct {
		App   string `json:"app"`
		Count int    `json:"count"`
	}
	type PlatformGroup struct {
		Platform string     `json:"platform"`
		Count    int        `json:"count"`
		ByApp    []AppCount `json:"byApp"`
	}

	var devRows []struct {
		Platform  *string `gorm:"column:platform"`
		UserAgent *string `gorm:"column:user_agent"`
	}
	s.db.Model(&database.HwidDevice{}).Where("platform IS NOT NULL").Select("platform, user_agent").Find(&devRows)

	type platAppKey struct {
		plat string
		app  string
	}
	counts := make(map[platAppKey]int)
	platTotals := make(map[string]int)

	for _, d := range devRows {
		if d.Platform == nil || *d.Platform == "" {
			continue
		}
		plat := *d.Platform
		app := "Unknown"
		if d.UserAgent != nil && *d.UserAgent != "" {
			ua := *d.UserAgent
			if strings.HasPrefix(ua, "https:") {
				continue
			}
			parts := strings.Split(ua, "/")
			if len(parts) > 0 && parts[0] != "" {
				app = parts[0]
			}
		}
		k := platAppKey{plat: plat, app: app}
		counts[k]++
		platTotals[plat]++
	}

	platMap := make(map[string]*PlatformGroup)
	for k, cnt := range counts {
		pg, ok := platMap[k.plat]
		if !ok {
			pg = &PlatformGroup{Platform: k.plat, Count: platTotals[k.plat], ByApp: []AppCount{}}
			platMap[k.plat] = pg
		}
		pg.ByApp = append(pg.ByApp, AppCount{App: k.app, Count: cnt})
	}

	byPlat := make([]PlatformGroup, 0, len(platMap))
	for _, pg := range platMap {
		sort.Slice(pg.ByApp, func(i, j int) bool {
			return pg.ByApp[i].Count > pg.ByApp[j].Count
		})
		byPlat = append(byPlat, *pg)
	}
	sort.Slice(byPlat, func(i, j int) bool {
		return byPlat[i].Count > byPlat[j].Count
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"byPlatform": byPlat,
			"stats": map[string]interface{}{
				"totalUniqueDevices":        totalUniqueDevices,
				"totalHwidDevices":          totalHwidDevices,
				"averageHwidDevicesPerUser": avg,
			},
		},
	})
}

func (s *StubHandler) GetHwidTopUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	}
	if size <= 0 {
		size = 10
	}
	type TopUserRow struct {
		ID           uint64 `json:"id"`
		Username     string `json:"username"`
		DevicesCount int    `json:"devicesCount"`
	}
	var topUsers []TopUserRow
	s.db.Raw(`
SELECT
    users.id as id,
    users.username as username,
    COUNT(hwid_user_devices.hwid) as devices_count
FROM hwid_user_devices
JOIN users ON users.id = hwid_user_devices.user_id
GROUP BY users.id, users.username
ORDER BY devices_count DESC, users.id ASC
LIMIT ? OFFSET ?;
`, size, start).Scan(&topUsers)
	if topUsers == nil {
		topUsers = []TopUserRow{}
	}

	var total int64
	s.db.Raw(`SELECT COUNT(DISTINCT user_id) FROM hwid_user_devices`).Scan(&total)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total": total,
			"users": topUsers,
		},
	})
}

func (s *StubHandler) GetHwidDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size = 50
	}

	query := s.db.Model(&database.HwidDevice{})
	if filtersStr := r.URL.Query().Get("filters"); filtersStr != "" {
		var filters []struct {
			ID    string      `json:"id"`
			Value interface{} `json:"value"`
		}
		if err := json.Unmarshal([]byte(filtersStr), &filters); err == nil {
			for _, f := range filters {
				if f.ID == "userId" {
					query = query.Where("user_id = ?", f.Value)
				} else if f.ID == "platform" {
					query = query.Where("LOWER(platform) LIKE LOWER(?)", fmt.Sprintf("%%%v%%", f.Value))
				} else if f.ID == "hwid" {
					query = query.Where("LOWER(hwid) LIKE LOWER(?)", fmt.Sprintf("%%%v%%", f.Value))
				} else if f.ID == "requestIp" {
					query = query.Where("LOWER(request_ip) LIKE LOWER(?)", fmt.Sprintf("%%%v%%", f.Value))
				}
			}
		}
	}

	var total int64
	query.Count(&total)

	var devices []database.HwidDevice
	query.Order("created_at desc").Offset(start).Limit(size).Find(&devices)
	if devices == nil {
		devices = []database.HwidDevice{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":   total,
			"devices": devices,
		},
	})
}

func (s *StubHandler) GetUserHwidDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIdStr := chi.URLParam(r, "userId")
	userId, err := strconv.ParseUint(userIdStr, 10, 64)
	if err != nil {
		var user database.User
		if err2 := s.db.Where("short_uuid = ? OR username = ?", userIdStr, userIdStr).First(&user).Error; err2 == nil {
			userId = user.ID
		}
	}
	var devices []database.HwidDevice
	s.db.Where("user_id = ?", userId).Order("created_at desc").Find(&devices)
	if devices == nil {
		devices = []database.HwidDevice{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":   len(devices),
			"devices": devices,
		},
	})
}

func (s *StubHandler) ConnectionsByUser(w http.ResponseWriter, r *http.Request) {
	userIdStr := chi.URLParam(r, "userId")
	userIdInt, _ := strconv.Atoi(userIdStr)
	jobId := uuid.New().String()

	s.jobs.Store(jobId, &JobResult{
		IsCompleted: false,
		IsFailed:    false,
		Progress: map[string]interface{}{
			"total":     0,
			"completed": 0,
			"percent":   0,
		},
		CreatedAt: time.Now(),
	})

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				s.jobs.Store(jobId, &JobResult{
					IsCompleted: true,
					IsFailed:    true,
					CreatedAt:   time.Now(),
				})
			}
		}()

		var nodeList []database.Node
		s.db.Where("is_connected = ? AND is_disabled = ?", true, false).Find(&nodeList)

		client := s.getNodeClient()
		type NodeUserConn struct {
			NodeUUID    string                   `json:"nodeUuid"`
			NodeName    string                   `json:"nodeName"`
			CountryCode string                   `json:"countryCode"`
			IPs         []map[string]interface{} `json:"ips"`
		}
		userNodes := make([]NodeUserConn, 0)

		if client != nil && len(nodeList) > 0 {
			for _, n := range nodeList {
				users, err := client.GetUsersIpList(&n)
				if err == nil {
					for _, u := range users {
						if u.UserID == userIdInt && len(u.IPs) > 0 {
							ipsList := make([]map[string]interface{}, 0, len(u.IPs))
							for _, ipInfo := range u.IPs {
								ipsList = append(ipsList, map[string]interface{}{
									"ip":       ipInfo.IP,
									"lastSeen": formatLastSeen(ipInfo.LastSeen),
								})
							}
							userNodes = append(userNodes, NodeUserConn{
								NodeUUID:    n.UUID,
								NodeName:    n.Name,
								CountryCode: n.CountryCode,
								IPs:         ipsList,
							})
						}
					}
				}
			}
		}

		s.jobs.Store(jobId, &JobResult{
			IsCompleted: true,
			IsFailed:    false,
			Progress: map[string]interface{}{
				"total":     len(nodeList),
				"completed": len(nodeList),
				"percent":   100,
			},
			Result: map[string]interface{}{
				"success": true,
				"userId":  userIdInt,
				"nodes":   userNodes,
			},
			CreatedAt: time.Now(),
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"jobId": jobId,
		},
	})
}

func (s *StubHandler) ConnectionsByUserResult(w http.ResponseWriter, r *http.Request) {
	jobId := chi.URLParam(r, "jobId")
	w.Header().Set("Content-Type", "application/json")

	val, ok := s.jobs.Load(jobId)
	if !ok {
		userIdInt, _ := strconv.Atoi(chi.URLParam(r, "userId"))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"isCompleted": true,
				"isFailed":    false,
				"progress": map[string]interface{}{
					"total":     1,
					"completed": 1,
					"percent":   100,
				},
				"result": map[string]interface{}{
					"success": true,
					"userId":  userIdInt,
					"nodes":   []interface{}{},
				},
			},
		})
		return
	}

	job := val.(*JobResult)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": job,
	})
}

func (s *StubHandler) ConnectionsByNode(w http.ResponseWriter, r *http.Request) {
	nodeUuid := chi.URLParam(r, "nodeUuid")
	jobId := uuid.New().String()

	s.jobs.Store(jobId, &JobResult{
		IsCompleted: false,
		IsFailed:    false,
		CreatedAt:   time.Now(),
	})

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				s.jobs.Store(jobId, &JobResult{
					IsCompleted: true,
					IsFailed:    true,
					Result: map[string]interface{}{
						"success":  false,
						"nodeUuid": nodeUuid,
						"users":    []interface{}{},
					},
					CreatedAt: time.Now(),
				})
			}
		}()

		var node database.Node
		if err := s.db.Where("uuid = ?", nodeUuid).First(&node).Error; err != nil {
			s.jobs.Store(jobId, &JobResult{
				IsCompleted: true,
				IsFailed:    true,
				Result: map[string]interface{}{
					"success":  false,
					"nodeUuid": nodeUuid,
					"users":    []interface{}{},
				},
				CreatedAt: time.Now(),
			})
			return
		}

		client := s.getNodeClient()
		if client == nil {
			s.jobs.Store(jobId, &JobResult{
				IsCompleted: true,
				IsFailed:    false,
				Result: map[string]interface{}{
					"success":  true,
					"nodeUuid": nodeUuid,
					"users":    []interface{}{},
				},
				CreatedAt: time.Now(),
			})
			return
		}

		rawUsers, err := client.GetUsersIpList(&node)
		if err != nil {
			s.jobs.Store(jobId, &JobResult{
				IsCompleted: true,
				IsFailed:    false,
				Result: map[string]interface{}{
					"success":  false,
					"nodeUuid": nodeUuid,
					"users":    []interface{}{},
				},
				CreatedAt: time.Now(),
			})
			return
		}

		type NodeUserFormatted struct {
			UserID int                      `json:"userId"`
			IPs    []map[string]interface{} `json:"ips"`
		}
		formattedUsers := make([]NodeUserFormatted, 0, len(rawUsers))
		for _, u := range rawUsers {
			ipsList := make([]map[string]interface{}, 0, len(u.IPs))
			for _, ipInfo := range u.IPs {
				ipsList = append(ipsList, map[string]interface{}{
					"ip":       ipInfo.IP,
					"lastSeen": formatLastSeen(ipInfo.LastSeen),
				})
			}
			formattedUsers = append(formattedUsers, NodeUserFormatted{
				UserID: u.UserID,
				IPs:    ipsList,
			})
		}

		s.jobs.Store(jobId, &JobResult{
			IsCompleted: true,
			IsFailed:    false,
			Result: map[string]interface{}{
				"success":  true,
				"nodeUuid": nodeUuid,
				"users":    formattedUsers,
			},
			CreatedAt: time.Now(),
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"jobId": jobId,
		},
	})
}

func (s *StubHandler) ConnectionsByNodeResult(w http.ResponseWriter, r *http.Request) {
	jobId := chi.URLParam(r, "jobId")
	w.Header().Set("Content-Type", "application/json")

	val, ok := s.jobs.Load(jobId)
	if !ok {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"isCompleted": true,
				"isFailed":    false,
				"result": map[string]interface{}{
					"success":  true,
					"nodeUuid": jobId,
					"users":    []interface{}{},
				},
			},
		})
		return
	}

	job := val.(*JobResult)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": job,
	})
}

func (s *StubHandler) GeocheckByNode(w http.ResponseWriter, r *http.Request) { s.geocheck(w, r) }

func (s *StubHandler) GeocheckByNodeResult(w http.ResponseWriter, r *http.Request) {
	s.geocheckResult(w, r)
}

func (s *StubHandler) DropConnections(w http.ResponseWriter, r *http.Request) {
	s.dropConnections(w, r)
}

func (s *StubHandler) GetNodeMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	var res map[string]interface{}
	var node database.Node
	if err := s.db.Select("id").Where("uuid = ?", uuidParam).First(&node).Error; err == nil {
		var meta database.NodeMeta
		if err := s.db.Where("node_id = ?", node.ID).First(&meta).Error; err == nil {
			_ = json.Unmarshal([]byte(meta.Metadata), &res)
		}
	}
	if res == nil {
		var legacy database.EntityMeta
		if err := s.db.Where("entity_id = ? AND entity_type = 'node'", uuidParam).First(&legacy).Error; err == nil {
			_ = json.Unmarshal([]byte(legacy.Metadata), &res)
		}
	}
	if res == nil {
		res = map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"metadata": res,
		},
	})
}

func (s *StubHandler) UpsertNodeMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	var body struct {
		Metadata interface{} `json:"metadata"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	b, _ := json.Marshal(body.Metadata)
	var node database.Node
	if err := s.db.Select("id").Where("uuid = ?", uuidParam).First(&node).Error; err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	meta := database.NodeMeta{NodeID: node.ID, Metadata: string(b)}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"metadata"}),
	}).Create(&meta).Error; err != nil {
		http.Error(w, "failed to save node metadata", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"metadata": body.Metadata,
		},
	})
}

func (s *StubHandler) GetUserMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userId := chi.URLParam(r, "userId")
	var res map[string]interface{}
	if id, err := strconv.ParseUint(userId, 10, 64); err == nil {
		var meta database.UserMeta
		if err := s.db.Where("user_id = ?", id).First(&meta).Error; err == nil {
			_ = json.Unmarshal([]byte(meta.Metadata), &res)
		}
	}
	if res == nil {
		var legacy database.EntityMeta
		if err := s.db.Where("entity_id = ? AND entity_type = 'user'", userId).First(&legacy).Error; err == nil {
			_ = json.Unmarshal([]byte(legacy.Metadata), &res)
		}
	}
	if res == nil {
		res = map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"metadata": res,
		},
	})
}

func (s *StubHandler) UpsertUserMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userId := chi.URLParam(r, "userId")
	var body struct {
		Metadata interface{} `json:"metadata"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	b, _ := json.Marshal(body.Metadata)
	parsedID, err := strconv.ParseUint(userId, 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	meta := database.UserMeta{UserID: parsedID, Metadata: string(b)}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"metadata"}),
	}).Create(&meta).Error; err != nil {
		http.Error(w, "failed to save user metadata", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"metadata": body.Metadata,
		},
	})
}

func (s *StubHandler) subscriptions() *subscription.Handler {
	h := subscription.NewHandler(s.db, s.subPublicDomain)
	if s.cfg != nil {
		h.SetDisableHistory(s.cfg.Service.DisableSrhRecords)
	}
	return h
}

func (s *StubHandler) GetAllSubscriptions(w http.ResponseWriter, r *http.Request) {
	s.subscriptions().ListProtected(w, r)
}

func (s *StubHandler) GetSubscriptionById(w http.ResponseWriter, r *http.Request) {
	s.subscriptions().InfoProtected(w, r)
}

func (s *StubHandler) GetSubscriptionByShortUuid(w http.ResponseWriter, r *http.Request) {
	s.subscriptions().InfoProtected(w, r)
}

func (s *StubHandler) GetRawSubscriptionByShortUuid(w http.ResponseWriter, r *http.Request) {
	s.subscriptions().RawProtected(w, r)
}

func (s *StubHandler) GetSubscriptionByUsername(w http.ResponseWriter, r *http.Request) {
	s.subscriptions().InfoProtected(w, r)
}

func (s *StubHandler) GetConnectionKeysByUserId(w http.ResponseWriter, r *http.Request) {
	s.subscriptions().ConnectionKeys(w, r)
}

func (s *StubHandler) GetSubpageConfigByShortUuid(w http.ResponseWriter, r *http.Request) {
	s.subscriptions().SubpageConfig(w, r)
}

func (s *StubHandler) GetSubHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size = 50
	}
	query := s.db.Model(&database.UserSubscriptionRequestHistory{})

	if filtersStr := r.URL.Query().Get("filters"); filtersStr != "" {
		var filters []struct {
			ID    string      `json:"id"`
			Value interface{} `json:"value"`
		}
		if err := json.Unmarshal([]byte(filtersStr), &filters); err == nil {
			for _, f := range filters {
				if f.ID == "userId" {
					query = query.Where("user_id = ?", f.Value)
				} else if f.ID == "requestIp" {
					query = query.Where("LOWER(request_ip) LIKE LOWER(?)", fmt.Sprintf("%%%v%%", f.Value))
				} else if f.ID == "userAgent" {
					query = query.Where("LOWER(user_agent) LIKE LOWER(?)", fmt.Sprintf("%%%v%%", f.Value))
				} else if f.ID == "srrRuleName" {
					query = query.Where("LOWER(srr_rule_name) LIKE LOWER(?)", fmt.Sprintf("%%%v%%", f.Value))
				} else if f.ID == "srrResponseType" {
					query = query.Where("srr_response_type = ?", f.Value)
				}
			}
		}
	}

	var total int64
	query.Count(&total)

	var records []database.UserSubscriptionRequestHistory
	query.Order("request_at desc").Offset(start).Limit(size).Find(&records)
	if records == nil {
		records = []database.UserSubscriptionRequestHistory{}
	}

	type RecordJSON struct {
		ID              uint64    `json:"id"`
		UserID          uint64    `json:"userId"`
		RequestAt       time.Time `json:"requestAt"`
		RequestIP       *string   `json:"requestIp"`
		UserAgent       *string   `json:"userAgent"`
		SrrRuleName     *string   `json:"srrRuleName"`
		SrrResponseType string    `json:"srrResponseType"`
	}

	result := make([]RecordJSON, len(records))
	for i, rec := range records {
		result[i] = RecordJSON{
			ID:              rec.ID,
			UserID:          rec.UserID,
			RequestAt:       rec.RequestAt,
			RequestIP:       rec.RequestIP,
			UserAgent:       rec.UserAgent,
			SrrRuleName:     rec.SrrRuleName,
			SrrResponseType: rec.SrrResponseType,
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":   total,
			"records": result,
		},
	})
}

func (s *StubHandler) GetSubHistoryStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	type AppStat struct {
		App   string `json:"app"`
		Count int    `json:"count"`
	}
	type userAgentCount struct {
		UserAgent string `gorm:"column:user_agent"`
		Count     int    `gorm:"column:count"`
	}
	var userAgents []userAgentCount
	if err := s.db.Model(&database.UserSubscriptionRequestHistory{}).
		Select("user_agent, COUNT(id) AS count").
		Where("user_agent IS NOT NULL").
		Group("user_agent").Scan(&userAgents).Error; err != nil {
		http.Error(w, "failed to load subscription history statistics", http.StatusInternalServerError)
		return
	}
	agentCounts := make(map[string]int)
	for _, row := range userAgents {
		app := row.UserAgent
		if slash := strings.IndexByte(app, '/'); slash >= 0 {
			app = app[:slash]
		} else if space := strings.IndexByte(app, ' '); space >= 0 {
			app = app[:space]
		}
		agentCounts[app] += row.Count
	}
	byParsedApp := make([]AppStat, 0, len(agentCounts))
	for app, count := range agentCounts {
		byParsedApp = append(byParsedApp, AppStat{App: app, Count: count})
	}
	sort.Slice(byParsedApp, func(i, j int) bool {
		if byParsedApp[i].Count == byParsedApp[j].Count {
			return byParsedApp[i].App < byParsedApp[j].App
		}
		return byParsedApp[i].Count > byParsedApp[j].Count
	})
	if len(byParsedApp) > 50 {
		byParsedApp = byParsedApp[:50]
	}

	type HourlyStat struct {
		DateTime     time.Time `json:"dateTime"`
		RequestCount int       `json:"requestCount"`
	}
	type hourlyRow struct {
		DateTime string `gorm:"column:date_time"`
		Count    int    `gorm:"column:request_count"`
	}
	dateExpression := "to_char(date_trunc('hour', request_at), 'YYYY-MM-DD HH24:MI:SS')"
	if s.db.Dialector.Name() == "sqlite" {
		dateExpression = "strftime('%Y-%m-%d %H:00:00', request_at)"
	}
	var hourlyRows []hourlyRow
	if err := s.db.Model(&database.UserSubscriptionRequestHistory{}).
		Select(dateExpression+" AS date_time, COUNT(id) AS request_count").
		Where("request_at >= ?", time.Now().UTC().Add(-48*time.Hour)).
		Group("date_time").Order("date_time ASC").Scan(&hourlyRows).Error; err != nil {
		http.Error(w, "failed to load hourly subscription statistics", http.StatusInternalServerError)
		return
	}
	hourlyRequestStats := make([]HourlyStat, 0, len(hourlyRows))
	for _, row := range hourlyRows {
		dateTime, err := time.ParseInLocation("2006-01-02 15:04:05", row.DateTime, time.UTC)
		if err != nil {
			continue
		}
		hourlyRequestStats = append(hourlyRequestStats, HourlyStat{DateTime: dateTime, RequestCount: row.Count})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"byParsedApp":        byParsedApp,
			"hourlyRequestStats": hourlyRequestStats,
		},
	})
}

func colorFromUUID(id string) string {
	var hash uint32
	for i := 0; i < len(id); i++ {
		hash = (hash << 5) - hash + uint32(id[i])
	}
	r := 64 + ((hash >> 16) & 0x7F)
	g := 64 + ((hash >> 8) & 0x7F)
	b := 64 + (hash & 0x7F)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func colorFromID(id uint64) string {
	return colorFromUUID(strconv.FormatUint(id, 10))
}

func parseDateRange(startStr, endStr string) (time.Time, time.Time, []string) {
	now := time.Now().UTC()
	defEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	defStart := defEnd.AddDate(0, 0, -6)

	startDate, err1 := time.Parse("2006-01-02", startStr)
	endDate, err2 := time.Parse("2006-01-02", endStr)
	if err1 != nil || err2 != nil || startDate.After(endDate) {
		startDate = defStart
		endDate = defEnd
	}

	var categories []string
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		categories = append(categories, d.Format("2006-01-02"))
	}
	return startDate, endDate, categories
}

func (s *StubHandler) GetBandwidthStatsNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	topNodesLimit, _ := strconv.Atoi(r.URL.Query().Get("topNodesLimit"))
	if topNodesLimit <= 0 {
		topNodesLimit = 10
	}

	startDate, endDate, categories := parseDateRange(startStr, endStr)
	endExclusive := endDate.AddDate(0, 0, 1)

	type DailyRow struct {
		Date  string
		Total int64
	}
	var dailyRows []DailyRow
	s.db.Raw(`
SELECT
    CAST(DATE(created_at) AS TEXT) AS date,
    COALESCE(SUM(total_bytes), 0) AS total
FROM nodes_usage_history
WHERE created_at >= ? AND created_at < ?
GROUP BY DATE(created_at)
`, startDate, endExclusive).Scan(&dailyRows)

	dateMap := make(map[string]int64)
	for _, dr := range dailyRows {
		dateMap[dr.Date] = dr.Total
	}
	sparklineData := make([]int64, len(categories))
	for i, cat := range categories {
		sparklineData[i] = dateMap[cat]
	}

	type TopNodeDBRow struct {
		UUID        string
		Name        string
		CountryCode string
		Total       int64
	}
	var topNodeRows []TopNodeDBRow
	s.db.Raw(`
SELECT
    n.uuid,
    n.name,
    n.country_code,
    COALESCE(SUM(nuh.total_bytes), 0) AS total
FROM nodes n
JOIN nodes_usage_history nuh ON nuh.node_uuid = n.uuid
WHERE nuh.created_at >= ? AND nuh.created_at < ?
GROUP BY n.uuid, n.name, n.country_code
ORDER BY total DESC
LIMIT ?
`, startDate, endExclusive, topNodesLimit).Scan(&topNodeRows)

	type TopNodeItem struct {
		UUID        string `json:"uuid"`
		Name        string `json:"name"`
		Color       string `json:"color"`
		CountryCode string `json:"countryCode"`
		Total       int64  `json:"total"`
	}
	type SeriesItem struct {
		UUID        string  `json:"uuid"`
		Name        string  `json:"name"`
		Color       string  `json:"color"`
		CountryCode string  `json:"countryCode"`
		Total       int64   `json:"total"`
		Data        []int64 `json:"data"`
	}

	topNodes := make([]TopNodeItem, 0, len(topNodeRows))
	series := make([]SeriesItem, 0, len(topNodeRows))

	if len(topNodeRows) > 0 {
		var nodeUuids []string
		for _, tn := range topNodeRows {
			nodeUuids = append(nodeUuids, tn.UUID)
		}

		type NodeDailyRow struct {
			NodeUUID string
			Date     string
			Total    int64
		}
		var nodeDailyRows []NodeDailyRow
		s.db.Raw(`
SELECT
    nuh.node_uuid,
    CAST(DATE(nuh.created_at) AS TEXT) AS date,
    COALESCE(SUM(nuh.total_bytes), 0) AS total
FROM nodes_usage_history nuh
WHERE nuh.node_uuid IN (?)
  AND nuh.created_at >= ?
  AND nuh.created_at < ?
GROUP BY nuh.node_uuid, DATE(nuh.created_at)
`, nodeUuids, startDate, endExclusive).Scan(&nodeDailyRows)

		nodeDateMap := make(map[string]map[string]int64)
		for _, ndr := range nodeDailyRows {
			if nodeDateMap[ndr.NodeUUID] == nil {
				nodeDateMap[ndr.NodeUUID] = make(map[string]int64)
			}
			nodeDateMap[ndr.NodeUUID][ndr.Date] = ndr.Total
		}

		for _, tn := range topNodeRows {
			color := colorFromUUID(tn.UUID)
			topNodes = append(topNodes, TopNodeItem{
				UUID:        tn.UUID,
				Name:        tn.Name,
				Color:       color,
				CountryCode: tn.CountryCode,
				Total:       tn.Total,
			})

			nodeData := make([]int64, len(categories))
			for i, cat := range categories {
				if nodeDateMap[tn.UUID] != nil {
					nodeData[i] = nodeDateMap[tn.UUID][cat]
				}
			}

			series = append(series, SeriesItem{
				UUID:        tn.UUID,
				Name:        tn.Name,
				Color:       color,
				CountryCode: tn.CountryCode,
				Total:       tn.Total,
				Data:        nodeData,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"categories":    categories,
			"sparklineData": sparklineData,
			"topNodes":      topNodes,
			"series":        series,
		},
	})
}

func (s *StubHandler) GetBandwidthStatsUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userIdStr := chi.URLParam(r, "userId")
	userId, err := strconv.ParseUint(userIdStr, 10, 64)
	if err != nil {
		var user database.User
		if err2 := s.db.Where("short_uuid = ? OR username = ?", userIdStr, userIdStr).First(&user).Error; err2 == nil {
			userId = user.ID
		}
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	topNodesLimit, _ := strconv.Atoi(r.URL.Query().Get("topNodesLimit"))
	if topNodesLimit <= 0 {
		topNodesLimit = 10
	}

	startDate, endDate, categories := parseDateRange(startStr, endStr)
	endExclusive := endDate.AddDate(0, 0, 1)

	type DailyRow struct {
		Date  string
		Total int64
	}
	var dailyRows []DailyRow
	s.db.Raw(`
SELECT
    CAST(DATE(created_at) AS TEXT) AS date,
    COALESCE(SUM(total_bytes), 0) AS total
FROM nodes_user_usage_history
WHERE user_id = ?
  AND created_at >= ?
  AND created_at < ?
GROUP BY created_at
`, userId, startDate.Format("2006-01-02"), endExclusive.Format("2006-01-02")).Scan(&dailyRows)

	dateMap := make(map[string]int64)
	for _, dr := range dailyRows {
		dateMap[dr.Date] = dr.Total
	}
	sparklineData := make([]int64, len(categories))
	for i, cat := range categories {
		sparklineData[i] = dateMap[cat]
	}

	type TopNodeDBRow struct {
		NodeID      uint64
		UUID        string
		Name        string
		CountryCode string
		Total       int64
	}
	var topNodeRows []TopNodeDBRow
	s.db.Raw(`
SELECT
    n.id AS node_id,
    n.uuid,
    n.name,
    n.country_code,
    COALESCE(SUM(nuh.total_bytes), 0) AS total
FROM nodes n
JOIN nodes_user_usage_history nuh ON nuh.node_id = n.id
WHERE nuh.user_id = ?
  AND nuh.created_at >= ?
  AND nuh.created_at < ?
GROUP BY n.id, n.uuid, n.name, n.country_code
ORDER BY total DESC
LIMIT ?
`, userId, startDate.Format("2006-01-02"), endExclusive.Format("2006-01-02"), topNodesLimit).Scan(&topNodeRows)

	type TopNodeItem struct {
		UUID        string `json:"uuid"`
		Name        string `json:"name"`
		Color       string `json:"color"`
		CountryCode string `json:"countryCode"`
		Total       int64  `json:"total"`
	}
	type SeriesItem struct {
		UUID        string  `json:"uuid"`
		Name        string  `json:"name"`
		Color       string  `json:"color"`
		CountryCode string  `json:"countryCode"`
		Total       int64   `json:"total"`
		Data        []int64 `json:"data"`
	}

	topNodes := make([]TopNodeItem, 0, len(topNodeRows))
	series := make([]SeriesItem, 0, len(topNodeRows))

	if len(topNodeRows) > 0 {
		var nodeIds []uint64
		for _, tn := range topNodeRows {
			nodeIds = append(nodeIds, tn.NodeID)
		}

		type NodeDailyRow struct {
			NodeID uint64
			Date   string
			Total  int64
		}
		var nodeDailyRows []NodeDailyRow
		s.db.Raw(`
SELECT
    node_id,
    CAST(DATE(created_at) AS TEXT) AS date,
    COALESCE(SUM(total_bytes), 0) AS total
FROM nodes_user_usage_history
WHERE user_id = ?
  AND node_id IN (?)
  AND created_at >= ?
  AND created_at < ?
GROUP BY node_id, created_at
`, userId, nodeIds, startDate.Format("2006-01-02"), endExclusive.Format("2006-01-02")).Scan(&nodeDailyRows)

		nodeDateMap := make(map[uint64]map[string]int64)
		for _, ndr := range nodeDailyRows {
			if nodeDateMap[ndr.NodeID] == nil {
				nodeDateMap[ndr.NodeID] = make(map[string]int64)
			}
			nodeDateMap[ndr.NodeID][ndr.Date] = ndr.Total
		}

		for _, tn := range topNodeRows {
			color := colorFromUUID(tn.UUID)
			topNodes = append(topNodes, TopNodeItem{
				UUID:        tn.UUID,
				Name:        tn.Name,
				Color:       color,
				CountryCode: tn.CountryCode,
				Total:       tn.Total,
			})

			nodeData := make([]int64, len(categories))
			for i, cat := range categories {
				if nodeDateMap[tn.NodeID] != nil {
					nodeData[i] = nodeDateMap[tn.NodeID][cat]
				}
			}

			series = append(series, SeriesItem{
				UUID:        tn.UUID,
				Name:        tn.Name,
				Color:       color,
				CountryCode: tn.CountryCode,
				Total:       tn.Total,
				Data:        nodeData,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"categories":    categories,
			"sparklineData": sparklineData,
			"topNodes":      topNodes,
			"series":        series,
		},
	})
}

func (s *StubHandler) GetBandwidthStatsNodeUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	topUsersLimit, _ := strconv.Atoi(r.URL.Query().Get("topUsersLimit"))
	if topUsersLimit <= 0 {
		topUsersLimit = 100
	}

	startDate, endDate, categories := parseDateRange(startStr, endStr)
	endExclusive := endDate.AddDate(0, 0, 1)

	var nodeUuids []string
	if uuidParam := chi.URLParam(r, "uuid"); uuidParam != "" {
		nodeUuids = append(nodeUuids, uuidParam)
	}

	if r.Method == http.MethodPost {
		var body struct {
			NodesUuids []string `json:"nodesUuids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.NodesUuids) > 0 {
			nodeUuids = append(nodeUuids, body.NodesUuids...)
		}
	}

	var nodeIds []uint64
	if len(nodeUuids) > 0 {
		s.db.Model(&database.Node{}).Where("uuid IN (?)", nodeUuids).Pluck("id", &nodeIds)
	} else {
		s.db.Model(&database.Node{}).Pluck("id", &nodeIds)
	}

	sparklineData := make([]int64, len(categories))
	type TopUserItem struct {
		Color    string `json:"color"`
		Username string `json:"username"`
		Total    int64  `json:"total"`
	}
	topUsers := make([]TopUserItem, 0)

	if len(nodeIds) > 0 {
		type DailyRow struct {
			Date  string
			Total int64
		}
		var dailyRows []DailyRow
		s.db.Raw(`
SELECT
    CAST(DATE(nuh.created_at) AS TEXT) AS date,
    COALESCE(SUM(nuh.total_bytes), 0) AS total
FROM nodes_user_usage_history nuh
WHERE nuh.node_id IN (?)
  AND nuh.created_at >= ?
  AND nuh.created_at < ?
GROUP BY nuh.created_at
`, nodeIds, startDate.Format("2006-01-02"), endExclusive.Format("2006-01-02")).Scan(&dailyRows)

		dateMap := make(map[string]int64)
		for _, dr := range dailyRows {
			dateMap[dr.Date] = dr.Total
		}
		for i, cat := range categories {
			sparklineData[i] = dateMap[cat]
		}

		type TopUserDBRow struct {
			UserID   uint64
			Username string
			Total    int64
		}
		var topUserRows []TopUserDBRow
		s.db.Raw(`
SELECT
    u.id AS user_id,
    u.username,
    COALESCE(SUM(nuh.total_bytes), 0) AS total
FROM users u
JOIN nodes_user_usage_history nuh ON nuh.user_id = u.id
WHERE nuh.node_id IN (?)
  AND nuh.created_at >= ?
  AND nuh.created_at < ?
GROUP BY u.id, u.username
ORDER BY total DESC
LIMIT ?
`, nodeIds, startDate.Format("2006-01-02"), endExclusive.Format("2006-01-02"), topUsersLimit).Scan(&topUserRows)

		for _, tu := range topUserRows {
			topUsers = append(topUsers, TopUserItem{
				Color:    colorFromID(tu.UserID),
				Username: tu.Username,
				Total:    tu.Total,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"categories":    categories,
			"sparklineData": sparklineData,
			"topUsers":      topUsers,
		},
	})
}

func (s *StubHandler) GetBandwidthNodesUsage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	minBytes, _ := strconv.ParseInt(r.URL.Query().Get("minTotalBytes"), 10, 64)

	startDate, endDate, _ := parseDateRange(startStr, endStr)
	endExclusive := endDate.AddDate(0, 0, 1)

	var body struct {
		NodesUuids []string `json:"nodesUuids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	type UsageRow struct {
		NodeUUID   string
		UserID     uint64
		TotalBytes int64
	}
	var rows []UsageRow

	query := s.db.Table("nodes_user_usage_history as nuh").
		Select("n.uuid as node_uuid, nuh.user_id, COALESCE(SUM(nuh.total_bytes), 0) as total_bytes").
		Joins("JOIN nodes n ON n.id = nuh.node_id").
		Where("nuh.created_at >= ? AND nuh.created_at < ?", startDate.Format("2006-01-02"), endExclusive.Format("2006-01-02")).
		Group("n.uuid, nuh.user_id")

	if len(body.NodesUuids) > 0 {
		query = query.Where("n.uuid IN (?)", body.NodesUuids)
	}
	if minBytes > 0 {
		query = query.Having("SUM(nuh.total_bytes) >= ?", minBytes)
	}

	query.Scan(&rows)

	type UserUsageItem struct {
		ID         uint64 `json:"id"`
		TotalBytes int64  `json:"totalBytes"`
	}
	type NodeUsageItem struct {
		UUID  string          `json:"uuid"`
		Users []UserUsageItem `json:"users"`
	}

	nodeMap := make(map[string]*NodeUsageItem)
	for _, r := range rows {
		item, ok := nodeMap[r.NodeUUID]
		if !ok {
			item = &NodeUsageItem{UUID: r.NodeUUID, Users: []UserUsageItem{}}
			nodeMap[r.NodeUUID] = item
		}
		item.Users = append(item.Users, UserUsageItem{ID: r.UserID, TotalBytes: r.TotalBytes})
	}

	result := make([]NodeUsageItem, 0, len(nodeMap))
	for _, item := range nodeMap {
		result = append(result, *item)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"nodes": result,
		},
	})
}
