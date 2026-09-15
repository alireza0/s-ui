package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

func clientTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	return database.GetDB()
}

func createClient(t *testing.T, db *gorm.DB, c *model.Client) *model.Client {
	t.Helper()
	if c.Config == nil {
		c.Config = json.RawMessage(`{}`)
	}
	if c.Inbounds == nil {
		c.Inbounds = json.RawMessage(`[]`)
	}
	if c.Links == nil {
		c.Links = json.RawMessage(`[]`)
	}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("creating client: %v", err)
	}
	return c
}

func reload(t *testing.T, db *gorm.DB, id uint) model.Client {
	t.Helper()
	var got model.Client
	if err := db.Model(model.Client{}).Where("id = ?", id).First(&got).Error; err != nil {
		t.Fatalf("reloading client: %v", err)
	}
	return got
}

// With reset_days zero, NextReset became dt + 0 == dt, so the row matched again
// every minute and a quota-limited account became effectively unlimited.
func TestPeriodicResetSkipsZeroResetDays(t *testing.T) {
	db := clientTestDB(t)
	s := &ClientService{}

	const now = int64(1_000_000)
	c := createClient(t, db, &model.Client{
		Name:      "zero-days",
		Enable:    true,
		AutoReset: true,
		ResetDays: 0,
		NextReset: now - 1, // already due
		Up:        500,
		Down:      700,
	})

	if _, err := s.ResetClients(db, now); err != nil {
		t.Fatalf("ResetClients: %v", err)
	}

	got := reload(t, db, c.Id)
	if got.Up != 500 || got.Down != 700 {
		t.Errorf("traffic was reset for a client with reset_days = 0: up=%d down=%d", got.Up, got.Down)
	}
	if got.TotalUp != 0 || got.TotalDown != 0 {
		t.Errorf("traffic was folded into the totals: total_up=%d total_down=%d", got.TotalUp, got.TotalDown)
	}
}

// The normal path: a due client is reset and its boundary moved forward.
func TestPeriodicResetRollsTrafficForward(t *testing.T) {
	db := clientTestDB(t)
	s := &ClientService{}

	const now = int64(1_000_000)
	c := createClient(t, db, &model.Client{
		Name:      "monthly",
		Enable:    true,
		AutoReset: true,
		ResetDays: 30,
		NextReset: now - 1,
		Up:        500,
		Down:      700,
		TotalUp:   100,
		TotalDown: 200,
	})

	if _, err := s.ResetClients(db, now); err != nil {
		t.Fatalf("ResetClients: %v", err)
	}

	got := reload(t, db, c.Id)
	if got.Up != 0 || got.Down != 0 {
		t.Errorf("period traffic was not cleared: up=%d down=%d", got.Up, got.Down)
	}
	if got.TotalUp != 600 || got.TotalDown != 900 {
		t.Errorf("totals = %d/%d, want 600/900", got.TotalUp, got.TotalDown)
	}
	if want := now + 30*86400; got.NextReset != want {
		t.Errorf("next_reset = %d, want %d", got.NextReset, want)
	}
}

// A client whose boundary is still in the future must be left alone.
func TestPeriodicResetLeavesFutureBoundariesAlone(t *testing.T) {
	db := clientTestDB(t)
	s := &ClientService{}

	const now = int64(1_000_000)
	c := createClient(t, db, &model.Client{
		Name:      "not-due",
		Enable:    true,
		AutoReset: true,
		ResetDays: 30,
		NextReset: now + 86400,
		Up:        500,
	})

	if _, err := s.ResetClients(db, now); err != nil {
		t.Fatalf("ResetClients: %v", err)
	}

	if got := reload(t, db, c.Id); got.Up != 500 {
		t.Errorf("a client that was not due had its traffic reset: up=%d", got.Up)
	}
}

