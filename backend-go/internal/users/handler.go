package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"remnawave-go/internal/database"
	"remnawave-go/internal/httpx"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type Handler struct {
	service         *Service
	subPublicDomain string
}

func NewHandler(service *Service, subPublicDomain string) *Handler {
	return &Handler{
		service:         service,
		subPublicDomain: subPublicDomain,
	}
}

type UsersListResponse struct {
	Response struct {
		Users []map[string]interface{} `json:"users"`
		Total int                      `json:"total"`
	} `json:"response"`
}

type SingleUserResponse struct {
	Response interface{} `json:"response"`
}

func FormatUser(u *database.User, subPublicDomain string) map[string]interface{} {
	traffic := map[string]interface{}{
		"usedTrafficBytes":         uint64(0),
		"lifetimeUsedTrafficBytes": uint64(0),
		"onlineAt":                 nil,
		"firstConnectedAt":         nil,
		"lastConnectedNodeUuid":    nil,
	}
	if u.Traffic != nil {
		traffic["usedTrafficBytes"] = u.Traffic.UsedTrafficBytes
		traffic["lifetimeUsedTrafficBytes"] = u.Traffic.LifetimeUsedTrafficBytes
		if u.Traffic.OnlineAt != nil {
			traffic["onlineAt"] = u.Traffic.OnlineAt.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		if u.Traffic.FirstConnectedAt != nil {
			traffic["firstConnectedAt"] = u.Traffic.FirstConnectedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		traffic["lastConnectedNodeUuid"] = u.Traffic.LastConnectedNodeUUID
	}

	trimmedDomain := strings.TrimRight(subPublicDomain, "/")
	subUrl := fmt.Sprintf("https://%s/%s", trimmedDomain, u.ShortUUID)
	if strings.HasPrefix(trimmedDomain, "http://") || strings.HasPrefix(trimmedDomain, "https://") {
		subUrl = fmt.Sprintf("%s/%s", trimmedDomain, u.ShortUUID)
	}

	var lastReset *string
	if u.LastTrafficResetAt != nil {
		s := u.LastTrafficResetAt.UTC().Format("2006-01-02T15:04:05.000Z")
		lastReset = &s
	}

	var subRevoked *string
	if u.SubRevokedAt != nil {
		s := u.SubRevokedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		subRevoked = &s
	}

	var email interface{} = nil
	if u.Email != "" {
		email = u.Email
	}

	var tag interface{} = nil
	if u.Tag != "" {
		tag = u.Tag
	}

	var description interface{} = nil
	if u.Description != "" {
		description = u.Description
	}

	var extSquadUuid interface{} = nil
	if u.ExternalSquadUUID != nil && *u.ExternalSquadUUID != "" {
		extSquadUuid = *u.ExternalSquadUUID
	}

	activeSquads := make([]map[string]interface{}, 0, len(u.ActiveInternalSquads))
	for _, sq := range u.ActiveInternalSquads {
		activeSquads = append(activeSquads, map[string]interface{}{
			"uuid": sq.UUID,
			"name": sq.Name,
		})
	}

	return map[string]interface{}{
		"id":                     u.ID,
		"shortUuid":              u.ShortUUID,
		"username":               u.Username,
		"status":                 u.Status,
		"trafficLimitBytes":      u.TrafficLimitBytes,
		"trafficLimitStrategy":   u.TrafficLimitStrategy,
		"expireAt":               u.ExpireAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		"telegramId":             u.TelegramID,
		"email":                  email,
		"description":            description,
		"tag":                    tag,
		"hwidDeviceLimit":        u.HWIDDeviceLimit,
		"externalSquadUuid":      extSquadUuid,
		"trojanPassword":         u.TrojanPassword,
		"vlessUuid":              u.VlessUUID,
		"ssPassword":             u.SsPassword,
		"lastTriggeredThreshold": u.LastTriggeredThreshold,
		"subRevokedAt":           subRevoked,
		"lastTrafficResetAt":     lastReset,
		"createdAt":              u.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		"updatedAt":              u.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		"subscriptionUrl":        subUrl,
		"activeInternalSquads":   activeSquads,
		"userTraffic":            traffic,
	}
}

func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var query GetUsersQuery
	query.Size = 25

	if s := r.URL.Query().Get("start"); s != "" {
		if val, err := strconv.Atoi(s); err == nil && val >= 0 {
			query.Start = val
		}
	}
	if s := r.URL.Query().Get("size"); s != "" {
		if val, err := strconv.Atoi(s); err == nil && val > 0 {
			query.Size = val
		}
	}
	query.Status = r.URL.Query().Get("status")
	query.Tag = r.URL.Query().Get("tag")

	if filtersStr := r.URL.Query().Get("filters"); filtersStr != "" {
		_ = json.Unmarshal([]byte(filtersStr), &query.Filters)
	}
	if sortingStr := r.URL.Query().Get("sorting"); sortingStr != "" {
		_ = json.Unmarshal([]byte(sortingStr), &query.Sorting)
	}

	usersList, total, err := h.service.GetAll(query)
	if err != nil {
		http.Error(w, `{"message":"Failed to fetch users"}`, http.StatusInternalServerError)
		return
	}

	formatted := make([]map[string]interface{}, 0, len(usersList))
	for _, u := range usersList {
		formatted = append(formatted, FormatUser(u, h.subPublicDomain))
	}

	var resp UsersListResponse
	resp.Response.Users = formatted
	resp.Response.Total = int(total)

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tags, err := h.service.GetTags()
	if err != nil {
		tags = []string{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tags": tags,
		},
	})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var dto CreateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	user, err := h.service.Create(dto)
	if err != nil {
		http.Error(w, `{"message":"Failed to create user"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SingleUserResponse{Response: FormatUser(user, h.subPublicDomain)})
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		idStr = chi.URLParam(r, "userId")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	user, err := h.service.GetByID(id)
	if err != nil {
		http.Error(w, `{"message":"User not found"}`, http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(SingleUserResponse{Response: FormatUser(user, h.subPublicDomain)})
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		idStr = chi.URLParam(r, "userId")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	delete(updates, "id")
	delete(updates, "shortUuid")

	user, err := h.service.Update(id, updates)
	if err != nil {
		http.Error(w, `{"message":"Failed to update user"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(SingleUserResponse{Response: FormatUser(user, h.subPublicDomain)})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		idStr = chi.URLParam(r, "userId")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.service.Delete(id); err != nil {
		http.Error(w, `{"message":"Failed to delete user"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ResetTraffic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		idStr = chi.URLParam(r, "userId")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.service.ResetTraffic(id); err != nil {
		http.Error(w, `{"message":"Failed to reset traffic"}`, http.StatusInternalServerError)
		return
	}

	user, err := h.service.GetByID(id)
	if err != nil {
		http.Error(w, `{"message":"User not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(SingleUserResponse{Response: FormatUser(user, h.subPublicDomain)})
}

func (h *Handler) EnableUser(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}

func (h *Handler) DisableUser(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *Handler) setEnabled(w http.ResponseWriter, r *http.Request, enable bool) {
	id, err := strconv.ParseUint(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "Invalid user ID")
		return
	}
	if enable {
		err = h.service.Enable(id)
	} else {
		err = h.service.Disable(id)
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	case errors.Is(err, ErrAlreadyEnabled):
		httpx.Error(w, r, http.StatusBadRequest, "A030", "User already enabled")
		return
	case errors.Is(err, ErrAlreadyDisabled):
		httpx.Error(w, r, http.StatusBadRequest, "A029", "User already disabled")
		return
	case err != nil && enable:
		httpx.Error(w, r, http.StatusInternalServerError, "A031", "Enable user error")
		return
	case err != nil:
		httpx.Error(w, r, http.StatusInternalServerError, "A028", "Disable user error")
		return
	}
	user, err := h.service.GetByID(id)
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	}
	httpx.Response(w, FormatUser(user, h.subPublicDomain))
}

func (h *Handler) ExtendUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		http.Error(w, `{"message":"Invalid user ID"}`, 400)
		return
	}
	var body struct {
		Days int `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid body"}`, 400)
		return
	}
	user, err := h.service.Extend(id, body.Days)
	if err != nil {
		http.Error(w, `{"message":"Cannot extend user"}`, 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SingleUserResponse{Response: FormatUser(user, h.subPublicDomain)})
}
func (h *Handler) RevokeUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		http.Error(w, `{"message":"Invalid user ID"}`, 400)
		return
	}
	var body struct {
		RevokeOnlyPasswords bool   `json:"revokeOnlyPasswords"`
		ShortUUID           string `json:"shortUuid"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"message":"Invalid body"}`, 400)
			return
		}
	}
	user, err := h.service.Revoke(id, body.RevokeOnlyPasswords, body.ShortUUID)
	if err != nil {
		http.Error(w, `{"message":"Cannot revoke subscription"}`, 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SingleUserResponse{Response: FormatUser(user, h.subPublicDomain)})
}

