package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func TestInboundCatalogSharedEditPreservesConnections(t *testing.T) {
	f := liveSetup(t, 2)
	f.run(t, 0)
	f.run(t, 1)
	var before []model.NodeInbound
	requireNodeTestSuccess(t, f.db.Order("node_id").Find(&before).Error)
	ids, err := catalogMembers(f.db, f.template.Id)
	if err != nil || len(ids) != 1 {
		t.Fatalf("expected one stable catalog member, got %v: %v", ids, err)
	}
	var inbound model.NodeTemplate
	requireNodeTestSuccess(t, f.db.First(&inbound, ids[0]).Error)
	body, _ := json.Marshal(map[string]interface{}{"id": 1, "expected_version": f.nodes[0].SettingsVersion, "inbound_template_ids": ids})
	requireNodeTestSuccess(t, Save("nodes", "selectinbounds", body, "test"))
	var node model.Node
	f.db.First(&node, 1)
	if node.TemplateId != f.template.Id || node.SettingsVersion != f.nodes[0].SettingsVersion {
		t.Fatal("equivalent selection changed the node")
	}
	spec, err := templateSpec(f.db, inbound)
	requireNodeTestSuccess(t, err)
	spec.Entries[0].Name = "Changed display name"
	body, _ = json.Marshal(map[string]interface{}{"id": inbound.Id, "name": spec.Entries[0].Name, "expected_version": inbound.Version, "spec": spec})
	requireNodeTestSuccess(t, Save("nodetemplates", "edit", body, "test"))
	f.run(t, 0)
	f.run(t, 1)
	var after []model.NodeInbound
	f.db.Order("node_id").Find(&after)
	if len(before) != 2 || len(after) != len(before) || f.links(t) != 2 {
		t.Fatal("shared edit lost published connections")
	}
	for i, row := range after {
		if row.Id != before[i].Id || row.ConnectionKey != before[i].ConnectionKey || row.DisplayName != spec.Entries[0].Name || !reflect.DeepEqual(row.Options, before[i].Options) {
			t.Fatal("shared edit changed a stable identity or listener")
		}
	}
}

func TestNodeSelectionAddsOneInboundWithoutChangingOtherNode(t *testing.T) {
	f := liveSetup(t, 2)
	ids, err := catalogMembers(f.db, f.template.Id)
	requireNodeTestSuccess(t, err)
	spec := liveSpec()
	spec.Entries[0].Key, spec.Entries[0].Name = "secondary", "Secondary"
	body, _ := json.Marshal(map[string]interface{}{"name": "Secondary", "kind": inboundTemplate, "spec": spec})
	requireNodeTestSuccess(t, Save("nodetemplates", "new", body, "test"))
	var extra model.NodeTemplate
	f.db.Where("name = ? AND kind = ?", "Secondary", inboundTemplate).First(&extra)
	body, _ = json.Marshal(map[string]interface{}{"id": 1, "expected_version": f.nodes[0].SettingsVersion, "inbound_template_ids": append(ids, extra.Id)})
	requireNodeTestSuccess(t, Save("nodes", "selectinbounds", body, "test"))
	f.run(t, 0)
	f.run(t, 1)
	if f.nodes[0].TemplateId == f.nodes[1].TemplateId || f.nodes[1].TemplateId != f.template.Id || f.links(t) != 3 {
		t.Fatal("selection did not remain local to one node")
	}
	if err := Save("nodes", "selectinbounds", body, "test"); err == nil || err.Error() != "node_settings_version_conflict" {
		t.Fatalf("stale selection was not rejected: %v", err)
	}
}

