package subscription

import (
	"context"
	"testing"
	"time"

	"gowave/internal/database"
)

func TestHistoryWriterBatchesTrimsAndDrains(t *testing.T) {
	db, user := subscriptionDB(t)
	writer := newHistoryWriter(db)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { writer.Run(ctx); close(done) }()
	for !writer.isRunning() {
		time.Sleep(time.Millisecond)
	}

	// PostgreSQL keeps microseconds.
	base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	for i := 0; i < 30; i++ {
		agent := "agent"
		writer.add(database.UserSubscriptionRequestHistory{UserID: user.ID, UserAgent: &agent,
			SrrResponseType: "XRAY_JSON", RequestAt: base.Add(time.Duration(i) * time.Second)})
	}
	cancel()
	<-done
	writer.Wait(time.Second)

	var rows []database.UserSubscriptionRequestHistory
	if err := db.Where("user_id = ?", user.ID).Order("request_at ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != historyKeep {
		t.Fatalf("kept %d rows, want %d", len(rows), historyKeep)
	}
	if oldest := base.Add(6 * time.Second); !rows[0].RequestAt.Equal(oldest) {
		t.Errorf("oldest kept row at %v, want %v (the 6 oldest are trimmed)", rows[0].RequestAt, oldest)
	}

	// After the writer stopped, rows are written inline instead of queued.
	agent := "late"
	writer.add(database.UserSubscriptionRequestHistory{UserID: user.ID, UserAgent: &agent, SrrResponseType: "XRAY_JSON", RequestAt: time.Now().UTC()})
	var late int64
	db.Model(&database.UserSubscriptionRequestHistory{}).Where("user_agent = ?", "late").Count(&late)
	if late != 1 {
		t.Errorf("row added after shutdown was not written inline")
	}
}
