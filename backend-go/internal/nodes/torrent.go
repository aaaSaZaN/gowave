package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"gowave/internal/database"
	"gowave/internal/jobs"
	"gowave/internal/notify"
	"gowave/internal/users"

	"gorm.io/gorm"
)

// enqueueReportCollection asks a worker to drain the node's torrent-blocker
// reports; one job per node and minute keeps repeated polls from piling up.
func (s *Service) enqueueReportCollection(node *database.Node, now time.Time) {
	revision := uint64(now.Truncate(time.Minute).Unix())
	if err := jobs.Enqueue(s.db, "torrent-collect", node.UUID, node.UUID, revision, map[string]string{"uuid": node.UUID}); err != nil {
		logf("[TORRENT] enqueue collection for %s: %v", node.UUID, err)
	}
}

// ApplyCollectJob stores collected reports and emits torrent_blocker.report.
// The agent clears reports when it answers, so a failed commit loses that
// batch, the same window upstream has.
func (s *Service) ApplyCollectJob(ctx context.Context, job *database.Job) error {
	node, err := s.GetByUUID(job.Target)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	client := s.Client()
	if client == nil {
		return errors.New("node client unavailable")
	}
	reports, err := client.CollectTorrentReports(node)
	if err != nil {
		return err
	}
	return s.storeTorrentReports(node, reports)
}

func (s *Service) storeTorrentReports(node *database.Node, reports []json.RawMessage) error {
	if len(reports) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		nodeData := formatNodeWith(tx, s, node)
		for _, raw := range reports {
			var report struct {
				ActionReport struct {
					UserID string `json:"userId"`
				} `json:"actionReport"`
			}
			if err := json.Unmarshal(raw, &report); err != nil {
				continue
			}
			id, err := strconv.ParseUint(report.ActionReport.UserID, 10, 64)
			if err != nil {
				continue
			}
			var user database.User
			if err := tx.Where("id = ?", id).First(&user).Error; err != nil {
				// Upstream drops reports for users that no longer exist.
				continue
			}
			if err := tx.Create(&database.TorrentBlockerReport{UserID: user.ID, NodeID: node.ID, Report: string(raw), CreatedAt: time.Now().UTC()}).Error; err != nil {
				return err
			}
			userData, err := users.EventData(tx, &user)
			if err != nil {
				return err
			}
			if err := notify.Emit(tx, notify.Event{Name: "torrent_blocker.report", Data: map[string]interface{}{
				"node": nodeData, "user": userData, "report": json.RawMessage(raw),
			}}); err != nil {
				return err
			}
		}
		return nil
	})
}
