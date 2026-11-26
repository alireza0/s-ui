package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/op/go-logging"
	"gorm.io/gorm"
)

type liveFixture struct {
	db       *gorm.DB
	nodes    []model.Node
	managers []*nodeagent.ManagerV2
	runtimes []*panelSafeRuntime
	client   model.Client
	template model.NodeTemplate
}

func liveSpec() TemplateSpec {
	return TemplateSpec{SchemaVersion: 1, Config: json.RawMessage(`{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`), Entries: []SpecEntry{{Key: "primary", Name: "Main", Protocol: "vless", Enable: true, Options: json.RawMessage(`{"listen":"127.0.0.1","tls":{"enabled":false}}`), Sources: map[string]ValueSource{"/listen_port": {Kind: "node_generated", Purpose: "listen_port"}, "/tls/server_name": {Kind: "node_parameter", Name: "primary_domain"}}}}}
}

func liveSetup(t *testing.T, count int, realCore ...bool) liveFixture {
	t.Helper()
	if len(realCore) > 0 && realCore[0] {
		logger.InitLogger(logging.CRITICAL)
	}
	db, node, client := fixture(t)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	requireNodeTestSuccess(t, db.Where("node_id = ?", node.Id).Delete(&model.NodeInbound{}).Error)
	f := liveFixture{db: db, client: client}
	for index := 0; index < count; index++ {
		n := model.Node{Id: uint(index + 1), Name: []string{"First", "Second"}[index], Enable: true, LinkAddress: []string{"first.example", "second.example"}[index], AgentProtocol: 2, AssignmentMode: model.NodeAssignmentWhole}
		runtime := &panelSafeRuntime{}
		state := filepath.Join(t.TempDir(), "agent")
		var safe nodeagent.SafeRuntime = runtime
		if len(realCore) > 0 && realCore[0] {
			safe = nodeagent.NewCoreRuntimeWithState(state)
		}
		manager := nodeagent.NewManagerV2(uint64(n.Id), nodeagent.FileStoreV2{Directory: state}, safe)
		requireNodeTestSuccess(t, manager.Restore())
		t.Cleanup(func() { manager.Close() })
		server, identity := controlHandlerServer(t, nodeagent.HandlerV2(manager))
		n.Address, n.ControlCA, n.ControlCertificate, n.ControlKey, n.ControlEpoch = server.URL, identity.CA, identity.Certificate, identity.Key, manager.Status().Epoch
		requireNodeTestSuccess(t, db.Save(&n).Error)
		requireNodeTestSuccess(t, db.Create(&model.ClientNode{NodeId: n.Id, ClientId: client.Id, Active: true, Generation: 1}).Error)
		f.nodes = append(f.nodes, n)
		f.managers = append(f.managers, manager)
		f.runtimes = append(f.runtimes, runtime)
	}
	f.save(t, liveSpec())
	for index := range f.nodes {
		f.bind(t, index, f.template.Id)
	}
	return f
}

func (f *liveFixture) save(t *testing.T, spec TemplateSpec) {
	t.Helper()
	if f.template.Id == 0 {
		f.template = model.NodeTemplate{Name: "deployment", Kind: deploymentTemplate, Scope: "shared", SchemaVersion: 1}
		requireNodeTestSuccess(t, f.db.Create(&f.template).Error)
		for pos, entry := range spec.Entries {
			member := model.NodeTemplate{Name: entry.Name, Kind: inboundTemplate}
			body, err := json.Marshal(map[string]any{"spec": TemplateSpec{SchemaVersion: 1, Config: json.RawMessage(`{}`), Entries: []SpecEntry{entry}}})
			requireNodeTestSuccess(t, err)
			requireNodeTestSuccess(t, saveVersionedTemplate(f.db, &member, model.NodeTemplate{}, body))
			requireNodeTestSuccess(t, f.db.Create(&model.NodeTemplateMember{TemplateId: f.template.Id, InboundId: member.Id, Position: pos}).Error)
		}
	}
	body, _ := json.Marshal(map[string]any{"spec": spec, "expected_version": f.template.Version})
	requireNodeTestSuccess(t, saveVersionedTemplate(f.db, &f.template, f.template, body))
}

