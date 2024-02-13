package service

import (
	"encoding/json"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ClientService struct {
}

func (s *ClientService) GetAll() (string, error) {
	db := database.GetDB()
	clients := []model.Client{}
	err := db.Model(model.Client{}).Scan(&clients).Error
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(clients)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *ClientService) Save(tx *gorm.DB, changes []model.Changes) error {
	var err error
	for _, change := range changes {
		client := model.Client{}
		err = json.Unmarshal(change.Obj, &client)
		if err != nil {
			return err
		}
		switch change.Action {
		case "new":
			err = tx.Create(&client).Error
		case "del":
			err = tx.Where("id = ?", change.Index).Delete(model.Client{}).Error
		default:
			err = tx.Save(client).Error
		}
		if err != nil {
			return err
		}
	}
	return err
}

func (s *ClientService) DepleteClients() ([]string, []string, error) {
	var err error
	var clients []model.Client
	var changes []model.Changes
	now := time.Now().Unix()
	db := database.GetDB()
	err = db.Model(model.Client{}).Where("enable = true AND ((volume >0 AND up+down > volume) OR (expiry > 0 AND expiry < ?))", now).Scan(&clients).Error
	if err != nil {
		return nil, nil, err
	}

	var users, inbounds []string
	for _, client := range clients {
		logger.Debug("Client ", client.Name, " is going to be disabled")
		users = append(users, client.Name)
		userInbounds := strings.Split(client.Inbounds, ",")
		inbounds = append(inbounds, userInbounds...)
		changes = append(changes, model.Changes{
			DateTime: time.Now().Unix(),
			Actor:    "DepleteJob",
			Key:      "clients",
			Action:   "disable",
			Obj:      json.RawMessage(client.Name),
		})
	}

	// Save changes
	if len(changes) > 0 {
		err = db.Model(model.Client{}).Where("enable = true AND ((volume >0 AND up+down > volume) OR (expiry > 0 AND expiry < ?))", now).Update("enable", false).Error
		if err != nil {
			return nil, nil, err
		}
		err = db.Model(model.Changes{}).Create(&changes).Error
		if err != nil {
			return nil, nil, err
		}
	}

	return users, inbounds, nil
}
