package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func wholeNodeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := nodeAssignmentDB(t)
	for _, node := range []model.Node{{Id: 1, Name: "whole", Enable: true, AssignmentMode: model.NodeAssignmentWhole, AgentProtocol: 2, TemplateTargetVersion: 1}, {Id: 2, Name: "legacy", Enable: true, AssignmentMode: "prototype"}} {
		requireNodeTestSuccess(t, db.Create(&node).Error)
	}
	return db
}

func saveWholeClient(t *testing.T, db *gorm.DB, action string, value interface{}) error {
	t.Helper()
	body, err := json.Marshal(value)
	requireNodeTestSuccess(t, err)
	return db.Transaction(func(tx *gorm.DB) error {
		_, err := (&ClientService{}).Save(tx, action, body, "example.com")
		return err
	})
}

func TestWholeNodesCredentialsOmissionAndGeneration(t *testing.T) {
	db := wholeNodeDB(t)
	initial := map[string]interface{}{"name": "whole-client", "enable": true, "config": map[string]interface{}{"vless": map[string]interface{}{"name": "whole-client", "uuid": "11111111-1111-4111-8111-111111111111"}}, "inbounds": []uint{21}, "nodes": []uint{1}, "links": []interface{}{}}
	requireNodeTestSuccess(t, saveWholeClient(t, db, "new", initial))
	loaded, err := (&ClientService{}).Get("1")
	requireNodeTestSuccess(t, err)
	client := (*loaded)[0]
	cfg := assertWholeNodeCredentials(t, client)
	original := string(client.Config)
	for _, kind := range []string{"vmess", "trojan", "anytls"} {
		if len(cfg[kind]) == 0 {
			t.Fatalf("missing credentials for %s", kind)
		}
		delete(cfg, kind)
	}
	client.Config, _ = json.Marshal(cfg)
	requireNodeTestSuccess(t, saveWholeClient(t, db, "edit", client))
	loaded, err = (&ClientService{}).Get("1")
	requireNodeTestSuccess(t, err)
	client = (*loaded)[0]
	if string(client.Config) != original {
		t.Fatal("old-client edit changed the new protocol credentials")
	}
	var task model.NodeTask
	db.First(&task)
	beforeTask := task.Generation
	requireNodeTestSuccess(t, saveWholeClient(t, db, "edit", client))
	db.First(&task, task.Id)
	if task.Generation != beforeTask {
		t.Fatal("idempotent edit produced an access event")
	}
	client.Nodes = nil
	client.NodeInbounds = nil
	requireNodeTestSuccess(t, saveWholeClient(t, db, "edit", client))
	var grant model.ClientNode
	db.First(&grant)
	if !grant.Active || grant.Generation != 1 {
		t.Fatal("omitted nodes changed grant")
	}
	client.Nodes = json.RawMessage(`[]`)
	requireNodeTestSuccess(t, saveWholeClient(t, db, "edit", client))
	db.First(&grant)
	if grant.Active || grant.Generation != 2 {
		t.Fatal("clear did not preserve revoke tombstone")
	}
	client.Nodes = json.RawMessage(`[1]`)
	requireNodeTestSuccess(t, saveWholeClient(t, db, "edit", client))
	db.First(&grant)
	if !grant.Active || grant.Generation != 3 {
		t.Fatal("re-add lost revoke generation")
	}
	loaded, _ = (&ClientService{}).Get("1")
	client = (*loaded)[0]
	if string(client.Config) != original || client.NodeGeneration < 2 {
		t.Fatal("revoke changed credentials or lost access epoch")
	}
}

func TestWholeNodesInvalidAndLegacyInputsAreAtomic(t *testing.T) {
	db := wholeNodeDB(t)
	if err := saveWholeClient(t, db, "new", map[string]interface{}{"name": "old-api", "enable": true, "config": map[string]interface{}{}, "inbounds": []uint{21}, "nodeinbounds": []uint{1}, "links": []interface{}{}}); err == nil || !strings.Contains(err.Error(), "read_only") {
		t.Fatal("legacy API silently accepted an inbound on a converted node", err)
	}
	for _, invalid := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`[0]`), json.RawMessage(`[1,1]`), json.RawMessage(`[21]`), json.RawMessage(`[2]`), json.RawMessage(`["1"]`), json.RawMessage(`[1.2]`), json.RawMessage(`[-1]`)} {
		err := saveWholeClient(t, db, "new", map[string]interface{}{"name": "invalid", "enable": true, "config": map[string]interface{}{}, "inbounds": []uint{21}, "nodes": invalid, "links": []interface{}{}})
		if err == nil {
			t.Fatal("invalid node list accepted")
		}
	}
	var count int64
	db.Model(&model.Client{}).Count(&count)
	if count != 0 {
		t.Fatal("invalid input committed client")
	}
	requireNodeTestSuccess(t, db.Model(&model.Node{}).Where("id = 1").Updates(map[string]any{
		"applied_revision": 1, "applied_snapshot": json.RawMessage(`{"inbounds":[{"id":21}]}`),
	}).Error)
	requireNodeTestSuccess(t, saveWholeClient(t, db, "new", map[string]any{
		"name": "separate-ids", "enable": true, "config": map[string]any{}, "inbounds": []uint{21}, "nodes": []uint{1}, "links": []any{},
	}))
	clients, err := (&ClientService{}).Get("1")
	requireNodeTestSuccess(t, err)
	client := (*clients)[0]
	if string(client.Inbounds) != "[21]" || string(client.NodeInbounds) != "[21]" || string(client.Nodes) != "[1]" {
		t.Fatal("local and remote ID namespaces changed")
	}
	requireNodeTestSuccess(t, saveWholeClient(t, db, "edit", client))
	client.NodeInbounds = json.RawMessage(`[]`)
	if err := saveWholeClient(t, db, "edit", client); err == nil || !strings.Contains(err.Error(), "read_only") {
		t.Fatal("edited read-only node inbound projection was accepted", err)
	}

}