func (f *liveFixture) bind(t *testing.T, index int, templateID uint) {
	t.Helper()
	var template model.NodeTemplate
	requireNodeTestSuccess(t, f.db.First(&template, templateID).Error)
	requireNodeTestSuccess(t, f.db.First(&f.nodes[index], f.nodes[index].Id).Error)
	requireNodeTestSuccess(t, bindLiveTemplate(f.db, &f.nodes[index], template))
}

func (f *liveFixture) run(t *testing.T, index int) {
	t.Helper()
	requireNodeTestSuccess(t, Reconcile(context.Background(), f.nodes[index].Id))
	f.db.First(&f.nodes[index], f.nodes[index].Id)
}
func (f *liveFixture) links(t *testing.T) int {
	t.Helper()
	links, err := ClientInbounds(f.db, f.client)
	requireNodeTestSuccess(t, err)
	return len(links)
}
func (f *liveFixture) overrides(t *testing.T, index int, values []SpecOverride) {
	t.Helper()
	f.db.First(&f.nodes[index], f.nodes[index].Id)
	body, _ := json.Marshal(map[string]interface{}{"id": f.nodes[index].Id, "expected_version": f.nodes[index].SettingsVersion, "overrides": values})
	requireNodeTestSuccess(t, Save("nodes", "setoverrides", body, "test"))
}

func TestLiveTemplateFanoutStableIDsAndPerNodeResources(t *testing.T) {
	f := liveSetup(t, 2)
	ReconcileAll(context.Background())
	if f.links(t) != 2 {
		t.Fatal("initial whole-node publication missing")
	}
	var before []model.NodeInbound
	f.db.Order("node_id, id").Find(&before)
	if len(before) != 2 || !bytes.Contains(before[0].Options, []byte("first.example")) || !bytes.Contains(before[1].Options, []byte("second.example")) {
		t.Fatal("individual domains mixed")
	}
	spec := liveSpec()
	spec.Entries[0].Name = "Renamed"
	extra := spec.Entries[0]
	extra.Key, extra.Name = "secondary", "Extra"
	spec.Entries = append(spec.Entries, extra)
	f.save(t, spec)
	if f.links(t) != 2 {
		t.Fatal("unapplied new connections published")
	}
	ReconcileAll(context.Background())
	if f.links(t) != 4 {
		t.Fatal("template save did not fan out")
	}
	assertLiveFanoutIdentity(t, f, before)
	var ports []model.NodePortReservation
	f.db.Find(&ports)
	if len(ports) != 4 {
		t.Fatal("port reservations not persistent")
	}
	for i := range f.nodes {
		f.run(t, i)
	}
	var after []model.NodeInbound
	f.db.Find(&after)
	if len(after) != 4 {
		t.Fatal("repeat created duplicate connections")
	}
}

func TestLiveRetireArchivesOverridesAndRestoreKeepsIdentity(t *testing.T) {
	f := liveSetup(t, 1)
	f.run(t, 0)
	f.overrides(t, 0, []SpecOverride{{ConnectionKey: "primary", Path: "/tls/server_name", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage(`"first.example"`)}}})
	f.run(t, 0)
	var old model.NodeInbound
	f.db.First(&old)
	spec := liveSpec()
	spec.Entries = []SpecEntry{}
	f.save(t, spec)
	if f.links(t) != 1 {
		t.Fatal("old applied connection disappeared before ACK")
	}
	f.run(t, 0)
	if f.links(t) != 0 {
		t.Fatal("retired connection remained published")
	}
	var retired model.NodeInbound
	f.db.First(&retired, old.Id)
	if !retired.Retired || retired.Enable {
		t.Fatal("removed entry became orphan")
	}
	var override model.NodeOverride
	f.db.First(&override)
	if !override.Archived {
		t.Fatal("removed entry override remained live")
	}
	f.save(t, liveSpec())
	f.run(t, 0)
	f.db.First(&retired, old.Id)
	if retired.Retired || !retired.Enable || retired.Id != old.Id || f.links(t) != 1 {
		t.Fatal("history restore lost connection identity")
	}
	var n model.Node
	f.db.First(&n)
	if n.AppliedTemplateVersion != 3 {
		t.Fatal("restore decreased template revision")
	}
}

