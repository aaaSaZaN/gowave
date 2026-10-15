package cron

import (
	"strings"
	"testing"
	"time"

	"remnawave-go/config"
	"remnawave-go/internal/database"
	"remnawave-go/internal/notify"
	"remnawave-go/internal/testdb"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB { return testdb.Open(t) }

func TestScheduledJobRetentionPreservesActiveAndRecentJobs(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	for _, job := range []database.Job{
		{ID: "old-completed", State: "completed", UpdatedAt: now.Add(-25 * time.Hour)},
		{ID: "recent-completed", State: "completed", UpdatedAt: now.Add(-23 * time.Hour)},
		{ID: "old-failed", State: "failed", UpdatedAt: now.Add(-8 * 24 * time.Hour)},
		{ID: "recent-failed", State: "failed", UpdatedAt: now.Add(-6 * 24 * time.Hour)},
		{ID: "pending", State: "pending", UpdatedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: "running", State: "running", UpdatedAt: now.Add(-30 * 24 * time.Hour)},
	} {
		if err := db.Create(&job).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := New(db)
	if err := s.Tick(now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.Tick(now); err != nil {
		t.Fatal(err)
	}
	var remaining []string
	if err := db.Model(&database.Job{}).Order("id").Pluck("id", &remaining).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(remaining, ",") != "pending,recent-completed,recent-failed,running" {
		t.Fatalf("remaining jobs: %v", remaining)
	}
}

func addUser(t *testing.T, db *gorm.DB, name, status, strategy string, expire time.Time, limit, used uint64) uint64 {
	t.Helper()
	user := database.User{ShortUUID: uuid.NewString()[:16], Username: name, Status: status, TrafficLimitStrategy: strategy,
		TrafficLimitBytes: limit, ExpireAt: expire, VlessUUID: uuid.NewString(), CreatedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.UserTraffic{ID: user.ID, UsedTrafficBytes: used}).Error; err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func used(t *testing.T, db *gorm.DB, id uint64) uint64 {
	var traffic database.UserTraffic
	db.First(&traffic, id)
	return traffic.UsedTrafficBytes
}

func TestCalendarResetsAreBaselinedCaughtUpAndIdempotent(t *testing.T) {
	db := testDB(t)
	future := time.Now().Add(365 * 24 * time.Hour)
	weekly := addUser(t, db, "weekly", "ACTIVE", "WEEK", future, 0, 500)
	s := New(db)

	// Wednesday: first sight of the task records a baseline, no reset.
	wednesday := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	if err := s.Tick(wednesday); err != nil {
		t.Fatal(err)
	}
	if used(t, db, weekly) != 500 {
		t.Fatal("first tick must not reset retroactively")
	}
	// The process was down over Monday 9 March; the next tick catches up once.
	thursday := time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC)
	if err := s.Tick(thursday); err != nil {
		t.Fatal(err)
	}
	if used(t, db, weekly) != 0 {
		t.Fatal("missed weekly reset was not caught up")
	}
	db.Model(&database.UserTraffic{}).Where("id = ?", weekly).Update("used_traffic_bytes", 42)
	if err := s.Tick(thursday.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if used(t, db, weekly) != 42 {
		t.Fatal("weekly reset ran twice for the same Monday")
	}
}

func TestRollingResetReplaysEveryMissedDay(t *testing.T) {
	db := testDB(t)
	future := time.Now().Add(365 * 24 * time.Hour)
	rolling := addUser(t, db, "rolling", "ACTIVE", "MONTH_ROLLING", future, 0, 900) // created on the 15th
	s := New(db)
	if err := s.Tick(time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	// Down on the 14th-16th; the 15th must still be processed.
	if err := s.Tick(time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if used(t, db, rolling) != 0 {
		t.Fatal("rolling reset for a missed creation day was skipped")
	}
}

func TestReviewUsersExpiresLimitsAndNotifies(t *testing.T) {
	db := testDB(t)
	notifier, err := notify.New(&config.Config{Notifications: config.NotificationsConfig{WebhookEnabled: true, WebhookURLs: []string{"https://h.example.com"}, WebhookSecretHeader: strings.Repeat("s", 32)}})
	if err != nil {
		t.Fatal(err)
	}
	notify.SetDefault(notifier)
	t.Cleanup(func() { notify.SetDefault(nil) })

	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(48*time.Hour)
	expiredActive := addUser(t, db, "expired_active", "ACTIVE", "NO_RESET", past, 0, 0)
	expiredLimited := addUser(t, db, "expired_limited", "LIMITED", "NO_RESET", past, 0, 0)
	overLimit := addUser(t, db, "over_limit", "ACTIVE", "NO_RESET", future, 100, 150)
	disabled := addUser(t, db, "disabled", "DISABLED", "NO_RESET", past, 0, 0)
	threshold := addUser(t, db, "threshold", "ACTIVE", "NO_RESET", future, 100, 85)

	s := New(db)
	s.SetConfig(&config.Config{Notifications: config.NotificationsConfig{BandwidthUsageThresholds: []int{50, 80}}})
	for i := 0; i < 2; i++ {
		if err := s.Tick(now); err != nil {
			t.Fatal(err)
		}
	}
	status := func(id uint64) string {
		var user database.User
		db.First(&user, id)
		return user.Status
	}
	if status(expiredActive) != "EXPIRED" || status(expiredLimited) != "EXPIRED" || status(overLimit) != "LIMITED" || status(disabled) != "DISABLED" {
		t.Fatalf("statuses: %s %s %s %s", status(expiredActive), status(expiredLimited), status(overLimit), status(disabled))
	}
	var user database.User
	db.First(&user, threshold)
	if user.LastTriggeredThreshold != 80 {
		t.Fatalf("threshold = %d, want 80", user.LastTriggeredThreshold)
	}
	var events []database.Job
	db.Where("kind = 'webhook'").Find(&events)
	count := map[string]int{}
	for _, job := range events {
		count[job.EntityID]++
	}
	if count["user.expired"] != 2 || count["user.limited"] != 1 || count["user.bandwidth_usage_threshold_reached"] != 1 {
		t.Fatalf("unexpected events %v", count)
	}
}
