package service

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestVlessVisionFlowWithTcpTransport(t *testing.T) {
	testVlessVisionFlowWithTcpTransport(t, map[string]interface{}{"enabled": true})
}

func TestVlessVisionFlowWithRealityTcpTransport(t *testing.T) {
	testVlessVisionFlowWithTcpTransport(t, map[string]interface{}{
		"enabled": true,
		"reality": map[string]interface{}{
			"enabled": true,
		},
	})
}

func TestVlessVisionFlowWithDisabledTLSTcpTransport(t *testing.T) {
	user := fetchVlessUser(t, map[string]interface{}{"enabled": false}, map[string]interface{}{"type": "tcp"})
	if got := user["flow"]; got != "" {
		t.Fatalf("flow was not stripped: %q", got)
	}
}

func testVlessVisionFlowWithTcpTransport(t *testing.T, tls map[string]interface{}) {
	t.Helper()
	user := fetchVlessUser(t, tls, map[string]interface{}{"type": "tcp"})
	if got := user["flow"]; got != "xtls-rprx-vision" {
		t.Fatalf("flow was changed: %q", got)
	}
}

func fetchVlessUser(t *testing.T, tls map[string]interface{}, transport map[string]interface{}) map[string]string {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Client{}); err != nil {
		t.Fatal(err)
	}

	clientConfig := json.RawMessage(`{"vless":{"uuid":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision"}}`)
	if err := db.Create(&model.Client{
		Enable: true,
		Config: clientConfig,
	}).Error; err != nil {
		t.Fatal(err)
	}

	users, err := (&InboundService{}).fetchUsers(db, "vless", "true", map[string]interface{}{
		"tls":       tls,
		"transport": transport,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users", len(users))
	}
	var user map[string]string
	if err := json.Unmarshal(users[0], &user); err != nil {
		t.Fatal(err)
	}
	return user
}

func TestVlessVisionFlowWithNonTcpTransport(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Client{}); err != nil {
		t.Fatal(err)
	}

	clientConfig := json.RawMessage(`{"vless":{"uuid":"11111111-1111-1111-1111-111111111111","flow":"xtls-rprx-vision"}}`)
	if err := db.Create(&model.Client{
		Enable: true,
		Config: clientConfig,
	}).Error; err != nil {
		t.Fatal(err)
	}

	users, err := (&InboundService{}).fetchUsers(db, "vless", "true", map[string]interface{}{
		"tls":       map[string]interface{}{"enabled": true},
		"transport": map[string]interface{}{"type": "grpc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users", len(users))
	}
	var user map[string]string
	if err := json.Unmarshal(users[0], &user); err != nil {
		t.Fatal(err)
	}
	if got := user["flow"]; got != "" {
		t.Fatalf("flow was not stripped: %q", got)
	}
}
