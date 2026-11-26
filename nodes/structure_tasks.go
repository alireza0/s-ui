package nodes

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func leaseStructure(node model.Node) (func(error), bool, error) {
	noop := func(error) { /* No lease was acquired, so there is nothing to release. */ }
	if node.TemplateTargetVersion == 0 {
		return noop, true, nil
	}
	db := database.GetDB()
	if err := db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		if err := tx.First(&current, node.Id).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "kind"}}, DoNothing: true}).Create(&model.NodeTask{NodeId: node.Id, Kind: "structure", Generation: 1, State: "queued"}).Error
	}); err != nil {
		return noop, false, err
	}
	var task model.NodeTask
	if err := db.First(&task, "node_id = ? AND kind = ?", node.Id, "structure").Error; err != nil {
		return noop, false, err
	}
	if task.NextAttempt > time.Now().Unix() || task.State == "rejected" {
		return noop, false, nil
	}
	id, err := uuid.NewV4()
	if err != nil {
		return noop, false, err
	}
	token := id.String()
	result := db.Model(&model.NodeTask{}).Where("id = ? AND generation = ? AND lease_until < ?", task.Id, task.Generation, time.Now().Unix()).Updates(map[string]interface{}{"lease_token": token, "lease_until": time.Now().Unix() + 90})
	if result.Error != nil || result.RowsAffected == 0 {
		return noop, false, result.Error
	}
	finish := func(failure error) {
		_ = db.Transaction(func(tx *gorm.DB) error { return finishStructureLease(tx, task, token, failure) })
	}
	return finish, true, nil
}

func finishStructureLease(tx *gorm.DB, task model.NodeTask, token string, failure error) error {
	var current model.NodeTask
	if err := tx.First(&current, "id = ? AND lease_token = ?", task.Id, token).Error; err != nil {
		return err
	}
	updates := map[string]interface{}{"lease_token": "", "lease_until": 0, "state": "queued", "next_attempt": 0}
	if current.Generation != task.Generation {
		return tx.Model(&current).Updates(updates).Error
	}
	if failure == nil {
		updates["state"], updates["attempts"], updates["last_error"] = "done", 0, ""
	} else if errors.Is(failure, errListenerDraftChanged) {
		return tx.Model(&current).Updates(updates).Error
	} else if errors.Is(failure, errNodeDomainPending) || errors.Is(failure, errNodeCertificatePending) || errors.Is(failure, errAgentProtocolsPending) || errors.Is(failure, errAgentBootstrapPending) {
		updates["state"], updates["attempts"] = "waiting", 0
		for _, reason := range []error{errNodeDomainPending, errNodeCertificatePending, errAgentProtocolsPending, errAgentBootstrapPending} {
			if errors.Is(failure, reason) {
				updates["last_error"] = reason.Error()
				break
			}
		}
		updates["next_attempt"] = time.Now().Unix() + 15
	} else {
		updates["attempts"], updates["last_error"] = current.Attempts+1, listenerFailureCode(failure)
		updates["next_attempt"] = time.Now().Unix() + (int64(1) << min(current.Attempts, 5)) + int64(rand.IntN(3))
		if errors.Is(failure, errV2Rejected) || errors.Is(failure, errRealityHandshakeLoop) || failure.Error() == "structural_revision_rejected" {
			updates["state"] = "rejected"
		}
	}
	return tx.Model(&current).Updates(updates).Error
}