func TestLiveOverrideEqualityAndReset(t *testing.T) {
	f := liveSetup(t, 2)
	f.run(t, 0)
	f.run(t, 1)
	f.overrides(t, 0, []SpecOverride{{ConnectionKey: "primary", Path: "/tls/server_name", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage(`"first.example"`)}}})
	spec := liveSpec()
	spec.Entries[0].Sources["/tls/server_name"] = ValueSource{Kind: "literal", Value: json.RawMessage(`"changed.example"`)}
	f.save(t, spec)
	f.run(t, 0)
	var in model.NodeInbound
	f.db.Where("node_id = ?", 1).First(&in)
	if !bytes.Contains(in.Options, []byte("first.example")) {
		t.Fatal("equal-valued explicit override disappeared")
	}
	f.overrides(t, 0, []SpecOverride{})
	f.run(t, 0)
	f.db.First(&in, in.Id)
	if !bytes.Contains(in.Options, []byte("changed.example")) {
		t.Fatal("reset did not inherit current template")
	}

}

func TestLiveFailureIsolationRejectionAndAccessPriority(t *testing.T) {
	f := liveSetup(t, 2)
	ReconcileAll(context.Background())
	before := f.managers[0].Status().SpecRevision
	f.overrides(t, 0, []SpecOverride{{ConnectionKey: "primary", Path: "/tls/server_name", Operation: "set", Source: ValueSource{Kind: "profile", Name: "missing"}}})
	spec := liveSpec()
	spec.Entries[0].Name = "Updated"
	f.save(t, spec)
	ReconcileAll(context.Background())
	if f.managers[0].Status().SpecRevision != before || f.links(t) != 2 {
		t.Fatal("broken node lost last-good")
	}
	var second model.Node
	f.db.First(&second, 2)
	if second.AppliedTemplateVersion != 2 {
		t.Fatal("broken node blocked other target")
	}
	requireNodeTestSuccess(t, f.db.Model(&model.Client{}).Where("id = ?", f.client.Id).Update("enable", false).Error)
	requireNodeTestSuccess(t, database.QueueClientNodeAccess(f.db, []uint{f.client.Id}))
	requireNodeTestSuccess(t, ReconcileAccess(context.Background(), 1))
	if bytes.Contains(f.runtimes[0].body, []byte("11111111")) {
		t.Fatal("bad template blocked revoke")
	}
	f.overrides(t, 0, []SpecOverride{})
	f.run(t, 0)
}

func TestLiveDelayedAckPreservesNewTargetAndLostAckRecovers(t *testing.T) {
	f := liveSetup(t, 1)
	f.run(t, 0)
	entered, release := make(chan struct{}), make(chan struct{})
	var blocked atomic.Bool
	var writes atomic.Int32
	handler := nodeagent.HandlerV2(f.managers[0])
	server, identity := controlHandlerServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/v2/spec" {
			writes.Add(1)
			if blocked.CompareAndSwap(false, true) {
				close(entered)
				<-release
			}
		}
		handler.ServeHTTP(w, r)
	}))
	n := f.nodes[0]
	n.Address, n.ControlCA, n.ControlCertificate, n.ControlKey = server.URL, identity.CA, identity.Certificate, identity.Key
	requireNodeTestSuccess(t, f.db.Save(&n).Error)
	spec := liveSpec()
	spec.Entries[0].Name = "B"
	f.save(t, spec)
	done := make(chan error, 1)
	go func() { done <- Reconcile(context.Background(), 1) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("B did not start")
	}
	spec.Entries[0].Name = "C"
	f.save(t, spec)
	close(release)
	requireNodeTestSuccess(t, <-done)
	f.db.First(&n, 1)
	if n.TemplateTargetVersion != 3 || n.AppliedTemplateVersion != 2 {
		t.Fatal("late B overwrote desired C")
	}
	f.run(t, 0)
	if f.nodes[0].AppliedTemplateVersion != 3 {
		t.Fatal("queued C was lost")
	}
	// Simulate a panel crash after remote apply and before recording its ACK.
	f.db.Model(&model.Node{}).Where("id = ?", 1).Updates(map[string]interface{}{"applied_revision": 0, "applied_snapshot": nil, "applied_template_version": 0})
	written := writes.Load()
	f.run(t, 0)
	if writes.Load() != written || f.nodes[0].AppliedTemplateVersion != 3 {
		t.Fatal("status did not recover lost ACK idempotently")
	}
	var old model.NodeDeployment
	f.db.Where("node_id = ?", 1).Order("revision").First(&old)
	var snapshot Snapshot
	json.Unmarshal(old.Snapshot, &snapshot)
	oldStatus := nodeagent.StatusV2{NodeID: 1, Epoch: n.ControlEpoch, Running: true, SpecRevision: old.Revision, SpecDigest: (nodeagent.Structure{NodeID: 1, Epoch: n.ControlEpoch, Revision: old.Revision, Config: snapshot.Deployment.Config}).Digest()}
	requireNodeTestSuccess(t, observeStructure(f.db, n, oldStatus))
	f.db.First(&n, 1)
	if n.AppliedTemplateVersion != 3 {
		t.Fatal("older observation replaced newer ACK")
	}
}

