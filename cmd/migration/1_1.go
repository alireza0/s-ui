package migration

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

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
	return nil
}

func deleteOldWebSecret(db *gorm.DB) error {
	return db.Exec("DELETE FROM settings WHERE key = ?", "webSecret").Error
}

func changesObj(db *gorm.DB) error {
	return db.Exec("UPDATE changes SET obj = CAST('\"' || CAST(obj AS TEXT) || '\"' AS BLOB) WHERE actor = ? and obj not like ?", "DepleteJob", "\"%\"").Error
}

func to1_1(db *gorm.DB) error {
	err := migrateClientSchema(db)
	if err != nil {
		return err
	}
	err = deleteOldWebSecret(db)
	if err != nil {
		return err
	}
	err = changesObj(db)
	if err != nil {
		return err
	}
	return nil
}