func TestNodeSelectionCanClearAllInbounds(t *testing.T) {
	f := liveSetup(t, 2)
	f.run(t, 0)
	f.run(t, 1)
	selectInbounds := func(ids []uint) error {
		body, err := json.Marshal(map[string]interface{}{
			"id": f.nodes[0].Id, "expected_version": f.nodes[0].SettingsVersion, "inbound_template_ids": ids,
		})
		requireNodeTestSuccess(t, err)
		return Save("nodes", "selectinbounds", body, "test")
	}
	if err := selectInbounds(nil); err == nil {
		t.Fatal("accepted a missing selection")
	}
	requireNodeTestSuccess(t, selectInbounds([]uint{}))
	if f.links(t) != 2 {
		t.Fatal("withdrew a connection before the agent confirmed the change")
	}
	f.run(t, 0)
	if !f.nodes[0].Enable || f.links(t) != 1 {
		t.Fatal("empty selection did not withdraw only the selected node's connections")
	}
	var reservations int64
	if err := f.db.Model(&model.NodePortReservation{}).Where("node_id = ?", f.nodes[0].Id).Count(&reservations).Error; err != nil || reservations != 0 {
		t.Fatalf("node listener reservations remained after confirmation: %v (%d)", err, reservations)
	}
	requireNodeTestSuccess(t, selectInbounds([]uint{}))
	var node model.Node
	if err := f.db.First(&node, f.nodes[0].Id).Error; err != nil || node.SettingsVersion != f.nodes[0].SettingsVersion {
		t.Fatalf("applying the same empty selection changed node settings: %v", err)
	}
}

func TestAccessFailureDoesNotReportStructuralFailure(t *testing.T) {
	f := liveSetup(t, 1)
	f.run(t, 0)
	node := f.nodes[0]
	controlKey := node.ControlKey
	requireNodeTestSuccess(t, f.db.Model(&node).Update("control_key", "").Error)
	if err := Reconcile(t.Context(), node.Id); err == nil {
		t.Fatal("invalid control credentials were accepted")
	}
	var access model.NodeAccessState
	if err := f.db.First(&access, "node_id = ?", node.Id).Error; err != nil || access.LastError == "" {
		t.Fatalf("access failure was not recorded: %v", err)
	}
	var saved model.Node
	if err := f.db.First(&saved, node.Id).Error; err != nil || saved.LastError != "" {
		t.Fatalf("access failure was reported as structural failure: %v, %q", err, saved.LastError)
	}
	requireNodeTestSuccess(t, f.db.Model(&node).Update("control_key", controlKey).Error)
	requireNodeTestSuccess(t, Save("nodes", "retry", json.RawMessage(`{"id":1}`), "test"))
	f.run(t, 0)
	if err := f.db.First(&access, "node_id = ?", node.Id).Error; err != nil || access.LastError != "" {
		t.Fatalf("confirmed access retained an old error: %v, %q", err, access.LastError)
	}
}

func TestPrivateInboundDeletionAfterNodeRemoval(t *testing.T) {
	for _, name := range []string{"delete node", "orphan from backup"} {
		t.Run(name, func(t *testing.T) {
			checkPrivateInboundDeletion(t, name)
		})
	}
}

func TestStructureResumesWhenDNSBecomesReadyWithoutManualRetry(t *testing.T) {
	f := liveSetup(t, 1)
	f.run(t, 0)
	oldLookup := resourceLookupIP
	ready := false
	resourceLookupIP = func(context.Context, string) ([]net.IPAddr, error) {
		if !ready {
			return nil, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("192.0.2.10")}}, nil
	}
	t.Cleanup(func() { resourceLookupIP = oldLookup })
	domain := model.NodeDomainBinding{NodeId: 1, Mode: "manual", FQDN: "first.example", AppliedFQDN: "first.example", ExpectedIP: "192.0.2.10", CertificateMode: "provided", Generation: 1, State: "ready"}
	requireNodeTestSuccess(t, f.db.Create(&domain).Error)
	if err := ReconcileResources(context.Background(), 1); err == nil || err.Error() != "dns_address_not_resolved" {
		t.Fatalf("first DNS lookup should be pending: %v", err)
	}
	if err := Reconcile(context.Background(), 1); !errors.Is(err, errNodeDomainPending) {
		t.Fatalf("structure should wait for DNS: %v", err)
	}
	var task model.NodeTask
	f.db.First(&task, "node_id = ? AND kind = ?", 1, "structure")
	if task.State != "waiting" || f.links(t) != 1 {
		t.Fatal("DNS waiting should preserve last applied access")
	}
	ready = true
	// Make the scheduled resource retry due; do not invoke the manual retry API.
	f.db.Model(&model.NodeTask{}).Where("kind = ?", "resources").Update("next_attempt", 0)
	requireNodeTestSuccess(t, ReconcileResources(context.Background(), 1))
	f.run(t, 0)
	f.db.First(&task, "node_id = ? AND kind = ?", 1, "structure")
	f.db.First(&domain, "node_id = ?", 1)
	if task.State != "done" || task.LastError != "" || domain.LastError != "" || f.nodes[0].LastError != "" {
		t.Fatal("recovered DNS left an error or a blocked structural task")
	}
}

