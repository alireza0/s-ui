package nodes

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

func liveCreateFixture(t *testing.T) (*gorm.DB, ProvisionRequest) {
	t.Helper()
	db, _, _ := fixture(t)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	return db, ProvisionRequest{ModelVersion: 2, RequestKey: "unit-request-00000001", Name: "Fresh", IP: "192.0.2.20", SSHUser: "root", SSHPort: 22, Password: "unit-password-never-persist", Protocols: []string{"vless"}}
}

func TestLiveProvisionCreateIsIdempotentAndDoesNotStorePassword(t *testing.T) {
	db, r := liveCreateFixture(t)
	first, created, err := createLiveProvision(r, "unit")
	if err != nil || !created {
		t.Fatal(err)
	}
	again, created, err := createLiveProvision(r, "unit")
	if err != nil || created || again.NodeId != first.NodeId || again.Id != first.Id {
		t.Fatal("duplicate create", err)
	}
	r.IP = "192.0.2.21"
	if _, _, err = createLiveProvision(r, "unit"); err == nil {
		t.Fatal("different intent reused key")
	}
	var jobs []model.NodeJob
	db.Where("node_id = ?", first.NodeId).Find(&jobs)
	if len(jobs) != 1 {
		t.Fatal("duplicate job")
	}
	if strings.Contains(jobs[0].Plan, r.Password) {
		t.Fatal("password in plan")
	}
	plan, err := loadLivePlan(jobs[0])
	if err != nil || plan.Request.Password != "" || plan.Request.Domain != nil {
		t.Fatal("password or upload retained", err)
	}
	public, _ := json.Marshal(jobs[0])
	if bytes.Contains(public, []byte(jobs[0].Plan)) || bytes.Contains(public, []byte(r.Password)) {
		t.Fatal("private job state returned")
	}
	var node model.Node
	db.First(&node, first.NodeId)
	if node.AssignmentMode != model.NodeAssignmentWhole || node.AgentProtocol != 2 || node.TemplateTargetVersion != 1 {
		t.Fatal("not live model")
	}
	ids, err := catalogMembers(db, node.TemplateId)
	requireNodeTestSuccess(t, err)
	if len(ids) != 1 {
		t.Fatal("cloned protocol did not create an inbound template")
	}
	var preset model.NodeTemplate
	requireNodeTestSuccess(t, db.First(&preset, ids[0]).Error)
	if preset.Kind != inboundTemplate || preset.Scope != "private" || preset.OwnerNodeId != node.Id {
		t.Fatal("preset does not belong to the cloned node")
	}
}

func TestProvisionWithoutInboundsCreatesAnEmptyDeployment(t *testing.T) {
	db, request := liveCreateFixture(t)
	request.Protocols = nil
	job, _, err := createLiveProvision(request, "unit")
	requireNodeTestSuccess(t, err)
	var node model.Node
	requireNodeTestSuccess(t, db.First(&node, job.NodeId).Error)
	spec, err := liveVersion(db, node)
	requireNodeTestSuccess(t, err)
	if !managedNodeModel(node) || len(spec.Entries) != 0 {
		t.Fatal("empty installation created unexpected inbounds or lost its deployment")
	}
}

