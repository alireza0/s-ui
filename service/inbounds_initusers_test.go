package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func initUsersTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Client{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clients := []model.Client{
		{Name: "alice", Enable: true, Config: json.RawMessage(`{"vless":{"uuid":"alice-uuid"}}`)},
		{Name: "bob", Enable: true, Config: json.RawMessage(`{"vless":{"uuid":"bob-uuid"}}`)},
	}
	for i := range clients {
		if err := db.Create(&clients[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return db
}

// The initUsers form value is spliced into a raw SQL condition today, so a
// crafted list can widen the selection to clients that were never picked.
func TestInitUsersSelectsOnlyRequestedClients(t *testing.T) {
	db := initUsersTestDB(t)
	s := &InboundService{}

	out, err := s.initUsers(db, json.RawMessage(`{"type":"vless"}`), "1", "vless")
	if err != nil {
		t.Fatalf("initUsers: %v", err)
	}
	if !strings.Contains(string(out), "alice-uuid") {
		t.Fatalf("expected alice in users, got %s", out)
	}
	if strings.Contains(string(out), "bob-uuid") {
		t.Fatalf("bob was not requested but was selected: %s", out)
	}
}

func TestInitUsersRejectsInjectedCondition(t *testing.T) {
	db := initUsersTestDB(t)
	s := &InboundService{}

	// With string splicing this becomes id IN (0) OR (1=1) and selects
	// every enabled client.
	out, err := s.initUsers(db, json.RawMessage(`{"type":"vless"}`), "0) OR (1=1", "vless")
	if err == nil {
		t.Fatalf("expected an error for a non-numeric client id, got users %s", out)
	}
	if strings.Contains(string(out), "uuid") {
		t.Fatalf("injected condition selected clients: %s", out)
	}
}

func TestInitUsersEmptyListIsNoOp(t *testing.T) {
	db := initUsersTestDB(t)
	s := &InboundService{}

	in := json.RawMessage(`{"type":"vless"}`)
	out, err := s.initUsers(db, in, "", "vless")
	if err != nil {
		t.Fatalf("initUsers: %v", err)
	}
	if string(out) != string(in) {
		t.Fatalf("expected unchanged inbound, got %s", out)
	}
}
