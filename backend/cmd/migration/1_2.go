package migration

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"s-ui/database/model"

	"gorm.io/gorm"
)

type InboundData struct {
	Id      uint
	Tag     string
	Addrs   json.RawMessage
	OutJson json.RawMessage
}

func moveJsonToDb(db *gorm.DB) error {
	binFolderPath := os.Getenv("SUI_BIN_FOLDER")
	if binFolderPath == "" {
		binFolderPath = "bin"
	}
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	configPath := dir + "/" + binFolderPath + "/config.json"
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var oldConfig map[string]interface{}
	err = json.Unmarshal(data, &oldConfig)
	if err != nil {
		return err
	}

	oldInbounds := oldConfig["inbounds"].([]interface{})
	db.Migrator().DropTable(&model.Inbound{})
	db.AutoMigrate(&model.Inbound{})
	for _, inbound := range oldInbounds {
		inbObj, _ := inbound.(map[string]interface{})
		tag, _ := inbObj["tag"].(string)
		if tlsObj, ok := inbObj["tls"]; ok {
			var tls_id uint
			err = db.Raw("SELECT id FROM tls WHERE inbounds like ?", `%"`+tag+`"%`).Find(&tls_id).Error
			if err != nil {
				return err
			}

			// Bind or Create tls_id
			if tls_id > 0 {
				inbObj["tls_id"] = tls_id
			} else {
				tls_server, _ := json.MarshalIndent(tlsObj, "", "  ")
				if len(tls_server) > 5 {
					newTls := &model.Tls{
						Name:   tag,
						Server: tls_server,
					}
					err = db.Create(newTls).Error
					if err != nil {
						return err
					}
					inbObj["tls_id"] = newTls.Id
				}
			}
		}

		var inbData InboundData
		db.Raw("select id,addrs,out_json from inbound_data where tag = ?", tag).Find(&inbData)
		if inbData.Id > 0 {
			inbObj["outJson"] = inbData.OutJson
			inbObj["addrs"] = inbData.Addrs
		} else {
			inbObj["outJson"] = json.RawMessage("{}")
			inbObj["addrs"] = json.RawMessage("[]")
		}
		inbJson, _ := json.Marshal(inbObj)

		var newInbound model.Inbound
		err = newInbound.UnmarshalJSON(inbJson)
		if err != nil {
			return err
		}
		err = db.Create(&newInbound).Error
		if err != nil {
			return err
		}
	}
	delete(oldConfig, "inbounds")

	oldOutbounds := oldConfig["outbounds"].([]interface{})
	db.Migrator().DropTable(&model.Outbound{}, &model.Endpoint{})
	db.AutoMigrate(&model.Outbound{}, &model.Endpoint{})
	for _, outbound := range oldOutbounds {
		outType, _ := outbound.(map[string]interface{})["type"].(string)
		outboundRaw, _ := json.MarshalIndent(outbound, "", "  ")
		if outType == "wireguard" { // Check if it is Entrypoint
			var newEntrypoint model.Endpoint
			err = newEntrypoint.UnmarshalJSON(outboundRaw)
			if err != nil {
				return err
			}
			err = db.Create(&newEntrypoint).Error
			if err != nil {
				return err
			}
		} else { // It is Outbound
			var newOutbound model.Outbound
			err = newOutbound.UnmarshalJSON(outboundRaw)
			if err != nil {
				return err
			}
			err = db.Create(&newOutbound).Error
			if err != nil {
				return err
			}
		}
	}
	delete(oldConfig, "outbounds")

	// Remove v2rayapi and clashapi from experimental config
	experimental := oldConfig["experimental"].(map[string]interface{})
	delete(experimental, "v2ray_api")
	delete(experimental, "clash_api")
	oldConfig["experimental"] = experimental

	// Save the other configs
	var otherConfigs json.RawMessage
	otherConfigs, err = json.MarshalIndent(oldConfig, "", "  ")
	if err != nil {
		return err
	}

	return db.Save(&model.Setting{
		Key:   "config",
		Value: string(otherConfigs),
	}).Error
}

func migrateTls(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&model.Tls{}, "inbounds") {
		return nil
	}
	return db.Migrator().DropColumn(&model.Tls{}, "inbounds")
}

func dropInboundData(db *gorm.DB) error {
	if !db.Migrator().HasTable(&InboundData{}) {
		return nil
	}
	return db.Migrator().DropTable(&InboundData{})
}

func migrateClients(db *gorm.DB) error {
	var oldClients []model.Client
	err := db.Model(model.Client{}).Scan(&oldClients).Error
	if err != nil {
		return err
	}

	for index, oldClient := range oldClients {
		var old_inbounds []string
		err = json.Unmarshal(oldClient.Inbounds, &old_inbounds)
		if err != nil {
			return err
		}
		var inbound_ids []uint
		err = db.Raw("SELECT id FROM inbounds WHERE tag in ?", old_inbounds).Find(&inbound_ids).Error
		if err != nil {
			return err
		}
		oldClients[index].Inbounds, _ = json.Marshal(inbound_ids)
	}
	return db.Save(oldClients).Error
}

func to1_2(db *gorm.DB) error {
	err := moveJsonToDb(db)
	if err != nil {
		return err
	}
	err = migrateTls(db)
	if err != nil {
		return err
	}
	err = dropInboundData(db)
	if err != nil {
		return err
	}
	return migrateClients(db)
}
