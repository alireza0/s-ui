package database

import (
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// Inbounds stored before the enable column existed must come out enabled, or
// an upgrade would take every inbound offline. #1327
func TestEnableColumnDefaultsExistingInboundsOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE inbounds DROP COLUMN enable").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO inbounds (type, tag) VALUES ('mixed', 'old')").Error; err != nil {
		t.Fatal(err)
	}
	if err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	var inbound model.Inbound
	if err := db.Where("tag = ?", "old").First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	if !inbound.Enable {
		t.Fatal("existing inbound came out disabled after the enable column was added")
	}
}
