package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

func TestV2NodeDeletionRemovesGrantsAndControlState(t *testing.T) {
	db, node, client := fixture(t)
	addFixtureAssignment(t, db, node.Id)
	node.Enable = false
	node.DesiredRevision, node.AppliedRevision = 3, 3
	node.AppliedSnapshot, _ = json.Marshal(Snapshot{Node: Publication{Enable: false}})
	requireNodeTestSuccess(t, db.Save(&node).Error)
	rows := []interface{}{
		&model.NodeAccessState{NodeId: node.Id, DesiredRevision: 2},
		&model.NodeClientTraffic{NodeId: node.Id, ClientId: client.Id, Ledger: "11111111-1111-4111-8111-111111111111", Up: 10, Down: 20},
		&model.NodeTask{NodeId: node.Id, Kind: "access"},
		&model.NodeOverride{NodeId: node.Id, ConnectionKey: "one", Path: "/listen_port"},
		&model.NodeGeneratedMaterial{NodeId: node.Id, ConnectionKey: "one", Purpose: "key"},
		&model.NodeInstallation{NodeId: node.Id},
		&model.NodeJob{Id: "done", NodeId: node.Id, State: "done"},
	}
	for _, row := range rows {
		requireNodeTestSuccess(t, db.Create(row).Error)
	}
	requireNodeTestSuccess(t, Save("nodes", "del", json.RawMessage(`1`), "test"))
	for _, row := range append(rows, &model.NodeInbound{}, &model.ClientNode{}) {
		var count int64
		if err := db.Model(row).Where("node_id = ?", node.Id).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("orphaned %T: %d, %v", row, count, err)
		}
	}
	var count int64
	if err := ReconcileAccess(context.Background(), node.Id); err == nil {
		t.Fatal("deleted node reconciled")
	}
	if err := db.Model(&model.NodeAccessState{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("reconcile resurrected access state", count, err)
	}
	// A database restore may reuse a deleted numeric ID. It must have no grants.
	node = model.Node{Id: 1, AssignmentMode: model.NodeAssignmentWhole}
	requireNodeTestSuccess(t, db.Create(&node).Error)
	if err := db.Model(&model.ClientNode{}).Where("node_id = ?", node.Id).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("new node inherited old client rights", count, err)
	}
}

func TestNodeDeletionAfterReinstallRequiresDisabledNodeAndExplicitConfirmation(t *testing.T) {
	db, node, _ := fixture(t)
	request := json.RawMessage(`{"id":1,"server_already_stopped":true}`)
	if err := DeleteNode(t.Context(), request, "unit"); err == nil || err.Error() != "node_disable_pending_delete" {
		t.Fatal("manual stop confirmation bypassed the enabled-node guard", err)
	}
	requireNodeTestSuccess(t, db.Model(&node).Update("enable", false).Error)
	if err := DeleteNode(t.Context(), json.RawMessage(`1`), "unit"); err == nil || err.Error() != "node_disable_pending_delete" {
		t.Fatal("unacknowledged node disappeared without confirmation", err)
	}
	requireNodeTestSuccess(t, DeleteNode(t.Context(), request, "unit"))
	var remaining int64
	if err := db.Model(&model.Node{}).Where("id = ?", node.Id).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatal("confirmed deleted OS node remains", err)
	}
	var audit model.Changes
	requireNodeTestSuccess(t, db.Where("key = ? AND action = ?", "nodes", "del").Last(&audit).Error)
	var recorded nodeDeleteRequest
	if json.Unmarshal(audit.Obj, &recorded) != nil || !recorded.ServerAlreadyStopped {
		t.Fatal("manual stop confirmation was not audited")
	}
}

