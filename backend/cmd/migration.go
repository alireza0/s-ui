package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"s-ui/config"
	"s-ui/database/model"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func migrateDb() {
	// void running on first install
	path := config.GetDBPath()
	_, err := os.Stat(path)
	if err != nil {
		return
	}

	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		log.Fatal(err)
	}
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()
	fmt.Println("Start migrating database...")
	err = migrateClientSchema(tx)
	if err != nil {
		log.Fatal(err)
	}
	err = changesObj(tx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Migration done!")
}

func migrateClientSchema(db *gorm.DB) error {
	rows, err := db.Raw("PRAGMA table_info(clients)").Rows()
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			cname     string
			ctype     string
			notnull   int
			dfltValue interface{}
			pk        int
		)

		rows.Scan(&cid, &cname, &ctype, &notnull, &dfltValue, &pk)
		if cname == "config" || cname == "inbounds" || cname == "links" {
			if ctype == "text" {
				fmt.Printf("Column %s has type TEXT\n", cname)
				oldData := make([]struct {
					Id   uint
					Data string
				}, 0)
				db.Model(model.Client{}).Select("id", cname+" as data").Scan(&oldData)
				for _, data := range oldData {
					var newData []byte
					switch cname {
					case "inbounds":
						inbounds := strings.Split(data.Data, ",")
						newData, _ = json.MarshalIndent(inbounds, "", "  ")
					case "config":
						jsonData := map[string]interface{}{}
						json.Unmarshal([]byte(data.Data), &jsonData)
						newData, _ = json.MarshalIndent(jsonData, "", "  ")
					case "links":
						jsonData := make([]interface{}, 0)
						json.Unmarshal([]byte(data.Data), &jsonData)
						newData, _ = json.MarshalIndent(jsonData, "", "  ")
					}
					err = db.Model(model.Client{}).Where("id = ?", data.Id).UpdateColumn(cname, newData).Error
					if err != nil {
						return err
					}
				}
			}
		}
	}
	db.AutoMigrate(model.Client{})
	return nil
}

func changesObj(db *gorm.DB) error {
	return db.Exec("UPDATE changes SET obj = CAST('\"' || CAST(obj AS TEXT) || '\"' AS BLOB) WHERE actor = ? and obj not like ?", "DepleteJob", "\"%\"").Error
}
