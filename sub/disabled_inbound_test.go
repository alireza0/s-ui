package sub

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// A disabled inbound is not listening, so its links must not be served. #1327
func TestSubsSkipDisabledInbounds(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	inbounds := []model.Inbound{{Type: "mixed", Tag: "on", Enable: true}, {Type: "mixed", Tag: "off", Enable: true}}
	if err := db.Create(&inbounds).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(model.Inbound{}).Where("tag = ?", "off").Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Name:     "alice",
		Enable:   true,
		Inbounds: json.RawMessage(`[1,2]`),
		Links: json.RawMessage(`[
			{"type":"local","remark":"on","uri":"socks://on"},
			{"type":"local","remark":"off","uri":"socks://off"},
			{"type":"external","remark":"off","uri":"socks://ext"}
		]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	result, _, err := (&SubService{}).GetSubs("alice")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(*result)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "socks://on\nsocks://ext" {
		t.Fatalf("links = %q, want the enabled local link and the external one", decoded)
	}

	_, ins, err := (&JsonService{}).getData("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(ins) != 1 || ins[0].Tag != "on" {
		t.Fatalf("json/clash subs got inbounds %v, want only the enabled one", ins)
	}
}
