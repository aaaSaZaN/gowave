package nodes

import (
	"cmp"
	"errors"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"gowave/internal/database"
	"gowave/internal/users"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UsageOptions mirrors USER_USAGE_IGNORE_BELOW_BYTES and
// SERVICE_DISABLE_USER_USAGE_RECORDS.
type UsageOptions struct {
	IgnoreBelowBytes   int64
	DisableUserRecords bool
}

// SetUsageOptions applies the environment's usage accounting switches.
func (s *Service) SetUsageOptions(options UsageOptions) { s.usage = options }

// multiplyConsumption ports upstream's nano multiplier: 0 counts nothing,
// 1e9 counts raw bytes, anything else is floored.
func multiplyConsumption(nano int64, bytes int64) int64 {
	switch nano {
	case 0:
		return 0
	case 1_000_000_000:
		return bytes
	}
	return int64(math.Floor(NanoToMultiplier(nano) * float64(bytes)))
}

type userUsage struct {
	id     uint64
	raw    int64
	billed int64
}

// recordUserUsage stores one stats poll (already reset on the agent): billed
// bytes go to the user's counters, raw bytes to per-node history. Users seen
// for the first time emit user.first_connected. It returns the number of
// users counted as online, like upstream's NODE_USERS_ONLINE.
//
// The agent resets its counters before this commit. A batch that cannot be
// written is kept in memory and added to the node's next poll, so a failed
// transaction (for example a deadlock between nodes reporting the same
// users) does not lose traffic; only a panel restart in between does, the
// same window upstream has between Redis and Postgres.
func (s *Service) recordUserUsage(node *database.Node, stats []NodeUserTraffic, now time.Time) int {
	raw := s.takePendingUsage(node.UUID)
	online := 0
	for _, stat := range stats {
		id, err := strconv.ParseUint(stat.Username, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		total := stat.Uplink + stat.Downlink
		if total < s.usage.IgnoreBelowBytes || total < 0 {
			continue
		}
		if raw == nil {
			raw = make(map[uint64]int64, len(stats))
		}
		raw[id] += total
		online++
	}
	usage := make([]userUsage, 0, len(raw))
	for id, total := range raw {
		usage = append(usage, userUsage{id: id, raw: total, billed: multiplyConsumption(node.ConsumptionMultiplier, total)})
	}
	// Every poll locks its users in ascending id order. Nodes report
	// overlapping users at the same time, and transactions that lock the
	// same rows in different orders deadlock on PostgreSQL.
	slices.SortFunc(usage, func(a, b userUsage) int { return cmp.Compare(a.id, b.id) })
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for start := 0; start < len(usage); start += 500 {
		batch := usage[start:min(start+500, len(usage))]
		var err error
		for attempt := 1; attempt <= usageWriteAttempts; attempt++ {
			if err = s.writeUsageBatch(node, batch, now, day); err == nil || !retryableTxError(err) {
				break
			}
		}
		if err != nil {
			log.Printf("[NODE STATS] record usage node=%s, kept %d users for the next poll: %v", node.UUID, len(batch), err)
			s.keepPendingUsage(node.UUID, batch)
		}
	}
	return online
}

const (
	usageWriteAttempts = 3
	// usageWriters bounds concurrent usage transactions. Hundreds of nodes
	// poll every few seconds; unbounded, their transactions (and the row
	// locks they wait on) took every pooled connection and API requests
	// queued behind them for seconds. Waiting polls hold no connection.
	usageWriters = 4
)

func (s *Service) writeUsageBatch(node *database.Node, batch []userUsage, now, day time.Time) error {
	s.usageOnce.Do(func() { s.usageSlots = make(chan struct{}, usageWriters) })
	s.usageSlots <- struct{}{}
	defer func() { <-s.usageSlots }()
	return s.db.Transaction(func(tx *gorm.DB) error {
		ids := make([]uint64, len(batch))
		for i, entry := range batch {
			ids[i] = entry.id
		}
		// Lock the rows up front, in id order, and skip users deleted since
		// the node reported them so their bytes are not retried forever.
		lock := tx.Model(&database.UserTraffic{}).Where("id IN ?", ids).Order("id")
		if tx.Dialector.Name() == "postgres" {
			lock = lock.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var existing []uint64
		if err := lock.Pluck("id", &existing).Error; err != nil {
			return err
		}
		if len(existing) == 0 {
			return nil
		}
		if len(existing) < len(batch) {
			kept := make([]userUsage, 0, len(existing))
			for _, entry := range batch {
				if _, found := slices.BinarySearch(existing, entry.id); found {
					kept = append(kept, entry)
				}
			}
			batch, ids = kept, existing
		}
		var first []uint64
		if err := tx.Model(&database.UserTraffic{}).Where("id IN ? AND first_connected_at IS NULL", ids).Pluck("id", &first).Error; err != nil {
			return err
		}
		rows := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*2+4)
		for i, entry := range batch {
			rows[i] = "SELECT CAST(? AS BIGINT) AS id, CAST(? AS BIGINT) AS b"
			args = append(args, entry.id, entry.billed)
		}
		args = append(args, now, now, node.UUID)
		if err := tx.Exec(`UPDATE user_traffic SET
			used_traffic_bytes = used_traffic_bytes + v.b,
			lifetime_used_traffic_bytes = lifetime_used_traffic_bytes + v.b,
			online_at = ?,
			first_connected_at = COALESCE(first_connected_at, ?),
			last_connected_node_uuid = ?
			FROM (`+strings.Join(rows, " UNION ALL ")+`) AS v
			WHERE user_traffic.id = v.id`, reorder(args)...).Error; err != nil {
			return err
		}
		if !s.usage.DisableUserRecords {
			history := make([]database.NodesUserUsageHistory, 0, len(batch))
			for _, entry := range batch {
				if entry.raw > 0 {
					history = append(history, database.NodesUserUsageHistory{NodeID: node.ID, UserID: entry.id, TotalBytes: uint64(entry.raw), CreatedAt: day, UpdatedAt: now})
				}
			}
			if len(history) > 0 {
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "node_id"}, {Name: "created_at"}, {Name: "user_id"}},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"total_bytes": gorm.Expr("nodes_user_usage_history.total_bytes + excluded.total_bytes"),
						"updated_at":  gorm.Expr("excluded.updated_at"),
					}),
				}).Create(&history).Error; err != nil {
					return err
				}
			}
		}
		if len(first) > 0 {
			return users.EmitForUsers(tx, first, "user.first_connected", nil, false)
		}
		return nil
	})
}

// retryableTxError reports PostgreSQL deadlocks and serialization failures,
// which succeed when the transaction is simply run again.
func retryableTxError(err error) bool {
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		switch state.SQLState() {
		case "40P01", "40001":
			return true
		}
	}
	return false
}

func (s *Service) takePendingUsage(nodeUUID string) map[uint64]int64 {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	pending := s.pendingUsage[nodeUUID]
	delete(s.pendingUsage, nodeUUID)
	return pending
}

func (s *Service) keepPendingUsage(nodeUUID string, batch []userUsage) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if s.pendingUsage == nil {
		s.pendingUsage = make(map[string]map[uint64]int64)
	}
	pending := s.pendingUsage[nodeUUID]
	if pending == nil {
		pending = make(map[uint64]int64, len(batch))
		s.pendingUsage[nodeUUID] = pending
	}
	for _, entry := range batch {
		pending[entry.id] += entry.raw
	}
}

// reorder moves the SET parameters (the last three) in front of the VALUES
// parameters, matching their position in the statement.
func reorder(args []interface{}) []interface{} {
	tail := args[len(args)-3:]
	return append(append([]interface{}{}, tail...), args[:len(args)-3]...)
}
