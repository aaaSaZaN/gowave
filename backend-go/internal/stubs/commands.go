package stubs

import (
	"encoding/json"
	"fmt"
	"gowave/internal/database"
	"gowave/internal/jobs"
	"gowave/internal/nodes"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func (s *StubHandler) enqueueCommands(kind, path string, body interface{}, target string, ids []string) error {
	if target != "allNodes" && target != "specificNodes" {
		return fmt.Errorf("invalid target")
	}
	if target == "specificNodes" && len(ids) == 0 {
		return fmt.Errorf("node UUIDs required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("is_disabled=?", false)
		if target == "specificNodes" {
			query = query.Where("uuid IN ?", ids)
		}
		var records []database.Node
		if err := query.Find(&records).Error; err != nil {
			return err
		}
		if target == "specificNodes" && len(records) != len(ids) {
			return fmt.Errorf("some target nodes are missing or disabled")
		}
		revision := uint64(time.Now().UnixNano())
		for _, node := range records {
			if err := jobs.Enqueue(tx, kind, node.UUID, node.UUID, revision, nodes.Command{Path: path, Body: body}); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *StubHandler) pluginExecute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command struct {
			Command string          `json:"command"`
			IPs     json.RawMessage `json:"ips"`
		} `json:"command"`
		Target struct {
			Target string   `json:"target"`
			IDs    []string `json:"nodeUuids"`
		} `json:"targetNodes"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	var command string
	switch body.Command.Command {
	case "blockIps":
		command = "block-ips"
		var ips []struct {
			IP      string `json:"ip"`
			Timeout int    `json:"timeout"`
		}
		if json.Unmarshal(body.Command.IPs, &ips) != nil || len(ips) == 0 {
			http.Error(w, "IPs required", 400)
			return
		}
		for _, ip := range ips {
			if net.ParseIP(ip.IP) == nil || ip.Timeout < 0 {
				http.Error(w, "invalid IP or timeout", 400)
				return
			}
		}
	case "unblockIps":
		command = "unblock-ips"
		var ips []string
		if json.Unmarshal(body.Command.IPs, &ips) != nil || len(ips) == 0 {
			http.Error(w, "IPs required", 400)
			return
		}
		for _, ip := range ips {
			if net.ParseIP(ip) == nil {
				http.Error(w, "invalid IP", 400)
				return
			}
		}
	case "recreateTables":
		command = "recreate-tables"
	default:
		http.Error(w, "invalid command", 400)
		return
	}
	payload := map[string]interface{}{}
	if len(body.Command.IPs) > 0 {
		payload["ips"] = body.Command.IPs
	}
	if err := s.enqueueCommands("plugin-exec", "/node/plugin/nftables/"+command, payload, body.Target.Target, body.Target.IDs); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.WriteHeader(204)
}
func (s *StubHandler) dropConnections(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Drop struct {
			By  string   `json:"by"`
			IDs []uint64 `json:"userIds"`
			IPs []string `json:"ipAddresses"`
		} `json:"dropBy"`
		Target struct {
			Target string   `json:"target"`
			IDs    []string `json:"nodeUuids"`
		} `json:"targetNodes"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	var path string
	payload := map[string]interface{}{}
	switch body.Drop.By {
	case "userIds":
		if len(body.Drop.IDs) == 0 {
			http.Error(w, "user IDs required", 400)
			return
		}
		ids := make([]string, len(body.Drop.IDs))
		for i, id := range body.Drop.IDs {
			ids[i] = strconv.FormatUint(id, 10)
		}
		path = "drop-users-connections"
		payload["userIds"] = ids
	case "ipAddresses":
		if len(body.Drop.IPs) == 0 {
			http.Error(w, "IPs required", 400)
			return
		}
		for _, ip := range body.Drop.IPs {
			if net.ParseIP(ip) == nil {
				http.Error(w, "invalid IP", 400)
				return
			}
		}
		path = "drop-ips"
		payload["ips"] = body.Drop.IPs
	default:
		http.Error(w, "invalid drop selector", 400)
		return
	}
	if err := s.enqueueCommands("connections-drop", "/node/handler/"+path, payload, body.Target.Target, body.Target.IDs); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.WriteHeader(204)
}
func (s *StubHandler) geocheck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "nodeUuid")
	var node database.Node
	if s.db.Where("uuid=?", id).First(&node).Error != nil {
		http.Error(w, "node not found", 404)
		return
	}
	var body struct {
		IP        string `json:"ip"`
		Interface string `json:"interface"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.IP != "" && body.Interface != "" || body.IP != "" && net.ParseIP(body.IP) == nil {
		http.Error(w, "invalid geocheck request", 400)
		return
	}
	revision := uint64(time.Now().UnixNano())
	if err := jobs.Enqueue(s.db, "geocheck", id, id, revision, nodes.Command{Path: "/node/stats/get-geocheck", Body: body}); err != nil {
		http.Error(w, "queue unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]string{"jobId": jobs.ID("geocheck", id, id, revision)}})
}
func (s *StubHandler) geocheckResult(w http.ResponseWriter, r *http.Request) {
	var job database.Job
	if s.db.Where("id=? AND kind='geocheck'", chi.URLParam(r, "jobId")).First(&job).Error != nil {
		http.Error(w, "job not found", 404)
		return
	}
	var result interface{}
	if job.Result != "" {
		json.Unmarshal([]byte(job.Result), &result)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"isCompleted": job.State == "completed", "isFailed": job.State == "failed", "result": result}})
}