func TestLiveLeaseExpiryAndNewNodeUsesLatestTemplate(t *testing.T) {
	f := liveSetup(t, 2)
	spec := liveSpec()
	spec.Entries[0].Name = "New"
	f.save(t, spec)
	f.bind(t, 1, f.template.Id)
	if f.nodes[1].TemplateTargetVersion != 2 {
		t.Fatal("attachment missed current template version")
	}
	requireNodeTestSuccess(t, f.db.Model(&model.NodeTask{}).Where("node_id = ? AND kind = ?", 1, "structure").Update("lease_until", time.Now().Unix()+60).Error)
	requireNodeTestSuccess(t, Reconcile(context.Background(), 1))
	if f.managers[0].Status().SpecRevision != 0 {
		t.Fatal("unexpired lease stolen")
	}
	f.db.Model(&model.NodeTask{}).Where("node_id = ? AND kind = ?", 1, "structure").Update("lease_until", time.Now().Unix()-1)
	f.run(t, 0)
	if f.nodes[0].AppliedTemplateVersion != 2 {
		t.Fatal("expired lease did not recover")
	}
	var task model.NodeTask
	f.db.Where("node_id = ? AND kind = ?", 1, "structure").First(&task)
	if task.State != "done" || task.LeaseToken != "" {
		t.Fatal("lease not released")
	}
}

func TestLiveGeneratedSecretsStableEncryptedAndHostPorts(t *testing.T) {
	f := liveSetup(t, 2)
	spec := liveSpec()
	entry := &spec.Entries[0]
	entry.Options = json.RawMessage(`{"listen":"127.0.0.1","tls":{"enabled":true,"reality":{"enabled":true,"handshake":{"server":"www.example.com","server_port":443}}}}`)
	entry.Sources["/tls/reality/private_key"] = ValueSource{Kind: "node_generated", Purpose: "reality_private_key"}
	entry.Sources["/tls/reality/short_id"] = ValueSource{Kind: "node_generated", Purpose: "reality_short_id"}
	f.save(t, spec)
	ReconcileAll(context.Background())
	var rows []model.NodeGeneratedMaterial
	f.db.Order("node_id, purpose").Find(&rows)
	if len(rows) != 6 {
		t.Fatal("generated material missing")
	}
	for _, row := range rows {
		if strings.Contains(row.EncryptedValue, "PRIVATE") || strings.Contains(row.EncryptedValue, "Value") {
			t.Fatal("unencrypted generated material")
		}
	}
	var first []model.NodeInbound
	f.db.Order("node_id, id").Find(&first)
	if len(first) != 2 || string(first[0].Options) == string(first[1].Options) {
		t.Fatal("keys shared across nodes")
	}
	ReconcileAll(context.Background())
	var again []model.NodeGeneratedMaterial
	f.db.Order("node_id, purpose").Find(&again)
	a, _ := json.Marshal(rows)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("material regenerated on repeat")
	}
	// Both management origins use the same physical loopback host. A conflict
	// cannot be hidden by choosing different control ports for two logical nodes.
	var option struct {
		Port int `json:"listen_port"`
	}
	json.Unmarshal(first[0].Options, &option)
	f.overrides(t, 1, []SpecOverride{{ConnectionKey: "primary", Path: "/listen_port", Operation: "set", Source: ValueSource{Kind: "literal", Value: json.RawMessage([]byte(strings.TrimSpace(jsonNumber(option.Port))))}}})
	if err := Reconcile(context.Background(), 2); err == nil {
		t.Fatal("same-host TCP port conflict accepted")
	}
}

