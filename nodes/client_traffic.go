package nodes

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errTrafficAccounting = errors.New("traffic_accounting_failed")

func clientTrafficID(client model.Client) string {
	return fmt.Sprintf("%d:%d", client.Id, client.CreatedAt)
}

// Never reset these cursors when resetting a client's quota: an unchanged
// cumulative agent sample must not charge the previous period a second time.
func recordClientTraffic(db *gorm.DB, node model.Node, sample *nodeagent.TrafficSample, observed time.Time) error {
	if sample == nil || len(sample.Users) == 0 {
		return nil
	}
	if id, err := uuid.FromString(sample.Process); err != nil || id == uuid.Nil {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		found := tx.Where("id = ? AND address = ? AND control_epoch = ? AND management_generation = ?", node.Id, node.Address, node.ControlEpoch, node.ManagementGeneration).Find(&current)
		if found.Error != nil || found.RowsAffected == 0 {
			return found.Error
		}
		// Revoked sessions may still report final usage, without regaining access.
		clients, err := accountingClients(tx, current, true)
		if err != nil {
			return err
		}
		for _, client := range clients {
			usage, ok := sample.Users[clientTrafficID(client)]
			if !ok || usage.Up < 0 || usage.Down < 0 {
				continue
			}
			if err := chargeNodeTraffic(tx, current.Id, client, sample.Process, usage, observed); err != nil {
				return err
			}
		}
		return nil
	})
}

func chargeNodeTraffic(tx *gorm.DB, nodeID uint, client model.Client, ledger string, usage nodeagent.UserTraffic, observed time.Time) error {
	previous := model.NodeClientTraffic{NodeId: nodeID, ClientId: client.Id, Ledger: ledger}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&previous).Error; err != nil {
		return err
	}
	if err := tx.First(&previous, "node_id = ? AND client_id = ? AND ledger = ?", nodeID, client.Id, ledger).Error; err != nil {
		return err
	}
	if usage.Up < previous.Up || usage.Down < previous.Down {
		return nil
	}
	up, down := usage.Up-previous.Up, usage.Down-previous.Down
	if up == 0 && down == 0 {
		return nil
	}
	if err := tx.Model(&previous).Updates(map[string]any{"up": usage.Up, "down": usage.Down}).Error; err != nil {
		return err
	}
	// Match S-UI's existing counters and deplete job, including local inbounds.
	updates := map[string]any{
		"up":   gorm.Expr("up + MIN(?, ? - MAX(up, 0))", up, int64(math.MaxInt64)),
		"down": gorm.Expr("down + MIN(?, ? - MAX(down, 0))", down, int64(math.MaxInt64)),
	}
	if usage.At >= client.CreatedAt && usage.At > 0 && usage.At <= observed.Add(5*time.Minute).Unix() {
		updates["online_at"] = gorm.Expr("MAX(COALESCE(online_at, 0), ?)", min(usage.At, observed.Unix()))
	}
	return tx.Model(&model.Client{}).Where("id = ?", client.Id).Updates(updates).Error
}