func TestV2NodeDeletionWaitsForProvisioning(t *testing.T) {
	db, node, _ := fixture(t)
	node.Enable = false
	node.DesiredRevision, node.AppliedRevision = 3, 3
	node.AppliedSnapshot, _ = json.Marshal(Snapshot{Node: Publication{Enable: false}})
	requireNodeTestSuccess(t, db.Save(&node).Error)
	provisionJobs.Store(node.Id, true)
	defer provisionJobs.Delete(node.Id)
	if err := Save("nodes", "del", json.RawMessage(`1`), "test"); err == nil {
		t.Fatal("node deleted during provisioning")
	}

}

func TestV2NodeRecreatedAtSameAddressStartsWithFreshSSHIdentity(t *testing.T) {
	db, node, _ := fixture(t)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	previous := model.NodeInstallation{
		NodeId: node.Id, IP: "192.0.2.30", SSHPort: 22, SSHUser: "root",
		OwnerID: "old-owner-fixture", HostKey: "old-host-fixture", PrivateKey: "old-ssh-fixture",
	}
	requireNodeTestSuccess(t, db.Create(&previous).Error)
	request := json.RawMessage(`1`)
	if err := DeleteNode(t.Context(), request, "test"); err == nil || err.Error() != "node_disable_pending_delete" {
		t.Fatalf("unconfirmed revocation allowed deletion: %v", err)
	}
	node.Enable = false
	node.AppliedRevision, node.DesiredRevision = 3, 3
	node.AppliedSnapshot, _ = json.Marshal(Snapshot{Node: Publication{Enable: false}})
	requireNodeTestSuccess(t, db.Save(&node).Error)
	requireNodeTestSuccess(t, DeleteNode(t.Context(), request, "test"))
	job, created, err := createLiveProvision(ProvisionRequest{
		ModelVersion: 2, RequestKey: "fresh-install-fixture", Name: node.Name,
		IP: previous.IP, SSHPort: previous.SSHPort, SSHUser: previous.SSHUser, Password: "fixture",
	}, "test")
	if err != nil || !created {
		t.Fatalf("fresh installation at the same address was blocked: %v", err)
	}
	plan, err := loadLivePlan(job)
	requireNodeTestSuccess(t, err)
	if plan.Target.OwnerID == "" || plan.Target.OwnerID == previous.OwnerID ||
		plan.Target.HostKey != "" || plan.Target.PrivateKey != "" {
		t.Fatal("fresh installation reused old SSH identity")
	}
}

func TestNodeDeletionWithStaleDNS(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		name := "provider reports record missing"
		if rejected {
			name = "administrator confirms manual cleanup"
		}
		t.Run(name, func(t *testing.T) {
			checkDeletionWithStaleDNS(t, rejected)
		})
	}
}

func TestLiveNodeCanShutDownAndBeDeletedWhileCertificateIsPending(t *testing.T) {
	f := liveSetup(t, 1)
	f.run(t, 0)
	if f.links(t) != 1 {
		t.Fatal("expected an applied listener before disabling the node")
	}
	spec := liveSpec()
	spec.Entries[0].Options = json.RawMessage(`{"listen":"127.0.0.1","tls":{"enabled":true}}`)
	spec.Entries[0].Sources["/tls/certificate_binding"] = ValueSource{Kind: "node_certificate", Name: "primary"}
	f.save(t, spec)
	if err := Reconcile(t.Context(), 1); !errors.Is(err, errNodeCertificatePending) {
		t.Fatal("expected the enabled draft to require its TLS resource", err)
	}
	requireNodeTestSuccess(t, Save("nodes", "edit", json.RawMessage(`{"id":1,"enable":false}`), "test"))
	if err := DeleteNode(t.Context(), json.RawMessage(`1`), "test"); err == nil || err.Error() != "node_disable_pending_delete" {
		t.Fatal("deletion accepted the previous enabled snapshot", err)
	}
	f.run(t, 0)
	node := f.nodes[0]
	status := f.managers[0].Status()
	if !disableConfirmed(node) || status.SpecRevision != node.AppliedRevision || f.links(t) != 0 {
		t.Fatal("agent did not confirm shutdown with the pending certificate")
	}
	var applied Snapshot
	requireNodeTestSuccess(t, json.Unmarshal(node.AppliedSnapshot, &applied))
	var config struct {
		Inbounds []json.RawMessage `json:"inbounds"`
	}
	if err := json.Unmarshal(applied.Deployment.Config, &config); err != nil || config.Inbounds == nil || len(config.Inbounds) != 0 {
		t.Fatal("disabled configuration did not remove the listeners", err)
	}
	var access model.NodeAccessState
	var grants nodeagent.AccessSnapshot
	requireNodeTestSuccess(t, f.db.First(&access, "node_id = ?", node.Id).Error)
	if err := json.Unmarshal(access.AppliedSnapshot, &grants); err != nil || grants.Clients == nil || len(grants.Clients) != 0 {
		t.Fatal("agent did not acknowledge empty access", err)
	}
	if err := DeleteNode(t.Context(), json.RawMessage(`1`), "test"); err != nil {
		t.Fatal("could not delete the disabled node", err)
	}
}

