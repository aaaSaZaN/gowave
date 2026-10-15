package cron

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"remnawave-go/config"
	"remnawave-go/internal/database"
	"remnawave-go/internal/jobs"
	"remnawave-go/internal/notify"
	"remnawave-go/internal/users"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserSyncer interface {
	SyncUserToNodes(*database.User, *string)
	RemoveUserFromNodes(*database.User)
}

// Scheduler runs upstream's periodic tasks. Calendar tasks keep a cursor in
// scheduled_executions so a restart neither repeats nor skips a run: missed
// occurrences are caught up (bounded) the next time the scheduler ticks.
type Scheduler struct {
	db     *gorm.DB
	syncer UserSyncer
	cfg    *config.Config
	// nodeNotified mirrors upstream's in-memory TRAFFIC_NOTIFY latch.
	nodeNotified map[string]bool
	// nodeData formats a node for notifications; defaults to basic fields.
	nodeData func(*database.Node) interface{}
}

// SetNodeFormatter makes node events carry the API node payload.
func (s *Scheduler) SetNodeFormatter(format func(*database.Node) interface{}) { s.nodeData = format }

func New(db *gorm.DB, syncers ...UserSyncer) *Scheduler {
	s := &Scheduler{db: db, cfg: &config.Config{}, nodeNotified: map[string]bool{}}
	if len(syncers) > 0 {
		s.syncer = syncers[0]
	}
	return s
}

func (s *Scheduler) SetSyncer(syncer UserSyncer) { s.syncer = syncer }

// SetConfig supplies notification intervals and service switches.
func (s *Scheduler) SetConfig(cfg *config.Config) { s.cfg = cfg }

func (s *Scheduler) Start(ctx context.Context) {
	if err := s.Tick(time.Now().UTC()); err != nil {
		log.Printf("[SCHEDULE] %v", err)
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.Tick(now.UTC()); err != nil {
				log.Printf("[SCHEDULE] %v", err)
			}
		}
	}
}

// calendarTask fires at the instants produced by latest(now); fn receives the
// instant being processed.
type calendarTask struct {
	name   string
	latest func(now time.Time) time.Time
	step   func(instant time.Time) time.Time
	fn     func(tx *gorm.DB, instant time.Time) error
	// maxCatchUp bounds how many missed instants are replayed.
	maxCatchUp int
}

func daily(hour, minute int) (func(time.Time) time.Time, func(time.Time) time.Time) {
	return func(now time.Time) time.Time {
			at := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
			if at.After(now) {
				at = at.AddDate(0, 0, -1)
			}
			return at
		}, func(instant time.Time) time.Time {
			return instant.AddDate(0, 0, 1)
		}
}

func weekly(weekday time.Weekday, hour, minute int) (func(time.Time) time.Time, func(time.Time) time.Time) {
	return func(now time.Time) time.Time {
			at := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
			for at.Weekday() != weekday || at.After(now) {
				at = at.AddDate(0, 0, -1)
			}
			return at
		}, func(instant time.Time) time.Time {
			return instant.AddDate(0, 0, 7)
		}
}

func monthly(day, hour, minute int) (func(time.Time) time.Time, func(time.Time) time.Time) {
	return func(now time.Time) time.Time {
			at := time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, time.UTC)
			if at.After(now) {
				at = at.AddDate(0, -1, 0)
			}
			return at
		}, func(instant time.Time) time.Time {
			return instant.AddDate(0, 1, 0)
		}
}

func every(interval time.Duration) (func(time.Time) time.Time, func(time.Time) time.Time) {
	return func(now time.Time) time.Time { return now.Truncate(interval) },
		func(instant time.Time) time.Time { return instant.Add(interval) }
}