func jsonNumber(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestLiveUnavailableNodeAndRejectedSpecDoNotBlockOtherNode(t *testing.T) {
	f := liveSetup(t, 2)
	ReconcileAll(context.Background())
	original := f.nodes[0].Address
	f.db.Model(&model.Node{}).Where("id = ?", 1).Update("address", "https://127.0.0.1:1")
	spec := liveSpec()
	spec.Entries[0].Name = "B"
	spec.Entries[0].Options = json.RawMessage(`{"listen":"127.0.0.1","tcp_fast_open":true,"tls":{"enabled":false}}`)
	f.save(t, spec)
	ReconcileAll(context.Background())
	var second model.Node
	f.db.First(&second, 2)
	if second.AppliedTemplateVersion != 2 || f.links(t) != 2 {
		t.Fatal("unreachable node blocked healthy node or last-good")
	}
	f.db.Model(&model.Node{}).Where("id = ?", 1).Update("address", original)
	f.db.Model(&model.NodeTask{}).Where("node_id = ? AND kind = ?", 1, "structure").Update("next_attempt", 0)
	f.runtimes[0].reject = true
	if err := Reconcile(context.Background(), 1); err == nil {
		t.Fatal("failed spec acknowledged")
	}
	attempts := f.runtimes[0].attempts
	requireNodeTestSuccess(t, Reconcile(context.Background(), 1))
	if f.runtimes[0].attempts != attempts {
		t.Fatal("rejected target restarted every poll")
	}
	f.runtimes[0].reject = false
	requireNodeTestSuccess(t, Save("nodes", "retry", json.RawMessage(`{"id":1}`), "test"))
	f.run(t, 0)
	if f.nodes[0].AppliedTemplateVersion != 2 {
		t.Fatal("explicit retry failed")
	}
}

func TestLiveGeneratedSecretsStableAndBoundToNode(t *testing.T) {
	f := liveSetup(t, 2)
	spec := liveSpec()
	spec.Entries[0].Options = json.RawMessage(`{"listen":"127.0.0.1","tls":{"enabled":true,"reality":{"enabled":true,"handshake":{"server":"www.example.com","server_port":443}}}}`)
	spec.Entries[0].Sources["/tls/reality/private_key"] = ValueSource{Kind: "node_generated", Purpose: "reality_private_key"}
	spec.Entries[0].Sources["/tls/reality/short_id"] = ValueSource{Kind: "node_generated", Purpose: "reality_short_id"}
	f.save(t, spec)
	ReconcileAll(context.Background())
	var rows []model.NodeInbound
	f.db.Order("id").Find(&rows)
	if len(rows) != 2 || string(rows[0].Options) == string(rows[1].Options) {
		t.Fatal("node keys reused")
	}
	var materials []model.NodeGeneratedMaterial
	f.db.Find(&materials)
	if len(materials) != 6 {
		t.Fatal("generated material missing")
	}
	before := string(rows[0].Options)
	f.run(t, 0)
	f.db.First(&rows[0], rows[0].Id)
	if before != string(rows[0].Options) {
		t.Fatal("retry regenerated server keys")
	}
	public, _ := json.Marshal(materials)
	if strings.Contains(string(public), "encrypted_value") {
		t.Fatal("secret material exposed")
	}
	var first, second model.NodeGeneratedMaterial
	f.db.Where("node_id = ? AND purpose = ?", 1, "reality_private_key").First(&first)
	f.db.Where("node_id = ? AND purpose = ?", 2, "reality_private_key").First(&second)
	f.db.Model(&first).Update("encrypted_value", second.EncryptedValue)
	if err := Reconcile(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatal("swapped generated ciphertext accepted", err)
	}
}

func TestLiveTwoRealCoresApplyTemplateAndServeTraffic(t *testing.T) {
	f := liveSetup(t, 2, true)
	// This fixture shares a host with other software. Account for unavailable
	// candidates before the two real cores use their loopback listeners.
	host := physicalNodeHost(f.db, f.nodes[0])
	available := 0
	for _, port := range automaticListenerPorts {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)))
		if err != nil {
			requireNodeTestSuccess(t, f.db.Create(&model.NodePortReservation{NodeId: 999, ConnectionKey: "fixture-busy", Host: host, Network: "tcp", Port: port}).Error)
			continue
		}
		listener.Close()
		available++
		if available == 4 {
			break
		}
	}
	if available < 4 {
		t.Fatal("four free loopback ports are required")
	}
	ReconcileAll(context.Background())
	for _, manager := range f.managers {
		if !manager.Status().Running {
			t.Fatal("real agent failed to start")
		}
	}
	spec := liveSpec()
	extra := spec.Entries[0]
	extra.Key, extra.Name = "extra", "Extra"
	spec.Entries = append(spec.Entries, extra)
	f.save(t, spec)
	ReconcileAll(context.Background())
	links, err := ClientInbounds(f.db, f.client)
	if err != nil || len(links) != 4 {
		t.Fatal("real applied connections missing", err)
	}
	assertLiveTraffic(t, links)
}

