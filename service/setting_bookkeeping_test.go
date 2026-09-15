package service

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// The settings table holds rows that are not operator settings: the schema
// version, and whatever bookkeeping a later release adds next to it. Those rows
// used to travel out through GetAllSetting, and the settings form posts back
// every key it was handed -- straight into a Save that rejects anything it does
// not recognise. One such row was enough to make every settings save fail with
// "unknown setting: ...", which is the whole settings page. #1258
func TestSettingsRoundTripWithBookkeepingRows(t *testing.T) {
	db := settingTestDB(t)
	s := &SettingService{}

	// version is written by GetAllSetting itself; the other stands for a
	// bookkeeping row a later release adds.
	if err := db.Create(&model.Setting{Key: "someLaterBookkeepingRow", Value: "1"}).Error; err != nil {
		t.Fatal(err)
	}

	bookkeeping := []string{"version", "someLaterBookkeepingRow"}

	all, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range bookkeeping {
		if _, handed := (*all)[key]; handed {
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

	// The rows themselves survive. Losing the version row makes the database
	// read as pre-1.2, and every `s-ui migrate` replays the legacy chain.
	for _, key := range bookkeeping {
		var count int64
		if err := db.Model(model.Setting{}).Where("key = ?", key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("the bookkeeping row %q did not survive the round trip", key)
		}
	}
}

// A row that is neither a known setting nor one the panel wrote gets the same
// treatment, so a future bookkeeping key cannot bring the page down again.
func TestGetAllSettingWithholdsUnknownRows(t *testing.T) {
	db := settingTestDB(t)
	s := &SettingService{}

	if err := db.Create(&model.Setting{Key: "someUnknownRow", Value: "1"}).Error; err != nil {
		t.Fatal(err)
	}

	all, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if _, handed := (*all)["someUnknownRow"]; handed {
		t.Error("GetAllSetting handed out a row that is not an operator setting")
	}
}
