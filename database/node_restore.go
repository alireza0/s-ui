package database

import (
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

// An authenticated import authorizes the archived intent. Workers resume it
// above the agent's observed checkpoints; agent state is never edited or reset.
func prepareNodeRestore() error {
	return db.Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Model(&model.Node{}).Where("agent_protocol = ? AND control_epoch <> ''", 2).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Model(&model.Node{}).Where("id IN ?", ids).Updates(map[string]any{
			"restore_access_pending": true, "restore_certificate_pending": true,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.NodeTask{}).Where("node_id IN ? AND kind IN ?", ids, []string{"access", "structure", "resources"}).Updates(map[string]any{
			"lease_token": "", "lease_until": 0, "next_attempt": 0, "attempts": 0,
			"generation": gorm.Expr("generation + ?", 1), "state": "queued", "last_error": "",
		}).Error
	})
}
