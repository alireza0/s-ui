package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
)

type panelSafeRuntime struct {
	testRuntime
	body []byte
}

func (r *panelSafeRuntime) ApplySafe(body []byte) error {
	err := r.Apply(body)
	if err == nil {
		r.body = append([]byte(nil), body...)
	}
	return err
}
func (r *panelSafeRuntime) Interrupt() {}

func TestV2PanelRevokesWithInvalidDraftAndProvisionJob(t *testing.T) {
	db, node, client := fixture(t)
	runtime := &panelSafeRuntime{}
	manager := nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: filepath.Join(t.TempDir(), "agent")}, runtime)
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	attachFixtureAgent(t, db, &node, manager)
	if err := Reconcile(context.Background(), node.Id); err != nil {
		t.Fatal(err)
	}
	if manager.Status().Protocol != 2 || !manager.Status().Running || !bytes.Contains(runtime.body, []byte("11111111")) {
		t.Fatal("initial v2 deployment not working")
	}
	if links, err := ClientInbounds(db, client); err != nil || len(links) != 1 {
		t.Fatal("confirmed v2 access not published", err)
	}
	before := manager.Status().SpecRevision
	if err := db.Model(&model.Node{}).Where("id = ?", node.Id).Update("template_target_version", 999).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Client{}).Where("id = ?", client.Id).Updates(map[string]interface{}{"enable": false, "node_generation": 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.QueueClientNodeAccess(db, []uint{client.Id}); err != nil {
		t.Fatal(err)
	}
	provisionJobs.Store(node.Id, true)
	defer provisionJobs.Delete(node.Id)
	requireNodeTestSuccess(t, database.QueueNodeTasks(db, []uint{node.Id}, "structure"))
	if err := ReconcileAccess(context.Background(), node.Id); err != nil {
		t.Fatal("provision job blocked access", err)
	}
	if err := Reconcile(context.Background(), node.Id); err == nil {
		t.Fatal("invalid draft unexpectedly compiled")
	}
	if bytes.Contains(runtime.body, []byte("11111111")) || !manager.Status().Running || manager.Status().SpecRevision != before {
		t.Fatal("bad structure blocked revoke or stopped last-good")
	}
	if links, err := ClientInbounds(db, client); err != nil || len(links) != 0 {
		t.Fatal("revoked user remains in publication", err)
	}
	var access model.NodeAccessState
	db.First(&access, "node_id = ?", node.Id)
	if access.AppliedRevision != access.DesiredRevision || access.LastError != "" {
		t.Fatal("ACL not acknowledged independently")
	}
}

func TestV2PanelRefusesOlderRestoredAccessState(t *testing.T) {
	db, node, client := fixture(t)
	runtime := &panelSafeRuntime{}
	manager := nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: t.TempDir()}, runtime)
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	attachFixtureAgent(t, db, &node, manager)
	if err := Reconcile(context.Background(), node.Id); err != nil {
		t.Fatal(err)
	}
	db.Model(&model.Client{}).Where("id = ?", client.Id).Update("enable", false)
	database.QueueClientNodeAccess(db, []uint{client.Id})
	if err := ReconcileAccess(context.Background(), node.Id); err != nil {
		t.Fatal(err)
	}
	db.Model(&model.NodeAccessState{}).Where("node_id = ?", node.Id).Update("desired_revision", 0)
	db.Model(&model.Client{}).Where("id = ?", client.Id).Update("enable", true)
	database.QueueClientNodeAccess(db, []uint{client.Id})
	if err := ReconcileAccess(context.Background(), node.Id); err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatal("old panel snapshot silently replaced newer ACL", err)
	}
	if bytes.Contains(runtime.body, []byte("11111111")) {
		t.Fatal("restored panel resurrected old user")
	}
}

func TestV2PanelRefusesEqualRevisionWithDifferentIntentDigest(t *testing.T) {
	db, node, _ := fixture(t)
	runtime := &panelSafeRuntime{}
	manager := nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: t.TempDir()}, runtime)
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	attachFixtureAgent(t, db, &node, manager)
	if err := Reconcile(context.Background(), node.Id); err != nil {
		t.Fatal(err)
	}
	var state model.NodeAccessState
	db.First(&state, "node_id = ?", node.Id)
	var altered nodeagent.AccessSnapshot
	json.Unmarshal(state.DesiredSnapshot, &altered)
	altered.Clients = []nodeagent.Grant{}
	body, _ := json.Marshal(altered)
	db.Model(&state).Update("desired_snapshot", json.RawMessage(body))
	database.QueueNodeTasks(db, []uint{node.Id}, "access")
	if err := ReconcileAccess(context.Background(), node.Id); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatal("conflicting restored payload accepted", err)
	}
	if manager.Status().AccessRevision != 1 {
		t.Fatal("conflict allocated a new permissive revision")
	}
}

func TestV2PublicationHidesConfirmedStopAndReturnsAfterRecovery(t *testing.T) {
	db, node, client := fixture(t)
	runtime := &panelSafeRuntime{}
	manager := nodeagent.NewManagerV2(uint64(node.Id), nodeagent.FileStoreV2{Directory: t.TempDir()}, runtime)
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	attachFixtureAgent(t, db, &node, manager)
	if err := Reconcile(context.Background(), node.Id); err != nil {
		t.Fatal(err)
	}
	if links, err := ClientInbounds(db, client); err != nil || len(links) != 1 {
		t.Fatal("initial publication missing", err)
	}
	manager.Close()
	runtime.reject = true
	database.QueueNodeTasks(db, []uint{node.Id}, "access")
	if err := ReconcileAccess(context.Background(), node.Id); err == nil {
		t.Fatal("failed runtime unexpectedly recovered")
	}
	if links, err := ClientInbounds(db, client); err != nil || len(links) != 0 {
		t.Fatal("known stopped core remained published", err)
	}
	runtime.reject = false
	database.QueueNodeTasks(db, []uint{node.Id}, "access")
	if err := ReconcileAccess(context.Background(), node.Id); err != nil {
		t.Fatal(err)
	}
	if links, err := ClientInbounds(db, client); err != nil || len(links) != 1 {
		t.Fatal("recovered core did not return publication", err)
	}
}
