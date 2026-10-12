package infrabilling

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"remnawave-go/internal/database"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) formatProvider(p *database.InfraProvider) map[string]interface{} {
	var hist struct {
		TotalAmount float64
		TotalBills  int64
	}
	h.db.Table("infra_billing_history").
		Select("COALESCE(SUM(amount), 0) as total_amount, COUNT(uuid) as total_bills").
		Where("provider_uuid = ?", p.UUID).
		Scan(&hist)

	type rawBillingNode struct {
		Name        string  `gorm:"column:name"`
		NodeName    *string `gorm:"column:node_name"`
		NodeUUID    *string `gorm:"column:node_uuid"`
		CountryCode *string `gorm:"column:country_code"`
	}

	var bNodes []rawBillingNode
	h.db.Table("infra_billing_nodes as ibn").
		Select("ibn.name as name, n.name as node_name, ibn.node_uuid as node_uuid, n.country_code as country_code").
		Joins("LEFT JOIN nodes as n ON ibn.node_uuid = n.uuid").
		Where("ibn.provider_uuid = ?", p.UUID).
		Order("ibn.created_at ASC").
		Scan(&bNodes)

	billingNodesResp := make([]map[string]interface{}, 0, len(bNodes))
	for _, bn := range bNodes {
		displayName := bn.Name
		if bn.NodeName != nil && *bn.NodeName != "" {
			displayName = *bn.NodeName
		}
		var details interface{}
		if bn.NodeUUID != nil && *bn.NodeUUID != "" && bn.CountryCode != nil && *bn.CountryCode != "" {
			details = map[string]interface{}{
				"nodeUuid":    *bn.NodeUUID,
				"countryCode": *bn.CountryCode,
			}
		}
		billingNodesResp = append(billingNodesResp, map[string]interface{}{
			"name":    displayName,
			"details": details,
		})
	}

	return map[string]interface{}{
		"uuid":        p.UUID,
		"name":        p.Name,
		"faviconLink": p.FaviconLink,
		"loginUrl":    p.LoginURL,
		"createdAt":   p.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":   p.UpdatedAt.UTC().Format(time.RFC3339),
		"billingHistory": map[string]interface{}{
			"totalAmount": math.Round(hist.TotalAmount*100) / 100,
			"totalBills":  hist.TotalBills,
		},
		"billingNodes": billingNodesResp,
	}
}

func (h *Handler) GetProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var providers []database.InfraProvider
	h.db.Order("name asc").Find(&providers)

	res := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		res = append(res, h.formatProvider(&p))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":     len(res),
			"providers": res,
		},
	})
}

func (h *Handler) GetProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")

	var p database.InfraProvider
	if err := h.db.Where("uuid = ?", uuidParam).First(&p).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Provider not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": h.formatProvider(&p),
	})
}

func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Name        string  `json:"name"`
		FaviconLink *string `json:"faviconLink"`
		LoginURL    *string `json:"loginUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	now := time.Now().UTC()
	p := database.InfraProvider{
		UUID:        uuid.New().String(),
		Name:        strings.TrimSpace(body.Name),
		FaviconLink: body.FaviconLink,
		LoginURL:    body.LoginURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.db.Create(&p).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to create provider"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": h.formatProvider(&p),
	})
}

func (h *Handler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID        string  `json:"uuid"`
		Name        *string `json:"name"`
		FaviconLink *string `json:"faviconLink"`
		LoginURL    *string `json:"loginUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var p database.InfraProvider
	if err := h.db.Where("uuid = ?", body.UUID).First(&p).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Provider not found"})
		return
	}

	if body.Name != nil {
		p.Name = strings.TrimSpace(*body.Name)
	}
	if body.FaviconLink != nil {
		p.FaviconLink = body.FaviconLink
	}
	if body.LoginURL != nil {
		p.LoginURL = body.LoginURL
	}
	p.UpdatedAt = time.Now().UTC()

	if err := h.db.Save(&p).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update provider"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": h.formatProvider(&p),
	})
}

func (h *Handler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	h.db.Where("uuid = ?", uuidParam).Delete(&database.InfraProvider{})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var billingNodes []database.InfraBillingNode
	h.db.Order("created_at asc").Find(&billingNodes)

	res := make([]map[string]interface{}, 0, len(billingNodes))
	for _, bn := range billingNodes {
		var prov database.InfraProvider
		h.db.Where("uuid = ?", bn.ProviderUUID).First(&prov)

		var nodeObj interface{} = nil
		if bn.NodeUUID != nil && *bn.NodeUUID != "" {
			var n database.Node
			if err := h.db.Where("uuid = ?", *bn.NodeUUID).First(&n).Error; err == nil {
				nodeObj = map[string]interface{}{
					"uuid":        n.UUID,
					"name":        n.Name,
					"countryCode": n.CountryCode,
				}
			}
		}

		res = append(res, map[string]interface{}{
			"uuid":         bn.UUID,
			"nodeUuid":     bn.NodeUUID,
			"name":         bn.Name,
			"providerUuid": bn.ProviderUUID,
			"provider": map[string]interface{}{
				"uuid":        prov.UUID,
				"name":        prov.Name,
				"loginUrl":    prov.LoginURL,
				"faviconLink": prov.FaviconLink,
			},
			"node":          nodeObj,
			"nextBillingAt": bn.NextBillingAt.UTC().Format(time.RFC3339),
			"createdAt":     bn.CreatedAt.UTC().Format(time.RFC3339),
			"updatedAt":     bn.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	// Available nodes that can be linked to a billing node (not yet assigned)
	var availableNodes []struct {
		UUID        string `gorm:"column:uuid"`
		Name        string `gorm:"column:name"`
		CountryCode string `gorm:"column:country_code"`
	}
	h.db.Table("nodes as n").
		Select("n.uuid, n.name, n.country_code").
		Joins("LEFT JOIN infra_billing_nodes as ibn ON ibn.node_uuid = n.uuid").
		Where("ibn.node_uuid IS NULL").
		Order("n.view_position asc").
		Scan(&availableNodes)

	availRes := make([]map[string]interface{}, 0, len(availableNodes))
	for _, an := range availableNodes {
		availRes = append(availRes, map[string]interface{}{
			"uuid":        an.UUID,
			"name":        an.Name,
			"countryCode": an.CountryCode,
		})
	}

	// Calculate stats
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)
	upcomingThreshold := now.AddDate(0, 0, 7)

	var upcomingCount int64
	h.db.Model(&database.InfraBillingNode{}).
		Where("next_billing_at <= ?", upcomingThreshold).
		Count(&upcomingCount)

	var currentMonthPayments float64
	h.db.Model(&database.InfraBillingHistory{}).
		Where("billed_at >= ? AND billed_at < ?", startOfMonth, startOfNextMonth).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&currentMonthPayments)

	var totalSpent float64
	h.db.Model(&database.InfraBillingHistory{}).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&totalSpent)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"totalBillingNodes":          len(res),
			"billingNodes":               res,
			"availableBillingNodes":      availRes,
			"totalAvailableBillingNodes": len(availRes),
			"stats": map[string]interface{}{
				"upcomingNodesCount":   upcomingCount,
				"currentMonthPayments": math.Round(currentMonthPayments*100) / 100,
				"totalSpent":           math.Round(totalSpent*100) / 100,
			},
		},
	})
}

