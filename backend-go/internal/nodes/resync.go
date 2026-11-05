package nodes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"gowave/internal/database"
	"gowave/internal/jobs"

	"gorm.io/gorm"
)

// resyncBatchUsers bounds one add-users request and the rows read for it.
const resyncBatchUsers = 1000

// SyncAllUsersToNode schedules a full user reconciliation after a node
// (re)connects. One job sends every active user to the node in bulk, instead
// of one job and one request per user: with hundreds of nodes the per-user
// form queued users × nodes jobs on every mass reconnect.
func (s *Service) SyncAllUsersToNode(node *database.Node) {
	if node == nil {
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.Job{}).Where("target = ? AND kind = ? AND state = ?", node.UUID, "user-sync", "failed").Updates(map[string]interface{}{"state": "pending", "attempts": 0, "available_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		// A resync that has not started yet will already send current data.
		var queued int64
		if err := tx.Model(&database.Job{}).Where("target = ? AND kind = ? AND state = ?", node.UUID, "user-resync", "pending").Count(&queued).Error; err != nil {
			return err
		}
		if queued > 0 {
			return nil
		}
		return jobs.Enqueue(tx, "user-resync", node.UUID, node.UUID, uint64(time.Now().UnixNano()), map[string]string{"uuid": node.UUID})
	}); err != nil {
		log.Printf("[NODES] schedule reconciliation: %v", err)
	}
}

// ApplyUserResyncJob sends all active users to the node with add-users and
// removes active users that have no inbound on it, matching what per-user
// user-sync jobs do. Nodes without the bulk endpoints get per-user jobs.
func (s *Service) ApplyUserResyncJob(ctx context.Context, job *database.Job) error {
	var node database.Node
	if err := s.db.Where("uuid = ?", job.Target).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if node.IsDisabled {
		return nil
	}
	if !node.IsConnected {
		return errors.New("node offline")
	}
	client := s.Client()
	if client == nil {
		return errors.New("node client unavailable")
	}
	tags := s.GetNodeActiveInboundTags(&node)
	var after uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var batch []database.User
		if err := s.db.Select("id", "vless_uuid", "trojan_password", "ss_password").
			Where("status = ? AND id > ?", "ACTIVE", after).Order("id").Limit(resyncBatchUsers).Find(&batch).Error; err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		after = batch[len(batch)-1].ID
		add, remove, err := s.resyncPayloads(batch, tags)
		if err != nil {
			return err
		}
		if len(add.Users) > 0 {
			err = client.AddUsers(ctx, &node, add)
		}
		if err == nil && len(remove.Users) > 0 {
			err = client.RemoveUsers(ctx, &node, remove)
		}
		if errors.Is(err, ErrBulkUnsupported) {
			return s.enqueuePerUserSync(&node)
		}
		if err != nil {
			return err
		}
	}
}

type resyncInbound struct {
	UserID uint64 `gorm:"column:user_id"`
	InboundInfo
}

// resyncPayloads resolves the batch's inbounds on the node's active tags in
// one query and splits the users into those to add and those to remove.
func (s *Service) resyncPayloads(batch []database.User, tags []string) (AddUsersRequestPayload, RemoveUsersRequestPayload, error) {
	add := AddUsersRequestPayload{AffectedInboundTags: tags, Users: []BulkUser{}}
	remove := RemoveUsersRequestPayload{Users: []BulkUserRef{}}
	var rows []resyncInbound
	if len(tags) > 0 {
		ids := make([]uint64, len(batch))
		for i, user := range batch {
			ids[i] = user.ID
		}
		if err := s.db.Table("internal_squad_members m").
			Select("m.user_id, ib.tag, ib.type, ib.network, ib.security, ib.raw_inbound").
			Joins("JOIN internal_squad_inbounds si ON si.internal_squad_uuid = m.internal_squad_uuid").
			Joins("JOIN config_profile_inbounds ib ON ib.uuid = si.inbound_uuid").
			Where("m.user_id IN ? AND ib.tag IN ?", ids, tags).
			Scan(&rows).Error; err != nil {
			return add, remove, err
		}
	}
	byUser := make(map[uint64][]BulkUserInbound, len(batch))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		// Squads may share inbounds; a repeated tag makes the node reject it.
		key := fmt.Sprintf("%d/%s", row.UserID, row.Tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		inbound := BulkUserInbound{Type: row.Type, Tag: row.Tag}
		switch {
		case row.Type == "shadowsocks" && isSS2022(row.RawInbound):
			inbound.Type = "shadowsocks22"
		case row.Type == "vless":
			inbound.Flow = getVlessFlow(row.Network, row.Security, row.RawInbound)
		case row.Type != "trojan" && row.Type != "shadowsocks" && row.Type != "hysteria":
			continue
		}
		byUser[row.UserID] = append(byUser[row.UserID], inbound)
	}
	for _, user := range batch {
		id := strconv.FormatUint(user.ID, 10)
		inbounds := byUser[user.ID]
		if len(inbounds) == 0 {
			remove.Users = append(remove.Users, BulkUserRef{UserID: id, HashUUID: user.VlessUUID})
			continue
		}
		add.Users = append(add.Users, BulkUser{
			InboundData: inbounds,
			UserData:    BulkUserData{UserID: id, HashUUID: user.VlessUUID, VlessUUID: user.VlessUUID, TrojanPassword: user.TrojanPassword, SSPassword: user.SsPassword},
		})
	}
	return add, remove, nil
}

// enqueuePerUserSync is the fallback for nodes without add-users: the old
// one-job-per-user reconciliation.
func (s *Service) enqueuePerUserSync(node *database.Node) error {
	revision := uint64(time.Now().UnixNano())
	return s.db.Transaction(func(tx *gorm.DB) error {
		var users []database.User
		return tx.Select("id", "vless_uuid").Where("status = ?", "ACTIVE").FindInBatches(&users, 500, func(_ *gorm.DB, _ int) error {
			for _, user := range users {
				if err := jobs.Enqueue(tx, "user-sync", strconv.FormatUint(user.ID, 10), node.UUID, revision, jobs.UserSync{ID: user.ID, VlessUUID: user.VlessUUID}); err != nil {
					return err
				}
			}
			return nil
		}).Error
	})
}
