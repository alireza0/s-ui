package sub

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"
)

type SubService struct {
	service.SettingService
	LinkService
}

func (s *SubService) GetSubs(subId string) (*string, []string, error) {
	var err error

	db := database.GetDB()
	client := &model.Client{}
	// Removed strict enable = true check to handle re-enabled clients
	err = db.Model(model.Client{}).Where("name = ?", subId).First(client).Error
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().Unix()

	// Auto-enable client if it was disabled but subscription is now valid
	// This must be done BEFORE checking expiry/volume to allow renewed subscriptions to work
	if !client.Enable {
		// Check if subscription is valid (not expired and not over volume limit)
		isExpired := client.Expiry > 0 && client.Expiry < now
		isOverVolume := client.Volume > 0 && (client.Up+client.Down) > client.Volume

		if !isExpired && !isOverVolume {
			// Re-enable the client since subscription is valid
			err = db.Model(model.Client{}).Where("id = ?", client.Id).Update("enable", true).Error
			if err != nil {
				return nil, nil, err
			}
			client.Enable = true
		}
	}

	// Check if client has expired
	if client.Expiry > 0 && client.Expiry < now {
		return nil, nil, fmt.Errorf("client subscription has expired")
	}

	// Check if client has exceeded volume limit
	if client.Volume > 0 && (client.Up+client.Down) > client.Volume {
		return nil, nil, fmt.Errorf("client has exceeded volume limit")
	}

	clientInfo := ""
	subShowInfo, _ := s.SettingService.GetSubShowInfo()
	if subShowInfo {
		clientInfo = s.getClientInfo(client)
	}

	linksArray := s.LinkService.GetLinks(&client.Links, "all", clientInfo)
	result := strings.Join(linksArray, "\n")

	updateInterval, _ := s.SettingService.GetSubUpdates()
	headers := util.GetHeaders(client, updateInterval)

	subEncode, _ := s.SettingService.GetSubEncode()
	if subEncode {
		result = base64.StdEncoding.EncodeToString([]byte(result))
	}

	return &result, headers, nil
}

func (s *SubService) getClientInfo(c *model.Client) string {
	now := time.Now().Unix()

	var result []string
	if vol := c.Volume - (c.Up + c.Down); vol > 0 {
		result = append(result, fmt.Sprintf("%s%s", s.formatTraffic(vol), "📊"))
	}
	if c.Expiry > 0 {
		result = append(result, fmt.Sprintf("%d%s⏳", (c.Expiry-now)/86400, "Days"))
	}
	if len(result) > 0 {
		return " " + strings.Join(result, " ")
	} else {
		return " ♾"
	}
}

func (s *SubService) formatTraffic(trafficBytes int64) string {
	if trafficBytes < 1024 {
		return fmt.Sprintf("%.2fB", float64(trafficBytes)/float64(1))
	} else if trafficBytes < (1024 * 1024) {
		return fmt.Sprintf("%.2fKB", float64(trafficBytes)/float64(1024))
	} else if trafficBytes < (1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fMB", float64(trafficBytes)/float64(1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fGB", float64(trafficBytes)/float64(1024*1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fTB", float64(trafficBytes)/float64(1024*1024*1024*1024))
	} else {
		return fmt.Sprintf("%.2fEB", float64(trafficBytes)/float64(1024*1024*1024*1024*1024))
	}
}