func assertLiveTraffic(t *testing.T, links []*model.Inbound) {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "live-template-traffic") }))
	defer backend.Close()
	for _, link := range links {
		var addresses []struct {
			Server string `json:"server"`
			Port   int    `json:"server_port"`
		}
		if err := json.Unmarshal(link.Addrs, &addresses); err != nil || len(addresses) != 1 {
			t.Fatal(err)
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		requireNodeTestSuccess(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		clientCore := core.NewCoreWithContext(context.Background())
		body := []byte(fmt.Sprintf(`{"inbounds":[{"type":"socks","tag":"socks","listen":"127.0.0.1","listen_port":%d}],"outbounds":[{"type":"vless","tag":"proxy","server":"127.0.0.1","server_port":%d,"uuid":"11111111-1111-4111-8111-111111111111"}],"route":{"final":"proxy"}}`, port, addresses[0].Port))
		requireNodeTestSuccess(t, clientCore.Start(body))
		proxy, _ := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", port))
		transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
		client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
		response, err := client.Get(backend.URL)
		if err != nil {
			clientCore.Stop()
			t.Fatal(err)
		}
		result, _ := io.ReadAll(response.Body)
		response.Body.Close()
		transport.CloseIdleConnections()
		clientCore.Stop()
		if string(result) != "live-template-traffic" {
			t.Fatal("materialized connection failed real traffic")
		}
	}
}

func TestLiveDisableAndOverrideRemovalPreserveIntent(t *testing.T) {
	f := liveSetup(t, 1)
	f.run(t, 0)
	var original model.NodeInbound
	f.db.First(&original)
	f.overrides(t, 0, []SpecOverride{{ConnectionKey: "primary", Path: "/tls", Operation: "remove"}})
	f.run(t, 0)
	var updated model.NodeInbound
	f.db.First(&updated, original.Id)
	if bytes.Contains(updated.Options, []byte("server_name")) {
		t.Fatal("parent removal retained inherited child source")
	}
	spec := liveSpec()
	spec.Entries[0].Enable = false
	f.save(t, spec)
	if f.links(t) != 1 {
		t.Fatal("pending disable changed applied publication")
	}
	f.run(t, 0)
	if f.links(t) != 0 {
		t.Fatal("disabled connection remained published")
	}
	f.db.First(&updated, original.Id)
	if updated.Retired || updated.Enable {
		t.Fatal("disable lost connection identity")
	}
	spec.Entries[0].Enable = true
	f.save(t, spec)
	f.run(t, 0)
	f.db.First(&updated, original.Id)
	if updated.Id != original.Id || !updated.Enable || f.links(t) != 1 {
		t.Fatal("re-enable changed identity or failed")
	}
	var options struct {
		Port int `json:"listen_port"`
	}
	var oldOptions struct {
		Port int `json:"listen_port"`
	}
	json.Unmarshal(updated.Options, &options)
	json.Unmarshal(original.Options, &oldOptions)
	if options.Port != oldOptions.Port {
		t.Fatal("disable/re-enable regenerated port")
	}
}

func TestLiveLegacyWritersCannotOverwriteManagedStructure(t *testing.T) {
	f := liveSetup(t, 1)
	f.run(t, 0)
	for _, request := range []struct {
		object, action string
		data           json.RawMessage
	}{
		{"nodes", "edit", json.RawMessage(`{"id":1,"config":{"log":{"level":"debug"}}}`)},
		{"nodes", "edit", json.RawMessage(`{"id":1,"template_id":0}`)},
		{"nodeinbounds", "new", json.RawMessage(`{"node_id":1,"type":"vless","tag":"manual","options":{"listen_port":8443}}`)},
	} {
		if err := Save(request.object, request.action, request.data, "test"); err == nil {
			t.Fatal("legacy writer overwrote live structure", request.action)
		}
	}
	if _, err := StartProvision(ProvisionRequest{NodeId: 1, ModelVersion: 1}, "test"); err == nil || err.Error() != "unsupported_node_model" {
		t.Fatal("old installer accepted live node", err)
	}
	var n model.Node
	f.db.First(&n, 1)
	if n.TemplateId != f.template.Id || n.TemplateTargetVersion != 1 {
		t.Fatal("rejected writer changed template relation")
	}
}

func TestLiveSchemaUpgradeKeepsExistingConnections(t *testing.T) {
	db, node, client := fixture(t)
	requireNodeTestSuccess(t, db.Create(&model.NodeOverride{NodeId: node.Id, ConnectionKey: "primary", Path: "/tls/server_name", Operation: "set", Source: json.RawMessage(`{"kind":"literal","value":"old.example"}`)}).Error)
	requireNodeTestSuccess(t, db.Migrator().DropColumn(&model.NodeInbound{}, "retired"))
	requireNodeTestSuccess(t, db.Migrator().DropColumn(&model.NodeOverride{}, "archived"))
	var files []struct{ Name, File string }
	requireNodeTestSuccess(t, db.Raw("PRAGMA database_list").Scan(&files).Error)
	path := ""
	for _, file := range files {
		if file.Name == "main" {
			path = file.File
		}
	}
	sqlDB, _ := db.DB()
	sqlDB.Close()
	requireNodeTestSuccess(t, database.InitDB(path))
	reopened := database.GetDB()
	connection, _ := reopened.DB()
	t.Cleanup(func() { connection.Close() })
	snapshot, err := Compile(reopened, node)
	if err != nil || len(snapshot.Inbounds) != 1 {
		t.Fatal("schema upgrade hid a configured inbound", err)
	}
	var count int64
	reopened.Model(&model.NodeOverride{}).Where("archived = ?", false).Count(&count)
	if count != 1 {
		t.Fatal("schema upgrade archived an existing override")
	}
	confirm(t, reopened, node)
	links, err := ClientInbounds(reopened, client)
	if err != nil || len(links) != 1 {
		t.Fatal("assignment lost after migration", err)
	}
}

func assertLiveFanoutIdentity(t *testing.T, f liveFixture, before []model.NodeInbound) {
	t.Helper()
	for _, old := range before {
		var current model.NodeInbound
		f.db.First(&current, old.Id)
		if current.ConnectionKey != old.ConnectionKey || string(current.Options) != string(old.Options) || current.DisplayName != "Renamed" {
			t.Fatal("rename changed identity/material")
		}
	}
	var versions []model.Node
	f.db.Order("id").Find(&versions)
	for _, n := range versions {
		if n.TemplateTargetVersion != 2 || n.MaterializedTemplateVersion != 2 || n.AppliedTemplateVersion != 2 {
			t.Fatal("template counters diverged")
		}
	}
}
