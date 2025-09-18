package migration

import (
	"fmt"
	"log"
	"os"

	"github.com/alireza0/s-ui/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func MigrateDb() {
	// void running on first install
	path := config.GetDBPath()
	_, err := os.Stat(path)
	if err != nil {
		println("Database not found")
		return
	}

	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		log.Fatal(err)
		return
	}
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()
	currentVersion := config.GetVersion()
	dbVersion := ""
	tx.Raw("SELECT value FROM settings WHERE key = ?", "version").Find(&dbVersion)
	fmt.Println("Current version:", currentVersion, "\nDatabase version:", dbVersion)

	if currentVersion == dbVersion {
		fmt.Println("Database is up to date, no need to migrate")
		return
	}

	fmt.Println("Start migrating database...")

	// Before 1.2
	if dbVersion == "" {
		err = to1_1(tx)
		if err != nil {
			log.Fatal("Migration to 1.1 failed: ", err)
			return
		}
		err = to1_2(tx)
		if err != nil {
			log.Fatal("Migration to 1.2 failed: ", err)
			return
		}
		dbVersion = "1.2"
	}

	// Before 1.3
	if dbVersion[0:3] == "1.2" {
		err = to1_3(tx)
		if err != nil {
			log.Fatal("Migration to 1.3 failed: ", err)
			return
		}
	}

	// Set version
	err = tx.Exec("UPDATE settings SET value = ? WHERE key = ?", currentVersion, "version").Error
	if err != nil {
		log.Fatal("Update version failed: ", err)
		return
	}
	fmt.Println("Migration done!")
}
