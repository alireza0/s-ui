package migration

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alireza0/s-ui/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MigrateDb brings an older database up to the current version. It returns an
// error rather than calling log.Fatal: ImportDB calls this mid-restore, with
// the live database already renamed away.
func MigrateDb() error {
	// void running on first install
	path := config.GetDBPath()
	if _, err := os.Stat(path); err != nil {
		fmt.Println("Database not found")
		return nil
	}

	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		return err
	}
	defer func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()

	tx := db.Begin()
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	currentVersion := config.GetVersion()
	dbVersion := ""
	// An unreadable settings table must not look like an unset version, which
	// would send the whole legacy chain through again.
	if err := tx.Raw("SELECT value FROM settings WHERE key = ?", "version").Find(&dbVersion).Error; err != nil {
		return fmt.Errorf("reading database version: %w", err)
	}
	fmt.Println("Current version:", currentVersion, "\nDatabase version:", dbVersion)

	if currentVersion == dbVersion {
		fmt.Println("Database is up to date, no need to migrate")
		return nil
	}

	fmt.Println("Start migrating database...")

	// Before 1.2
	if dbVersion == "" {
		if err := to1_1(tx); err != nil {
			return fmt.Errorf("migration to 1.1 failed: %w", err)
		}
		if err := to1_2(tx); err != nil {
			return fmt.Errorf("migration to 1.2 failed: %w", err)
		}
		dbVersion = "1.2"
	}

	// Before 1.3. Was dbVersion[0:3], which panics on a short value.
	if major, minor := majorMinor(dbVersion); major == 1 && minor == 2 {
		if err := to1_3(tx); err != nil {
			return fmt.Errorf("migration to 1.3 failed: %w", err)
		}
	}

	// Before 1.5.1: back-fill self-signed TLS public-key pins and rewrite OutJson
	if compareVersions(dbVersion, "1.5.1") < 0 {
		if err := to1_5_1(tx); err != nil {
			return fmt.Errorf("migration to 1.5.1 failed: %w", err)
		}
	}

	if compareVersions(dbVersion, "1.5.2") < 0 {
		if err := to1_5_2(tx); err != nil {
			return fmt.Errorf("migration to 1.5.2 failed: %w", err)
		}
	}

	if compareVersions(dbVersion, "1.6.0") < 0 {
		if err := to1_6_0(tx); err != nil {
			return fmt.Errorf("migration to 1.6.0 failed: %w", err)
		}
	}

	if compareVersions(dbVersion, "1.6.1") < 0 {
		if err := to1_6_1(tx); err != nil {
			return fmt.Errorf("migration to 1.6.1 failed: %w", err)
		}
	}

	if compareVersions(dbVersion, "1.6.3") < 0 {
		if err := to1_6_3(tx); err != nil {
			return fmt.Errorf("migration to 1.6.3 failed: %w", err)
		}
	}

	if err := setVersion(tx, currentVersion); err != nil {
		return fmt.Errorf("update version failed: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing migration: %w", err)
	}
	committed = true
	fmt.Println("Migration done!")
	return nil
}

// setVersion records the schema version, inserting the row when it is absent.
// A plain UPDATE affects zero rows and reports no error, so after a settings
// reset every migrate replayed the whole legacy chain and recorded nothing.
func setVersion(tx *gorm.DB, version string) error {
	res := tx.Exec("UPDATE settings SET value = ? WHERE key = ?", version, "version")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	return tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "version", version).Error
}

// compareVersions orders two dotted versions numerically, returning -1, 0 or 1.
// Not as strings: "1.10.0" < "1.5.1" lexicographically, so the release after
// 1.9 would replay to1_5_1 against every database. A missing component counts
// as zero, so "1.2" and "1.2.0" compare equal.
func compareVersions(a, b string) int {
	av, bv := parseVersion(a), parseVersion(b)
	for i := range av {
		switch {
		case av[i] < bv[i]:
			return -1
		case av[i] > bv[i]:
			return 1
		}
	}
	return 0
}

func majorMinor(v string) (int, int) {
	parsed := parseVersion(v)
	return parsed[0], parsed[1]
}

// parseVersion reads up to three numeric components. Anything unparseable
// stops the scan and leaves the rest zero, so garbage sorts as the oldest
// version rather than panicking.
func parseVersion(v string) [3]int {
	var parsed [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return parsed
	}
	for i, part := range strings.SplitN(v, ".", 3) {
		n, err := strconv.Atoi(part)
		if err != nil {
			return parsed
		}
		parsed[i] = n
	}
	return parsed
}
