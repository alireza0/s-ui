package database

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/alireza0/s-ui/cmd/migration"
	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func GetDb(exclude string) ([]byte, error) {
	excluded := make(map[string]bool)
	for _, name := range strings.Split(exclude, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			excluded[name] = true
		}
	}

	// CreateTemp, not a hand-built timestamp: two downloads in the same period
	// used to render the same name and delete the file from under each other.
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(dir, config.GetName()+"-backup-*.db")
	if err != nil {
		return nil, err
	}
	dbPath := tmp.Name()
	// SQLite opens the path itself; this handle only reserves the name.
	tmp.Close()
	defer os.Remove(dbPath)

	backupDb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	backupClosed := false
	closeBackup := func() {
		if backupClosed {
			return
		}
		backupClosed = true
		if sqlDB, e := backupDb.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}
	defer closeBackup()

	// Same list InitDB migrates, so no table can be live but unbacked-up.
	if err = backupDb.AutoMigrate(schemaModels()...); err != nil {
		return nil, err
	}

	for _, t := range schema() {
		if excluded[t.name] {
			continue
		}
		if err := t.copyRows(db, backupDb); err != nil {
			return nil, common.NewErrorf("backing up %s: %v", t.name, err)
		}
	}

	// Fold the WAL in, or the bytes read below miss whatever is still in it.
	if err = backupDb.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		return nil, err
	}
	closeBackup()

	return os.ReadFile(dbPath)
}

func ImportDB(file multipart.File) error {
	// Check if the file is a SQLite database
	isValidDb, err := IsSQLiteDB(file)
	if err != nil {
		return common.NewErrorf("Error checking db file format: %v", err)
	}
	if !isValidDb {
		return common.NewError("Invalid db file format")
	}

	// Reset the file reader to the beginning
	if _, err = file.Seek(0, 0); err != nil {
		return common.NewErrorf("Error resetting file reader: %v", err)
	}

	dbPath := config.GetDBPath()
	tempPath := fmt.Sprintf("%s.temp", dbPath)
	if err = os.RemoveAll(tempPath); err != nil {
		return common.NewErrorf("Error removing existing temporary db file: %v", err)
	}

	// Everything up to the rename works on the upload alone, and the live pool
	// stays open: closing it here means a failed import breaks every later
	// query with "sql: database is closed" until someone restarts the service.
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return common.NewErrorf("Error creating temporary db file: %v", err)
	}
	_, err = io.Copy(tempFile, file)
	tempFile.Close()
	if err != nil {
		os.Remove(tempPath)
		return common.NewErrorf("Error saving db: %v", err)
	}
	defer os.Remove(tempPath)

	// The header check above passes for any SQLite file at all, including a
	// browser profile. Importing one of those replaces the live database and
	// then crashes the migration, into a systemd restart loop.
	if err = validateImport(tempPath); err != nil {
		return err
	}

	// Renaming out from under a -wal/-shm pair leaves those sidecars beside the
	// imported file, and SQLite recovers the wrong database's pages into it.
	if sqlDB, e := db.DB(); e == nil {
		_ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
		_ = sqlDB.Close()
	}
	removeSidecars(dbPath)

	// Keep the pre-import database, timestamped, so a bad import is recoverable.
	fallbackPath := fmt.Sprintf("%s.backup-%s", dbPath, time.Now().Format("20060102-150405"))
	if err = os.Rename(dbPath, fallbackPath); err != nil {
		reopen(dbPath)
		return common.NewErrorf("Error backing up current db file: %v", err)
	}

	if err = os.Rename(tempPath, dbPath); err != nil {
		if errRename := os.Rename(fallbackPath, dbPath); errRename != nil {
			return common.NewErrorf("Error moving db file and restoring fallback: %v", errRename)
		}
		reopen(dbPath)
		return common.NewErrorf("Error moving db file: %v", err)
	}

	if err = migration.MigrateDb(); err != nil {
		restoreFallback(dbPath, fallbackPath)
		return common.NewErrorf("Error migrating db: %v", err)
	}
	if err = InitDB(dbPath); err != nil {
		restoreFallback(dbPath, fallbackPath)
		return common.NewErrorf("Error initialising imported db: %v", err)
	}

	logger.Info("database imported; previous database kept at ", fallbackPath)

	// Restart app
	if err = SendSighup(); err != nil {
		return common.NewErrorf("Error restarting app: %v", err)
	}

	return nil
}

// validateImport rejects a SQLite file that is not an s-ui database, before it
// can replace the live one.
func validateImport(path string) error {
	candidate, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		return common.NewErrorf("Error checking db: %v", err)
	}
	defer func() {
		if sqlDB, e := candidate.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()

	// settings carries the schema version and session secret; without clients
	// and inbounds it is not a panel backup whatever else it holds.
	for _, required := range []string{"settings", "clients", "inbounds"} {
		if !candidate.Migrator().HasTable(required) {
			return common.NewErrorf("Not an s-ui database: table %q is missing", required)
		}
	}

	// The file parses and carries the expected tables, but its pages may still
	// be corrupt: quick_check is the cheap integrity scan (a full
	// integrity_check re-validates every index entry, which an upload is not
	// worth while the panel waits on the answer).
	var integrityResult string
	if err := candidate.Raw("PRAGMA quick_check").Scan(&integrityResult).Error; err != nil {
		return common.NewErrorf("Error checking db integrity: %v", err)
	}
	if integrityResult != "ok" {
		return common.NewErrorf("Database integrity check failed: %s", integrityResult)
	}
	return nil
}

// removeSidecars drops the -wal and -shm files belonging to path; they describe
// the database being replaced, not the one taking its place.
func removeSidecars(path string) {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			logger.Warning("unable to remove ", path+suffix, ": ", err)
		}
	}
}

// reopen restores the connection pool after an import gave up.
func reopen(path string) {
	if err := InitDB(path); err != nil {
		logger.Error("unable to reopen the database after a failed import: ", err)
	}
}

func restoreFallback(dbPath, fallbackPath string) {
	removeSidecars(dbPath)
	if err := os.Rename(fallbackPath, dbPath); err != nil {
		logger.Error("unable to restore the database after a failed import: ", err)
		return
	}
	reopen(dbPath)
}

func IsSQLiteDB(file io.Reader) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	// ReadFull: a single Read may return short and compare a partial buffer.
	if _, err := io.ReadFull(file, buf); err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func SendSighup() error {
	// Get the current process
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	// Send SIGHUP to the current process
	go func() {
		time.Sleep(3 * time.Second)
		if runtime.GOOS == "windows" {
			err = process.Kill()
		} else {
			err = process.Signal(syscall.SIGHUP)
		}
		if err != nil {
			logger.Error("send signal SIGHUP failed:", err)
		}
	}()
	return nil
}