func (s *Scheduler) tasks() []calendarTask {
	var tasks []calendarTask
	add := func(name string, schedule func() (func(time.Time) time.Time, func(time.Time) time.Time), maxCatchUp int, fn func(*gorm.DB, time.Time) error) {
		latest, step := schedule()
		tasks = append(tasks, calendarTask{name: name, latest: latest, step: step, fn: fn, maxCatchUp: maxCatchUp})
	}
	add("reset/DAY", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return daily(0, 5) }, 1, s.resetStrategy("DAY"))
	add("reset/MONTH_ROLLING", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return daily(0, 10) }, 31, s.resetRolling)
	add("reset/WEEK", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return weekly(time.Monday, 0, 15) }, 1, s.resetStrategy("WEEK"))
	add("reset/MONTH", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return monthly(1, 0, 20) }, 1, s.resetStrategy("MONTH"))
	add("reset/nodes", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return daily(1, 0) }, 31, s.resetNodes)
	add("crm/billing", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return daily(17, 0) }, 1, s.billingNotifications)
	add("service/clean-jobs", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return every(time.Hour) }, 1, jobs.Cleanup)
	if s.cfg.Service.CleanUsageHistory {
		add("service/clean-usage", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return weekly(time.Monday, 0, 30) }, 1, s.cleanUsage)
	}
	if s.cfg.Notifications.ExpirationNotifications != nil {
		add("notify/expiration", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return every(time.Minute) }, 1440, s.expirationNotifications)
	}
	if s.cfg.Notifications.NotConnectedAfterHours != nil {
		add("notify/not-connected", func() (func(time.Time) time.Time, func(time.Time) time.Time) { return every(10 * time.Minute) }, 144, s.notConnectedNotifications)
	}
	return tasks
}

