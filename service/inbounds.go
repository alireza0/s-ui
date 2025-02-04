package service

import (
	"encoding/json"
	"os"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/util"
	"s-ui/util/common"
	"strings"

	"gorm.io/gorm"
)

type InboundService struct{}

func (s *InboundService) Get(ids string) (*[]map[string]interface{}, error) {
	if ids == "" {
		return s.GetAll()
	}
	return s.getById(ids)
}

func (s *InboundService) getById(ids string) (*[]map[string]interface{}, error) {
	var inbound []model.Inbound
	var result []map[string]interface{}
	db := database.GetDB()
	err := db.Model(model.Inbound{}).Where("id in ?", strings.Split(ids, ",")).Scan(&inbound).Error
	if err != nil {
		return nil, err
	}
	for _, inb := range inbound {
		inbData, err := inb.MarshalFull()
		if err != nil {
			return nil, err
		}
		result = append(result, *inbData)
	}
	return &result, nil
}

func (s *InboundService) GetAll() (*[]map[string]interface{}, error) {
	db := database.GetDB()
	inbounds := []model.Inbound{}
	err := db.Model(model.Inbound{}).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	var data []map[string]interface{}
	for _, inbound := range inbounds {
		var shadowtls_version uint
		inbData := map[string]interface{}{
			"id":     inbound.Id,
			"type":   inbound.Type,
			"tag":    inbound.Tag,
			"tls_id": inbound.TlsId,
		}
		if inbound.Options != nil {
			var restFields map[string]json.RawMessage
			if err := json.Unmarshal(inbound.Options, &restFields); err != nil {
				return nil, err
			}
			inbData["listen"] = restFields["listen"]
			inbData["listen_port"] = restFields["listen_port"]
			if inbound.Type == "shadowtls" {
				json.Unmarshal(restFields["version"], &shadowtls_version)
			}
		}
		switch inbound.Type {
		case "mixed", "socks", "http", "shadowsocks", "vmess", "trojan", "naive", "hysteria", "shadowtls", "tuic", "hysteria2", "vless":
			if inbound.Type == "shadowtls" && shadowtls_version < 3 {
				break
			}
			users := []string{}
			err = db.Raw("SELECT clients.name FROM clients, json_each(clients.inbounds) as je WHERE je.value = ?", inbound.Id).Scan(&users).Error
			if err != nil {
				return nil, err
			}
			if len(users) > 0 || inbound.Type != "shadowsocks" {
				inbData["users"] = users
			}
		}

		data = append(data, inbData)
	}
	return &data, nil
}

func (s *InboundService) FromIds(ids []uint) ([]*model.Inbound, error) {
	db := database.GetDB()
	inbounds := []*model.Inbound{}
	err := db.Model(model.Inbound{}).Where("id in ?", ids).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) Save(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) (uint, error) {
	var err error
	var id uint

	switch act {
	case "new", "edit":
		var inbound model.Inbound
		err = inbound.UnmarshalJSON(data)
		if err != nil {
			return 0, err
		}
		if inbound.TlsId > 0 {
			err = tx.Model(model.Tls{}).Where("id = ?", inbound.TlsId).Find(&inbound.Tls).Error
			if err != nil {
				return 0, err
			}
		}

		err = util.FillOutJson(&inbound, hostname)
		if err != nil {
			return 0, err
		}

		err = tx.Save(&inbound).Error
		if err != nil {
			return 0, err
		}
		id = inbound.Id

		if corePtr.IsRunning() {
			if act == "edit" {
				var oldTag string
				err = tx.Model(model.Inbound{}).Select("tag").Where("id = ?", inbound.Id).Find(&oldTag).Error
				if err != nil {
					return 0, err
				}
				err = corePtr.RemoveInbound(oldTag)
				if err != nil && err != os.ErrInvalid {
					return 0, err
				}
			}

			inboundConfig, err := inbound.MarshalJSON()
			if err != nil {
				return 0, err
			}

			if act == "edit" {
				inboundConfig, err = s.addUsers(tx, inboundConfig, inbound.Id, inbound.Type)
			} else {
				inboundConfig, err = s.initUsers(tx, inboundConfig, initUserIds, inbound.Type)
			}
			if err != nil {
				return 0, err
			}

			err = corePtr.AddInbound(inboundConfig)
			if err != nil {
				return 0, err
			}
		}
	case "del":
		var tag string
		err = json.Unmarshal(data, &tag)
		if err != nil {
			return 0, err
		}
		if corePtr.IsRunning() {
			err = corePtr.RemoveInbound(tag)
			if err != nil && err != os.ErrInvalid {
				return 0, err
			}
		}
		err = tx.Model(model.Inbound{}).Select("id").Where("tag = ?", tag).Scan(&id).Error
		if err != nil {
			return 0, err
		}
		err = tx.Where("tag = ?", tag).Delete(model.Inbound{}).Error
		if err != nil {
			return 0, err
		}
	default:
		return 0, common.NewErrorf("unknown action: %s", act)
	}
	return id, nil
}

