package service

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func TestInboundNodeIdNullIsLocal(t *testing.T) {
	// An inbound with NodeId=nil should be treated as local
	inbound := &model.Inbound{Tag: "test", Type: "vless"}
	if IsRemoteInbound(inbound) {
		t.Error("nil NodeId should not be remote")
	}
}

func TestInboundNodeIdSetIsRemote(t *testing.T) {
	nid := uint(1)
	inbound := &model.Inbound{Tag: "test", Type: "vless", NodeId: &nid}
	if !IsRemoteInbound(inbound) {
		t.Error("non-nil NodeId should be remote")
	}
}

func TestInboundNodeIdZeroIsNotRemote(t *testing.T) {
	nid := uint(0)
	inbound := &model.Inbound{Tag: "test", Type: "vless", NodeId: &nid}
	if IsRemoteInbound(inbound) {
		t.Error("NodeId=0 should not be treated as remote")
	}
}

func TestInboundUnmarshalNodeId(t *testing.T) {
	raw := `{"type":"vless","tag":"test","node_id":5,"listen":"0.0.0.0","listen_port":443}`
	var inbound model.Inbound
	if err := inbound.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatal("unmarshal:", err)
	}
	if inbound.NodeId == nil {
		t.Fatal("NodeId should not be nil")
	}
	if *inbound.NodeId != 5 {
		t.Errorf("NodeId = %d, want 5", *inbound.NodeId)
	}
}

func TestInboundMarshalFullIncludesNodeId(t *testing.T) {
	nid := uint(3)
	inbound := model.Inbound{
		Id:     1,
		Type:   "vless",
		Tag:    "test",
		NodeId: &nid,
		Addrs:  json.RawMessage("[]"),
	}
	full, err := inbound.MarshalFull()
	if err != nil {
		t.Fatal("MarshalFull:", err)
	}
	got, ok := (*full)["node_id"].(*uint)
	if !ok || got == nil || *got != 3 {
		t.Errorf("node_id in MarshalFull = %v, want 3", (*full)["node_id"])
	}
}

func TestNodeApplyRequestJSON(t *testing.T) {
	req := &NodeApplyRequest{
		Tag:     "vless-in",
		Type:    "vless",
		Inbound: json.RawMessage(`{"type":"vless","tag":"vless-in"}`),
		Users:   []json.RawMessage{json.RawMessage(`{"uuid":"abc"}`)},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal("marshal:", err)
	}
	var decoded NodeApplyRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("unmarshal:", err)
	}
	if decoded.Tag != "vless-in" {
		t.Errorf("tag = %s, want vless-in", decoded.Tag)
	}
	if len(decoded.Users) != 1 {
		t.Errorf("users len = %d, want 1", len(decoded.Users))
	}
}

func TestGetAllConfigExcludesRemoteInbounds(t *testing.T) {
	setupNodeTestDB(t)
	db := database.GetDB()

	// Create a local inbound
	localInbound := &model.Inbound{
		Type:    "vless",
		Tag:     "local-vless",
		Options: json.RawMessage(`{"listen":"0.0.0.0","listen_port":443}`),
	}
	db.Create(localInbound)

	// Create a remote inbound (node_id set)
	nid := uint(999)
	remoteInbound := &model.Inbound{
		Type:    "vless",
		Tag:     "remote-vless",
		NodeId:  &nid,
		Options: json.RawMessage(`{"listen":"0.0.0.0","listen_port":444}`),
	}
	db.Create(remoteInbound)

	is := &InboundService{}
	configs, err := is.GetAllConfig(db)
	if err != nil {
		t.Fatal("GetAllConfig:", err)
	}

	// Should only include the local inbound
	if len(configs) != 1 {
		t.Fatalf("expected 1 local config, got %d", len(configs))
	}

	// Verify it's the local one by checking the tag
	var parsed map[string]interface{}
	if err := json.Unmarshal(configs[0], &parsed); err != nil {
		t.Fatal("unmarshal config:", err)
	}
	if tag, _ := parsed["tag"].(string); tag != "local-vless" {
		t.Errorf("expected local-vless tag, got %s", tag)
	}
}
