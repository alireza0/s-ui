package service

import (
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// ResetSettings used to delete every row, schema version included. A database
// with no version reads as pre-1.2, so the next `s-ui migrate` replayed the
// whole legacy chain against a current database and failed -- every run, with
// no way out but editing the database by hand.
func TestResetSettingsKeepsBookkeepingRows(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	s := &SettingService{}

	// The defaults, version among them, are written on first read.
	if _, err := s.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	// An operator-facing setting that reset is supposed to clear.
	if err := s.setString("webListen", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	if err := s.ResetSettings(); err != nil {
		t.Fatalf("ResetSettings: %v", err)
	}

	var count int64
	if err := db.Model(model.Setting{}).Where("key = ?", "version").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("reset deleted the version row")
	}

	count = 0
	if err := db.Model(model.Setting{}).Where("key = ?", "webListen").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Error("reset kept an operator-facing setting it should have cleared")
	}
}