// The traffic counters have to be restored as well as the timestamps, or an
// editor left open for a minute writes stale values back over that usage.
func TestEditDoesNotRollBackTraffic(t *testing.T) {
	db := clientTestDB(t)
	s := &ClientService{}

	c := createClient(t, db, &model.Client{
		Name:   "someone",
		Enable: true,
		Up:     100,
		Down:   200,
	})

	// The stats job lands while the editor is open.
	if err := db.Model(model.Client{}).Where("id = ?", c.Id).
		Updates(map[string]any{"up": 1500, "down": 2500}).Error; err != nil {
		t.Fatal(err)
	}

	// The form posts back what it was rendered with.
	stale := &model.Client{
		Id:     c.Id,
		Name:   "someone",
		Enable: true,
		Up:     100,
		Down:   200,
		Desc:   "edited",
	}
	s.preserveServerOwnedFields(db, stale)

	if stale.Up != 1500 || stale.Down != 2500 {
		t.Errorf("stale form values were not replaced: up=%d down=%d, want 1500/2500", stale.Up, stale.Down)
	}
}

// Built by string concatenation, a name with a quote or backslash produced a
// changes-log row no reader could parse.
func TestClientNameJSONIsValid(t *testing.T) {
	for _, name := range []string{
		`plain`,
		`he"llo`,
		`back\slash`,
		`both"and\`,
		"new\nline",
		`emoji 🎈`,
	} {
		raw := clientNameJSON(name)
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Errorf("name %q produced invalid JSON %s: %v", name, raw, err)
			continue
		}
		if decoded != name {
			t.Errorf("round trip of %q gave %q", name, decoded)
		}
	}
}

// The counter-preserving guard above also swallowed the panel's reset-usage
// button, which is the only legitimate way a form writes these columns: the
// zeros it posted were replaced by the stored values, so usage never reset and
// a client over its quota was disabled again on the next deplete run. #1261
func TestResetUsageThroughTheClientForm(t *testing.T) {
	db := clientTestDB(t)
	s := &ClientService{}

	c := createClient(t, db, &model.Client{
		Name:      "someone",
		Enable:    true,
		Up:        4_000,
		Down:      6_000,
		TotalUp:   10_000,
		TotalDown: 20_000,
	})

	// What the reset button posts: counters zeroed, everything else as shown.
	reset := &model.Client{
		Id:     c.Id,
		Name:   "someone",
		Enable: true,
		Up:     0,
		Down:   0,
	}
	s.preserveServerOwnedFields(db, reset)

	if reset.Up != 0 || reset.Down != 0 {
		t.Errorf("reset was discarded: up=%d down=%d, want 0/0", reset.Up, reset.Down)
	}
	// The lifetime counters take on what was just cleared.
	if reset.TotalUp != 14_000 || reset.TotalDown != 26_000 {
		t.Errorf("totals = %d/%d, want 14000/26000", reset.TotalUp, reset.TotalDown)
	}
}

// Traffic that arrives while the form is open is counted into the totals, not
// lost: the totals come from the stored row, never from the form.
func TestResetUsageCountsTrafficArrivingWhileOpen(t *testing.T) {
	db := clientTestDB(t)
	s := &ClientService{}

	c := createClient(t, db, &model.Client{
		Name:    "someone",
		Enable:  true,
		Up:      1_000,
		Down:    1_000,
		TotalUp: 500,
	})

	// The stats job lands after the operator opened the form.
	if err := db.Model(model.Client{}).Where("id = ?", c.Id).
		Updates(map[string]any{"up": 3_000, "down": 9_000}).Error; err != nil {
		t.Fatal(err)
	}

	// A form rendered before that, with the totals it computed for itself.
	reset := &model.Client{
		Id:        c.Id,
		Name:      "someone",
		Enable:    true,
		TotalUp:   1_500,
		TotalDown: 1_000,
	}
	s.preserveServerOwnedFields(db, reset)

	if reset.TotalUp != 3_500 || reset.TotalDown != 9_000 {
		t.Errorf("totals = %d/%d, want 3500/9000", reset.TotalUp, reset.TotalDown)
	}
}