func (h *Handler) GetUserByShortUuid(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	shortUuid := chi.URLParam(r, "shortUuid")
	user, err := h.service.GetByShortUUID(shortUuid)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "User not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": FormatUser(user, h.subPublicDomain),
	})
}

func (h *Handler) GetUserByUsername(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	username := chi.URLParam(r, "username")
	user, err := h.service.GetByUsername(username)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "User not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": FormatUser(user, h.subPublicDomain),
	})
}

// ResolveUser finds a user by any one unique field and returns its identifiers.
func (h *Handler) ResolveUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID        *uint64 `json:"id"`
		ShortUUID string  `json:"shortUuid"`
		Username  string  `json:"username"`
	}
	if err := httpx.Decode(r, 4096, &body); err != nil || (body.ID == nil && body.ShortUUID == "" && body.Username == "") {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "one of id, shortUuid or username is required")
		return
	}
	user, err := h.service.Resolve(body.ID, body.ShortUUID, body.Username)
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	}
	httpx.Response(w, map[string]interface{}{"id": user.ID, "shortUuid": user.ShortUUID, "username": user.Username})
}

func (h *Handler) GetUsersStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	q := StreamQuery{}
	if cStr := r.URL.Query().Get("cursor"); cStr != "" {
		if c, err := strconv.ParseUint(cStr, 10, 64); err == nil {
			q.Cursor = &c
		}
	}
	if sStr := r.URL.Query().Get("size"); sStr != "" {
		if s, err := strconv.Atoi(sStr); err == nil {
			q.Size = s
		}
	}
	if q.Size <= 0 {
		q.Size = 250
	}
	if tgStr := r.URL.Query().Get("telegramId"); tgStr != "" {
		if tg, err := strconv.ParseInt(tgStr, 10, 64); err == nil {
			q.TelegramID = &tg
		}
	}
	q.Status = r.URL.Query().Get("status")
	q.TrafficLimitStrategy = r.URL.Query().Get("trafficLimitStrategy")
	q.Email = r.URL.Query().Get("email")
	q.Tag = r.URL.Query().Get("tag")
	q.ExternalSquadUUID = r.URL.Query().Get("externalSquadUuid")

	users, nextCursor, hasMore, err := h.service.GetStream(q)
	if err != nil {
		http.Error(w, `{"message":"Failed to fetch users stream"}`, http.StatusInternalServerError)
		return
	}

	res := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		res = append(res, FormatUser(u, h.subPublicDomain))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"users":      res,
			"nextCursor": nextCursor,
			"hasMore":    hasMore,
		},
	})
}

