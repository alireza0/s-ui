package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func TestSubRemarkPrefixRebuildsLinks(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	inbound := model.Inbound{Type: "vless", Tag: "vl", Options: json.RawMessage(`{"listen_port": 443}`), Addrs: json.RawMessage(`[]`), OutJson: json.RawMessage(`{}`)}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Enable:   true,
		Name:     "c1",
		Remark:   "alice",
		Config:   json.RawMessage(`{"vless": {"name": "c1", "uuid": "8b5e6d3c-3f3e-4c1a-9b1e-2b8a4f6f0a11"}}`),
		Inbounds: json.RawMessage(fmt.Sprintf("[%d]", inbound.Id)),
		Links:    json.RawMessage(`[]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	links := func(prefix string) string {
		t.Helper()
		tx := db.Begin()
		defer tx.Commit()
		if err := (&SettingService{}).Save(tx, json.RawMessage(`{"subRemarkPrefix": "`+prefix+`"}`)); err != nil {
			t.Fatal(err)
		}
		inbounds := []model.Inbound{inbound}
		if err := (&ClientService{}).UpdateLinksByInboundChange(tx, &inbounds, "example.com", ""); err != nil {
			t.Fatal(err)
		}
		var stored model.Client
		if err := tx.First(&stored, client.Id).Error; err != nil {
			t.Fatal(err)
		}
		return string(stored.Links)
	}

	if got := links("false"); strings.Contains(got, "alice") || !strings.Contains(got, "vless://") {
		t.Errorf("prefix off: links still carry the remark or are missing: %s", got)
	}
	if got := links("true"); !strings.Contains(got, "alice-vl") {
		t.Errorf("prefix on: links lack the remark: %s", got)
	}
}
