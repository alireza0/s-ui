package service

import (
	"encoding/json"
	"os"
	"s-ui/config"
	"s-ui/core"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"strconv"
	"time"
)

var (
	LastUpdate int64
	IsSystemd  bool
	corePtr    *core.Core
)

type ConfigService struct {
	ClientService
	TlsService
	SettingService
	InboundService
	OutboundService
	EndpointService
}

type SingBoxConfig struct {
	Log          json.RawMessage   `json:"log"`
	Dns          json.RawMessage   `json:"dns"`
	Ntp          json.RawMessage   `json:"ntp"`
	Inbounds     []json.RawMessage `json:"inbounds"`
	Outbounds    []json.RawMessage `json:"outbounds"`
	Endpoints    []json.RawMessage `json:"endpoints"`
	Route        json.RawMessage   `json:"route"`
	Experimental json.RawMessage   `json:"experimental"`
}

func NewConfigService(core *core.Core) *ConfigService {
	corePtr = core
	return &ConfigService{}
}

func (s *ConfigService) InitConfig() error {
	IsSystemd = config.IsSystemd()
	return nil
}

func (s *ConfigService) GetConfig() (*SingBoxConfig, error) {
	data, err := s.SettingService.GetConfig()
	if err != nil {
		return nil, err
	}
	singboxConfig := SingBoxConfig{}
	err = json.Unmarshal([]byte(data), &singboxConfig)
	if err != nil {
		return nil, err
	}

	singboxConfig.Inbounds, err = s.InboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Outbounds, err = s.OutboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Endpoints, err = s.EndpointService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	return &singboxConfig, nil
}

func (s *ConfigService) StartCore() error {
	singboxConfig, err := s.GetConfig()
	if err != nil {
		return err
	}
	rawConfig, err := json.MarshalIndent(singboxConfig, "", "  ")
	if err != nil {
		return err
	}
	err = corePtr.Start(rawConfig)
	if err != nil {
		logger.Error("start sing-box err:", err.Error())
		return err
	}
	logger.Info("sing-box started")
	return nil
}

func (s *ConfigService) RestartCore() error {
	err := s.StartCore()
	if err != nil {
		return err
	}
	return s.StartCore()
}

func (s *ConfigService) StopCore() error {
	err := corePtr.Stop()
	if err != nil {
		return err
	}
	logger.Info("sing-box stopped")
	return nil
}

