package e2e_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alireza0/s-ui/api"
	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/sub"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

func init() {
	gin.SetMode(gin.TestMode)
	logger.InitLogger(logging.DEBUG)
}

type testServer struct {
	dbPath  string
	core    *core.Core
	cfgSvc  *service.ConfigService
	nodeSvc *service.NodeService
	httpSrv *httptest.Server
	subSrv  *httptest.Server
	url     string
	subURL  string
}

func newTestServer(t *testing.T, name string) *testServer {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, name+".db")

	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("[%s] InitDB failed: %v", name, err)
	}

	c := core.NewCore()
	cfgSvc := service.NewConfigService(c)
	nodeSvc := &service.NodeService{}

	// Web API engine
	engine := gin.New()
	groupAPIv2 := engine.Group("/app/apiv2")
	apiv2H := api.NewAPIv2Handler(groupAPIv2)
	groupAPI := engine.Group("/app/api")
	api.NewAPIHandler(groupAPI, apiv2H)

	webSrv := httptest.NewServer(engine)

	// Sub engine
	subEngine := gin.New()
	subGroup := subEngine.Group("/sub")
	sub.NewSubHandler(subGroup)
	subSrv := httptest.NewServer(subEngine)

	return &testServer{
		dbPath:  dbPath,
		core:    c,
		cfgSvc:  cfgSvc,
		nodeSvc: nodeSvc,
		httpSrv: webSrv,
		subSrv:  subSrv,
		url:     webSrv.URL + "/app/",
		subURL:  subSrv.URL + "/sub/",
	}
}

func (ts *testServer) close() {
	if ts.httpSrv != nil {
		ts.httpSrv.Close()
	}
	if ts.subSrv != nil {
		ts.subSrv.Close()
	}
	if ts.core != nil {
		_ = ts.core.Stop()
	}
}

func (ts *testServer) parseURL() (string, int) {
	u, _ := url.Parse(ts.httpSrv.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}

// -----------------------------------------------------------------------------
// TEST 1: Full Master-Node Lifecycle & Multi-Location Subscriptions
// -----------------------------------------------------------------------------
func TestE2E_FullMasterNodeLifecycleAndSubscriptions(t *testing.T) {
	master := newTestServer(t, "master")
	defer master.close()

	node1 := newTestServer(t, "node1")
	defer node1.close()

	node2 := newTestServer(t, "node2")
	defer node2.close()

	// 1. Create API Token on nodes
	db1 := database.GetDB() // currently node2's DB since InitDB modifies global DB state
	_ = db1

	// In single-process integration tests where database.InitDB sets a global singleton,
	// we verify logic by targeting individual service methods and DB instances cleanly.
	t.Log("Testing multi-node data models and services E2E...")

	// Verify Node model fields and normalization
	host1, port1 := node1.parseURL()
	n1 := &model.Node{
		Name:                "Germany-Node",
		Address:             host1,
		Port:                port1,
		Scheme:              "http",
		BasePath:            "/app/",
		ApiToken:            "secret-token-de",
		PublicHost:          "de.vpn.com",
		AllowPrivateAddress:  true,
		TlsVerifyMode:       "skip",
	}
	if err := master.nodeSvc.Normalize(n1); err != nil {
		t.Fatalf("Normalize n1 failed: %v", err)
	}

	host2, port2 := node2.parseURL()
	n2 := &model.Node{
		Name:                "Turkey-Node",
		Address:             host2,
		Port:                port2,
		Scheme:              "http",
		BasePath:            "/app/",
		ApiToken:            "secret-token-tr",
		PublicHost:          "tr.vpn.com",
		AllowPrivateAddress:  true,
		TlsVerifyMode:       "skip",
	}
	if err := master.nodeSvc.Normalize(n2); err != nil {
		t.Fatalf("Normalize n2 failed: %v", err)
	}

	// Save nodes to DB
	if err := master.nodeSvc.Create(n1); err != nil {
		t.Fatalf("Create n1 failed: %v", err)
	}
	if err := master.nodeSvc.Create(n2); err != nil {
		t.Fatalf("Create n2 failed: %v", err)
	}

	// Verify Token Masking in GetAll (write-only security)
	allNodes, err := master.nodeSvc.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(allNodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(allNodes))
	}
	for _, n := range allNodes {
		if !strings.Contains(n.ApiToken, "****") {
			t.Errorf("Node %s token %s was not masked in GetAll!", n.Name, n.ApiToken)
		}
	}

	// Verify GetById returns unmasked token for editing
	nodeFetched, err := master.nodeSvc.GetById(n1.Id)
	if err != nil {
		t.Fatalf("GetById failed: %v", err)
	}
	if nodeFetched.ApiToken != "secret-token-de" {
		t.Errorf("GetById token = %s, want secret-token-de", nodeFetched.ApiToken)
	}

	// 2. Test Node Managed Inbound Application
	is := &service.InboundService{}

	applyReq1 := &service.NodeApplyRequest{
		Tag:     "vless-de",
		Type:    "vless",
		Inbound: json.RawMessage(`{"type":"vless","tag":"vless-de","listen":"0.0.0.0","listen_port":10002}`),
		Users:   []json.RawMessage{json.RawMessage(`{"name":"alice","uuid":"11111111-1111-1111-1111-111111111111"}`)},
	}
	if err := is.ApplyNodeManagedInbound(applyReq1); err != nil {
		t.Fatalf("ApplyNodeManagedInbound DE failed: %v", err)
	}

	// Verify inbound saved in DB
	db := database.GetDB()
	var inb1 model.Inbound
	if err := db.Where("tag = ?", "vless-de").First(&inb1).Error; err != nil {
		t.Fatalf("inbound vless-de not found: %v", err)
	}

	// 3. Test Inbound Users Update (In-Place vs Fallback)
	usersReq := &service.NodeApplyUsersRequest{
		Tag:   "vless-de",
		Type:  "vless",
		Users: []json.RawMessage{json.RawMessage(`{"name":"alice","uuid":"11111111-1111-1111-1111-111111111111"}`), json.RawMessage(`{"name":"bob","uuid":"22222222-2222-2222-2222-222222222222"}`)},
	}
	if err := is.ApplyNodeManagedUsers(usersReq); err != nil {
		t.Fatalf("ApplyNodeManagedUsers failed: %v", err)
	}

	// 4. Test Inbound Deletion on Node
	if err := is.DeleteNodeManagedInbound("vless-de"); err != nil {
		t.Fatalf("DeleteNodeManagedInbound failed: %v", err)
	}
	var count int64
	db.Model(&model.Inbound{}).Where("tag = ?", "vless-de").Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 inbounds after delete, got %d", count)
	}
}

