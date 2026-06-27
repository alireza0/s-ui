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
	err := os.MkdirAll(dir, 01740)
	if err != nil {
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
	dsn := dbPath + sep + "_busy_timeout=10000&_journal_mode=WAL&_cache_size=-200"
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

	err = db.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Service{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.Client{},
		&model.Changes{},
	)
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

	keepIds := "SELECT MIN(id) FROM stats GROUP BY resource, tag, date_time, direction"
	if err = db.Exec(`UPDATE stats SET traffic = (
		SELECT SUM(s2.traffic) FROM stats s2
		WHERE s2.resource = stats.resource AND s2.tag = stats.tag
		  AND s2.date_time = stats.date_time AND s2.direction = stats.direction)
		WHERE id IN (` + keepIds + `)`).Error; err != nil {
		return err
	}
	return db.Exec("DELETE FROM stats WHERE id NOT IN (" + keepIds + ")").Error
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