func checkPrivateInboundDeletion(t *testing.T, name string) {
	t.Helper()
	db, node, _ := fixture(t)
	group, inbound := addPrivateDeploymentFixture(t, db, node.Id)
	activeGroup, activeInbound := addPrivateDeploymentFixture(t, db, node.Id+1)
	activeNode := model.Node{Id: node.Id + 1, Name: "Other node", TemplateId: activeGroup.Id}
	requireNodeTestSuccess(t, db.Create(&activeNode).Error)
	node.TemplateId, node.Enable = group.Id, false
	node.DesiredRevision, node.AppliedRevision = 3, 3
	node.AppliedSnapshot, _ = json.Marshal(Snapshot{Node: Publication{Enable: false}})
	requireNodeTestSuccess(t, db.Save(&node).Error)
	deleteTemplate := func(id uint) error {
		body, _ := json.Marshal(id)
		return Save("nodetemplates", "del", body, "test")
	}
	if err := deleteTemplate(inbound.Id); err == nil || err.Error() != "node_template_in_use" {
		t.Fatalf("template deletion should be blocked while its node exists: %v", err)
	}
	if name == "delete node" {
		body, _ := json.Marshal(node.Id)
		requireNodeTestSuccess(t, Save("nodes", "del", body, "test"))
	} else {
		requireNodeTestSuccess(t, db.Delete(&node).Error)
	}
	if err := deleteTemplate(inbound.Id); err != nil {
		t.Fatalf("deleted node still blocks the inbound template: %v", err)
	}
	if err := deleteTemplate(activeInbound.Id); err == nil || err.Error() != "node_template_in_use" {
		t.Fatalf("the other node's template lost protection: %v", err)
	}
	versions, err := TemplateHistory(group.Id)
	if err != nil || len(versions) != 1 {
		t.Fatalf("historical deployment version lost: %v %v", versions, err)
	}
}

func addPrivateDeploymentFixture(t *testing.T, db *gorm.DB, owner uint) (model.NodeTemplate, model.NodeTemplate) {
	t.Helper()
	group := model.NodeTemplate{Name: "Private deployment", Kind: deploymentTemplate, Scope: "private", OwnerNodeId: owner, SchemaVersion: 1, Version: 1}
	inbound := model.NodeTemplate{Name: "Private inbound", Kind: inboundTemplate, Scope: "private", OwnerNodeId: owner, SchemaVersion: 1, Version: 1}
	for _, row := range []*model.NodeTemplate{&group, &inbound} {
		requireNodeTestSuccess(t, db.Create(row).Error)
	}
	payload, _ := json.Marshal(liveSpec())
	version := model.NodeTemplateVersion{TemplateId: group.Id, Version: 1, SchemaVersion: 1, Payload: payload, Digest: versionDigest(payload)}
	member := model.NodeTemplateMember{TemplateId: group.Id, InboundId: inbound.Id}
	for _, row := range []interface{}{&version, &member} {
		requireNodeTestSuccess(t, db.Create(row).Error)
	}
	return group, inbound
}
