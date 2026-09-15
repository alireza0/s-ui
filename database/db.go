package database

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func initUser() error {
	var count int64
	err := db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		user := &model.User{
			Username: "admin",
			Password: "admin",
		}
		return db.Create(user).Error
	}
	return nil
}

func OpenDB(dbPath string) error {
	dir := path.Dir(dbPath)
	// 0700, not the old 01740: that 01000 bit is not Go's sticky bit
	// (os.ModeSticky is 1<<24) and was dropped, leaving 0740. MkdirAll is a
	// no-op on an existing directory, hence the explicit Chmod.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	// _cache_size=-200 caps each connection's page cache at ~200 KiB
	// (default is ~2 MiB), reducing memory amplification if a connection
	// escapes the pool.
	// _txlock=immediate makes transactions take the write lock up front.
	// Deferred transactions that read first and then write (e.g. the deplete
	// job) cannot wait on the lock upgrade and fail instantly with "database
	// is locked" whenever another writer (stats job) is active (#1209).
	dsn := dbPath + sep + "_busy_timeout=10000&_journal_mode=WAL&_cache_size=-200&_txlock=immediate"
	var err error
	db, err = gorm.Open(sqlite.Open(dsn), c)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	// SQLite creates the file 0666 & ~umask. The owner-only directory already
	// shields it, but a copied or moved database carries its own mode. The
	// sidecars hold the same pages.
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if config.IsDebug() {
		db = db.Debug()
	}
	return nil
}

func InitDB(dbPath string) error {
	err := OpenDB(dbPath)
	if err != nil {
		return err
	}

	// Default Outbounds
	if !db.Migrator().HasTable(&model.Outbound{}) {
		db.Migrator().CreateTable(&model.Outbound{})
		defaultOutbound := []model.Outbound{
			{Type: "direct", Tag: "direct", Options: json.RawMessage(`{}`)},
		}
		db.Create(&defaultOutbound)
	}

	if err = dedupStats(); err != nil {
		return err
	}

	err = db.AutoMigrate(schemaModels()...)
	if err != nil {
		return err
	}
	err = initUser()
	if err != nil {
		return err
	}

	return nil
}

// dedupStats merges traffic for duplicate groups of (resource, tag, date_time, direction)
func dedupStats() error {
	if !db.Migrator().HasTable(&model.Stats{}) {
		return nil
	}

	var dupGroups int64
	err := db.Raw("SELECT COUNT(*) FROM (SELECT 1 FROM stats GROUP BY resource, tag, date_time, direction HAVING COUNT(*) > 1)").Scan(&dupGroups).Error
	if err != nil {
		return err
	}
	if dupGroups == 0 {
		return nil
	}
	log.Printf("stats: collapsing %d duplicate group(s) before adding unique index", dupGroups)

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE stats_dedup AS
			SELECT MIN(id) AS id, resource, tag, date_time, direction, SUM(traffic) AS traffic
			FROM stats GROUP BY resource, tag, date_time, direction`).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM stats").Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO stats (id, resource, tag, date_time, direction, traffic)
			SELECT id, resource, tag, date_time, direction, traffic FROM stats_dedup`).Error; err != nil {
			return err
		}
		return tx.Exec("DROP TABLE stats_dedup").Error
	})
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
