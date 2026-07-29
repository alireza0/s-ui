package service

import (
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func TestGetAllMasksApiToken(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	node := &model.Node{Name: "test", Address: "1.2.3.4", Port: 2095, Scheme: "https", BasePath: "/", ApiToken: "verysecretpassword123", Enable: true}
	db.Create(node)

	ns := &NodeService{}
	nodes, err := ns.GetAll()
	if err != nil {
		t.Fatal("GetAll:", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ApiToken == "verysecretpassword123" {
		t.Error("API token should be masked in GetAll response")
	}
	if nodes[0].ApiToken != "very****" {
		t.Errorf("masked token = %s, want 'very****'", nodes[0].ApiToken)
	}

	// GetById should return full token
	got, err := ns.GetById(node.Id)
	if err != nil {
		t.Fatal("GetById:", err)
	}
	if got.ApiToken != "verysecretpassword123" {
		t.Errorf("GetById token = %s, want full token", got.ApiToken)
	}
}

func TestMergeNodeTrafficSkipsZeroTraffic(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	client := &model.Client{Name: "zero-user", Enable: true, Inbounds: []byte("[]"), Links: []byte("[]")}
	db.Create(client)

	ns := &NodeService{}
	// Push zero traffic
	traffic := map[string]*UserTraffic{
		"zero-user": {Up: 0, Down: 0},
	}
	ns.MergeNodeTraffic(99, traffic)

	// Client should have 0 traffic (zero entries skipped)
	var got model.Client
	db.Where("name = ?", "zero-user").First(&got)
	if got.Up != 0 || got.Down != 0 {
		t.Errorf("zero traffic should be skipped, got up=%d down=%d", got.Up, got.Down)
	}
}

func TestGetUsersForInboundReturnsEmptySliceNotNil(t *testing.T) {
	setupNodeTestDB(t)
	ns := &NodeService{}
	// Call with a nonexistent inbound - should return empty slice, not nil
	users, err := ns.getUsersForInbound(99999, "vless")
	if err != nil {
		t.Fatal("getUsersForInbound:", err)
	}
	if users == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}