func (h *Handler) CreateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Name          *string   `json:"name"`
		NodeUUID      *string   `json:"nodeUuid"`
		ProviderUUID  string    `json:"providerUuid"`
		NextBillingAt time.Time `json:"nextBillingAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	nameStr := ""
	if body.Name != nil && strings.TrimSpace(*body.Name) != "" {
		nameStr = strings.TrimSpace(*body.Name)
	} else if body.NodeUUID != nil && *body.NodeUUID != "" {
		var n database.Node
		if err := h.db.Where("uuid = ?", *body.NodeUUID).First(&n).Error; err == nil {
			nameStr = n.Name
		}
	}

	now := time.Now().UTC()
	bn := database.InfraBillingNode{
		UUID:          uuid.New().String(),
		Name:          nameStr,
		NodeUUID:      body.NodeUUID,
		ProviderUUID:  body.ProviderUUID,
		NextBillingAt: body.NextBillingAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.db.Create(&bn).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to create billing node"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	h.GetNodes(w, r)
}

func (h *Handler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID          string     `json:"uuid"`
		UUIDs         []string   `json:"uuids"`
		Name          *string    `json:"name"`
		NodeUUID      *string    `json:"nodeUuid"`
		ProviderUUID  *string    `json:"providerUuid"`
		NextBillingAt *time.Time `json:"nextBillingAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	// Support both bulk updates (uuids: [...]) and single update (uuid: "...")
	targetUUIDs := body.UUIDs
	if len(targetUUIDs) == 0 && body.UUID != "" {
		targetUUIDs = []string{body.UUID}
	}
	if len(targetUUIDs) == 0 {
		if urlUUID := chi.URLParam(r, "uuid"); urlUUID != "" {
			targetUUIDs = []string{urlUUID}
		}
	}

	if len(targetUUIDs) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "UUID or UUIDs required"})
		return
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}
	if body.Name != nil {
		updates["name"] = strings.TrimSpace(*body.Name)
	}
	if body.NodeUUID != nil {
		updates["node_uuid"] = body.NodeUUID
	}
	if body.ProviderUUID != nil {
		updates["provider_uuid"] = *body.ProviderUUID
	}
	if body.NextBillingAt != nil {
		updates["next_billing_at"] = *body.NextBillingAt
	}

	res := h.db.Model(&database.InfraBillingNode{}).Where("uuid IN ?", targetUUIDs).Updates(updates)
	if res.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update billing node"})
		return
	}
	if res.RowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Billing node not found"})
		return
	}

	h.GetNodes(w, r)
}

func (h *Handler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	h.db.Where("uuid = ?", uuidParam).Delete(&database.InfraBillingNode{})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var records []database.InfraBillingHistory
	h.db.Order("billed_at desc").Find(&records)

	res := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		var prov database.InfraProvider
		h.db.Where("uuid = ?", rec.ProviderUUID).First(&prov)

		res = append(res, map[string]interface{}{
			"uuid":         rec.UUID,
			"providerUuid": rec.ProviderUUID,
			"amount":       rec.Amount,
			"billedAt":     rec.BilledAt.UTC().Format(time.RFC3339),
			"provider": map[string]interface{}{
				"uuid":        prov.UUID,
				"name":        prov.Name,
				"faviconLink": prov.FaviconLink,
			},
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":   len(res),
			"records": res,
		},
	})
}

func (h *Handler) CreateHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		ProviderUUID string    `json:"providerUuid"`
		Amount       float64   `json:"amount"`
		BilledAt     time.Time `json:"billedAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	rec := database.InfraBillingHistory{
		UUID:         uuid.New().String(),
		ProviderUUID: body.ProviderUUID,
		Amount:       body.Amount,
		BilledAt:     body.BilledAt,
	}

	h.db.Create(&rec)
	w.WriteHeader(http.StatusCreated)
	h.GetHistory(w, r)
}

func (h *Handler) DeleteHistory(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	h.db.Where("uuid = ?", uuidParam).Delete(&database.InfraBillingHistory{})
	w.WriteHeader(http.StatusNoContent)
}
