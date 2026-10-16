package stubs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"gowave/internal/database"
	"gowave/internal/httpx"

	"gorm.io/gorm"
)

var sharedListName = regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`)

// validateSharedListConfig ports SharedListConfigSchema from
// @remnawave/node-plugins: an ipList of IPv4/IPv6 addresses or CIDR ranges, or
// an asList of AS numbers in [1, 4294967295].
func validateSharedListConfig(raw json.RawMessage) (map[string]interface{}, error) {
	var config struct {
		Type  string            `json:"type"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, errors.New("config must be an object")
	}
	if config.Items == nil {
		return nil, errors.New("items: Invalid input: expected array")
	}
	switch config.Type {
	case "ipList":
		items := make([]string, 0, len(config.Items))
		for i, item := range config.Items {
			var value string
			if json.Unmarshal(item, &value) != nil || !validIPOrCIDR(value) {
				return nil, fmt.Errorf("items.%d: Invalid IP address or CIDR range", i)
			}
			items = append(items, value)
		}
		return map[string]interface{}{"type": "ipList", "items": items}, nil
	case "asList":
		items := make([]uint64, 0, len(config.Items))
		for i, item := range config.Items {
			var value float64
			if json.Unmarshal(item, &value) != nil || value != float64(uint64(value)) || value < 1 || value > 4294967295 {
				return nil, fmt.Errorf("items.%d: AS number must be an integer from 1 to 4294967295", i)
			}
			items = append(items, uint64(value))
		}
		return map[string]interface{}{"type": "asList", "items": items}, nil
	}
	return nil, errors.New(`type: Invalid input: expected "ipList" or "asList"`)
}

func validIPOrCIDR(value string) bool {
	if strings.Contains(value, "%") {
		return false
	}
	if strings.Contains(value, "/") {
		_, err := netip.ParsePrefix(value)
		return err == nil
	}
	_, err := netip.ParseAddr(value)
	return err == nil
}

func validSharedListName(name string) bool {
	return len(name) >= 2 && len(name) <= 255 && sharedListName.MatchString(name)
}

func sharedListResponse(list *database.SharedList) map[string]interface{} {
	var config interface{}
	_ = json.Unmarshal([]byte(list.Config), &config)
	return map[string]interface{}{"name": list.Name, "config": config}
}

func (s *StubHandler) GetSharedLists(w http.ResponseWriter, r *http.Request) {
	var lists []database.SharedList
	if err := s.db.Order("created_at ASC").Find(&lists).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A247", "Get all shared lists error")
		return
	}
	previews := make([]map[string]interface{}, 0, len(lists))
	for _, list := range lists {
		var config struct {
			Type  string        `json:"type"`
			Items []interface{} `json:"items"`
		}
		_ = json.Unmarshal([]byte(list.Config), &config)
		previews = append(previews, map[string]interface{}{"name": list.Name, "type": config.Type, "itemsCount": len(config.Items)})
	}
	httpx.Response(w, map[string]interface{}{"total": len(previews), "sharedLists": previews})
}

func (s *StubHandler) GetSharedListByName(w http.ResponseWriter, r *http.Request) {
	var list database.SharedList
	err := s.db.Where("name = ?", r.URL.Query().Get("name")).First(&list).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "A245", "Shared list not found")
		return
	}
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A248", "Get shared list by name error")
		return
	}
	httpx.Response(w, sharedListResponse(&list))
}

type sharedListBody struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

// decodeSharedList validates the name and config; it writes the error itself.
func decodeSharedList(w http.ResponseWriter, r *http.Request) (*sharedListBody, string, bool) {
	var body sharedListBody
	if err := httpx.Decode(r, 4<<20, &body); err != nil || !validSharedListName(body.Name) {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "name must be 2-255 characters of letters, digits, _ or -, optionally separated by /")
		return nil, "", false
	}
	config, err := validateSharedListConfig(body.Config)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "A252", err.Error())
		return nil, "", false
	}
	encoded, _ := json.Marshal(config)
	return &body, string(encoded), true
}

func (s *StubHandler) CreateSharedList(w http.ResponseWriter, r *http.Request) {
	body, config, ok := decodeSharedList(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	list := database.SharedList{Name: body.Name, Config: config, CreatedAt: now, UpdatedAt: now}
	var count int64
	if err := s.db.Model(&database.SharedList{}).Where("name = ?", body.Name).Count(&count).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A249", "Create shared list error")
		return
	}
	if count > 0 {
		httpx.Error(w, r, http.StatusBadRequest, "A246", "Shared list with this name already exists")
		return
	}
	if err := s.db.Create(&list).Error; err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "A246", "Shared list with this name already exists")
		return
	}
	httpx.Response(w, sharedListResponse(&list))
}

func (s *StubHandler) UpdateSharedList(w http.ResponseWriter, r *http.Request) {
	body, config, ok := decodeSharedList(w, r)
	if !ok {
		return
	}
	result := s.db.Model(&database.SharedList{}).Where("name = ?", body.Name).
		Updates(map[string]interface{}{"config": config, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A250", "Update shared list error")
		return
	}
	if result.RowsAffected == 0 {
		httpx.Error(w, r, http.StatusNotFound, "A245", "Shared list not found")
		return
	}
	var list database.SharedList
	s.db.Where("name = ?", body.Name).First(&list)
	httpx.Response(w, sharedListResponse(&list))
}

func (s *StubHandler) DeleteSharedList(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, 4096, &body); err != nil || body.Name == "" {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "name is required")
		return
	}
	result := s.db.Where("name = ?", body.Name).Delete(&database.SharedList{})
	if result.Error != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A251", "Delete shared list error")
		return
	}
	if result.RowsAffected == 0 {
		httpx.Error(w, r, http.StatusNotFound, "A245", "Shared list not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