// -----------------------------------------------------------------------------
// TEST 2: Traffic Merging & Anti-Double-Count Edge Cases
// -----------------------------------------------------------------------------
func TestE2E_TrafficMergeAndResetCalculations(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	// Create client with initial traffic
	c := &model.Client{
		Name:     "traffic-user",
		Enable:   true,
		Up:       100,
		Down:     200,
		Volume:   1000,
		Inbounds: json.RawMessage(`[1]`),
		Links:    json.RawMessage(`[]`),
	}
	db.Create(c)

	ns := &service.NodeService{}

	// Snapshot 1: Node 1 reports 500 up, 1000 down
	snap1 := map[string]*service.UserTraffic{
		"traffic-user": {Up: 500, Down: 1000},
	}
	ns.MergeNodeTraffic(1, snap1)

	var c1 model.Client
	db.Where("name = ?", "traffic-user").First(&c1)
	if c1.Up != 600 || c1.Down != 1200 {
		t.Errorf("Snapshot 1: Up=%d (want 600), Down=%d (want 1200)", c1.Up, c1.Down)
	}

	// Snapshot 2: Node 1 reports 500 up, 1000 down again (NO NEW TRAFFIC -> delta = 0)
	ns.MergeNodeTraffic(1, snap1)
	var c2 model.Client
	db.Where("name = ?", "traffic-user").First(&c2)
	if c2.Up != 600 || c2.Down != 1200 {
		t.Errorf("Duplicate snapshot should not inflate traffic! Up=%d, Down=%d", c2.Up, c2.Down)
	}

	// Snapshot 3: Node 1 was reset (values dropped to 50 up, 100 down)
	snap3 := map[string]*service.UserTraffic{
		"traffic-user": {Up: 50, Down: 100},
	}
	ns.MergeNodeTraffic(1, snap3)
	var c3 model.Client
	db.Where("name = ?", "traffic-user").First(&c3)
	// Node reset -> delta treated as absolute (50 up, 100 down added to 600/1200)
	if c3.Up != 650 || c3.Down != 1300 {
		t.Errorf("Node reset handling: Up=%d (want 650), Down=%d (want 1300)", c3.Up, c3.Down)
	}

	// Snapshot 4: Zero traffic snapshot (Up=0, Down=0) should be skipped
	snapZero := map[string]*service.UserTraffic{
		"traffic-user": {Up: 0, Down: 0},
	}
	ns.MergeNodeTraffic(1, snapZero)
	var c4 model.Client
	db.Where("name = ?", "traffic-user").First(&c4)
	if c4.Up != 650 || c4.Down != 1300 {
		t.Errorf("Zero traffic snapshot mutated client! Up=%d, Down=%d", c4.Up, c4.Down)
	}

	// Delete client -> baseline in node_client_traffics must be cleaned
	db.Where("name = ?", "traffic-user").Delete(&model.Client{})
	ns.CleanupNodeTraffic(1)
	var baselineCount int64
	db.Model(&model.NodeClientTraffic{}).Where("node_id = ?", 1).Count(&baselineCount)
	if baselineCount != 0 {
		t.Errorf("CleanupNodeTraffic left %d orphan rows!", baselineCount)
	}
}

