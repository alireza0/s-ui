package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func nodeAssignmentDB(t *testing.T) *gorm.DB {
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
	if err := db.Create(&model.Inbound{Id: 21, Type: "direct", Tag: "local", Enable: true, Options: json.RawMessage(`{"listen_port":12345}`)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.NodeInbound{Id: 1, NodeId: 1, Tag: "remote", Type: "vless", Enable: true, Options: json.RawMessage(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestQuickDisableReenablePersistsNodeSessionRevocation(t *testing.T) {
	db := wholeNodeDB(t)
	s := &ClientService{}
	save := func(action string, value interface{}) {
		t.Helper()
		body, _ := json.Marshal(value)
		if err := db.Transaction(func(tx *gorm.DB) error { _, err := s.Save(tx, action, body, "localhost"); return err }); err != nil {
			t.Fatal(err)
		}
	}
	save("new", map[string]interface{}{"name": "epoch", "enable": true, "config": map[string]interface{}{}, "inbounds": []uint{21}, "nodes": []uint{1}, "links": []interface{}{}})
	clients, err := s.Get("1")
	if err != nil {
		t.Fatal(err)
	}
	client := (*clients)[0]
	originalConfig := string(client.Config)
	client.Enable = false
	save("edit", client)
	clients, _ = s.Get("1")
	disabled := (*clients)[0]
	client = disabled
	client.Enable = true
	save("edit", client)
	clients, _ = s.Get("1")
	enabled := (*clients)[0]
	if disabled.NodeGeneration == 0 || enabled.NodeGeneration <= disabled.NodeGeneration {
		t.Fatal("quick disable/re-enable was coalesced without a persisted node access change")
	}
	if string(enabled.Config) != originalConfig {
		t.Fatal("session revocation changed customer credentials")
	}
	before := enabled.NodeGeneration
	save("edit", enabled)
	clients, _ = s.Get("1")
	if (*clients)[0].NodeGeneration != before {
		t.Fatal("idempotent edit unnecessarily changed node access epoch")
	}
}
