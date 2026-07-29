package service

import (
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MergeNodeTraffic takes a snapshot's UserTraffic from a node and merges
// the delta into master's client counters. Uses node_client_traffics as
// baseline to avoid double-counting.
func (s *NodeService) MergeNodeTraffic(nodeID uint, traffic map[string]*UserTraffic) {
	if len(traffic) == 0 {
		return
	}

	db := database.GetDB()

	for clientName, t := range traffic {
		if t == nil {
			continue
		}
		// Skip zero-traffic entries (prevents stale snapshots from being applied)
		if t.Up == 0 && t.Down == 0 {
			continue
		}

		// Get or create baseline
		var baseline model.NodeClientTraffic
		err := db.Where("node_id = ? AND client_name = ?", nodeID, clientName).
			FirstOrCreate(&baseline, model.NodeClientTraffic{
				NodeId:     nodeID,
				ClientName: clientName,
			}).Error
		if err != nil {
			logger.Warning("traffic merge: failed to get baseline for ", clientName, ": ", err)
			continue
		}

		// Calculate delta (new value minus baseline)
		deltaUp := t.Up - baseline.Up
		deltaDown := t.Down - baseline.Down

		// If delta is negative, the node was reset — treat current as absolute
		if deltaUp < 0 {
			deltaUp = t.Up
		}
		if deltaDown < 0 {
			deltaDown = t.Down
		}

		// Skip if no new traffic
		if deltaUp <= 0 && deltaDown <= 0 {
			// Still update baseline to track node counter
			db.Model(&model.NodeClientTraffic{}).
				Where("node_id = ? AND client_name = ?", nodeID, clientName).
				Updates(map[string]interface{}{
					"up":   t.Up,
					"down": t.Down,
				})
			continue
		}

		// Add delta to master client counters
		updates := map[string]interface{}{}
		if deltaUp > 0 {
			updates["up"] = gorm.Expr("up + ?", deltaUp)
		}
		if deltaDown > 0 {
			updates["down"] = gorm.Expr("down + ?", deltaDown)
		}
		if err := db.Model(&model.Client{}).Where("name = ?", clientName).Updates(updates).Error; err != nil {
			logger.Warning("traffic merge: failed to update client ", clientName, ": ", err)
			continue
		}

		// Update baseline to current node values
		db.Model(&model.NodeClientTraffic{}).
			Where("node_id = ? AND client_name = ?", nodeID, clientName).
			Updates(map[string]interface{}{
				"up":   t.Up,
				"down": t.Down,
			})
	}
}

// CleanupNodeTraffic removes baselines for a deleted node.
func (s *NodeService) CleanupNodeTraffic(nodeID uint) {
	db := database.GetDB()
	db.Where("node_id = ?", nodeID).Delete(&model.NodeClientTraffic{})
}

// UpsertNodeClientTraffic is used for bulk upsert during initial sync.
func (s *NodeService) UpsertNodeClientTraffic(records []model.NodeClientTraffic) error {
	if len(records) == 0 {
		return nil
	}
	db := database.GetDB()
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}, {Name: "client_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"up", "down"}),
	}).Create(&records).Error
}
