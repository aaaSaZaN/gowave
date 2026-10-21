package nodes

import (
	"time"

	"gowave/internal/database"

	"gorm.io/gorm"
)

// Ways to start the counter when traffic tracking is enabled. Gowave
// extension of UpdateNodeCommand ("trafficTrackingStart"); without it the
// counter keeps everything accumulated since the last reset, as upstream does.
const (
	TrackingStartKeep   = "keep"
	TrackingStartZero   = "zero"
	TrackingStartPeriod = "period"
)

// trafficResetHour matches the daily 01:00 UTC "reset/nodes" cron job.
const trafficResetHour = 1

func ValidTrackingStart(mode string) bool {
	return mode == TrackingStartKeep || mode == TrackingStartZero || mode == TrackingStartPeriod
}

// trafficPeriodStart returns when the current traffic period began: the
// latest reset at or before now. Like the reset job, a reset day beyond the
// end of a short month falls on its last day.
func trafficPeriodStart(now time.Time, resetDay int) time.Time {
	now = now.UTC()
	start := trafficResetAt(now.Year(), now.Month(), resetDay)
	if now.Before(start) {
		start = trafficResetAt(now.Year(), now.Month()-1, resetDay)
	}
	return start
}

func trafficResetAt(year int, month time.Month, resetDay int) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := first.AddDate(0, 1, -1).Day()
	if resetDay < 1 {
		resetDay = 1
	}
	if resetDay > lastDay {
		resetDay = lastDay
	}
	return time.Date(first.Year(), first.Month(), resetDay, trafficResetHour, 0, 0, 0, time.UTC)
}

// applyTrackingStart sets the counter of a node whose traffic tracking was
// just enabled: "zero" starts from nothing and "period" counts the current
// period from the usage history.
func applyTrackingStart(tx *gorm.DB, nodeUUID, mode string, now time.Time) error {
	if mode != TrackingStartZero && mode != TrackingStartPeriod {
		return nil
	}
	var node database.Node
	if err := tx.Where("uuid = ?", nodeUUID).First(&node).Error; err != nil {
		return err
	}
	if !node.IsTrafficTrackingActive {
		return nil
	}
	used := int64(0)
	if mode == TrackingStartPeriod {
		var raw int64
		if err := tx.Model(&database.NodesUsageHistory{}).Select("COALESCE(SUM(total_bytes), 0)").
			Where("node_uuid = ? AND created_at >= ?", nodeUUID, trafficPeriodStart(now, node.TrafficResetDay)).
			Scan(&raw).Error; err != nil {
			return err
		}
		// History holds raw bytes; the counter applies the node multiplier.
		used = multiplyConsumption(node.NodeConsumptionMultiplier, raw)
	}
	return tx.Model(&database.Node{}).Where("uuid = ?", nodeUUID).Update("traffic_used_bytes", used).Error
}

// LifetimeTraffic returns the raw bytes each node has served since it was
// added, summed from the usage history, which no reset clears.
func (s *Service) LifetimeTraffic() (map[string]int64, error) {
	var rows []struct {
		NodeUUID string
		Total    int64
	}
	if err := s.db.Model(&database.NodesUsageHistory{}).
		Select("node_uuid, COALESCE(SUM(total_bytes), 0) AS total").
		Group("node_uuid").Scan(&rows).Error; err != nil {
		return nil, err
	}
	totals := make(map[string]int64, len(rows))
	for _, row := range rows {
		totals[row.NodeUUID] = row.Total
	}
	return totals, nil
}
