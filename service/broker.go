package service

import (
	"encoding/json"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const (
	ClientEventSubject = "s-ui.clients.events"
	ClientCreated      = "CLIENT_CREATED"
	ClientUpdated      = "CLIENT_UPDATED"
	ClientDeleted      = "CLIENT_DELETED"
)

// ClientEvent represents a change to a client.
type ClientEvent struct {
	Action        string       `json:"action"`
	Client        model.Client `json:"client"`
	SourcePanelID string       `json:"sourcePanelId"`
}

// BrokerService handles the connection and messaging with the NATS broker.
type BrokerService struct {
	nc      *nats.Conn
	panelID string
}

// NewBrokerService creates a new BrokerService and connects to the NATS server.
func NewBrokerService(settingService *SettingService) (*BrokerService, error) {
	natsUrl, err := settingService.GetNatsUrl()
	if err != nil {
		return nil, err
	}

	panelID := uuid.New().String()

	if natsUrl == "" {
		logger.Info("NATS URL is not configured, broker service will be disabled.")
		return &BrokerService{nc: nil, panelID: panelID}, nil
	}

	nc, err := nats.Connect(natsUrl)
	if err != nil {
		return nil, err
	}

	logger.Infof("Successfully connected to NATS server at %s", natsUrl)
	logger.Infof("This panel's unique ID is %s", panelID)

	return &BrokerService{nc: nc, panelID: panelID}, nil
}

// PublishClientEvent publishes a client event to the broker.
func (s *BrokerService) PublishClientEvent(action string, client *model.Client) error {
	if s.nc == nil {
		return nil // Broker is disabled
	}

	event := ClientEvent{
		Action:        action,
		Client:        *client,
		SourcePanelID: s.panelID,
	}

	data, err := json.Marshal(event)
	if err != nil {
		logger.Errorf("failed to marshal client event: %v", err)
		return err
	}

	return s.nc.Publish(ClientEventSubject, data)
}

// SubscribeClientEvents subscribes to client events and passes them to the handler.
func (s *BrokerService) SubscribeClientEvents(handler func(event *ClientEvent)) (*nats.Subscription, error) {
	if s.nc == nil {
		return nil, nil // Broker is disabled
	}

	return s.nc.Subscribe(ClientEventSubject, func(msg *nats.Msg) {
		var event ClientEvent
		err := json.Unmarshal(msg.Data, &event)
		if err != nil {
			logger.Errorf("failed to unmarshal client event: %v", err)
			return
		}

		// Do not process events from the same panel
		if event.SourcePanelID == s.panelID {
			return
		}

		handler(&event)
	})
}

// Close closes the connection to the NATS server.
func (s *BrokerService) Close() {
	if s.nc != nil {
		s.nc.Close()
	}
}
