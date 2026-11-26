package nodes

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

func fixture(t *testing.T) (*gorm.DB, model.Node, model.Client) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "nodes.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	node := model.Node{Id: 1, Name: "one", Enable: true, LinkAddress: "first.example", AgentProtocol: 2, AssignmentMode: model.NodeAssignmentWhole}
	client := model.Client{Id: 1, Name: "one", Enable: true, Inbounds: json.RawMessage(`[1]`), Links: json.RawMessage(`[]`), Config: json.RawMessage(`{"vless":{"name":"one","uuid":"11111111-1111-4111-8111-111111111111","flow":"xtls-rprx-vision"}}`)}
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	options, _ := json.Marshal(map[string]interface{}{"listen": "::", "listen_port": 14443, "tls": map[string]interface{}{"enabled": true, "server_name": "www.example.com", "reality": map[string]interface{}{"enabled": true, "private_key": base64.RawURLEncoding.EncodeToString(private.Bytes()), "short_id": []string{"01234567"}, "handshake": map[string]interface{}{"server": "www.example.com", "server_port": 443}}}})
	for _, value := range []interface{}{&node, &client, &model.Inbound{Id: 1, Tag: "same", Type: "vless", Enable: true, Options: json.RawMessage(`{}`)}, &model.NodeInbound{Id: 1, NodeId: 1, Tag: "same", Type: "vless", Enable: true, Options: options, ConnectionKey: "fixture"}} {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, node, client
}

func confirm(t *testing.T, db *gorm.DB, node model.Node) Snapshot {
	t.Helper()
	node.ControlEpoch, node.TemplateTargetVersion = "test-epoch", 1
	addFixtureAssignment(t, db, node.Id)
	s, err := Compile(db, node)
	if err != nil {
		t.Fatal(err)
	}
	s.Deployment.Revision = 5
	body, _ := json.Marshal(s)
	if err := db.Model(&model.Node{}).Where("id = ?", node.Id).Updates(map[string]interface{}{"applied_revision": 5, "applied_snapshot": json.RawMessage(body), "agent_running": true, "control_epoch": node.ControlEpoch, "template_target_version": node.TemplateTargetVersion}).Error; err != nil {
		t.Fatal(err)
	}
	acl, err := compileAccess(db, node, false, false)
	requireNodeTestSuccess(t, err)
	acl.Revision = 1
	body, _ = json.Marshal(acl)
	requireNodeTestSuccess(t, db.Save(&model.NodeAccessState{NodeId: node.Id, AppliedRevision: acl.Revision, AppliedSnapshot: body}).Error)
	return s
}

func TestPublicationUsesConfirmedSnapshotAndSeparateIDs(t *testing.T) {
	db, node, client := fixture(t)
	endpoints, err := ClientInbounds(db, client)
	if err != nil || len(endpoints) != 0 {
		t.Fatal("unconfirmed configuration was published", err)
	}
	s := confirm(t, db, node)
	if !strings.Contains(string(s.Deployment.Config), `"tag":"node-1-inbound-1"`) || strings.Contains(string(s.Deployment.Config), `"users"`) {
		t.Fatal("unexpected structural snapshot")
	}
	endpoints, err = ClientInbounds(db, client)
	if err != nil || len(endpoints) != 1 {
		t.Fatal("confirmed endpoint missing", err)
	}
	if !strings.Contains(string(endpoints[0].Addrs), "first.example") {
		t.Fatal("incorrect published node address")
	}
	public, _ := json.Marshal(endpoints[0])
	public = append(public, endpoints[0].OutJson...)
	var options map[string]interface{}
	_ = json.Unmarshal(s.Inbounds[0].Options, &options)
	key := options["tls"].(map[string]interface{})["reality"].(map[string]interface{})["private_key"].(string)
	if strings.Contains(string(public), key) || strings.Contains(string(public), "private_key") || strings.Contains(string(public), "handshake") {
		t.Fatal("server-only material leaked into endpoint")
	}
	if !strings.Contains(string(public), "public_key") {
		t.Fatal("REALITY public key was not derived")
	}
	if err := db.Model(&model.Node{}).Where("id = 1").Update("link_address", "pending.example").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.NodeInbound{}).Where("id = 1").Update("options", json.RawMessage(`{"listen_port":29999}`)).Error; err != nil {
		t.Fatal(err)
	}
	endpoints, err = ClientInbounds(db, client)
	if err != nil || len(endpoints) != 1 || !strings.Contains(string(endpoints[0].Addrs), "first.example") {
		t.Fatal("pending edit replaced working publication", err)
	}
	client.Config = json.RawMessage(`{"vless":{"uuid":"22222222-2222-4222-8222-222222222222"}}`)
	endpoints, err = ClientInbounds(db, client)
	if err != nil || len(endpoints) != 0 {
		t.Fatal("unconfirmed changed user credentials were published", err)
	}
	var count int64
	db.Model(&model.Inbound{}).Count(&count)
	if count != 1 {
		t.Fatal("remote publication modified local inbounds")
	}
}

