package service

import (
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func TestMergeNodeTrafficDelta(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	// Create a client
	client := &model.Client{
		Name:     "alice",
		Enable:   true,
		Up:       0,
		Down:     0,
		Inbounds: []byte("[]"),
		Links:    []byte("[]"),
	}
	db.Create(client)

	ns := &NodeService{}

	// First snapshot: alice has 1000 up, 2000 down on node 1
	traffic1 := map[string]*UserTraffic{
		"alice": {Up: 1000, Down: 2000},
	}
	ns.MergeNodeTraffic(1, traffic1)

	// Check master client has the traffic added
	var updated model.Client
	db.Model(&model.Client{}).Where("name = ?", "alice").First(&updated)
	if updated.Up != 1000 {
		t.Errorf("after first merge: up = %d, want 1000", updated.Up)
	}
	if updated.Down != 2000 {
		t.Errorf("after first merge: down = %d, want 2000", updated.Down)
	}

	// Second snapshot: alice has 1500 up, 3000 down (delta: +500 up, +1000 down)
	traffic2 := map[string]*UserTraffic{
		"alice": {Up: 1500, Down: 3000},
	}
	ns.MergeNodeTraffic(1, traffic2)

	db.Model(&model.Client{}).Where("name = ?", "alice").First(&updated)
	if updated.Up != 1500 {
		t.Errorf("after second merge: up = %d, want 1500", updated.Up)
	}
	if updated.Down != 3000 {
		t.Errorf("after second merge: down = %d, want 3000", updated.Down)
	}
}

func TestMergeNodeTrafficNodeReset(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	client := &model.Client{
		Name:     "bob",
		Enable:   true,
		Up:       5000,
		Down:     10000,
		Inbounds: []byte("[]"),
		Links:    []byte("[]"),
	}
	db.Create(client)

	ns := &NodeService{}

	// Set initial baseline
	traffic1 := map[string]*UserTraffic{
		"bob": {Up: 3000, Down: 6000},
	}
	ns.MergeNodeTraffic(2, traffic1)

	// Node was reset — new values are lower than baseline
	traffic2 := map[string]*UserTraffic{
		"bob": {Up: 500, Down: 1000},
	}
	ns.MergeNodeTraffic(2, traffic2)

	var updated model.Client
	db.Model(&model.Client{}).Where("name = ?", "bob").First(&updated)
	// After reset: master should have original + initial delta + reset amount
	// original=5000 + first delta=3000 + reset delta=500 = 8500
	if updated.Up != 8500 {
		t.Errorf("after reset merge: up = %d, want 8500", updated.Up)
	}
}

func TestMergeNodeTrafficNoDoubleCount(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	client := &model.Client{
		Name:     "carol",
		Enable:   true,
		Up:       0,
		Down:     0,
		Inbounds: []byte("[]"),
		Links:    []byte("[]"),
	}
	db.Create(client)

	ns := &NodeService{}

	// Same traffic reported twice — should only count once
	traffic := map[string]*UserTraffic{
		"carol": {Up: 1000, Down: 2000},
	}
	ns.MergeNodeTraffic(3, traffic)
	ns.MergeNodeTraffic(3, traffic) // same values

	var updated model.Client
	db.Model(&model.Client{}).Where("name = ?", "carol").First(&updated)
	if updated.Up != 1000 {
		t.Errorf("double report: up = %d, want 1000", updated.Up)
	}
	if updated.Down != 2000 {
		t.Errorf("double report: down = %d, want 2000", updated.Down)
	}
}

func TestCleanupNodeTraffic(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	// Create some baselines
	db.Create(&model.NodeClientTraffic{NodeId: 10, ClientName: "alice", Up: 100, Down: 200})
	db.Create(&model.NodeClientTraffic{NodeId: 10, ClientName: "bob", Up: 300, Down: 400})
	db.Create(&model.NodeClientTraffic{NodeId: 11, ClientName: "alice", Up: 50, Down: 100})

	ns := &NodeService{}
	ns.CleanupNodeTraffic(10)

	// Node 10 baselines should be gone
	var count int64
	db.Model(&model.NodeClientTraffic{}).Where("node_id = ?", 10).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 baselines for node 10, got %d", count)
	}

	// Node 11 should still exist
	db.Model(&model.NodeClientTraffic{}).Where("node_id = ?", 11).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 baseline for node 11, got %d", count)
	}
}
