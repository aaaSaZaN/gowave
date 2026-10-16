package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"gowave/internal/configrender"
	"gowave/internal/database"
	"io"
	"net"
	"strconv"
	"strings"
)

type Command struct {
	Path string      `json:"path"`
	Body interface{} `json:"body"`
}

// Call is restricted to the node's pinned control API and retains mutual TLS
// and signed node authentication from Client.doRequest.
func (c *Client) Call(ctx context.Context, node *database.Node, path string, body interface{}) (map[string]interface{}, error) {
	if !strings.HasPrefix(path, "/node/") {
		return nil, fmt.Errorf("invalid node API path")
	}
	address := "https://" + net.JoinHostPort(node.Address, strconv.Itoa(getNodePort(node))) + path
	response, err := c.doRequest(ctx, "POST", address, body)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 && response.StatusCode != 201 {
		return nil, fmt.Errorf("node returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Response map[string]interface{} `json:"response"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&result); err != nil {
		return nil, err
	}
	if result.Response == nil {
		return nil, fmt.Errorf("invalid node response")
	}
	for _, field := range []string{"success", "accepted"} {
		if value, ok := result.Response[field].(bool); ok && !value {
			return nil, fmt.Errorf("node rejected %s", path)
		}
	}
	return result.Response, nil
}
func (s *Service) ApplyCommandJob(ctx context.Context, job *database.Job) error {
	var node database.Node
	if err := s.db.Where("uuid=?", job.Target).First(&node).Error; err != nil {
		return err
	}
	if node.IsDisabled {
		return fmt.Errorf("node disabled")
	}
	client := s.Client()
	if client == nil {
		return fmt.Errorf("node client unavailable")
	}
	var command Command
	if err := json.Unmarshal([]byte(job.Payload), &command); err != nil {
		return err
	}
	if job.Kind == "plugin-sync" {
		var plugin database.NodePlugin
		if node.ActivePluginUUID == nil {
			return nil
		}
		if err := s.db.Where("uuid=?", *node.ActivePluginUUID).First(&plugin).Error; err != nil {
			return err
		}
		config, err := configrender.Plugin(s.db, plugin.PluginConfig)
		if err != nil {
			return err
		}
		command.Path = "/node/plugin/sync"
		command.Body = map[string]interface{}{"plugin": map[string]interface{}{"uuid": plugin.UUID, "name": plugin.Name, "config": config}}
	}
	result, err := client.Call(ctx, &node, command.Path, command.Body)
	if err != nil {
		return err
	}
	if job.Kind == "geocheck" {
		result = map[string]interface{}{"success": true, "nodeUuid": node.UUID, "image": result["image"], "rawReport": result}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.db.Model(&database.Job{}).Where("id=? AND state='running' AND lease_until=?", job.ID, job.LeaseUntil).Update("result", string(data)).Error
}
