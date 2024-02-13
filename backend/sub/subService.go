package sub

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"s-ui/service"
	"strings"
	"time"
)

type SubService struct {
	service.SettingService
}

type Link struct {
	Type   string `json:"type"`
	Remark string `json:"remark"`
	Uri    string `json:"uri"`
}

func (s *SubService) GetSubs(subId string) (*string, []string, error) {
	var err error

	db := database.GetDB()
	client := &model.Client{}
	err = db.Model(model.Client{}).Where("enable = true and name = ?", subId).First(client).Error
	if err != nil {
		return nil, nil, err
	}

	links := []Link{}
	err = json.Unmarshal([]byte(client.Links), &links)
	if err != nil {
		return nil, nil, err
	}

	clientInfo := ""
	subShowInfo, _ := s.SettingService.GetSubShowInfo()
	if subShowInfo {
		clientInfo = s.getClientInfo(client)
	}

	var result string
	for _, link := range links {
		switch link.Type {
		case "external":
			result += fmt.Sprintln(link.Uri)
		case "sub":
			result += s.getExternalSub(link.Uri)
		case "local":
			result += fmt.Sprintln(s.addClientInfo(link.Uri, clientInfo))
		}
	}

	var headers []string
	updateInterval, _ := s.SettingService.GetSubUpdates()
	headers = append(headers, fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", client.Up, client.Down, client.Volume, client.Expiry))
	headers = append(headers, fmt.Sprintf("%d", updateInterval))
	headers = append(headers, subId)

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

func (s *SubService) addClientInfo(uri string, clientInfo string) string {
	protocol := strings.Split(uri, "://")
	if len(protocol) < 2 {
		return uri
	}
	switch protocol[0] {
	case "vmess":
		var vmessJson map[string]interface{}
		config, err := base64.StdEncoding.DecodeString(protocol[1])
		if err != nil {
			logger.Warning("sub: Error decoding vmess content:", err)
			return uri
		}
		err = json.Unmarshal(config, &vmessJson)
		if err != nil {
			logger.Warning("sub: Error decoding vmess content:", err)
			return uri
		}
		vmessJson["ps"] = vmessJson["ps"].(string) + clientInfo
		result, err := json.MarshalIndent(vmessJson, "", "  ")
		if err != nil {
			logger.Warning("sub: Error decoding vmess + clientInfo content:", err)
			return uri
		}
		return "vmess://" + base64.StdEncoding.EncodeToString(result)
	default:
		return uri + clientInfo
	}
}

func (s *SubService) getExternalSub(url string) string {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}

	// Make the HTTP request
	response, err := client.Get(url)
	if err != nil {
		logger.Warning("sub: Error making HTTP request:", err)
		return ""
	}
	defer response.Body.Close()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		logger.Warning("sub: Error reading response body:", err)
		return ""
	}

	// Check if the content is Base64 encoded
	isBase64 := s.isBase64Encoded(string(body))
	if isBase64 {
		// Decode Base64 content
		decodedText, err := base64.StdEncoding.DecodeString(string(body))
		if err != nil {
			logger.Warning("sub: Error decoding Base64 content:", err)
			return ""
		}

		return string(decodedText)
	} else {
		return string(body)
	}
}

// Function to check if a string is Base64 encoded
func (s *SubService) isBase64Encoded(str string) bool {
	_, err := base64.StdEncoding.DecodeString(str)
	return err == nil
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
