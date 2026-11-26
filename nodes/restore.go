package nodes

import (
	"encoding/json"
	"errors"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

func validateAccessCheckpoint(state model.NodeAccessState, status nodeagent.StatusV2) error {
	if status.AccessRevision > state.DesiredRevision {
		return errors.New("access_state_rollback_detected")
	}
	if state.DesiredRevision > 0 && status.AccessRevision == state.DesiredRevision {
		var planned nodeagent.AccessSnapshot
		if json.Unmarshal(state.DesiredSnapshot, &planned) != nil || planned.Digest() != status.AccessDigest {
			return errors.New("access_state_rollback_or_conflict_detected")
		}
	}
	return nil
}

func finishAccessRestore(tx *gorm.DB, node model.Node) error {
	if !node.RestoreAccessPending {
		return nil
	}
	return tx.Model(&node).Update("restore_access_pending", false).Error
}

func restoredCertificateCheckpoint(tx *gorm.DB, node model.Node, current *model.NodeCertificateBinding, remote *nodeagent.CertificateResourceStatus) (bool, error) {
	var registered model.Node
	if err := tx.First(&registered, node.Id).Error; err != nil {
		return false, err
	}
	if registered.ControlEpoch != node.ControlEpoch || registered.ManagementGeneration != node.ManagementGeneration || registered.Address != node.Address {
		return false, errors.New("certificate_target_changed")
	}
	if !registered.RestoreCertificatePending {
		return false, validateRemoteCertificateRevision(*current, remote)
	}
	if remote != nil {
		current.DesiredRevision = max(current.DesiredRevision, remote.Revision)
	}
	return true, nil
}
