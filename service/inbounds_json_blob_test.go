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

// json.RawMessage columns are stored as BLOB, and SQLite reads a BLOB whose
// first byte and length match a JSONB header as JSONB. Text such as "[1, 2]"
// or "[1,23]" then fails with "malformed JSON" (#1326).
func jsonBlobTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Client{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clients := []model.Client{
		{Name: "alice", Enable: true, Inbounds: json.RawMessage(`[1, 2]`),
			Config: json.RawMessage(`{"vless":{"uuid":"alice-uuid"},"hysteria2":{"password":"alice-pw"}}`)},
		{Name: "bob", Enable: true, Inbounds: json.RawMessage(`[1,23]`),
			Config: json.RawMessage(`{"vless":{"uuid":"bob-uuid"},"hysteria2":{"password":"bob-pw"}}`)},
	}
	for i := range clients {
		if err := db.Create(&clients[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return db
}

func TestAddUsersReadsJSONBLookalikeInbounds(t *testing.T) {
	db := jsonBlobTestDB(t)
	s := &InboundService{}

	cases := []struct {
		id    uint
		typ   string
		want  []string
		avoid []string
	}{
		{1, "vless", []string{"alice-uuid", "bob-uuid"}, nil},
		{2, "hysteria2", []string{"alice-pw"}, []string{"bob-pw"}},
		{23, "hysteria2", []string{"bob-pw"}, []string{"alice-pw"}},
	}
	for _, c := range cases {
		out, err := s.addUsers(db, json.RawMessage(`{"type":"`+c.typ+`"}`), c.id, c.typ)
		if err != nil {
			t.Fatalf("addUsers inbound %d: %v", c.id, err)
		}
		for _, w := range c.want {
			if !strings.Contains(string(out), w) {
				t.Fatalf("inbound %d: expected %s in users, got %s", c.id, w, out)
			}
		}
		for _, a := range c.avoid {
			if strings.Contains(string(out), a) {
				t.Fatalf("inbound %d: %s is not assigned but was selected: %s", c.id, a, out)
			}
		}
	}
}

func TestEnabledClientNamesReadsJSONBLookalikeInbounds(t *testing.T) {
	db := jsonBlobTestDB(t)
	s := &InboundService{}

	names, err := s.enabledClientNames(db, 1)
	if err != nil {
		t.Fatalf("enabledClientNames: %v", err)
	}
	if _, ok := names["alice"]; !ok {
		t.Fatalf("expected alice, got %v", names)
	}
	if _, ok := names["bob"]; !ok {
		t.Fatalf("expected bob, got %v", names)
	}
}
