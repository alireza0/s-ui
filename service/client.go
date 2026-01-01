package service

import (
	"encoding/json"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
	"gorm.io/gorm"
)

type ClientService struct {
	brokerService *BrokerService
}

func NewClientService(brokerService *BrokerService) *ClientService {
	return &ClientService{brokerService: brokerService}
}

// HandleClientEvent processes incoming client events from the broker.
func (s *ClientService) HandleClientEvent(event *ClientEvent) {
	db := database.GetDB()
	client := event.Client

	logger.Infof("Received client event '%s' for client '%s' from another panel", event.Action, client.Name)

	var err error
	switch event.Action {
	case ClientCreated:
		// Save will create or update depending on PK presence. We accept that behavior for now.
		err = db.Save(&client).Error
	case ClientUpdated:
		err = db.Save(&client).Error
	case ClientDeleted:
		err = db.Delete(&model.Client{}, client.Id).Error
	default:
		logger.Warningf("Unknown client event action: %s", event.Action)
	}

	if err != nil {
		logger.Errorf("Failed to handle client event '%s' for client '%s': %v", event.Action, client.Name, err)
	}
}

func (s *ClientService) Get(id string) (*[]model.Client, error) {
	if id == "" {
		var clients []model.Client
		db := database.GetDB()
		err := db.Find(&clients).Error
		if err != nil {
			return nil, err
		}
		return &clients, nil
	}

	var client model.Client
	db := database.GetDB()
	err := db.Where("id = ?", id).Find(&client).Error
	if err != nil {
		return nil, err
	}
	var clients []model.Client
	clients = append(clients, client)
	return &clients, nil
}

func (s *ClientService) Save(tx *gorm.DB, act string, data json.RawMessage, host string) (*[]model.Client, error) {
	switch act {
	case "add", "update":
		var client model.Client
		err := json.Unmarshal(data, &client)
		if err != nil {
			return nil, err
		}
		// declare inboundIds before using
		var inboundIds []uint
		if client.Id != 0 {
			err = tx.Save(&client).Error
			if err != nil {
				return nil, err
			}
			// Guard publish: brokerService may be disabled or failed to initialize.
			if s.brokerService != nil {
				if perr := s.brokerService.PublishClientEvent(ClientUpdated, &client); perr != nil {
					logger.Errorf("failed to publish client updated event for client %s: %v", client.Name, perr)
				}
			}
		} else {
			err = json.Unmarshal(client.Inbounds, &inboundIds)
			if err != nil {
				return nil, err
			}
			err = tx.Save(&client).Error
			if err != nil {
				return nil, err
			}
			if s.brokerService != nil {
				if perr := s.brokerService.PublishClientEvent(ClientCreated, &client); perr != nil {
					logger.Errorf("failed to publish client created event for client %s: %v", client.Name, perr)
				}
			}
		}
	case "addbulk":
		var clients []*model.Client
		err := json.Unmarshal(data, &clients)
		if err != nil {
			return nil, err
		}
		for _, client := range clients {
			if s.brokerService != nil {
				if perr := s.brokerService.PublishClientEvent(ClientCreated, client); perr != nil {
					logger.Errorf("failed to publish client created event for client %s: %v", client.Name, perr)
				}
			}
		}
	case "del":
		var id uint
		err := json.Unmarshal(data, &id)
		if err != nil {
			return nil, err
		}
		var client model.Client
		err = tx.Where("id = ?", id).Find(&client).Error
		if err != nil {
			return nil, err
		}
		err = tx.Delete(&client).Error
		if err != nil {
			return nil, err
		}
		if s.brokerService != nil {
			if perr := s.brokerService.PublishClientEvent(ClientDeleted, &client); perr != nil {
				logger.Errorf("failed to publish client deleted event for client %s: %v", client.Name, perr)
			}
		}
	default:
		return nil, common.NewErrorf("unknown action: %s", act)
	}
	return nil, nil
}
