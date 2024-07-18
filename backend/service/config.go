package service

import (
	"encoding/json"
	"os"
	"s-ui/config"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"s-ui/singbox"
	"strconv"
	"time"
)

var ApiAddr string
var LastUpdate int64

type ConfigService struct {
	ClientService
	TlsService
	InDataService
	singbox.Controller
	SettingService
}

type SingBoxConfig struct {
	Log          json.RawMessage   `json:"log"`
	Dns          json.RawMessage   `json:"dns"`
	Ntp          json.RawMessage   `json:"ntp"`
	Inbounds     []json.RawMessage `json:"inbounds"`
	Outbounds    []json.RawMessage `json:"outbounds"`
	Route        json.RawMessage   `json:"route"`
	Experimental json.RawMessage   `json:"experimental"`
}

func NewConfigService() *ConfigService {
	return &ConfigService{}
}

func (s *ConfigService) InitConfig() error {
	configPath := config.GetBinFolderPath()
	data, err := os.ReadFile(configPath + "/config.json")
	if err != nil {
		if os.IsNotExist(err) {
			defaultConfig := []byte(config.GetDefaultConfig())
			err = os.MkdirAll(configPath, 01764)
			if err != nil {
				return err
			}
			err = os.WriteFile(configPath+"/config.json", defaultConfig, 0764)
			if err != nil {
				return err
			}
			data = defaultConfig
		} else {
			return err
		}
	}
	var singboxConfig SingBoxConfig
	err = json.Unmarshal(data, &singboxConfig)
	if err != nil {
		return err
	}

	return s.RefreshApiAddr(&singboxConfig)
}

func (s *ConfigService) GetConfig() (*SingBoxConfig, error) {
	configPath := config.GetBinFolderPath()
	data, err := os.ReadFile(configPath + "/config.json")
	if err != nil {
		return nil, err
	}
	singboxConfig := SingBoxConfig{}
	err = json.Unmarshal(data, &singboxConfig)
	if err != nil {
		return nil, err
	}
	return &singboxConfig, nil
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
	if len(inChanges) > 0 {
		err = s.InDataService.Save(tx, inChanges)
		if err != nil {
			return err
		}
	}
	if len(settingChanges) > 0 {
		err = s.SettingService.Save(tx, settingChanges)
		if err != nil {
			return err
		}
	}
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
			case "log":
				newConfig.Log = rawObject
			case "dns":
				newConfig.Dns = rawObject
			case "ntp":
				newConfig.Ntp = rawObject
			case "route":
				newConfig.Route = rawObject
			case "experimental":
				newConfig.Experimental = rawObject
			case "inbounds":
				if change.Action == "edit" {
					newConfig.Inbounds[change.Index] = rawObject
				} else if change.Action == "del" {
					newConfig.Inbounds = append(newConfig.Inbounds[:change.Index], newConfig.Inbounds[change.Index+1:]...)
				} else {
					newConfig.Inbounds = append(newConfig.Inbounds, rawObject)
				}
			case "outbounds":
				if change.Action == "edit" {
					newConfig.Outbounds[change.Index] = rawObject
				} else if change.Action == "del" {
					newConfig.Outbounds = append(newConfig.Outbounds[:change.Index], newConfig.Outbounds[change.Index+1:]...)
				} else {
					newConfig.Outbounds = append(newConfig.Outbounds, rawObject)
				}
			}
		}

		err = s.Save(&newConfig)
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

func (s *ConfigService) Save(singboxConfig *SingBoxConfig) error {
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

	s.RefreshApiAddr(singboxConfig)
	s.Controller.Restart()

	return nil
}

func (s *ConfigService) RefreshApiAddr(singboxConfig *SingBoxConfig) error {
	Env_API := config.GetEnvApi()
	if len(Env_API) > 0 {
		ApiAddr = Env_API
	} else {
		var err error
		if singboxConfig == nil {
			singboxConfig, err = s.GetConfig()
			if err != nil {
				return err
			}

		}

		var experimental struct {
			V2rayApi struct {
				Listen string      `json:"listen"`
				Stats  interface{} `jaon:"stats"`
			} `json:"v2ray_api"`
		}
		err = json.Unmarshal(singboxConfig.Experimental, &experimental)
		if err != nil {
			return err
		}

		ApiAddr = experimental.V2rayApi.Listen
	}
	return nil
}

func (s *ConfigService) DepleteClients() error {
	users, inbounds, err := s.ClientService.DepleteClients()
	if err != nil || len(users) == 0 || len(inbounds) == 0 {
		return err
	}

	singboxConfig, err := s.GetConfig()
	if err != nil {
		return err
	}
	for inbound_index, inbound := range singboxConfig.Inbounds {
		var inboundJson map[string]interface{}
		json.Unmarshal(inbound, &inboundJson)
		if s.contains(inbounds, inboundJson["tag"].(string)) {
			inbound_users, ok := inboundJson["users"].([]interface{})
			if ok {
				var updatedUsers []interface{}
				for _, user := range inbound_users {
					userMap, ok := user.(map[string]interface{})
					if ok {
						name, exists := userMap["name"].(string)
						if exists && s.contains(users, name) {
							// Skip the user exists
							continue
						}
						username, exists := userMap["username"].(string)
						if exists && s.contains(users, username) {
							// Skip the username exists
							continue
						}
					}
					updatedUsers = append(updatedUsers, user)
				}
				// Exception for Naive and ShadowTLSv3
				if len(updatedUsers) == 0 {
					if inboundJson["type"].(string) == "naive" ||
						(inboundJson["type"].(string) == "shadowtls" &&
							inboundJson["version"].(float64) == 3) {
						updatedUsers = append(updatedUsers, make(map[string]interface{}))
					}
				}

				inboundJson["users"] = updatedUsers
			}
		}
		modifiedInbound, err := json.MarshalIndent(inboundJson, "", "  ")
		if err != nil {
			return err
		}
		singboxConfig.Inbounds[inbound_index] = modifiedInbound
	}

	err = s.Save(singboxConfig)
	if err != nil {
		return err
	}
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
