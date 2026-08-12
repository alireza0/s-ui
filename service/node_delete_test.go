package service

import (
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func TestBlockNodeDeleteWithAssignedInbounds(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	ns := &NodeService{}

	node := testNode()
	if err := ns.Create(node); err != nil {
		t.Fatal("create node:", err)
	}

	// Attach an inbound to this node
	inbound := &model.Inbound{
		Type:   "vless",
		Tag:    "node-bound-inbound",
		NodeId: &node.Id,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal("create inbound:", err)
	}

	// Delete should fail because inbound is attached
	err := ns.Delete(node.Id)
	if err == nil {
		t.Fatal("expected error deleting node with assigned inbounds, got nil")
	}

	// Now remove the inbound
	db.Where("id = ?", inbound.Id).Delete(&model.Inbound{})

	// Delete should succeed
	if err := ns.Delete(node.Id); err != nil {
		t.Fatalf("unexpected error deleting node without inbounds: %v", err)
	}
}