func TestLiveProvisionPreflightIncludesEveryConnectionAndRejectsHostReservation(t *testing.T) {
	db, r := liveCreateFixture(t)
	entries := []SpecEntry{
		{Key: "first-vless", Name: "First", Protocol: "vless", Enable: true, Options: json.RawMessage(`{"tls":{"enabled":false}}`), Sources: map[string]ValueSource{"/listen_port": {Kind: "node_generated", Purpose: "listen_port"}}},
		{Key: "another-vless", Name: "Second", Protocol: "vless", Enable: true, Options: json.RawMessage(`{"listen_port":24567,"tls":{"enabled":false}}`), Sources: map[string]ValueSource{}},
	}
	for _, entry := range entries {
		spec := TemplateSpec{SchemaVersion: 1, Config: json.RawMessage("{}"), Entries: []SpecEntry{entry}}
		data, _ := json.Marshal(map[string]interface{}{"name": entry.Name, "kind": inboundTemplate, "spec": spec})
		if err := Save("nodetemplates", "new", data, "unit"); err != nil {
			t.Fatal(err)
		}
		var template model.NodeTemplate
		if err := db.First(&template, "name = ? AND kind = ?", entry.Name, inboundTemplate).Error; err != nil {
			t.Fatal(err)
		}
		r.InboundTemplateIds = append(r.InboundTemplateIds, template.Id)
	}
	job, _, err := createLiveProvision(r, "unit")
	if err != nil {
		t.Fatal(err)
	}
	var node model.Node
	db.First(&node, job.NodeId)
	var listeners []provisionListener
	err = db.Transaction(func(tx *gorm.DB) error {
		var e error
		listeners, e = plannedLiveListeners(tx, node, r.IP, nil)
		return e
	})
	if err != nil || len(listeners) != 2 {
		t.Fatal("one entry per protocol lost", err)
	}
	if listeners[0].Port == listeners[1].Port {
		t.Fatal("port collision")
	}
	// Preserve established ports when changing physical hosts; a conflicting
	// reservation must fail before a remote fence or DNS change.
	if err = db.Create(&model.NodePortReservation{NodeId: 999, ConnectionKey: "foreign", Host: "192.0.2.21", Network: "tcp", Port: 24567}).Error; err != nil {
		t.Fatal(err)
	}
	err = db.Transaction(func(tx *gorm.DB) error { _, e := plannedLiveListeners(tx, node, "192.0.2.21", nil); return e })
	if err == nil || err.Error() != "node_listener_port_reserved" {
		t.Fatal("foreign reservation accepted", err)
	}
	var count int64
	db.Model(&model.NodeInbound{}).Where("node_id = ?", node.Id).Count(&count)
	if count != 0 {
		t.Fatal("preflight materialized an unready connection")
	}
}

func TestLiveTargetCommitRequiresFenceAndPreservesCounters(t *testing.T) {
	db, r := liveCreateFixture(t)
	job, _, err := createLiveProvision(r, "unit")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := loadLivePlan(job)
	if err != nil {
		t.Fatal(err)
	}
	db.Model(&model.Node{}).Where("id = ?", job.NodeId).Updates(map[string]interface{}{"desired_revision": 200, "applied_revision": 199})
	if err = db.Create(&model.NodeAccessState{NodeId: job.NodeId, DesiredRevision: 100, AppliedRevision: 99}).Error; err != nil {
		t.Fatal(err)
	}
	status := nodeagent.StatusV2{NodeID: uint64(job.NodeId), Protocol: 2, Epoch: "explicit-new-registration", BootstrapID: plan.Target.BootstrapID}
	if err = commitLiveTarget(&job, &plan, status); err == nil {
		t.Fatal("unfenced handover accepted")
	}
	plan.Fenced = true
	if err = commitLiveTarget(&job, &plan, status); err != nil {
		t.Fatal(err)
	}
	saved, err := loadLivePlan(job)
	if err != nil || !saved.Committed || saved.ExpectedGeneration != plan.ExpectedGeneration {
		t.Fatal("commit receipt not atomic", err)
	}
	var current model.Node
	db.First(&current, job.NodeId)
	if current.DesiredRevision != 200 || current.AppliedRevision != 199 || current.ControlEpoch != status.Epoch || current.ManagementGeneration != 2 {
		t.Fatal("counter reset or registration missing")
	}
	var a model.NodeAccessState
	db.First(&a, "node_id = ?", job.NodeId)
	if a.DesiredRevision != 100 || a.AppliedRevision != 99 {
		t.Fatal("ACL high-water reset")
	}
	old := current
	old.ManagementGeneration--
	if err = recordV2Health(old, nodeagent.StatusV2{Epoch: current.ControlEpoch, Running: true}); err == nil {
		t.Fatal("old target ACK accepted")
	}
	db.First(&current, job.NodeId)
	if current.AgentRunning {
		t.Fatal("stale health republished endpoints")
	}
}

func TestLiveHostScriptsAreValidPythonAndRequireOwnerAndBootstrap(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python syntax check requires python3")
	}
	for name, body := range map[string]string{"inspect": liveInspectionScript, "key": liveAuthorizeKeyScript, "cache": liveCachedAgentScript, "fence": liveFenceScript, "install": liveInstallScript} {
		path := filepath.Join(t.TempDir(), name+".py")
		if err = os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command(python, "-m", "py_compile", path).CombinedOutput(); err != nil {
			t.Fatalf("%s: %s", name, output)
		}
	}
	script := liveInstallScript
	if !strings.Contains(script, "--bootstrap-id") || !strings.Contains(script, "owner='+owner") || strings.Contains(script, "p['ports']") {
		t.Fatal("legacy ownership/protocol writer in live installer")
	}
}