func checkDeletionWithStaleDNS(t *testing.T, rejected bool) {
	t.Helper()
	db, node, _ := fixture(t)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	cloud := newCloudFixture(t)
	cloud.missingStatus, cloud.rejectReads = http.StatusBadRequest, rejected
	profile := profileFixture(t, "token")
	record := model.NodeDNSRecord{NodeId: node.Id, ProfileId: profile.Id, ZoneId: fixtureZone, Name: "retired.example.com", RemoteId: "00000000000000000000000000000002", Type: "A", Content: "192.0.2.1", Owner: "fixture"}
	requireNodeTestSuccess(t, db.Create(&record).Error)
	ordinary := json.RawMessage(`1`)
	manual := json.RawMessage(`{"id":1,"dns_already_removed":true}`)
	if err := DeleteNode(t.Context(), manual, "test"); err == nil || err.Error() != "node_disable_pending_delete" {
		t.Fatal("manual confirmation bypassed access revocation", err)
	}
	if err := DeleteNode(t.Context(), ordinary, "test"); err == nil || cloud.Calls() != 0 {
		t.Fatal("deletion contacted DNS for an active node", err)
	}
	node.Enable = false
	node.AppliedRevision, node.DesiredRevision = 3, 3
	node.AppliedSnapshot, _ = json.Marshal(Snapshot{Node: Publication{Enable: false}})
	requireNodeTestSuccess(t, db.Save(&node).Error)
	err := DeleteNode(t.Context(), ordinary, "test")
	if rejected {
		err = confirmStaleDNSCleanup(t, db, cloud, record, manual, err)
	}
	if err != nil || cloud.Writes() != 0 {
		t.Fatal("could not delete a node whose DNS records were removed", err)
	}
	var remaining int64
	if err := db.Model(&model.NodeDNSRecord{}).Where("node_id = ?", node.Id).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatal("stale DNS records survived deletion", err)
	}
	if err := db.Model(&model.Node{}).Where("id = ?", node.Id).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatal("deleted node still exists", err)
	}
}

func confirmStaleDNSCleanup(t *testing.T, db *gorm.DB, cloud *cloudFixture, record model.NodeDNSRecord, manual json.RawMessage, err error) error {
	t.Helper()
	if err == nil || err.Error() != "dns_api_rejected" {
		t.Fatal("provider authorization error treated as record absence", err)
	}
	var saved model.NodeDNSRecord
	if err := db.First(&saved, record.Id).Error; err != nil || saved.Retired {
		t.Fatal("failed provider check discarded the record", err)
	}
	calls := cloud.Calls()
	err = DeleteNode(t.Context(), manual, "test")
	if cloud.Calls() != calls {
		t.Fatal("manual confirmation contacted the DNS provider")
	}
	var change model.Changes
	if db.Where("key = ? AND action = ?", "nodes", "del").First(&change).Error != nil || string(change.Obj) != string(manual) {
		t.Fatal("manual DNS confirmation was not audited")
	}
	return err
}