func (s *ConfigService) SaveChanges(changes map[string]string, loginUser string) error {
	var err error
	var clientChanges, tlsChanges, inChanges, settingChanges, configChanges []model.Changes
	if _, ok := changes["clients"]; ok {
		err = json.Unmarshal([]byte(changes["clients"]), &clientChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["tls"]; ok {
		err = json.Unmarshal([]byte(changes["tls"]), &tlsChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["inData"]; ok {
		err = json.Unmarshal([]byte(changes["inData"]), &inChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["settings"]; ok {
		err = json.Unmarshal([]byte(changes["settings"]), &settingChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["config"]; ok {
		err = json.Unmarshal([]byte(changes["config"]), &configChanges)
		if err != nil {
			return err
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	if len(clientChanges) > 0 {
		err = s.ClientService.Save(tx, clientChanges)
		if err != nil {
			return err
		}
	}
	if len(tlsChanges) > 0 {
		err = s.TlsService.Save(tx, tlsChanges)
		if err != nil {
			return err
		}
	}
	// if len(inChanges) > 0 {
	// 	err = s.InDataService.Save(tx, inChanges)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	if len(settingChanges) > 0 {
		err = s.SettingService.Save(tx, settingChanges)
		if err != nil {
			return err
		}
	}
	needRestart := false
	if len(configChanges) > 0 {
		singboxConfig, err := s.GetConfig()
		if err != nil {
			return err
		}
		newConfig := *singboxConfig
		for _, change := range configChanges {
			rawObject := change.Obj
			switch change.Key {
			case "all":
				err = json.Unmarshal(rawObject, &newConfig)
				if err != nil {
					return err
				}
				needRestart = true
			case "log":
				newConfig.Log = rawObject
				needRestart = true
			case "dns":
				newConfig.Dns = rawObject
				needRestart = true
			case "ntp":
				newConfig.Ntp = rawObject
				needRestart = true
			case "route":
				newConfig.Route = rawObject
				needRestart = true
			case "experimental":
				newConfig.Experimental = rawObject
				needRestart = true
			case "inbounds":
				if change.Action == "edit" {
					var object map[string]interface{}
					err = json.Unmarshal(newConfig.Inbounds[change.Index], &object)
					if err != nil {
						return err
					}
					if tag, ok := object["tag"].(string); ok {
						err = corePtr.RemoveInbound(tag)
						if err == nil {
							err = corePtr.AddInbound(rawObject)
							if err != nil {
								needRestart = true
							}
						} else {
							needRestart = true
						}
					} else {
						needRestart = true
					}
					newConfig.Inbounds[change.Index] = rawObject
				} else if change.Action == "del" {
					var object map[string]interface{}
					err = json.Unmarshal(newConfig.Inbounds[change.Index], &object)
					if err != nil {
						return err
					}
					if tag, ok := object["tag"].(string); ok {
						err = corePtr.RemoveInbound(tag)
						if err != nil {
							needRestart = true
						}
					} else {
						needRestart = true
					}
					newConfig.Inbounds = append(newConfig.Inbounds[:change.Index], newConfig.Inbounds[change.Index+1:]...)
				} else {
					newConfig.Inbounds = append(newConfig.Inbounds, rawObject)
					err = corePtr.AddInbound(rawObject)
					if err != nil {
						logger.Debug(err)
						needRestart = true
					}
				}
			case "outbounds":
				if change.Action == "edit" {
					var object map[string]interface{}
					err = json.Unmarshal(newConfig.Outbounds[change.Index], &object)
					if err != nil {
						return err
					}
					if tag, ok := object["tag"].(string); ok {
						err = corePtr.RemoveOutbound(tag)
						if err == nil {
							err = corePtr.AddOutbound(rawObject)
							if err != nil {
								needRestart = true
							}
						} else {
							needRestart = true
						}
					} else {
						needRestart = true
					}
					newConfig.Outbounds[change.Index] = rawObject
				} else if change.Action == "del" {
					var object map[string]interface{}
					err = json.Unmarshal(newConfig.Outbounds[change.Index], &object)
					if err != nil {
						return err
					}
					if tag, ok := object["tag"].(string); ok {
						err = corePtr.RemoveOutbound(tag)
						if err != nil {
							needRestart = true
						}
					} else {
						needRestart = true
					}
					newConfig.Outbounds = append(newConfig.Outbounds[:change.Index], newConfig.Outbounds[change.Index+1:]...)
				} else {
					err = corePtr.AddOutbound(rawObject)
					if err != nil {
						logger.Debug(err)
						needRestart = true
					}
					newConfig.Outbounds = append(newConfig.Outbounds, rawObject)
				}
			}
		}

		err = s.Save(&newConfig, needRestart)
		if err != nil {
			return err
		}
	}

	// Log changes
	dt := time.Now().Unix()
	allChanges := append(clientChanges, settingChanges...)
	allChanges = append(allChanges, configChanges...)
	allChanges = append(allChanges, tlsChanges...)
	allChanges = append(allChanges, inChanges...)
	if len(allChanges) > 0 {
		for index := range allChanges {
			allChanges[index].DateTime = dt
			allChanges[index].Actor = loginUser
		}
		err = tx.Model(model.Changes{}).Create(&allChanges).Error
		if err != nil {
			return err
		}
	}

	LastUpdate = dt

	return nil
}

func (s *ConfigService) CheckChanges(lu string) (bool, error) {
	if lu == "" {
		return true, nil
	}
	if LastUpdate == 0 {
		db := database.GetDB()
		var count int64
		err := db.Model(model.Changes{}).Where("date_time > " + lu).Count(&count).Error
		if err == nil {
			LastUpdate = time.Now().Unix()
		}
		return count > 0, err
	} else {
		intLu, err := strconv.ParseInt(lu, 10, 64)
		return LastUpdate > intLu, err
	}
}

func (s *ConfigService) Save(singboxConfig *SingBoxConfig, needRestart bool) error {
	configPath := config.GetBinFolderPath()
	_, err := os.Stat(configPath + "/config.json")
	if os.IsNotExist(err) {
		err = os.MkdirAll(configPath, 01764)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	data, err := json.MarshalIndent(singboxConfig, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(configPath+"/config.json", data, 0764)
	if err != nil {
		return err
	}

	if needRestart {
		err = s.RestartCore()
		if err != nil {
			return err
		}
	}
	// s.Controller.Restart()

	return nil
}

func (s *ConfigService) DepleteClients() error {
	users, inboundIds, err := s.ClientService.DepleteClients()
	if err != nil || len(users) == 0 || len(inboundIds) == 0 {
		return err
	}

	// inbounds, err := s.InboundService.FromIds(inboundIds)
	// if err != nil {
	// 	return err
	// }
	// for inbound_index, inbound := range inbounds {
	// 	var inboundJson map[string]interface{}
	// 	json.Unmarshal(inbound.Options, &inboundJson)
	// 	inbound_users, ok := inboundJson["users"].([]interface{})
	// 	if ok {
	// 		var updatedUsers []interface{}
	// 		for _, user := range inbound_users {
	// 			userMap, ok := user.(map[string]interface{})
	// 			if ok {
	// 				name, exists := userMap["name"].(string)
	// 				if exists && s.contains(users, name) {
	// 					// Skip the user exists
	// 					continue
	// 				}
	// 				username, exists := userMap["username"].(string)
	// 				if exists && s.contains(users, username) {
	// 					// Skip the username exists
	// 					continue
	// 				}
	// 			}
	// 			updatedUsers = append(updatedUsers, user)
	// 		}
	// 		// Exception for Naive and ShadowTLSv3
	// 		if len(updatedUsers) == 0 {
	// 			if inboundJson["type"].(string) == "naive" ||
	// 				(inboundJson["type"].(string) == "shadowtls" &&
	// 					inboundJson["version"].(float64) == 3) {
	// 				updatedUsers = append(updatedUsers, make(map[string]interface{}))
	// 			}
	// 		}

	// 		inboundJson["users"] = updatedUsers
	// 	}
	// 	modifiedInbound, err := json.MarshalIndent(inboundJson, "", "  ")
	// 	if err != nil {
	// 		return err
	// 	}
	// 	inbounds[inbound_index] = modifiedInbound
	// }

	// err = s.Save(singboxConfig, true)
	// if err != nil {
	// 	return err
	// }
	return nil
}

func (s *ConfigService) contains(slice []string, item string) bool {
	for _, str := range slice {
		if str == item {
			return true
		}
	}
	return false
}

func (s *ConfigService) GetChanges(actor string, chngKey string, count string) []model.Changes {
	c, _ := strconv.Atoi(count)
	whereString := "`id`>0"
	if len(actor) > 0 {
		whereString += " and `actor`='" + actor + "'"
	}
	if len(chngKey) > 0 {
		whereString += " and `key`='" + chngKey + "'"
	}
	db := database.GetDB()
	var chngs []model.Changes
	err := db.Model(model.Changes{}).Where(whereString).Order("`id` desc").Limit(c).Scan(&chngs).Error
	if err != nil {
		logger.Warning(err)
	}
	return chngs
}