// GetUserAccessibleNodes lists nodes reachable through the user's internal
// squads, grouped by squad with the inbound tags each squad grants.
func (h *Handler) GetUserAccessibleNodes(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseUint(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "userId must be a number")
		return
	}
	rows, err := h.service.AccessibleNodes(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		} else {
			httpx.Error(w, r, http.StatusInternalServerError, "A125", "Get user accessible nodes error")
		}
		return
	}
	type squad struct {
		SquadName      string   `json:"squadName"`
		ActiveInbounds []string `json:"activeInbounds"`
	}
	type node struct {
		UUID              string   `json:"uuid"`
		NodeName          string   `json:"nodeName"`
		CountryCode       string   `json:"countryCode"`
		ConfigProfileUUID string   `json:"configProfileUuid"`
		ConfigProfileName string   `json:"configProfileName"`
		ActiveSquads      []*squad `json:"activeSquads"`
		squads            map[string]*squad
	}
	var ordered []*node
	byUUID := map[string]*node{}
	for _, row := range rows {
		entry := byUUID[row.NodeUUID]
		if entry == nil {
			entry = &node{UUID: row.NodeUUID, NodeName: row.NodeName, CountryCode: row.CountryCode, ConfigProfileUUID: row.ConfigProfileUUID, ConfigProfileName: row.ConfigProfileName, ActiveSquads: []*squad{}, squads: map[string]*squad{}}
			byUUID[row.NodeUUID] = entry
			ordered = append(ordered, entry)
		}
		group := entry.squads[row.SquadUUID]
		if group == nil {
			group = &squad{SquadName: row.SquadName, ActiveInbounds: []string{}}
			entry.squads[row.SquadUUID] = group
			entry.ActiveSquads = append(entry.ActiveSquads, group)
		}
		group.ActiveInbounds = append(group.ActiveInbounds, row.InboundTag)
	}
	if ordered == nil {
		ordered = []*node{}
	}
	httpx.Response(w, map[string]interface{}{"userId": userID, "activeNodes": ordered})
}

func (h *Handler) GetUserSubscriptionRequestHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idStr := chi.URLParam(r, "userId")
	if idStr == "" {
		idStr = chi.URLParam(r, "id")
	}
	userId, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		var u database.User
		if err2 := h.service.db.Where("short_uuid = ? OR username = ?", idStr, idStr).First(&u).Error; err2 == nil {
			userId = u.ID
		}
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

	var records []database.UserSubscriptionRequestHistory
	h.service.db.Where("user_id = ?", userId).Order("request_at desc").Limit(50).Find(&records)

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
			"records": result,
			"total":   len(result),
		},
	})
}

func (h *Handler) BulkExtendExpirationDate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserIDs    []uint64 `json:"userIds"`
		ExtendDays int      `json:"extendDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkExtendExpirationDate(body.UserIDs, body.ExtendDays); err != nil {
		http.Error(w, `{"message":"Failed to extend expiration date"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) BulkAllExtendExpirationDate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExtendDays int `json:"extendDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkAllExtendExpirationDate(body.ExtendDays); err != nil {
		http.Error(w, `{"message":"Failed to extend expiration date"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) BulkUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserIDs []uint64               `json:"userIds"`
		Fields  map[string]interface{} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkUpdate(body.UserIDs, body.Fields); err != nil {
		http.Error(w, `{"message":"Failed to update users"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) BulkAllUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Fields map[string]interface{} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkAllUpdate(body.Fields); err != nil {
		http.Error(w, `{"message":"Failed to update users"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) BulkResetTraffic(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserIDs []uint64 `json:"userIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkResetTraffic(body.UserIDs); err != nil {
		http.Error(w, `{"message":"Failed to reset traffic"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) BulkAllResetTraffic(w http.ResponseWriter, r *http.Request) {
	if err := h.service.BulkAllResetTraffic(); err != nil {
		http.Error(w, `{"message":"Failed to reset traffic"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) BulkRevokeSubscription(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserIDs []uint64 `json:"userIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkRevokeSubscription(body.UserIDs); err != nil {
		http.Error(w, `{"message":"Failed to revoke subscription"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserIDs []uint64 `json:"userIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkDelete(body.UserIDs); err != nil {
		http.Error(w, `{"message":"Failed to delete users"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) BulkDeleteByStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkDeleteByStatus(body.Status); err != nil {
		http.Error(w, `{"message":"Failed to delete users by status"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) BulkUpdateSquads(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserIDs              []uint64 `json:"userIds"`
		ActiveInternalSquads []string `json:"activeInternalSquads"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.service.BulkUpdateSquads(body.UserIDs, body.ActiveInternalSquads); err != nil {
		http.Error(w, `{"message":"Failed to update user squads"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) BulkUserAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"success": true,
		},
	})
}

func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	var user *database.User
	if rawID, ok := body["id"]; ok && string(rawID) != "null" {
		var id uint64
		if err := json.Unmarshal(rawID, &id); err != nil || id == 0 {
			http.Error(w, `{"message":"Invalid user ID"}`, http.StatusBadRequest)
			return
		}
		user, _ = h.service.GetByID(id)
	} else if rawShortUUID, ok := body["shortUuid"]; ok {
		var shortUUID string
		if err := json.Unmarshal(rawShortUUID, &shortUUID); err != nil || shortUUID == "" {
			http.Error(w, `{"message":"Invalid short UUID"}`, http.StatusBadRequest)
			return
		}
		user, _ = h.service.GetByShortUUID(shortUUID)
	} else if rawUsername, ok := body["username"]; ok {
		var username string
		if err := json.Unmarshal(rawUsername, &username); err != nil || username == "" {
			http.Error(w, `{"message":"Invalid username"}`, http.StatusBadRequest)
			return
		}
		user, _ = h.service.GetByUsername(username)
	} else {
		http.Error(w, `{"message":"User ID or username is required"}`, http.StatusBadRequest)
		return
	}
	if user == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "User not found"})
		return
	}

	updates := make(map[string]interface{}, len(body))
	for key, rawValue := range body {
		if key == "id" || key == "shortUuid" {
			continue
		}
		var value interface{}
		if err := json.Unmarshal(rawValue, &value); err != nil {
			http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
			return
		}
		updates[key] = value
	}

	updatedUser, err := h.service.Update(user.ID, updates)
	if err != nil {
		log.Printf("[USERS] failed to patch user id=%d: %v", user.ID, err)
		http.Error(w, `{"message":"Failed to update user"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(SingleUserResponse{Response: FormatUser(updatedUser, h.subPublicDomain)})
}
