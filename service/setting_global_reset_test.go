package service

import (
	"encoding/json"
	"testing"
)

// A bad spec used to save fine and only log a warning at the next panel
// start, so the reset silently never ran.
func TestSaveRejectsInvalidGlobalReset(t *testing.T) {
	db := settingTestDB(t)
	s := &SettingService{}

	payload, _ := json.Marshal(map[string]string{"globalReset": "every monday"})
	if err := s.Save(db, payload); err == nil {
		t.Fatal("Save accepted an invalid cron spec")
	}

	for _, spec := range []string{"", "off", "0 0 1 * *", "30 0 0 * * *", "@weekly"} {
		payload, _ := json.Marshal(map[string]string{"globalReset": spec})
		if err := s.Save(db, payload); err != nil {
			t.Errorf("Save rejected %q: %v", spec, err)
		}
	}
}

// globalResetLast holds the next boundary of the schedule in force. Left
// alone on a change, a monthly-to-daily switch waited out the old month.
func TestSaveClearsArmedResetOnScheduleChange(t *testing.T) {
	db := settingTestDB(t)
	s := &SettingService{}

	save := func(spec string) {
		t.Helper()
		payload, _ := json.Marshal(map[string]string{"globalReset": spec})
		if err := s.Save(db, payload); err != nil {
			t.Fatal(err)
		}
	}

	save("0 0 1 * *")
	if err := s.SetGlobalResetLast(4102444800); err != nil {
		t.Fatal(err)
	}

	// Same spec saved again, as the form does with every other change.
	save("0 0 1 * *")
	if last, _ := s.GetGlobalResetLast(); last != 4102444800 {
		t.Errorf("unchanged spec cleared the armed boundary: %d", last)
	}

	save("0 0 * * *")
	if last, _ := s.GetGlobalResetLast(); last != 0 {
		t.Errorf("changed spec kept the old boundary: %d", last)
	}
}
