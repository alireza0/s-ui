package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// A row in every table the schema describes, so a dropped table shows up as a
// missing row rather than as an empty database nobody notices until a restore.
func seedEveryTable(t *testing.T) {
	t.Helper()

	rows := []any{
		// The settings table used to be filled in by the per-migration flags
		// the old InitDB wrote; migrations record the version instead, and
		// that is what a live database carries here.
		&model.Setting{Key: "version", Value: "1.6.2"},
		&model.Tls{Name: "cert", Server: json.RawMessage(`{"enabled":true}`), Client: json.RawMessage(`{}`)},
		&model.Inbound{Type: "vless", Tag: "in-1", Options: json.RawMessage(`{}`), OutJson: json.RawMessage(`{}`)},
		&model.Outbound{Type: "direct", Tag: "out-1", Options: json.RawMessage(`{}`)},
		&model.Service{Type: "derp", Tag: "svc-1", Options: json.RawMessage(`{}`)},
		&model.Endpoint{Type: "wireguard", Tag: "ep-1", Options: json.RawMessage(`{}`)},
		&model.Tokens{UserId: 1, Token: "a-token", Desc: "for the test", Expiry: 0},
		&model.Stats{DateTime: 1, Resource: "user", Tag: "someone", Direction: true, Traffic: 42},
		&model.Client{Name: "someone", Enable: true, Config: json.RawMessage(`{}`), Inbounds: json.RawMessage(`[]`), Links: json.RawMessage(`[]`)},
		&model.Changes{DateTime: 1, Actor: "admin", Key: "clients", Action: "new", Obj: json.RawMessage(`"someone"`)},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seeding %T: %v", row, err)
		}
	}
}

func openBackup(t *testing.T, contents []byte) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("opening the backup: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := restored.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return restored
}

// The schema drift: InitDB migrated eleven models while GetDb backed up nine,
// so services and API tokens were absent from every backup.
func TestBackupCarriesEveryTable(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	seedEveryTable(t)

	contents, err := GetDb("")
	if err != nil {
		t.Fatalf("GetDb: %v", err)
	}
	restored := openBackup(t, contents)

	for _, table := range schema() {
		var live, backed int64
		if err := db.Table(table.name).Count(&live).Error; err != nil {
			t.Fatalf("counting live %s: %v", table.name, err)
		}
		if err := restored.Table(table.name).Count(&backed).Error; err != nil {
			t.Errorf("table %q is missing from the backup: %v", table.name, err)
			continue
		}
		if live == 0 {
			t.Errorf("table %q had no rows to back up; the fixture needs one", table.name)
		}
		if backed != live {
			t.Errorf("table %q: backup has %d rows, live database has %d", table.name, backed, live)
		}
	}
}

// exclude must skip exactly the named tables and nothing else.
func TestBackupExcludesOnlyWhatWasAsked(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	seedEveryTable(t)

	contents, err := GetDb("stats,changes")
	if err != nil {
		t.Fatalf("GetDb: %v", err)
	}
	restored := openBackup(t, contents)

	for _, name := range []string{"stats", "changes"} {
		var count int64
		if err := restored.Table(name).Count(&count).Error; err != nil {
			t.Errorf("excluded table %q should still exist but be empty: %v", name, err)
			continue
		}
		if count != 0 {
			t.Errorf("excluded table %q still holds %d rows", name, count)
		}
	}

	// Everything not named must survive.
	for _, name := range []string{"clients", "inbounds", "services", "tokens"} {
		var count int64
		if err := restored.Table(name).Count(&count).Error; err != nil {
			t.Errorf("counting %s: %v", name, err)
			continue
		}
		if count == 0 {
			t.Errorf("table %q was dropped even though it was not excluded", name)
		}
	}
}

// The temp path used the layout "20060102-200203", a typo for "150405", so
// every call in a period rendered the same name and two downloads collided.
func TestBackupsDoNotShareATempFile(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	seedEveryTable(t)

	first, err := GetDb("")
	if err != nil {
		t.Fatalf("first GetDb: %v", err)
	}
	second, err := GetDb("")
	if err != nil {
		t.Fatalf("second GetDb: %v", err)
	}
	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("a backup came back empty: %d and %d bytes", len(first), len(second))
	}

	// A shared temp file leaves one of them truncated or gone.
	for i, contents := range [][]byte{first, second} {
		restored := openBackup(t, contents)
		var count int64
		if err := restored.Table("clients").Count(&count).Error; err != nil {
			t.Errorf("backup %d is not a usable database: %v", i+1, err)
			continue
		}
		if count == 0 {
			t.Errorf("backup %d lost its clients", i+1)
		}
	}
}

// GetDb leaves no scratch files next to the binary.
func TestBackupCleansUpAfterItself(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		t.Fatal(err)
	}
	before := backupTempFiles(t, dir)

	if _, err := GetDb(""); err != nil {
		t.Fatalf("GetDb: %v", err)
	}

	if after := backupTempFiles(t, dir); len(after) != len(before) {
		t.Errorf("GetDb left scratch files behind: %v", after)
	}
}

func backupTempFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, e := range entries {
		if strings.Contains(e.Name(), "-backup-") {
			found = append(found, e.Name())
		}
	}
	return found
}
