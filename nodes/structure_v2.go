package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func prepareStructure(db *gorm.DB, node model.Node, status nodeagent.StatusV2, occupied map[listenerSocket]bool) (Snapshot, nodeagent.Structure, error) {
	var snapshot Snapshot
	var spec nodeagent.Structure
	err := db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		if err := tx.First(&current, node.Id).Error; err != nil {
			return err
		}
		if current.ManagementGeneration != node.ManagementGeneration || current.ControlEpoch != status.Epoch || current.Address != node.Address {
			return errors.New("structural_target_changed")
		}
		if !sameListenerDraft(node, current) {
			return errListenerDraftChanged
		}
		if err := materializeTemplate(tx, &current, occupied); err != nil {
			return err
		}
		fresh, err := Compile(tx, current)
		if err != nil {
			return err
		}
		if err := validateStructureProtocols(tx, current, status, fresh); err != nil {
			return err
		}
		fingerprint, err := snapshotFingerprint(fresh)
		if err != nil {
			return err
		}
		var existing Snapshot
		oldErr := json.Unmarshal(current.DesiredSnapshot, &existing)
		existingSpec := nodeagent.Structure{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Revision: existing.Deployment.Revision, Config: existing.Deployment.Config}
		conflict := status.SpecRevision == current.DesiredRevision && status.SpecDigest != existingSpec.Digest()
		if current.DesiredRevision == 0 || oldErr != nil || fingerprint != current.DesiredFingerprint || status.SpecRevision > current.DesiredRevision || conflict {
			revision := current.DesiredRevision
			if status.SpecRevision > revision {
				revision = status.SpecRevision
			}
			revision++
			fresh.Deployment.Revision = revision
			body, err := json.Marshal(fresh)
			if err != nil {
				return err
			}
			if err := tx.Model(&current).Updates(map[string]interface{}{"desired_revision": revision, "desired_fingerprint": fingerprint, "desired_snapshot": json.RawMessage(body)}).Error; err != nil {
				return err
			}
			snapshot = fresh
		} else {
			snapshot = existing
		}
		spec = nodeagent.Structure{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Revision: snapshot.Deployment.Revision, Config: snapshot.Deployment.Config}
		if err := spec.Validate(); err != nil {
			return err
		}
		body, _ := json.Marshal(snapshot)
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.NodeDeployment{NodeId: node.Id, Revision: spec.Revision, Fingerprint: fingerprint, Snapshot: json.RawMessage(body), Created: time.Now().Unix()}).Error
	})
	return snapshot, spec, err
}

func reconcileStructureV2(ctx context.Context, node model.Node) (failure error) {
	db := database.GetDB()
	// Access has its own delivery and error state. Reconcile it first so
	// revocations can still apply when a structural draft cannot compile.
	if err := ReconcileAccess(ctx, node.Id); err != nil {
		return err
	}
	finish, run, err := leaseStructure(node)
	if err != nil || !run {
		return err
	}
	defer func() { finish(failure) }()
	defer func() {
		if failure != nil && !errors.Is(failure, errListenerDraftChanged) && ctx.Err() == nil {
			target := db.Model(&model.Node{}).Where("id = ? AND management_generation = ? AND control_epoch = ?", node.Id, node.ManagementGeneration, node.ControlEpoch)
			if errors.Is(failure, errNodeDomainPending) || errors.Is(failure, errNodeCertificatePending) || errors.Is(failure, errAgentProtocolsPending) || errors.Is(failure, errAgentBootstrapPending) {
				target.Where("last_error = ?", "structural_update_failed").Update("last_error", "")
				return
			}
			target.Update("last_error", listenerFailureCode(failure))
		}
	}()
	client, closeClient, err := controlClient(ctx, node)
	if err != nil {
		return err
	}
	defer closeClient()
	status, err := getStatusV2(ctx, client, node)
	if err != nil {
		return err
	}
	if status.BootstrapPending {
		return errAgentBootstrapPending
	}
	if err := confirmNodeFirewall(ctx, db, node, status); err != nil {
		return err
	}
	if err := observeStructure(db, node, status); err != nil {
		return err
	}
	occupied, err := inspectChangedListeners(ctx, db, node)
	if err != nil {
		return err
	}
	snapshot, spec, err := prepareStructure(db, node, status, occupied)
	if err != nil {
		return err
	}
	if status.SpecRevision != spec.Revision || status.SpecDigest != spec.Digest() || !status.Running {
		status, err = deliverStructure(ctx, db, node, snapshot, spec, client, status)
		if err != nil {
			return err
		}
	}
	if status.SpecRevision != spec.Revision || status.SpecDigest != spec.Digest() || !status.Running {
		return errors.New("structural_ack_mismatch")
	}
	if err := confirmNodeFirewall(ctx, db, node, status); err != nil {
		return err
	}
	return observeStructure(db, node, status)
}

