package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
)

type ClientService struct{}

func (s *ClientService) Get(id string) (*[]model.Client, error) {
	if id == "" {
		return s.GetAll()
	}
	return s.getById(id)
}

func (s *ClientService) getById(id string) (*[]model.Client, error) {
	db := database.GetDB()
	var client []model.Client
	err := db.Model(model.Client{}).Where("id in ?", strings.Split(id, ",")).Scan(&client).Error
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func (s *ClientService) GetAll() (*[]model.Client, error) {
	db := database.GetDB()
	var clients []model.Client
	err := db.Model(model.Client{}).Select("`id`, `enable`, `name`, `desc`, `group`, `inbounds`, `up`, `down`, `volume`, `expiry`").Scan(&clients).Error
	if err != nil {
		return nil, err
	}
	return &clients, nil
}

func (s *ClientService) Save(tx *gorm.DB, act string, data json.RawMessage, hostname string) ([]uint, error) {
	var err error
	var inboundIds []uint

	switch act {
	case "new", "edit":
		var client model.Client
		err = json.Unmarshal(data, &client)
		if err != nil {
			return nil, err
		}
		err = s.updateLinksWithFixedInbounds(tx, []*model.Client{&client}, hostname)
		if err != nil {
			return nil, err
		}
		if act == "edit" {
			// Find changed inbounds
			inboundIds, err = s.findInboundsChanges(tx, client)
			if err != nil {
				return nil, err
			}
		} else {
			err = json.Unmarshal(client.Inbounds, &inboundIds)
			if err != nil {
				return nil, err
			}
		}
		err = tx.Save(&client).Error
		if err != nil {
			return nil, err
		}
	case "addbulk":
		var clients []*model.Client
		err = json.Unmarshal(data, &clients)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(clients[0].Inbounds, &inboundIds)
		if err != nil {
			return nil, err
		}
		err = s.updateLinksWithFixedInbounds(tx, clients, hostname)
		if err != nil {
			return nil, err
		}
		err = tx.Save(clients).Error
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
	default:
		return nil, common.NewErrorf("unknown action: %s", act)
	}

	return inboundIds, nil
}

func (s *ClientService) updateLinksWithFixedInbounds(tx *gorm.DB, clients []*model.Client, hostname string) error {
	var err error
	var inbounds []model.Inbound
	var inboundIds []uint

	err = json.Unmarshal(clients[0].Inbounds, &inboundIds)
	if err != nil {
		return err
	}

	// Zero inbounds means removing local links only
	if len(inboundIds) > 0 {
		err = tx.Model(model.Inbound{}).Preload("Tls").Where("id in ? and type in ?", inboundIds, util.InboundTypeWithLink).Find(&inbounds).Error
		if err != nil {
			return err
		}
	}
	for index, client := range clients {
		var clientLinks []map[string]string
		err = json.Unmarshal(client.Links, &clientLinks)
		if err != nil {
			return err
		}

		newClientLinks := []map[string]string{}
		for _, inbound := range inbounds {
			newLinks := util.LinkGenerator(client.Config, &inbound, hostname)
			for _, newLink := range newLinks {
				newClientLinks = append(newClientLinks, map[string]string{
					"remark": inbound.Tag,
					"type":   "local",
					"uri":    newLink,
				})
			}
		}

		// Add non local links
		for _, clientLink := range clientLinks {
			if clientLink["type"] != "local" {
				newClientLinks = append(newClientLinks, clientLink)
			}
		}

		clients[index].Links, err = json.MarshalIndent(newClientLinks, "", "  ")
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientService) UpdateClientsOnInboundAdd(tx *gorm.DB, initIds string, inboundId uint, hostname string) error {
	clientIds := strings.Split(initIds, ",")
	var clients []model.Client
	err := tx.Model(model.Client{}).Where("id in ?", clientIds).Find(&clients).Error
	if err != nil {
		return err
	}
	var inbound model.Inbound
	err = tx.Model(model.Inbound{}).Preload("Tls").Where("id = ?", inboundId).Find(&inbound).Error
	if err != nil {
		return err
	}
	for _, client := range clients {
		// Add inbounds
		var clientInbounds []uint
		json.Unmarshal(client.Inbounds, &clientInbounds)
		clientInbounds = append(clientInbounds, inboundId)
		client.Inbounds, err = json.MarshalIndent(clientInbounds, "", "  ")
		if err != nil {
			return err
		}
		// Add links
		var clientLinks, newClientLinks []map[string]string
		json.Unmarshal(client.Links, &clientLinks)
		newLinks := util.LinkGenerator(client.Config, &inbound, hostname)
		for _, newLink := range newLinks {
			newClientLinks = append(newClientLinks, map[string]string{
				"remark": inbound.Tag,
				"type":   "local",
				"uri":    newLink,
			})
		}
		for _, clientLink := range clientLinks {
			if clientLink["remark"] != inbound.Tag {
				newClientLinks = append(newClientLinks, clientLink)
			}
		}

		client.Links, err = json.MarshalIndent(newClientLinks, "", "  ")
		if err != nil {
			return err
		}
		err = tx.Save(&client).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientService) UpdateClientsOnInboundDelete(tx *gorm.DB, id uint, tag string) error {
	var clients []model.Client
	err := tx.Table("clients").
		Where("EXISTS (SELECT 1 FROM json_each(clients.inbounds) WHERE json_each.value = ?)", id).
		Find(&clients).Error
	if err != nil {
		return err
	}
	for _, client := range clients {
		// Delete inbounds
		var clientInbounds, newClientInbounds []uint
		json.Unmarshal(client.Inbounds, &clientInbounds)
		for _, clientInbound := range clientInbounds {
			if clientInbound != id {
				newClientInbounds = append(newClientInbounds, clientInbound)
			}
		}
		client.Inbounds, err = json.MarshalIndent(newClientInbounds, "", "  ")
		if err != nil {
			return err
		}
		// Delete links
		var clientLinks, newClientLinks []map[string]string
		json.Unmarshal(client.Links, &clientLinks)
		for _, clientLink := range clientLinks {
			if clientLink["remark"] != tag {
				newClientLinks = append(newClientLinks, clientLink)
			}
		}
		client.Links, err = json.MarshalIndent(newClientLinks, "", "  ")
		if err != nil {
			return err
		}
		err = tx.Save(&client).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientService) UpdateLinksByInboundChange(tx *gorm.DB, inbounds *[]model.Inbound, hostname string, oldTag string) error {
	var err error
	for _, inbound := range *inbounds {
		var clients []model.Client
		err = tx.Table("clients").
			Where("EXISTS (SELECT 1 FROM json_each(clients.inbounds) WHERE json_each.value = ?)", inbound.Id).
			Find(&clients).Error
		if err != nil {
			return err
		}
		for _, client := range clients {
			var clientLinks, newClientLinks []map[string]string
			json.Unmarshal(client.Links, &clientLinks)
			newLinks := util.LinkGenerator(client.Config, &inbound, hostname)
			for _, newLink := range newLinks {
				newClientLinks = append(newClientLinks, map[string]string{
					"remark": inbound.Tag,
					"type":   "local",
					"uri":    newLink,
				})
			}
			for _, clientLink := range clientLinks {
				if clientLink["type"] != "local" || (clientLink["remark"] != inbound.Tag && clientLink["remark"] != oldTag) {
					newClientLinks = append(newClientLinks, clientLink)
				}
			}

			client.Links, err = json.MarshalIndent(newClientLinks, "", "  ")
			if err != nil {
				return err
			}
			err = tx.Save(&client).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ClientService) DepleteClients() ([]uint, error) {
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
		} else {
			tx.Rollback()
		}
	}()

	err = tx.Model(model.Client{}).Where("enable = true AND ((volume >0 AND up+down > volume) OR (expiry > 0 AND expiry < ?))", now).Scan(&clients).Error
	if err != nil {
		return nil, err
	}

	dt := time.Now().Unix()
	for _, client := range clients {
		logger.Debug("Client ", client.Name, " is going to be disabled")
		users = append(users, client.Name)
		var userInbounds []uint
		json.Unmarshal(client.Inbounds, &userInbounds)
		// Find changed inbounds
		inboundIds = common.UnionUintArray(inboundIds, userInbounds)
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
			return nil, err
		}
		err = tx.Model(model.Changes{}).Create(&changes).Error
		if err != nil {
			return nil, err
		}
		LastUpdate = dt
	}

	return inboundIds, nil
}

func (s *ClientService) findInboundsChanges(tx *gorm.DB, client model.Client) ([]uint, error) {
	var err error
	var oldClient model.Client
	var oldInboundIds, newInboundIds []uint
	err = tx.Model(model.Client{}).Where("id = ?", client.Id).First(&oldClient).Error
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(oldClient.Inbounds, &oldInboundIds)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(client.Inbounds, &newInboundIds)
	if err != nil {
		return nil, err
	}

	// Check client.Config changes
	if !bytes.Equal(oldClient.Config, client.Config) ||
		oldClient.Name != client.Name ||
		oldClient.Enable != client.Enable {
		return common.UnionUintArray(oldInboundIds, newInboundIds), nil
	}

	// Check client.Inbounds changes
	diffInbounds := common.DiffUintArray(oldInboundIds, newInboundIds)

	return diffInbounds, nil
}

// ResetClientsTraffic checks all clients and resets traffic for those that meet reset criteria
// Returns the list of inbound IDs that need to be restarted (for re-enabled clients)
func (s *ClientService) ResetClientsTraffic() ([]uint, error) {
	var err error
	var inboundIds []uint

	now := time.Now()
	nowUnix := now.Unix()

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	// Get all clients with reset enabled
	var clients []model.Client
	err = tx.Model(model.Client{}).Where("reset_mode > 0").Find(&clients).Error
	if err != nil {
		return nil, err
	}

	var changes []model.Changes
	var trafficHistories []model.TrafficHistory

	for _, client := range clients {
		shouldReset := false
		var periodStartTime int64

		switch client.ResetMode {
		case model.ResetModeMonthly:
			shouldReset, periodStartTime = s.checkMonthlyReset(client, now)
		case model.ResetModePeriodic:
			shouldReset, periodStartTime = s.checkPeriodicReset(client, now)
		}

		if !shouldReset {
			continue
		}

		logger.Debug("Resetting traffic for client: ", client.Name)

		// Record traffic history before reset
		trafficHistories = append(trafficHistories, model.TrafficHistory{
			ClientId:   client.Id,
			ClientName: client.Name,
			StartTime:  periodStartTime,
			EndTime:    nowUnix,
			Up:         client.Up,
			Down:       client.Down,
			ResetMode:  client.ResetMode,
		})

		// Check if client was disabled due to traffic limit
		wasDisabledByTraffic := !client.Enable && client.Volume > 0 && (client.Up+client.Down) >= client.Volume

		// Reset traffic
		updateData := map[string]interface{}{
			"up":            0,
			"down":          0,
			"last_reset_at": nowUnix,
		}

		// Re-enable client if it was disabled due to traffic limit
		if wasDisabledByTraffic {
			updateData["enable"] = true

			// Collect inbound IDs for restart
			var clientInbounds []uint
			json.Unmarshal(client.Inbounds, &clientInbounds)
			inboundIds = common.UnionUintArray(inboundIds, clientInbounds)

			changes = append(changes, model.Changes{
				DateTime: nowUnix,
				Actor:    "ResetTrafficJob",
				Key:      "clients",
				Action:   "enable",
				Obj:      json.RawMessage("\"" + client.Name + "\""),
			})
		}

		err = tx.Model(model.Client{}).Where("id = ?", client.Id).Updates(updateData).Error
		if err != nil {
			return nil, err
		}

		changes = append(changes, model.Changes{
			DateTime: nowUnix,
			Actor:    "ResetTrafficJob",
			Key:      "clients",
			Action:   "reset_traffic",
			Obj:      json.RawMessage("\"" + client.Name + "\""),
		})
	}

	// Save traffic histories
	if len(trafficHistories) > 0 {
		err = tx.Create(&trafficHistories).Error
		if err != nil {
			return nil, err
		}
	}

	// Save changes
	if len(changes) > 0 {
		err = tx.Create(&changes).Error
		if err != nil {
			return nil, err
		}
		LastUpdate = nowUnix
	}

	return inboundIds, nil
}

// checkMonthlyReset checks if a client should have its traffic reset based on monthly schedule
// Returns (shouldReset, periodStartTime)
func (s *ClientService) checkMonthlyReset(client model.Client, now time.Time) (bool, int64) {
	today := now.Day()
	currentMonth := now.Month()
	currentYear := now.Year()

	// Determine reset day
	resetDay := client.ResetDayOfMonth
	if resetDay <= 0 {
		// Use creation day as default
		createdAt := time.Unix(client.CreatedAt, 0)
		resetDay = createdAt.Day()
	}

	// Handle months with fewer days
	daysInMonth := time.Date(currentYear, currentMonth+1, 0, 0, 0, 0, 0, now.Location()).Day()
	if resetDay > daysInMonth {
		resetDay = daysInMonth
	}

	// Check if today is the reset day
	if today != resetDay {
		return false, 0
	}

	// Check if already reset this month
	if client.LastResetAt > 0 {
		lastReset := time.Unix(client.LastResetAt, 0)
		if lastReset.Month() == currentMonth && lastReset.Year() == currentYear {
			return false, 0
		}
	}

	// Calculate period start time (last reset or creation time)
	periodStart := client.LastResetAt
	if periodStart == 0 {
		periodStart = client.CreatedAt
	}

	return true, periodStart
}

// checkPeriodicReset checks if a client should have its traffic reset based on N-day period
// Returns (shouldReset, periodStartTime)
func (s *ClientService) checkPeriodicReset(client model.Client, now time.Time) (bool, int64) {
	if client.ResetPeriodDays <= 0 {
		return false, 0
	}

	nowUnix := now.Unix()
	periodSeconds := int64(client.ResetPeriodDays) * 24 * 60 * 60

	// Determine the reference point (last reset or creation time)
	referenceTime := client.LastResetAt
	if referenceTime == 0 {
		referenceTime = client.CreatedAt
	}

	// Check if enough time has passed since last reset
	timeSinceReference := nowUnix - referenceTime
	if timeSinceReference < periodSeconds {
		return false, 0
	}

	return true, referenceTime
}

// GetTrafficHistory retrieves traffic history for a specific client
func (s *ClientService) GetTrafficHistory(clientId uint, limit int) ([]model.TrafficHistory, error) {
	var histories []model.TrafficHistory
	db := database.GetDB()

	query := db.Model(model.TrafficHistory{}).Where("client_id = ?", clientId).Order("end_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&histories).Error
	return histories, err
}

// GetAllTrafficHistory retrieves traffic history for all clients
func (s *ClientService) GetAllTrafficHistory(limit int) ([]model.TrafficHistory, error) {
	var histories []model.TrafficHistory
	db := database.GetDB()

	query := db.Model(model.TrafficHistory{}).Order("end_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&histories).Error
	return histories, err
}
