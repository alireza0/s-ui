package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// The one-off data migrations record that they have run as rows in the
// settings table. Those rows used to travel out through GetAllSetting, and the
// settings form posts back every key it was handed -- straight into a Save that
// rejects anything it does not recognise. One migration flag was enough to make
// every settings save fail with "unknown setting: migratedSingBox114", which is
// the whole settings page on any panel that had ever migrated. #1258
func TestSettingsRoundTripWithBookkeepingRows(t *testing.T) {
	db := settingTestDB(t)
	s := &SettingService{}

	// InitDB runs the data migrations, so these rows are already here on a
	// fresh database -- which is why this reached every install, not only the
	// ones that had something to migrate. Seeded anyway, so the test still
	// covers the case if a migration stops recording itself.
	for _, key := range []string{
		"migratedSingBox114",
		"migratedCertProviders",
		"migratedHTTPClientFix",
		"migratedEndpointTls",
		"migratedRemovedOptions",
		"migratedHysteriaQuic",
	} {
		var existing model.Setting
		if err := db.Where("key = ?", key).First(&existing).Error; err == nil {
			continue
		}
		if err := db.Create(&model.Setting{Key: key, Value: "true"}).Error; err != nil {
			t.Fatalf("seeding %s: %v", key, err)
		}
	}

	flagCount := func() int64 {
		t.Helper()
		var n int64
		if err := db.Model(model.Setting{}).Where("key LIKE ?", "migrated%").Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := flagCount()
	if before == 0 {
		t.Fatal("no migration flags to test with")
	}

	all, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	for key := range *all {
		if strings.HasPrefix(key, "migrated") {
			t.Errorf("GetAllSetting handed out the bookkeeping row %q", key)
		}
	}

	// What the settings form does: post back everything it was given.
	payload, err := json.Marshal(*all)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(db, payload); err != nil {
		t.Fatalf("saving the settings that were just read back: %v", err)
	}

	// The flags themselves survive. Losing them replays every data migration
	// on the next start.
	if after := flagCount(); after != before {
		t.Errorf("migration flags in the database = %d, want %d", after, before)
	}
}

// A settings row that is neither a known setting nor a migration flag gets the
// same treatment, so a future bookkeeping key cannot bring the page down again.
func TestGetAllSettingWithholdsUnknownRows(t *testing.T) {
	db := settingTestDB(t)
	s := &SettingService{}

	if err := db.Create(&model.Setting{Key: "someLaterBookkeepingRow", Value: "1"}).Error; err != nil {
		t.Fatal(err)
	}

	all, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if _, handed := (*all)["someLaterBookkeepingRow"]; handed {
		t.Error("GetAllSetting handed out a row that is not an operator setting")
	}
}
