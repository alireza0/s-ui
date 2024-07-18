package sub

import (
	"encoding/json"
	"fmt"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/service"
	"s-ui/util"
)

const defaultJson = `
{
  "inbounds": [
    {
      "type": "tun",
      "inet4_address": "172.19.0.1/30",
      "mtu": 9000,
      "auto_route": true,
      "strict_route": false,
      "sniff": true,
      "endpoint_independent_nat": false,
      "stack": "system",
      "platform": {
        "http_proxy": {
          "enabled": true,
          "server": "127.0.0.1",
          "server_port": 2080
        }
      }
    },
    {
      "type": "mixed",
      "listen": "127.0.0.1",
      "listen_port": 2080,
      "sniff": true,
      "users": []
    }
  ]
}
`

type JsonService struct {
	service.SettingService
	LinkService
}

func (j *JsonService) GetJson(subId string, format string) (*string, error) {
	var jsonConfig map[string]interface{}

	client, inDatas, err := j.getData(subId)
	if err != nil {
		return nil, err
	}

	outbounds, outTags, err := j.getOutbounds(client.Config, inDatas)
	if err != nil {
		return nil, err
	}

	links := j.LinkService.GetLinks(&client.Links, "external", "")
	for index, link := range links {
		json, tag, err := util.GetOutbound(link, index)
		if err == nil && len(tag) > 0 {
			*outbounds = append(*outbounds, *json)
			*outTags = append(*outTags, tag)
		}
	}

	j.addDefaultOutbounds(outbounds, outTags)

	err = json.Unmarshal([]byte(defaultJson), &jsonConfig)
	if err != nil {
		return nil, err
	}

	jsonConfig["outbounds"] = outbounds

	// Add other objects from settings
	j.addOthers(&jsonConfig)

	result, _ := json.MarshalIndent(jsonConfig, "", "  ")
	resultStr := string(result)
	return &resultStr, nil
}

func (j *JsonService) getData(subId string) (*model.Client, *[]model.InboundData, error) {
	db := database.GetDB()
	client := &model.Client{}
	err := db.Model(model.Client{}).Where("enable = true and name = ?", subId).First(client).Error
	if err != nil {
		return nil, nil, err
	}
	var inbounds []string
	err = json.Unmarshal(client.Inbounds, &inbounds)
	if err != nil {
		return nil, nil, err
	}
	inDatas := &[]model.InboundData{}
	err = db.Model(model.InboundData{}).Where("tag in ?", inbounds).Find(&inDatas).Error
	if err != nil {
		return nil, nil, err
	}
	return client, inDatas, nil
}

func (j *JsonService) getOutbounds(clientConfig json.RawMessage, inDatas *[]model.InboundData) (*[]map[string]interface{}, *[]string, error) {
	var outbounds []map[string]interface{}
	var configs map[string]interface{}
	var outTags []string

	err := json.Unmarshal(clientConfig, &configs)
	if err != nil {
		return nil, nil, err
	}
	for _, inData := range *inDatas {
		if len(inData.OutJson) < 5 {
			continue
		}
		var outbound map[string]interface{}
		err = json.Unmarshal(inData.OutJson, &outbound)
		if err != nil {
			return nil, nil, err
		}
		protocol, _ := outbound["type"].(string)
		config, _ := configs[protocol].(map[string]interface{})
		for key, value := range config {
			if key != "alterId" && key != "name" && key != "username" {
				outbound[key] = value
			}
		}

		var addrs []map[string]interface{}
		err = json.Unmarshal(inData.Addrs, &addrs)
		if err != nil {
			return nil, nil, err
		}
		tag := outbound["tag"].(string)
		if len(addrs) == 0 {
			outTags = append(outTags, tag)
			outbounds = append(outbounds, outbound)
		} else {
			for index, addr := range addrs {
				// Copy original config
				newOut := make(map[string]interface{}, len(outbound))
				for key, value := range outbound {
					newOut[key] = value
				}
				// Change and push copied config
				newOut["server"], _ = addr["server"].(string)
				port, _ := addr["server_port"].(float64)
				newOut["server_port"] = int(port)
				remark, _ := addr["remark"].(string)
				newTag := fmt.Sprintf("%d.%s%s", index+1, tag, remark)
				outTags = append(outTags, newTag)
				newOut["tag"] = newTag
				outbounds = append(outbounds, newOut)
			}
		}
	}
	return &outbounds, &outTags, nil
}

func (j *JsonService) addDefaultOutbounds(outbounds *[]map[string]interface{}, outTags *[]string) {
	outbound := []map[string]interface{}{
		{
			"outbounds": append([]string{"auto", "direct"}, *outTags...),
			"tag":       "proxy",
			"type":      "selector",
		},
		{
			"tag":       "auto",
			"type":      "urltest",
			"outbounds": outTags,
			"url":       "http://www.gstatic.com/generate_204",
			"interval":  "10m",
			"tolerance": 50,
		},
		{
			"type": "direct",
			"tag":  "direct",
		},
		{
			"type": "dns",
			"tag":  "dns-out",
		},
		{
			"type": "block",
			"tag":  "block",
		},
	}
	*outbounds = append(outbound, *outbounds...)
}

func (j *JsonService) addOthers(jsonConfig *map[string]interface{}) error {
	rules := []interface{}{
		map[string]interface{}{
			"clash_mode": "Direct",
			"outbound":   "direct",
		},
		map[string]interface{}{
			"clash_mode": "Global",
			"outbound":   "proxy",
		},
	}
	route := map[string]interface{}{
		"auto_detect_interface": true,
		"final":                 "proxy",
		"rules":                 rules,
	}

	othersStr, err := j.SettingService.GetSubJsonExt()
	if err != nil {
		return err
	}
	if len(othersStr) == 0 {
		(*jsonConfig)["route"] = route
		return nil
	}
	var othersJson map[string]interface{}
	err = json.Unmarshal([]byte(othersStr), &othersJson)
	if err != nil {
		return err
	}
	if _, ok := othersJson["log"]; ok {
		(*jsonConfig)["log"] = othersJson["log"]
	}
	if _, ok := othersJson["dns"]; ok {
		(*jsonConfig)["dns"] = othersJson["dns"]
	}
	if _, ok := othersJson["experimental"]; ok {
		(*jsonConfig)["experimental"] = othersJson["lexperimentalog"]
	}
	if _, ok := othersJson["rule_set"]; ok {
		route["rule_set"] = othersJson["rule_set"]
	}
	if settingRules, ok := othersJson["rules"].([]interface{}); ok {
		route["rules"] = append(rules, settingRules...)
	}
	(*jsonConfig)["route"] = route

	return nil
}
