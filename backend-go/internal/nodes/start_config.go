package nodes

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"remnawave-go/internal/database"
	"strconv"
	"strings"
	"unicode/utf16"
)

// inboundUsersHash matches @remnawave/hashed-set 0.0.4. Set membership is
// order-independent and JavaScript charCodeAt operates on UTF-16 code units.
func inboundUsersHash(ids []string) string {
	seen := map[string]bool{}
	var high, low uint32
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		h, l := uint32(5381), uint32(5387)
		for _, c := range utf16.Encode([]rune(id)) {
			h = h*33 + uint32(c)
			l = l*65 + uint32(c)*37
		}
		high ^= h
		low ^= l
	}
	format := func(value uint32) string {
		raw := strconv.FormatInt(int64(int32(value)), 16)
		if len(raw) < 8 {
			raw = strings.Repeat("0", 8-len(raw)) + raw
		}
		return raw
	}
	return format(high) + format(low)
}
func (s *Service) populateConfigUsers(config map[string]interface{}, profileID string) (map[string]interface{}, error) {
	inbounds, _ := config["inbounds"].([]interface{})
	hashes := []map[string]interface{}{}
	// Hash the empty, expanded configuration separately from changing users.
	empty, _ := json.Marshal(config)
	digest := sha256.Sum256(empty)
	for _, raw := range inbounds {
		inbound, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		protocol, _ := inbound["protocol"].(string)
		if isSystemOrUnsecureProtocol(protocol) {
			continue
		}
		tag, _ := inbound["tag"].(string)
		var records []database.User
		query := s.db.Model(&database.User{}).Where("status='ACTIVE' AND id IN (SELECT m.user_id FROM internal_squad_members m JOIN internal_squad_inbounds si ON si.internal_squad_uuid=m.internal_squad_uuid JOIN config_profile_inbounds ib ON ib.uuid=si.inbound_uuid WHERE ib.tag=? AND ib.profile_uuid=?)", tag, profileID).Select("id,vless_uuid,trojan_password,ss_password")
		if err := query.Find(&records).Error; err != nil {
			return nil, err
		}
		settings, ok := inbound["settings"].(map[string]interface{})
		if !ok {
			settings = map[string]interface{}{}
			inbound["settings"] = settings
		}
		clients := make([]interface{}, 0, len(records))
		ids := make([]string, 0, len(records))
		method, _ := settings["method"].(string)
		if method == "" {
			method = "chacha20-ietf-poly1305"
		}
		for _, user := range records {
			client := map[string]interface{}{"id": user.VlessUUID, "email": fmt.Sprint(user.ID)}
			ids = append(ids, user.VlessUUID)
			switch protocol {
			case "trojan":
				client["password"] = user.TrojanPassword
			case "shadowsocks":
				password := user.SsPassword
				if strings.HasPrefix(method, "2022-") {
					password = base64.StdEncoding.EncodeToString([]byte(password))
				} else {
					client["method"] = method
				}
				client["password"] = password
			case "hysteria":
				client["auth"] = user.VlessUUID
			}
			clients = append(clients, client)
		}
		settings["clients"] = clients
		hashes = append(hashes, map[string]interface{}{"usersCount": len(ids), "hash": inboundUsersHash(ids), "tag": tag})
	}
	return map[string]interface{}{"emptyConfig": hex.EncodeToString(digest[:]), "inbounds": hashes}, nil
}