func TestCurrentRevocationHidesPreviouslyAppliedEndpoint(t *testing.T) {
	db, node, client := fixture(t)
	confirm(t, db, node)
	client.Enable = false
	endpoints, err := ClientInbounds(db, client)
	if err != nil || len(endpoints) != 0 {
		t.Fatal("disabled client retained endpoint", err)
	}
	client.Enable = true
	client.Expiry = time.Now().Unix() - 1
	endpoints, err = ClientInbounds(db, client)
	if err != nil || len(endpoints) != 0 {
		t.Fatal("expired client retained endpoint", err)
	}
	client.Expiry = 0
	if err := db.Where("client_id = 1").Delete(&model.ClientNode{}).Error; err != nil {
		t.Fatal(err)
	}
	endpoints, err = ClientInbounds(db, client)
	if err != nil || len(endpoints) != 0 {
		t.Fatal("removed assignment retained endpoint", err)
	}
}

func TestAccessEpochRequiresNewConfirmationWithoutChangingCustomerKeys(t *testing.T) {
	db, node, client := fixture(t)
	confirm(t, db, node)
	client.NodeGeneration = 2
	endpoints, err := ClientInbounds(db, client)
	if err != nil || len(endpoints) != 0 {
		t.Fatal("prior access epoch was treated as confirmed", err)
	}
	if err := db.Save(&client).Error; err != nil {
		t.Fatal(err)
	}
	requireNodeTestSuccess(t, db.First(&node, node.Id).Error)
	acl, err := compileAccess(db, node, false, false)
	requireNodeTestSuccess(t, err)
	if len(acl.Clients) != 1 || acl.Clients[0].Name != "client/1/g2/n1" {
		t.Fatal("new node session identity missing")
	}
	if !strings.Contains(string(acl.Clients[0].Credentials["vless"]), "11111111-1111-4111-8111-111111111111") {
		t.Fatal("access epoch rotated customer UUID")
	}
}

func TestNodeClientTLSRejectsServerKeysAndNodeLocalPaths(t *testing.T) {
	_, _, _ = fixture(t)
	for _, raw := range []string{`{"private_key":"secret"}`, `{"reality":{"private_key":"secret"}}`, `{"certificate_path":"/node/cert.pem"}`, `{"client_key_path":"/node/client-key.pem"}`, `{"ech":{"config_path":"/node/ech.txt"}}`} {
		inbound := model.NodeInbound{Options: json.RawMessage(`{"tls":{"enabled":true}}`), ClientTLS: json.RawMessage(raw)}
		if _, _, err := clientTLS(inbound); err == nil {
			t.Fatal("accepted key/path in public node TLS", raw)
		}
	}
}

type testRuntime struct {
	running  bool
	reject   bool
	applies  int
	attempts int
}

func (r *testRuntime) Validate([]byte) error { return nil }
func (r *testRuntime) Apply([]byte) error {
	r.attempts++
	if r.reject {
		return errors.New("test rejection")
	}
	r.applies++
	r.running = true
	return nil
}
func (r *testRuntime) Close() error  { r.running = false; return nil }
func (r *testRuntime) Running() bool { return r.running }

func controlHandlerServer(t *testing.T, handler http.Handler) (*httptest.Server, ControlTLS) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err = x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	identity, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, Certificates: []tls.Certificate{identity}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, ControlTLS{CA: string(caPEM), Certificate: string(certPEM), Key: string(keyPEM)}
}

func addFixtureAssignment(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	requireNodeTestSuccess(t, db.Create(&model.ClientNode{ClientId: 1, NodeId: id, Active: true, Generation: 1}).Error)
}

func attachFixtureAgent(t *testing.T, db *gorm.DB, node *model.Node, manager *nodeagent.ManagerV2) {
	t.Helper()
	server, identity := controlHandlerServer(t, nodeagent.HandlerV2(manager))
	node.Address, node.ControlCA, node.ControlCertificate, node.ControlKey = server.URL, identity.CA, identity.Certificate, identity.Key
	node.ControlEpoch = manager.Status().Epoch
	addFixtureAssignment(t, db, node.Id)
	var inbound model.NodeInbound
	requireNodeTestSuccess(t, db.First(&inbound, whereNodeID, node.Id).Error)
	template := model.NodeTemplate{Name: "fixture", Kind: deploymentTemplate, SchemaVersion: 1, Scope: "private", OwnerNodeId: node.Id}
	spec := liveSpec()
	spec.Entries[0] = SpecEntry{Key: inbound.ConnectionKey, Name: inbound.Tag, Protocol: inbound.Type, Enable: true, Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":14443,"tls":{"enabled":false}}`)}
	requireNodeTestSuccess(t, db.Create(&template).Error)
	requireNodeTestSuccess(t, appendDeploymentSpec(db, &template, spec))
	requireNodeTestSuccess(t, bindLiveTemplate(db, node, template))
}
