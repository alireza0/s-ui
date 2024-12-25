package service

import (
	"encoding/json"
	"s-ui/database"
	"s-ui/database/model"

	"gorm.io/gorm"
)

type InboundService struct{}

func (s *InboundService) GetAll() (*[]map[string]interface{}, error) {
	db := database.GetDB()
	inbounds := []map[string]interface{}{}
	err := db.Model(model.Inbound{}).Select("id, tag, type, address, port, tls_id , count(users) as ucount").Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return &inbounds, nil
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

func (s *InboundService) Save(db *gorm.DB, inbounds []*model.Inbound) error {
	return db.Save(inbounds).Error
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
		switch inbound.Type {
		case "mixed", "socks", "http", "shadowsocks", "vmess", "trojan", "naive", "hysteria", "shadowtls", "tuic", "hysteria2", "vless":
			inboundJson, err = s.addUsers(db, inboundJson, inbound.Id, inbound.Type)
			if err != nil {
				return nil, err
			}
		}
		inboundsJson = append(inboundsJson, inboundJson)
	}
	return inboundsJson, nil
}

func (s *InboundService) addUsers(db *gorm.DB, inboundJson []byte, inboundId uint, inboundType string) ([]byte, error) {
	var inbound map[string]interface{}
	err := json.Unmarshal(inboundJson, &inbound)
	if err != nil {
		return nil, err
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

	inbound["users"] = usersJson
	return json.Marshal(inbound)
}