func TestWholeNodesBulkFailureAndDeletionQueue(t *testing.T) {
	db := wholeNodeDB(t)
	client := func(name string, node uint) map[string]interface{} {
		return map[string]interface{}{"name": name, "enable": true, "config": map[string]interface{}{}, "inbounds": []uint{21}, "nodes": []uint{node}, "links": []interface{}{}}
	}
	if err := saveWholeClient(t, db, "addbulk", []interface{}{client("first", 1), client("second", 999)}); err == nil {
		t.Fatal("invalid bulk accepted")
	}
	var count int64
	db.Model(&model.Client{}).Count(&count)
	if count != 0 {
		t.Fatal("bulk partially saved")
	}
	requireNodeTestSuccess(t, saveWholeClient(t, db, "addbulk", []interface{}{client("first", 1), client("second", 1)}))
	requireNodeTestSuccess(t, saveWholeClient(t, db, "delbulk", []uint{1, 2}))
	db.Model(&model.ClientNode{}).Where("active = ?", true).Count(&count)
	if count != 0 {
		t.Fatal("deletion left active grants")
	}
	db.Model(&model.ClientNode{}).Count(&count)
	if count != 2 {
		t.Fatal("deletion lost grant tombstones")
	}
	db.Model(&model.NodeTask{}).Where("node_id = ? AND kind = ?", 1, "access").Count(&count)
	if count != 1 {
		t.Fatal("deletion lost durable access intent")
	}
}

func TestWholeNodesQuotaAndResetPersistAccessEpoch(t *testing.T) {
	db := wholeNodeDB(t)
	requireNodeTestSuccess(t, saveWholeClient(t, db, "new", map[string]interface{}{"name": "quota-client", "enable": true, "config": map[string]interface{}{}, "inbounds": []uint{21}, "nodes": []uint{1}, "links": []interface{}{}}))
	requireNodeTestSuccess(t, db.Model(&model.Client{}).Where("id = ?", 1).Updates(map[string]interface{}{"volume": 1, "down": 2}).Error)
	requireNodeTestSuccess(t, db.Model(&model.Client{}).Where("id = 1").Updates(map[string]interface{}{"volume": 1, "up": 2}).Error)
	if _, err := (&ClientService{}).DepleteClients(); err != nil {
		t.Fatal(err)
	}
	var disabled model.Client
	db.First(&disabled, 1)
	if disabled.Enable || disabled.NodeGeneration == 0 {
		t.Fatal("depletion lost remote access event")
	}
	if _, err := (&ClientService{}).ResetAllClientsTraffic(); err != nil {
		t.Fatal(err)
	}
	var enabled model.Client
	db.First(&enabled, 1)
	if !enabled.Enable || enabled.NodeGeneration <= disabled.NodeGeneration {
		t.Fatal("reset lost re-enable generation")
	}
}

func assertWholeNodeCredentials(t *testing.T, client model.Client) map[string]map[string]interface{} {
	t.Helper()
	if string(client.Nodes) != "[1]" || string(client.Inbounds) != "[21]" {
		t.Fatal("whole-node/local selection mismatch")
	}
	var cfg map[string]map[string]interface{}
	json.Unmarshal(client.Config, &cfg)
	if cfg["vless"]["uuid"] != "11111111-1111-4111-8111-111111111111" || cfg["tuic"]["uuid"] == nil || cfg["hysteria2"]["password"] == nil {
		t.Fatal("credentials were replaced or not prepared for future protocols")
	}
	if _, ok := cfg["vless"]["flow"]; ok {
		t.Fatal("assigning a node changed existing VLESS settings")
	}
	return cfg
}
