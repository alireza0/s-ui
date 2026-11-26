package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestR1AdditiveSchemaAndPrivateStateBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	old, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := old.AutoMigrate(&model.Node{}, &model.Client{}); err != nil {
		t.Fatal(err)
	}
	if err := old.Create(&model.Node{Id: 1, Name: "legacy", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := old.Migrator().DropColumn(&model.Node{}, "AssignmentMode"); err != nil {
		t.Fatal(err)
	}
	sql, _ := old.DB()
	sql.Close()
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	liveSQL, _ := db.DB()
	t.Cleanup(func() { liveSQL.Close() })
	var node model.Node
	db.First(&node, 1)
	if node.AssignmentMode == model.NodeAssignmentWhole {
		t.Fatal("schema migration activated whole-node access")
	}
	var count int64
	db.Model(&model.ClientNode{}).Count(&count)
	if count != 0 {
		t.Fatal("schema migration generated grants")
	}
	for _, row := range []interface{}{
		&model.ClientNode{ClientId: 7, NodeId: 1, Active: false, Generation: 9},
		&model.NodeTemplateVersion{TemplateId: 3, Version: 4, SchemaVersion: 1, Digest: "fixture", Payload: json.RawMessage(`{"schema_version":1,"config":{},"entries":[]}`)},
		&model.NodeGeneratedMaterial{NodeId: 1, ConnectionKey: "stable", Purpose: "reality_private_key", EncryptedValue: "synthetic-encrypted-material"},
		&model.NodeOverride{NodeId: 1, ConnectionKey: "stable", Path: "/tls/server_name", Operation: "set", Source: json.RawMessage(`{"kind":"node_parameter","name":"primary_domain"}`)},
		&model.NodeTask{NodeId: 1, Kind: "access", Generation: 5, State: "queued"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	contents, err := GetDb("")
	if err != nil {
		t.Fatal(err)
	}
	restored := openBackup(t, contents)
	var material model.NodeGeneratedMaterial
	var grant model.ClientNode
	var version model.NodeTemplateVersion
	if err := restored.First(&material).Error; err != nil {
		t.Fatal(err)
	}
	if err := restored.First(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if err := restored.First(&version).Error; err != nil {
		t.Fatal(err)
	}
	if material.EncryptedValue != "synthetic-encrypted-material" || grant.Active || grant.Generation != 9 || len(version.Payload) == 0 {
		t.Fatal("backup lost private material, revocation or version state")
	}
}