func observeStructure(db *gorm.DB, node model.Node, status nodeagent.StatusV2) error {
	if !status.Running || status.SpecRevision == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error { return observeStructureTransaction(tx, node, status) })
}

func reservationMatchesSnapshot(row model.NodePortReservation, snapshot Snapshot) bool {
	for _, in := range snapshot.Inbounds {
		var options struct {
			Port      int               `json:"listen_port"`
			Transport listenerTransport `json:"transport"`
			Network   string            `json:"network"`
		}
		_ = json.Unmarshal(in.Options, &options)
		network := listenerNetwork(in.Type, options.Transport, options.Network)
		if in.ConnectionKey == row.ConnectionKey && options.Port == row.Port && (network == row.Network || network == "both") {
			return true
		}
	}
	return false
}

func retireObservedReservations(tx *gorm.DB, id uint, snapshot Snapshot) error {
	var reservations []model.NodePortReservation
	if err := tx.Where(whereNodeID, id).Find(&reservations).Error; err != nil {
		return err
	}
	for _, row := range reservations {
		keep := reservationMatchesSnapshot(row, snapshot)
		if !keep {
			if err := tx.Delete(&row).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func observeStructureTransaction(tx *gorm.DB, node model.Node, status nodeagent.StatusV2) error {
	var deployment model.NodeDeployment
	if err := tx.First(&deployment, "node_id = ? AND revision = ?", node.Id, status.SpecRevision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var snapshot Snapshot
	if json.Unmarshal(deployment.Snapshot, &snapshot) != nil {
		return errors.New("invalid_saved_structural_delivery")
	}
	spec := nodeagent.Structure{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Revision: status.SpecRevision, Config: snapshot.Deployment.Config}
	if spec.Digest() != status.SpecDigest {
		return errors.New("structural_observation_digest_mismatch")
	}
	updates := map[string]interface{}{"applied_revision": status.SpecRevision, "applied_snapshot": deployment.Snapshot, "applied_template_version": snapshot.TemplateVersion, "last_seen": time.Now().Unix(), "tls_status": certificateReport(status.SpecRevision, status.Certificates)}
	var current model.Node
	if err := tx.First(&current, node.Id).Error; err != nil {
		return err
	}
	if current.ManagementGeneration != node.ManagementGeneration {
		return errors.New("management_target_changed")
	}
	if current.TemplateId == snapshot.TemplateId && current.TemplateTargetVersion == snapshot.TemplateVersion && current.SettingsVersion == snapshot.SettingsVersion {
		updates["last_error"], updates["rejected_revision"] = "", 0
	}
	result := tx.Model(&model.Node{}).Where("id = ? AND control_epoch = ? AND address = ? AND management_generation = ? AND applied_revision <= ?", node.Id, node.ControlEpoch, node.Address, node.ManagementGeneration, status.SpecRevision).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	// Do not free reservations for a newer materialized target while an old ACK arrives.
	if result.RowsAffected > 0 && current.TemplateTargetVersion > 0 && current.TemplateId == snapshot.TemplateId && current.TemplateTargetVersion == snapshot.TemplateVersion && current.SettingsVersion == snapshot.SettingsVersion {
		if err := retireObservedReservations(tx, node.Id, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func validateStructureProtocols(tx *gorm.DB, current model.Node, status nodeagent.StatusV2, fresh Snapshot) error {
	for _, inbound := range fresh.Inbounds {
		if nodeagent.ProxyInbound(inbound.Type) && !proxyInboundsAvailable(tx, current, status) {
			return errAgentProtocolsPending
		}
		if nodeagent.ExtendedInbound(inbound.Type) && !extendedInboundsAvailable(tx, current, status) {
			return errAgentProtocolsPending
		}
	}
	return nil
}

func deliverStructure(ctx context.Context, db *gorm.DB, node model.Node, snapshot Snapshot, spec nodeagent.Structure, client *http.Client, status nodeagent.StatusV2) (nodeagent.StatusV2, error) {
	var err error
	var current model.Node
	db.First(&current, node.Id)
	if current.RejectedRevision == spec.Revision && status.Running {
		return status, errors.New("structural_revision_rejected")
	}
	if err := ensureNodeFirewall(ctx, db, node, snapshot, false); err != nil {
		return status, err
	}
	status, err = putV2(ctx, client, node, "/v2/spec", spec)
	if err != nil {
		if errors.Is(err, errV2Rejected) {
			observed, readErr := getStatusV2(ctx, client, node)
			if readErr != nil {
				return status, readErr
			}
			if observed.BootstrapPending {
				return status, errAgentBootstrapPending
			}
			db.Model(&model.Node{}).Where("id = ? AND desired_revision = ? AND management_generation = ?", node.Id, spec.Revision, node.ManagementGeneration).Update("rejected_revision", spec.Revision)
			if cleanupErr := confirmNodeFirewall(ctx, db, node, observed); cleanupErr != nil {
				return status, cleanupErr
			}
		}
		return status, err
	}
	return status, nil
}
