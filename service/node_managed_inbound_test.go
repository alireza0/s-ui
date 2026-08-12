package service

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func TestApplyNodeManagedInbound(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	is := &InboundService{}

	req := &NodeApplyRequest{
		Tag:     "managed-vless",
		Type:    "vless",
		Inbound: json.RawMessage(`{"type":"vless","tag":"managed-vless","listen":"0.0.0.0","listen_port":20001}`),
		Users:   []json.RawMessage{json.RawMessage(`{"uuid":"user-1-uuid"}`)},
	}

	if err := is.ApplyNodeManagedInbound(req); err != nil {
		t.Fatal("ApplyNodeManagedInbound:", err)
	}

	// Verify inbound saved in DB
	var inbound model.Inbound
	if err := db.Where("tag = ?", "managed-vless").First(&inbound).Error; err != nil {
		t.Fatal("inbound not found in DB:", err)
	}
	if inbound.Type != "vless" {
		t.Errorf("type = %s, want vless", inbound.Type)
	}
}

func TestDeleteNodeManagedInbound(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	is := &InboundService{}

	req := &NodeApplyRequest{
		Tag:     "managed-to-delete",
		Type:    "vless",
		Inbound: json.RawMessage(`{"type":"vless","tag":"managed-to-delete","listen":"0.0.0.0","listen_port":20002}`),
	}

	if err := is.ApplyNodeManagedInbound(req); err != nil {
		t.Fatal("ApplyNodeManagedInbound:", err)
	}

	// Delete it
	if err := is.DeleteNodeManagedInbound("managed-to-delete"); err != nil {
		t.Fatal("DeleteNodeManagedInbound:", err)
	}

	// Verify gone from DB
	var count int64
	db.Model(&model.Inbound{}).Where("tag = ?", "managed-to-delete").Count(&count)
	if count != 0 {
		t.Errorf("expected 0 inbounds after delete, got %d", count)
	}
}

func TestApplyNodeManagedUsersValidation(t *testing.T) {
	is := &InboundService{}
	if err := is.ApplyNodeManagedUsers(nil); err == nil {
		t.Error("expected error on nil request")
	}
	if err := is.ApplyNodeManagedUsers(&NodeApplyUsersRequest{}); err == nil {
		t.Error("expected error on empty tag")
	}
}

func TestBuildConfigWithUsers(t *testing.T) {
	is := &InboundService{}

	inboundConfig := json.RawMessage(`{"type":"vless","tag":"v1"}`)
	users := []json.RawMessage{json.RawMessage(`{"uuid":"123"}`)}

	built, err := is.buildConfigWithUsers(inboundConfig, users)
	if err != nil {
		t.Fatal("buildConfigWithUsers:", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(built, &parsed); err != nil {
		t.Fatal("unmarshal built config:", err)
	}

	uList, ok := parsed["users"].([]interface{})
	if !ok || len(uList) != 1 {
		t.Fatalf("expected 1 user in parsed config, got %v", parsed["users"])
	}
}