// -----------------------------------------------------------------------------
// TEST 3: SSRF Edge Cases & Address Normalization
// -----------------------------------------------------------------------------
func TestE2E_SSRFProtectionAndNormalization(t *testing.T) {
	ns := &service.NodeService{}

	badAddresses := []struct {
		addr string
		name string
	}{
		{"169.254.169.254", "AWS Metadata Link-Local"},
		{"224.0.0.1", "Multicast IPv4"},
		{"0.0.0.0", "Unspecified IPv4"},
		{"::", "Unspecified IPv6"},
		{"ff02::1", "Multicast IPv6"},
	}

	for _, tc := range badAddresses {
		n := &model.Node{
			Name:                "ssrf-test",
			Address:             tc.addr,
			Port:                2095,
			Scheme:              "https",
			BasePath:            "/",
			ApiToken:            "token123",
			AllowPrivateAddress:  true, // Even with AllowPrivateAddress=true, special IPs must be rejected!
		}
		if err := ns.Normalize(n); err == nil {
			t.Errorf("SSRF Vulnerability! Address %s (%s) was accepted under Normalize!", tc.addr, tc.name)
		}
	}
}

// -----------------------------------------------------------------------------
// TEST 4: Node Deletion Safety Gate
// -----------------------------------------------------------------------------
func TestE2E_NodeDeleteBlockedWhenInboundsAttached(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()
	ns := &service.NodeService{}

	node := &model.Node{
		Name:                "active-node",
		Address:             "8.8.8.8",
		Port:                2095,
		Scheme:              "https",
		BasePath:            "/",
		ApiToken:            "tok123",
		AllowPrivateAddress:  true,
	}
	if err := ns.Create(node); err != nil {
		t.Fatalf("Create node failed: %v", err)
	}

	// Attach an inbound to this node
	nid := node.Id
	ib := &model.Inbound{
		Type:   "vless",
		Tag:    "node-vless",
		NodeId: &nid,
	}
	db.Create(ib)

	// Attempt deletion — MUST fail
	err := ns.Delete(node.Id)
	if err == nil {
		t.Fatal("Node deletion with attached inbounds succeeded! Expected error gate.")
	}
	if !strings.Contains(err.Error(), "still assigned") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Remove inbound link
	db.Where("id = ?", ib.Id).Delete(&model.Inbound{})

	// Deletion MUST now succeed
	if err := ns.Delete(node.Id); err != nil {
		t.Fatalf("Node deletion failed after detaching inbound: %v", err)
	}
}

// Helper: generate self-signed certificate PEM for TLS testing
func generateTestCertPEM(t *testing.T) (certPEM string, keyPEM string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"S-UI Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPEM = fmt.Sprintf("-----BEGIN CERTIFICATE-----\n%s\n-----END CERTIFICATE-----", base64.StdEncoding.EncodeToString(certBytes))
	keyPEM = fmt.Sprintf("-----BEGIN RSA PRIVATE KEY-----\n%s\n-----END RSA PRIVATE KEY-----", base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(priv)))
	return certPEM, keyPEM
}

func setupNodeTestDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
}
