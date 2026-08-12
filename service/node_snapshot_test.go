package service

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func testNode() *model.Node {
	return &model.Node{
		Name:                "test-node",
		Address:             "1.2.3.4",
		Port:                2095,
		Scheme:              "https",
		BasePath:            "/app/",
		ApiToken:            "test-token-123",
		Enable:              true,
		AllowPrivateAddress:  true,
		TlsVerifyMode:       "skip",
		InboundSyncMode:     "selected",
		InboundTags:         "[]",
	}
}

func setupTestDB(t *testing.T) {
	t.Helper()
	setupNodeTestDB(t) // reuse from node_test.go
}

func TestBuildNodeSnapshotType(t *testing.T) {
	setupTestDB(t)
	// BuildNodeSnapshot should return a valid struct even without a running core
	ns := &NodeService{}
	snap, err := ns.BuildNodeSnapshot()
	if err != nil {
		t.Fatal("BuildNodeSnapshot failed:", err)
	}
	if snap == nil {
		t.Fatal("snapshot is nil")
	}
	// Core is not running in tests
	if snap.CoreRunning {
		t.Error("expected CoreRunning=false without a running core")
	}
}

func TestNodeSnapshotJSON(t *testing.T) {
	snap := &NodeSnapshot{
		PanelVersion: "1.5.4",
		CoreRunning:  true,
		CpuPercent:   12.5,
		MemPercent:   45.2,
		Uptime:       3600,
		UserTraffic: map[string]*UserTraffic{
			"alice": {Up: 1000, Down: 2000},
			"bob":   {Up: 500, Down: 1500},
		},
		OnlineUsers: []string{"alice"},
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal("marshal failed:", err)
	}

	var decoded NodeSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("unmarshal failed:", err)
	}
	if decoded.PanelVersion != "1.5.4" {
		t.Errorf("panelVersion = %s, want 1.5.4", decoded.PanelVersion)
	}
	if decoded.UserTraffic["alice"].Up != 1000 {
		t.Errorf("alice up = %d, want 1000", decoded.UserTraffic["alice"].Up)
	}
	if len(decoded.OnlineUsers) != 1 || decoded.OnlineUsers[0] != "alice" {
		t.Errorf("onlineUsers = %v, want [alice]", decoded.OnlineUsers)
	}
}

func TestApplySnapshotUpdatesFields(t *testing.T) {
	setupTestDB(t)
	ns := &NodeService{}
	node := testNode()
	if err := ns.Create(node); err != nil {
		t.Fatal("create:", err)
	}

	snap := &NodeSnapshot{
		PanelVersion: "1.5.4",
		CoreRunning:  true,
		CpuPercent:   25.0,
		MemPercent:   50.0,
		Uptime:       7200,
	}
	if err := ns.ApplySnapshot(node.Id, snap); err != nil {
		t.Fatal("ApplySnapshot:", err)
	}

	got, err := ns.GetById(node.Id)
	if err != nil {
		t.Fatal("GetById:", err)
	}
	if got.Status != "online" {
		t.Errorf("status = %s, want online", got.Status)
	}
	if got.PanelVersion != "1.5.4" {
		t.Errorf("panelVersion = %s, want 1.5.4", got.PanelVersion)
	}
	if !got.CoreRunning {
		t.Error("coreRunning should be true")
	}
	if got.CpuPercent != 25.0 {
		t.Errorf("cpuPercent = %f, want 25.0", got.CpuPercent)
	}
	if got.MemPercent != 50.0 {
		t.Errorf("memPercent = %f, want 50.0", got.MemPercent)
	}
	if got.LastHeartbeat == 0 {
		t.Error("lastHeartbeat should be set")
	}
}

func TestMarkNodeOffline(t *testing.T) {
	setupTestDB(t)
	ns := &NodeService{}
	node := testNode()
	if err := ns.Create(node); err != nil {
		t.Fatal("create:", err)
	}

	// First make it online
	snap := &NodeSnapshot{CoreRunning: true}
	_ = ns.ApplySnapshot(node.Id, snap)

	// Then mark offline
	if err := ns.MarkNodeOffline(node.Id, "connection refused"); err != nil {
		t.Fatal("MarkNodeOffline:", err)
	}

	got, err := ns.GetById(node.Id)
	if err != nil {
		t.Fatal("GetById:", err)
	}
	if got.Status != "offline" {
		t.Errorf("status = %s, want offline", got.Status)
	}
	if got.LastError != "connection refused" {
		t.Errorf("lastError = %q, want 'connection refused'", got.LastError)
	}
	if got.CoreRunning {
		t.Error("coreRunning should be false when offline")
	}
}

func TestGetEnabledNodes(t *testing.T) {
	setupTestDB(t)
	ns := &NodeService{}

	// Create two nodes, one enabled, one disabled
	n1 := testNode()
	n1.Name = "enabled-node"
	n1.Enable = true
	if err := ns.Create(n1); err != nil {
		t.Fatal("create n1:", err)
	}

	n2 := testNode()
	n2.Name = "disabled-node"
	n2.Enable = true
	if err := ns.Create(n2); err != nil {
		t.Fatal("create n2:", err)
	}
	// Disable n2
	if err := ns.SetEnable(n2.Id, false); err != nil {
		t.Fatal("disable n2:", err)
	}

	enabled, err := ns.GetEnabledNodes()
	if err != nil {
		t.Fatal("GetEnabledNodes:", err)
	}
	if len(enabled) != 1 {
		t.Fatalf("expected 1 enabled node, got %d", len(enabled))
	}
	if enabled[0].Name != "enabled-node" {
		t.Errorf("got name %s, want enabled-node", enabled[0].Name)
	}
}