// Tick runs the continuous checks and every calendar task that is due.
func (s *Scheduler) Tick(now time.Time) error {
	now = now.UTC()
	var errs []error
	if err := s.reviewUsers(now); err != nil {
		errs = append(errs, err)
	}
	if s.cfg.Notifications.BandwidthUsageThresholds != nil {
		if err := s.thresholdNotifications(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := s.reviewNodes(); err != nil {
		errs = append(errs, err)
	}
	for _, task := range s.tasks() {
		if err := s.runCalendar(task, now); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", task.name, err))
		}
	}
	return errors.Join(errs...)
}

// runCalendar executes each missed instant after the stored cursor, one
// transaction per instant. A task seen for the first time only records its
// latest instant, so an imported database is not reset retroactively.
func (s *Scheduler) runCalendar(task calendarTask, now time.Time) error {
	latest := task.latest(now)
	id := "cursor/" + task.name
	var cursor database.ScheduledExecution
	err := s.db.Where("id = ?", id).First(&cursor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&database.ScheduledExecution{ID: id, ExecutedAt: latest}).Error
	}
	if err != nil {
		return err
	}
	next := task.step(cursor.ExecutedAt.UTC())
	if next.After(latest) {
		return nil
	}
	// Replay at most maxCatchUp of the most recent missed instants.
	var instants []time.Time
	for at := next; !at.After(latest); at = task.step(at) {
		instants = append(instants, at)
		if len(instants) > task.maxCatchUp {
			instants = instants[1:]
		}
	}
	for _, instant := range instants {
		instant := instant
		err := s.db.Transaction(func(tx *gorm.DB) error {
			// The conditional update is the lock: a concurrent instance that
			// already advanced the cursor makes this a no-op.
			result := tx.Model(&database.ScheduledExecution{}).Where("id = ? AND executed_at < ?", id, instant).Update("executed_at", instant)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			return task.fn(tx, instant)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// reviewUsers applies upstream's FIND_EXPIRED_USERS and
// FIND_EXCEEDED_TRAFFIC_USAGE_USERS; above 10,000 users events are skipped.
func (s *Scheduler) reviewUsers(now time.Time) error {
	svc := users.NewService(s.db)
	var expired []uint64
	if err := s.db.Model(&database.User{}).Where("status IN ? AND expire_at < ?", []string{"ACTIVE", "LIMITED"}, now).Pluck("id", &expired).Error; err != nil {
		return err
	}
	if len(expired) > 0 {
		if err := svc.SetStatusFor(expired, "EXPIRED", len(expired) < 10000); err != nil {
			return err
		}
	}
	var limited []uint64
	if err := s.db.Table("users").Joins("JOIN user_traffic ON users.id = user_traffic.id").
		Where("users.status = 'ACTIVE' AND users.traffic_limit_bytes > 0 AND user_traffic.used_traffic_bytes >= users.traffic_limit_bytes").
		Pluck("users.id", &limited).Error; err != nil {
		return err
	}
	if len(limited) > 0 {
		return svc.SetStatusFor(limited, "LIMITED", len(limited) < 10000)
	}
	return nil
}

// thresholdNotifications raises last_triggered_threshold to the highest
// crossed percentage and notifies once per crossing.
func (s *Scheduler) thresholdNotifications() error {
	type candidate struct {
		ID   uint64
		Used uint64
		Lim  uint64
		Last int
	}
	var rows []candidate
	if err := s.db.Table("users").Joins("JOIN user_traffic ON user_traffic.id = users.id").
		Select("users.id, user_traffic.used_traffic_bytes AS used, users.traffic_limit_bytes AS lim, users.last_triggered_threshold AS last").
		Where("users.status = 'ACTIVE' AND users.traffic_limit_bytes > 0").Order("users.created_at").Scan(&rows).Error; err != nil {
		return err
	}
	var ids []uint64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			best := 0
			for _, pct := range s.cfg.Notifications.BandwidthUsageThresholds {
				if pct > row.Last && float64(row.Used) >= float64(row.Lim)*float64(pct)/100 && pct > best {
					best = pct
				}
			}
			if best == 0 {
				continue
			}
			if err := tx.Model(&database.User{}).Where("id = ?", row.ID).Updates(map[string]interface{}{"last_triggered_threshold": best, "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
			ids = append(ids, row.ID)
		}
		return nil
	})
	if err != nil || len(ids) == 0 {
		return err
	}
	return users.EmitForUsers(s.db, ids, "user.bandwidth_usage_threshold_reached", nil, len(ids) >= 500)
}

func (s *Scheduler) expirationNotifications(tx *gorm.DB, instant time.Time) error {
	for _, hours := range s.cfg.Notifications.ExpirationNotifications {
		target := instant.Add(-time.Duration(hours) * time.Hour)
		var ids []uint64
		if err := tx.Model(&database.User{}).Where("expire_at >= ? AND expire_at < ?", target, target.Add(time.Minute)).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			continue
		}
		if err := users.EmitForUsers(tx, ids, "user.expiration", map[string]int{"expiration": hours}, len(ids) >= 500); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) notConnectedNotifications(tx *gorm.DB, instant time.Time) error {
	for _, hours := range s.cfg.Notifications.NotConnectedAfterHours {
		end := instant.Add(-time.Duration(hours) * time.Hour)
		var ids []uint64
		if err := tx.Table("users").Joins("JOIN user_traffic ON user_traffic.id = users.id").
			Where("users.status = 'ACTIVE' AND user_traffic.first_connected_at IS NULL AND user_traffic.online_at IS NULL AND users.created_at >= ? AND users.created_at < ?", end.Add(-10*time.Minute), end).
			Pluck("users.id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			continue
		}
		if err := users.EmitForUsers(tx, ids, "user.not_connected", map[string]int{"notConnectedAfterHours": hours}, len(ids) >= 500); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) resetStrategy(strategy string) func(*gorm.DB, time.Time) error {
	return func(tx *gorm.DB, _ time.Time) error {
		var ids []uint64
		if err := tx.Model(&database.User{}).Where("traffic_limit_strategy = ?", strategy).Pluck("id", &ids).Error; err != nil {
			return err
		}
		return users.NewService(tx).BulkResetTraffic(ids)
	}
}

// resetRolling resets MONTH_ROLLING users whose creation day is the instant's
// day; on short months the last day also covers later creation days.
func (s *Scheduler) resetRolling(tx *gorm.DB, instant time.Time) error {
	var records []database.User
	if err := tx.Select("id", "created_at").Where("traffic_limit_strategy = 'MONTH_ROLLING'").Find(&records).Error; err != nil {
		return err
	}
	lastDay := time.Date(instant.Year(), instant.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	var ids []uint64
	for _, u := range records {
		day := u.CreatedAt.UTC().Day()
		if day > lastDay {
			day = lastDay
		}
		if day == instant.Day() && u.CreatedAt.UTC().Format("2006-01") < instant.Format("2006-01") {
			ids = append(ids, u.ID)
		}
	}
	return users.NewService(tx).BulkResetTraffic(ids)
}

func (s *Scheduler) resetNodes(tx *gorm.DB, instant time.Time) error {
	lastDay := time.Date(instant.Year(), instant.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	var nodes []database.Node
	query := tx.Where("is_traffic_tracking_active = ?", true)
	if instant.Day() == lastDay {
		// On a short month's last day, also reset nodes scheduled for later days.
		query = query.Where("traffic_reset_day >= ?", lastDay)
	} else {
		query = query.Where("traffic_reset_day = ?", instant.Day())
	}
	if err := query.Find(&nodes).Error; err != nil {
		return err
	}
	for _, node := range nodes {
		if err := tx.Model(&node).Update("traffic_used_bytes", 0).Error; err != nil {
			return err
		}
		delete(s.nodeNotified, node.UUID)
		if err := jobs.EnqueueNode(tx, node.UUID, "node-start"); err != nil {
			return err
		}
	}
	return nil
}

// reviewNodes emits node.traffic_notify once per crossing of notifyPercent.
func (s *Scheduler) reviewNodes() error {
	var nodes []database.Node
	if err := s.db.Where("is_disabled = ? AND is_traffic_tracking_active = ? AND notify_percent > 0 AND traffic_limit_bytes > 0", false, true).Find(&nodes).Error; err != nil {
		return err
	}
	for i := range nodes {
		node := &nodes[i]
		percent := float64(node.TrafficUsedBytes) * 100 / float64(node.TrafficLimitBytes)
		switch {
		case percent >= float64(node.NotifyPercent) && !s.nodeNotified[node.UUID]:
			s.nodeNotified[node.UUID] = true
			var data interface{} = basicNodeData(node)
			if s.nodeData != nil {
				data = s.nodeData(node)
			}
			if err := notify.Emit(s.db, notify.Event{Name: "node.traffic_notify", Data: data}); err != nil {
				return err
			}
		case percent < float64(node.NotifyPercent):
			delete(s.nodeNotified, node.UUID)
		}
	}
	return nil
}

// basicNodeData carries the fields the node templates read.
func basicNodeData(node *database.Node) map[string]interface{} {
	port := 0
	if node.Port != nil {
		port = *node.Port
	}
	return map[string]interface{}{
		"uuid": node.UUID, "name": node.Name, "address": node.Address, "port": port,
		"trafficUsedBytes": node.TrafficUsedBytes, "trafficLimitBytes": node.TrafficLimitBytes,
		"trafficResetDay": node.TrafficResetDay, "notifyPercent": node.NotifyPercent,
		"isTrafficTrackingActive": node.IsTrafficTrackingActive, "countryCode": node.CountryCode,
	}
}

var billingWindows = []struct {
	event string
	days  int
}{
	{"crm.infra_billing_node_payment_in_7_days", 7},
	{"crm.infra_billing_node_payment_in_48hrs", 2},
	{"crm.infra_billing_node_payment_in_24hrs", 1},
	{"crm.infra_billing_node_payment_due_today", 0},
	{"crm.infra_billing_node_payment_overdue_24hrs", -1},
	{"crm.infra_billing_node_payment_overdue_48hrs", -2},
	{"crm.infra_billing_node_payment_overdue_7_days", -7},
}

func (s *Scheduler) billingNotifications(tx *gorm.DB, instant time.Time) error {
	day := time.Date(instant.Year(), instant.Month(), instant.Day(), 0, 0, 0, 0, time.UTC)
	for _, window := range billingWindows {
		from := day.AddDate(0, 0, window.days)
		var rows []struct {
			NodeName      string
			LoginURL      *string
			ProviderName  string
			NextBillingAt time.Time
		}
		if err := tx.Table("infra_billing_nodes AS ibn").
			Joins("LEFT JOIN nodes AS n ON n.uuid = ibn.node_uuid").
			Joins("JOIN infra_providers AS ip ON ip.uuid = ibn.provider_uuid").
			Select("COALESCE(n.name, ibn.name) AS node_name, ip.login_url, ip.name AS provider_name, ibn.next_billing_at").
			Where("ibn.next_billing_at >= ? AND ibn.next_billing_at < ?", from, from.AddDate(0, 0, 1)).
			Order("ibn.next_billing_at").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			login := "https://docs.rw"
			if row.LoginURL != nil && *row.LoginURL != "" {
				login = *row.LoginURL
			}
			if err := notify.Emit(tx, notify.Event{Name: window.event, Data: map[string]interface{}{
				"nodeName": row.NodeName, "providerName": row.ProviderName, "loginUrl": login,
				"nextBillingAt": row.NextBillingAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			}}); err != nil {
				return err
			}
		}
	}
	return nil
}

// cleanUsage truncates per-user usage history (SERVICE_CLEAN_USAGE_HISTORY).
func (s *Scheduler) cleanUsage(tx *gorm.DB, _ time.Time) error {
	if tx.Dialector.Name() == "postgres" {
		return tx.Exec("TRUNCATE nodes_user_usage_history").Error
	}
	return tx.Exec("DELETE FROM nodes_user_usage_history").Error
}
