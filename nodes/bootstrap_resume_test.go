package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
)

func TestStructureWaitsForBootstrapAccessAfterRestart(t *testing.T) {
	db, node, _ := fixture(t)
	directory := filepath.Join(t.TempDir(), "agent")
	manager := nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: directory}, &panelSafeRuntime{})
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	attachFixtureAgent(t, db, &node, manager)
	if err := Reconcile(t.Context(), node.Id); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}

	manager = nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: directory}, &panelSafeRuntime{})
	manager.RequireBootstrap("new-installation-operation")
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	server, identity := controlHandlerServer(t, nodeagent.HandlerV2(manager))
	if err := db.First(&node, node.Id).Error; err != nil {
		t.Fatal(err)
	}
	node.Address, node.ControlCA, node.ControlCertificate, node.ControlKey = server.URL, identity.CA, identity.Certificate, identity.Key
	if err := db.Save(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.NodeTask{}).Where("node_id = ? AND kind = ?", node.Id, "access").
		Update("lease_until", time.Now().Add(time.Minute).Unix()).Error; err != nil {
		t.Fatal(err)
	}
	if err := ReconcileAccess(t.Context(), node.Id); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(t.Context(), node.Id); !errors.Is(err, errAgentBootstrapPending) {
		t.Fatal("bootstrap must remain retryable", err)
	}
	var saved model.Node
	db.First(&saved, node.Id)
	if saved.RejectedRevision != 0 || saved.LastError == "structural_update_failed" {
		t.Fatal("waiting for access rejected a valid structure")
	}
	if err := db.Model(&model.NodeTask{}).Where("node_id = ? AND kind = ?", node.Id, "access").Update("lease_until", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := ReconcileAccess(t.Context(), node.Id); err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(t.Context(), node.Id); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status(); status.BootstrapPending || !status.Running {
		t.Fatal("bootstrap did not recover after the saved lease expired")
	}
}

func TestBootstrapRejectionBetweenStatusAndPutIsRetryable(t *testing.T) {
	db, node, _ := fixture(t)
	manager := nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: t.TempDir()}, &panelSafeRuntime{})
	manager.RequireBootstrap("new-installation-operation")
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	attachFixtureAgent(t, db, &node, manager)
	var reads, writes atomic.Int32
	handler := nodeagent.HandlerV2(manager)
	server, identity := controlHandlerServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/status" && reads.Add(1) == 1 {
			status := manager.Status()
			status.BootstrapPending = false
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		if r.URL.Path == "/v2/spec" {
			writes.Add(1)
		}
		handler.ServeHTTP(w, r)
	}))
	node.AgentProtocol, node.ControlEpoch = 2, manager.Status().Epoch
	node.Address, node.ControlCA, node.ControlCertificate, node.ControlKey = server.URL, identity.CA, identity.Certificate, identity.Key
	if err := db.Save(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.NodeTask{NodeId: node.Id, Kind: "access", Generation: 1, State: "queued", LeaseUntil: time.Now().Add(time.Minute).Unix()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Reconcile(context.Background(), node.Id); !errors.Is(err, errAgentBootstrapPending) {
		t.Fatal("bootstrap race was not retried", err)
	}
	if writes.Load() != 1 {
		t.Fatal("test did not reach the rejected PUT")
	}
	var saved model.Node
	db.First(&saved, node.Id)
	if saved.RejectedRevision != 0 {
		t.Fatal("bootstrap 422 was recorded as a permanent rejection")
	}
}
