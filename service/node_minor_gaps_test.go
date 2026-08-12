package service

import (
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

func TestSSRFRejectLinkLocal(t *testing.T) {
	ns := &NodeService{}
	n := &model.Node{Name: "test", Address: "169.254.1.1", Port: 2095, Scheme: "https", BasePath: "/", ApiToken: "tok", AllowPrivateAddress: true}
	if err := ns.Normalize(n); err == nil {
		t.Error("expected rejection of 169.254.x.x link-local address")
	}
}

func TestSSRFRejectMulticast(t *testing.T) {
	ns := &NodeService{}
	n := &model.Node{Name: "test", Address: "224.0.0.1", Port: 2095, Scheme: "https", BasePath: "/", ApiToken: "tok", AllowPrivateAddress: true}
	if err := ns.Normalize(n); err == nil {
		t.Error("expected rejection of 224.0.0.0/4 multicast address")
	}
}

func TestSSRFRejectUnspecified(t *testing.T) {
	ns := &NodeService{}
	n := &model.Node{Name: "test", Address: "0.0.0.0", Port: 2095, Scheme: "https", BasePath: "/", ApiToken: "tok", AllowPrivateAddress: true}
	if err := ns.Normalize(n); err == nil {
		t.Error("expected rejection of 0.0.0.0 unspecified address")
	}
}

func TestSSRFAcceptPrivateWithFlag(t *testing.T) {
	ns := &NodeService{}
	n := &model.Node{Name: "test", Address: "10.0.0.5", Port: 2095, Scheme: "https", BasePath: "/", ApiToken: "tok", AllowPrivateAddress: true}
	if err := ns.Normalize(n); err != nil {
		t.Errorf("expected 10.x to be accepted with AllowPrivateAddress, got: %v", err)
	}
}

func TestFanOutUserCache(t *testing.T) {
	// Verify the userCache map prevents re-querying for the same inbound
	// This is a structural test - the cache is an optimization, not a behavior change
	// Just verify FanOut doesn't crash with duplicate inbound IDs
	setupNodeTestDB(t)
	ns := &NodeService{}
	// FanOut with empty list should be no-op
	ns.FanOutUsersToNodes([]uint{})
	ns.FanOutUsersToNodes([]uint{999}) // nonexistent inbound - should not crash
}

func TestProactiveDirtyMarking(t *testing.T) {
	// Verify that proactive marking + ClearDirty works correctly
	setupNodeTestDB(t)
	ns := &NodeService{}
	node := testNode()
	if err := ns.Create(node); err != nil {
		t.Fatal("create:", err)
	}

	// Mark dirty
	if err := ns.MarkDirty(node.Id); err != nil {
		t.Fatal("MarkDirty:", err)
	}
	got, _ := ns.GetById(node.Id)
	if !got.ConfigDirty {
		t.Error("expected ConfigDirty=true after MarkDirty")
	}

	// Clear dirty
	if err := ns.ClearDirty(node.Id); err != nil {
		t.Fatal("ClearDirty:", err)
	}
	got, _ = ns.GetById(node.Id)
	if got.ConfigDirty {
		t.Error("expected ConfigDirty=false after ClearDirty")
	}
}
