package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

func TestExplicitRestoreResumesAccessAboveAgentCheckpoint(t *testing.T) {
	db, node, client := fixture(t)
	runtime := &panelSafeRuntime{}
	manager := nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: t.TempDir()}, runtime)
	requireNodeTestSuccess(t, manager.Restore())
	defer manager.Close()
	attachFixtureAgent(t, db, &node, manager)
	requireNodeTestSuccess(t, Reconcile(context.Background(), node.Id))
	var archived model.NodeAccessState
	requireNodeTestSuccess(t, db.First(&archived, whereNodeID, node.Id).Error)
	var old nodeagent.AccessSnapshot
	requireNodeTestSuccess(t, json.Unmarshal(archived.DesiredSnapshot, &old))
	requireNodeTestSuccess(t, db.Model(&client).Update("enable", false).Error)
	requireNodeTestSuccess(t, database.QueueClientNodeAccess(db, []uint{client.Id}))
	requireNodeTestSuccess(t, ReconcileAccess(context.Background(), node.Id))
	newer := manager.Status().AccessRevision
	// Simulate only the import's committed database state; the agent is untouched.
	requireNodeTestSuccess(t, db.Save(&archived).Error)
	requireNodeTestSuccess(t, db.Model(&client).Update("enable", true).Error)
	requireNodeTestSuccess(t, db.Model(&node).Update("restore_access_pending", true).Error)
	requireNodeTestSuccess(t, ReconcileAccess(context.Background(), node.Id))
	if manager.Status().AccessRevision <= newer || !bytes.Contains(runtime.body, []byte("11111111")) {
		t.Fatal("restored access did not resume above the live checkpoint")
	}
	requireNodeTestSuccess(t, db.First(&node, node.Id).Error)
	if node.RestoreAccessPending {
		t.Fatal("restore permission was not consumed with the new durable intent")
	}
	if err := manager.ApplyAccess(old); err == nil {
		t.Fatal("agent accepted a replay of the archived command")
	}
	requireNodeTestSuccess(t, db.Save(&archived).Error)
	if err := ReconcileAccess(context.Background(), node.Id); err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatal("ordinary rollback inherited an already consumed restore permission", err)
	}
}

func TestExplicitRestoreResumesCertificateAboveAgentCheckpoint(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	db, node, _ := fixture(t)
	node.ControlEpoch = "fixture-epoch"
	node.RestoreCertificatePending = true
	requireNodeTestSuccess(t, db.Save(&node).Error)
	domain := model.NodeDomainBinding{NodeId: node.Id, FQDN: "node.example.com", Generation: 1}
	binding := model.NodeCertificateBinding{NodeId: node.Id, Generation: 1, DesiredRevision: 2}
	requireNodeTestSuccess(t, db.Create(&domain).Error)
	requireNodeTestSuccess(t, db.Create(&binding).Error)
	request := nodeagent.CertificateRequest{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Domain: domain.FQDN, BindingGeneration: 1, DomainGeneration: 1}
	remote := &nodeagent.CertificateResourceStatus{Revision: 9, Digest: "newer-fixture-digest"}
	requireNodeTestSuccess(t, db.Transaction(func(tx *gorm.DB) error {
		return prepareCertificateTransaction(tx, node, domain, binding, remote, "restored-fixture", &request)
	}))
	if request.Revision != 10 || request.Domain != domain.FQDN || request.BindingGeneration != 1 {
		t.Fatal("restored certificate intent was not rebased without changing its settings")
	}
	requireNodeTestSuccess(t, db.First(&node, node.Id).Error)
	if node.RestoreCertificatePending {
		t.Fatal("certificate restore permission was not consumed")
	}
	remote.Revision = 11
	err := db.Transaction(func(tx *gorm.DB) error {
		return prepareCertificateTransaction(tx, node, domain, binding, remote, "restored-fixture", &request)
	})
	if err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatal("unmarked certificate rollback was accepted", err)
	}
}