func (s *InboundService) UpdateOutJsons(tx *gorm.DB, inboundIds []uint, hostname string) error {
	var inbounds []model.Inbound
	err := tx.Model(model.Inbound{}).Preload("Tls").Where("id in ?", inboundIds).Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		err = util.FillOutJson(&inbound, hostname)
		if err != nil {
			return err
		}
		err = tx.Model(model.Inbound{}).Where("tag = ?", inbound.Tag).Update("out_json", inbound.OutJson).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *InboundService) GetAllConfig(db *gorm.DB) ([]json.RawMessage, error) {
	var inboundsJson []json.RawMessage
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("Tls").Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		inboundJson, err := inbound.MarshalJSON()
		if err != nil {
			return nil, err
		}
		inboundJson, err = s.addUsers(db, inboundJson, inbound.Id, inbound.Type)
		if err != nil {
			return nil, err
		}
		inboundsJson = append(inboundsJson, inboundJson)
	}
	return inboundsJson, nil
}

func (s *InboundService) addUsers(db *gorm.DB, inboundJson []byte, inboundId uint, inboundType string) ([]byte, error) {
	switch inboundType {
	case "mixed", "socks", "http", "shadowsocks", "vmess", "trojan", "naive", "hysteria", "shadowtls", "tuic", "hysteria2", "vless":
		break
	default:
		return inboundJson, nil
	}

	var inbound map[string]interface{}
	err := json.Unmarshal(inboundJson, &inbound)
	if err != nil {
		return nil, err
	}

	if inboundType == "shadowtls" {
		version, _ := inbound["version"].(float64)
		if int(version) < 3 {
			return inboundJson, nil
		}
	}
	if inboundType == "shadowsocks" {
		method, _ := inbound["method"].(string)
		if method == "2022-blake3-aes-128-gcm" {
			inboundType = "shadowsocks16"
		}
	}
	var users []string
	err = db.Raw(`SELECT json_extract(clients.config, ?)
								FROM clients, json_each(clients.inbounds) as je
								WHERE clients.enable = true AND je.value = ?;`,
		"$."+inboundType, inboundId).Scan(&users).Error
	if err != nil {
		return nil, err
	}
	var usersJson []json.RawMessage
	for _, user := range users {
		usersJson = append(usersJson, json.RawMessage(user))
	}

	if len(usersJson) > 0 || inboundType != "shadowsocks" {
		inbound["users"] = usersJson
	}

	return json.Marshal(inbound)
}

func (s *InboundService) initUsers(db *gorm.DB, inboundJson []byte, clientIds string, inboundType string) ([]byte, error) {
	ClientIds := strings.Split(clientIds, ",")
	if len(ClientIds) == 0 {
		return inboundJson, nil
	}
	switch inboundType {
	case "mixed", "socks", "http", "shadowsocks", "vmess", "trojan", "naive", "hysteria", "shadowtls", "tuic", "hysteria2", "vless":
		break
	default:
		return inboundJson, nil
	}

	var inbound map[string]interface{}
	err := json.Unmarshal(inboundJson, &inbound)
	if err != nil {
		return nil, err
	}

	if inboundType == "shadowtls" {
		version, _ := inbound["version"].(float64)
		if int(version) < 3 {
			return inboundJson, nil
		}
	}
	if inboundType == "shadowsocks" {
		method, _ := inbound["method"].(string)
		if method == "2022-blake3-aes-128-gcm" {
			inboundType = "shadowsocks16"
		}
	}
	var users []string
	err = db.Raw(`SELECT json_extract(clients.config, ?)
								FROM clients
								WHERE enable = true AND id in ?`,
		"$."+inboundType, ClientIds).Scan(&users).Error
	if err != nil {
		return nil, err
	}
	var usersJson []json.RawMessage
	for _, user := range users {
		usersJson = append(usersJson, json.RawMessage(user))
	}

	if len(usersJson) > 0 || inboundType != "shadowsocks" {
		inbound["users"] = usersJson
	}

	return json.Marshal(inbound)
}

func (s *InboundService) RestartInbounds(tx *gorm.DB, ids []uint) error {
	var inbounds []*model.Inbound
	err := tx.Model(model.Inbound{}).Preload("Tls").Where("id in ?", ids).Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		err = corePtr.RemoveInbound(inbound.Tag)
		if err != nil && err != os.ErrInvalid {
			return err
		}
		inboundConfig, err := inbound.MarshalJSON()
		if err != nil {
			return err
		}
		inboundConfig, err = s.addUsers(tx, inboundConfig, inbound.Id, inbound.Type)
		err = corePtr.AddInbound(inboundConfig)
		if err != nil {
			return err
		}
	}
	return nil
}
