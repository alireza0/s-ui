package service

import (
	"encoding/json"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"time"

	"gorm.io/gorm"
)

type ClientService struct {
	InboundService
}

func (s *ClientService) GetAll() ([]model.Client, error) {
	db := database.GetDB()
	clients := []model.Client{}
	err := db.Model(model.Client{}).Scan(&clients).Error
	if err != nil {
		return nil, err
	}
	return clients, nil
}

func (s *ClientService) Save(tx *gorm.DB, act string, data json.RawMessage) ([]uint, error) {
	var err error
	var inboundIds []uint

	switch act {
	case "new", "edit":
		var client model.Client
		err = json.Unmarshal(data, &client)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(client.Inbounds, &inboundIds)
		if err != nil {
			return nil, err
		}
		err = tx.Save(&client).Error
		if err != nil {
			return nil, err
		}
	case "del":
		var id uint
		err = json.Unmarshal(data, &id)
		if err != nil {
			return nil, err
		}
		var client model.Client
		err = tx.Where("id = ?", id).First(&client).Error
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(client.Inbounds, &inboundIds)
		if err != nil {
			return nil, err
		}
		err = tx.Where("id = ?", id).Delete(model.Client{}).Error
		if err != nil {
			return nil, err
		}
	}

	return inboundIds, nil
}

func (s *ClientService) UpdateLinks(tx *gorm.DB, links json.RawMessage) error {
	var userLinks []interface{}
	err := json.Unmarshal(links, &userLinks)
	if err != nil {
		return err
	}
	for _, userLink := range userLinks {
		userLinkData, _ := userLink.(map[string]interface{})
		userId, _ := userLinkData["id"].(float64)
		links, err := json.MarshalIndent(userLinkData["links"], "", "  ")
		if err != nil {
			return err
		}
		if inbounds, ok := userLinkData["inbounds"]; ok {
			inbounds, err := json.MarshalIndent(inbounds, "", "  ")
			if err != nil {
				return err
			}
			err = tx.Model(model.Client{}).Where("id = ?", uint(userId)).Update("inbounds", inbounds).Error
			if err != nil {
				return err
			}
		}
		err = tx.Model(model.Client{}).Where("id = ?", uint(userId)).Update("links", links).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientService) DepleteClients() error {
	var err error
	var clients []model.Client
	var changes []model.Changes
	var users []string
	var inboundIds []uint

	now := time.Now().Unix()
	db := database.GetDB()

	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
			if len(inboundIds) > 0 && corePtr.IsRunning() {
				err1 := s.InboundService.RestartInbounds(tx, inboundIds)
				if err1 != nil {
					logger.Error("unable to restart inbounds: ", err1)
				}
			}
		} else {
			tx.Rollback()
		}
	}()

	err = tx.Model(model.Client{}).Where("enable = true AND ((volume >0 AND up+down > volume) OR (expiry > 0 AND expiry < ?))", now).Scan(&clients).Error
	if err != nil {
		return err
	}

	dt := time.Now().Unix()
	for _, client := range clients {
		logger.Debug("Client ", client.Name, " is going to be disabled")
		users = append(users, client.Name)
		var userInbounds []uint
		json.Unmarshal(client.Inbounds, &userInbounds)
		inboundIds = s.uniqueAppendInboundIds(inboundIds, userInbounds)
		changes = append(changes, model.Changes{
			DateTime: dt,
			Actor:    "DepleteJob",
			Key:      "clients",
			Action:   "disable",
			Obj:      json.RawMessage("\"" + client.Name + "\""),
		})
	}

	// Save changes
	if len(changes) > 0 {
		err = tx.Model(model.Client{}).Where("enable = true AND ((volume >0 AND up+down > volume) OR (expiry > 0 AND expiry < ?))", now).Update("enable", false).Error
		if err != nil {
			return err
		}
		err = tx.Model(model.Changes{}).Create(&changes).Error
		if err != nil {
			return err
		}
		LastUpdate = dt
	}

	return nil
}

// avoid duplicate inboundIds
func (s *ClientService) uniqueAppendInboundIds(a []uint, b []uint) []uint {
	m := make(map[uint]bool)
	for _, v := range a {
		m[v] = true
	}
	for _, v := range b {
		m[v] = true
	}
	var res []uint
	for k := range m {
		res = append(res, k)
	}
	return res
}
