package sub

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"
)

const defaultJson = `
{
  "inbounds": [
    {
      "type": "tun",
      "address": [
				"172.19.0.1/30",
				"fdfe:dcba:9876::1/126"
			],
      "mtu": 9000,
      "auto_route": true,
      "strict_route": false,
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
      "users": []
    }
  ]
}
`

type JsonService struct {
	service.SettingService
	LinkService
}

func (j *JsonService) GetJson(subId string, format string) (*string, []string, error) {
	var jsonConfig map[string]interface{}

	client, inDatas, err := j.getData(subId)
	if err != nil {
		return nil, nil, err
	}

	outbounds, outTags, err := j.getOutbounds(client.Config, inDatas, client.Remark)
	if err != nil {
		return nil, nil, err
	}

	extOutbounds, extTags := j.LinkService.GetExternalOutbounds(&client.Links)
	*outbounds = append(*outbounds, extOutbounds...)
	*outTags = append(*outTags, extTags...)

	j.addDefaultOutbounds(outbounds, outTags)

	err = json.Unmarshal([]byte(defaultJson), &jsonConfig)
	if err != nil {
		return nil, nil, err
	}

	jsonConfig["outbounds"] = outbounds

	// Add other objects from settings
	j.addOthers(&jsonConfig)

	result, _ := json.MarshalIndent(jsonConfig, "", "  ")
	resultStr := string(result)

	updateInterval, _ := j.SettingService.GetSubUpdates()
	headers := util.GetHeaders(client, updateInterval)

	return &resultStr, headers, nil
}

func (j *JsonService) getData(subId string) (*model.Client, []*model.Inbound, error) {
	db := database.GetDB()
	client := &model.Client{}
	err := db.Model(model.Client{}).Where("enable = true and name = ?", subId).First(client).Error
	if err != nil {
		return nil, nil, err
	}
	var clientInbounds []uint
	err = json.Unmarshal(client.Inbounds, &clientInbounds)
	if err != nil {
		return nil, nil, err
	}
	var inbounds []*model.Inbound
	err = db.Model(model.Inbound{}).Preload("Tls").Where("id in ?", clientInbounds).Find(&inbounds).Error
	if err != nil {
		return nil, nil, err
	}
	return client, inbounds, nil
}

func (j *JsonService) getOutbounds(clientConfig json.RawMessage, inbounds []*model.Inbound, clientRemark string) (*[]map[string]interface{}, *[]string, error) {
	var outbounds []map[string]interface{}
	var configs map[string]interface{}
	var outTags []string

	err := json.Unmarshal(clientConfig, &configs)
	if err != nil {
		return nil, nil, err
	}
	for _, inData := range inbounds {
		if len(inData.OutJson) < 5 {
			continue
		}
		var outbound map[string]interface{}
		err = json.Unmarshal(inData.OutJson, &outbound)
		if err != nil {
			return nil, nil, err
		}
		protocol, _ := outbound["type"].(string)

		// Shadowsocks
		if protocol == "shadowsocks" {
			var userPass []string
			var inbOptions map[string]interface{}
			err = json.Unmarshal(inData.Options, &inbOptions)
			if err != nil {
				return nil, nil, err
			}
			method, _ := inbOptions["method"].(string)
			if strings.HasPrefix(method, "2022") {
				inbPass, _ := inbOptions["password"].(string)
				userPass = append(userPass, inbPass)
			}
			var pass string
			pass, _ = configs[util.ShadowsocksClientConfigKey(method)].(map[string]interface{})["password"].(string)
			userPass = append(userPass, pass)
			outbound["password"] = strings.Join(userPass, ":")
		} else { // Other protocols
			config, _ := configs[protocol].(map[string]interface{})
			for key, value := range config {
				if key == "name" || key == "alterId" {
					continue
				}
				if key == "flow" {
					if inData.TlsId == 0 {
						continue
					}
					if tr, ok := outbound["transport"].(map[string]interface{}); ok {
						if transportType, _ := tr["type"].(string); transportType != "" {
							continue
						}
					}
				}
				outbound[key] = value
			}
		}

		var addrs []map[string]interface{}
		err = json.Unmarshal(inData.Addrs, &addrs)
		if err != nil {
			return nil, nil, err
		}
		tag, _ := outbound["tag"].(string)
		if len(addrs) == 0 {
			tag = util.JoinRemark(clientRemark, tag)
			outbound["tag"] = tag
			// For mixed protocol, use separated socks and http
			if protocol == "mixed" {
				j.pushMixed(&outbounds, &outTags, outbound)
			} else {
				outTags = append(outTags, tag)
				outbounds = append(outbounds, outbound)
			}
		} else {
			for index, addr := range addrs {
				// Copy original config
				newOut := make(map[string]interface{}, len(outbound))
				for key, value := range outbound {
					newOut[key] = value
				}
				// Change and push copied config
				server, _ := addr["server"].(string)
				newOut["server"] = util.NormalizeHost(server)
				port, _ := addr["server_port"].(float64)
				newOut["server_port"] = int(port)

				// Override TLS
				if addrTls, ok := addr["tls"].(map[string]interface{}); ok {
					outTls, _ := newOut["tls"].(map[string]interface{})
					if outTls == nil {
						outTls = make(map[string]interface{})
					}
					for key, value := range addrTls {
						outTls[key] = value
					}
					newOut["tls"] = outTls
				}

				remark, _ := addr["remark"].(string)
				newTag := fmt.Sprintf("%d.%s", index+1, util.JoinRemark(clientRemark, tag+remark))
				newOut["tag"] = newTag
				// For mixed protocol, use separated socks and http
				if protocol == "mixed" {
					j.pushMixed(&outbounds, &outTags, newOut)
				} else {
					outTags = append(outTags, newTag)
					outbounds = append(outbounds, newOut)
				}
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
	}
	*outbounds = append(outbound, *outbounds...)
}

func (j *JsonService) addOthers(jsonConfig *map[string]interface{}) error {
	// Default routing rules, used only when the template doesn't define its own.
	// When the template provides `rules`, they are used verbatim so the user has
	// full control over ordering (e.g. rules before sniff) and which rules exist.
	defaultRules := []interface{}{
		map[string]interface{}{
			"action": "sniff",
		},
		map[string]interface{}{
			"clash_mode": "Direct",
			"action":     "route",
			"outbound":   "direct",
		},
		map[string]interface{}{
			"clash_mode": "Global",
			"action":     "route",
			"outbound":   "proxy",
		},
	}
	route := map[string]interface{}{
		"auto_detect_interface": true,
		"final":                 "proxy",
		"rules":                 defaultRules,
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
	if _, ok := othersJson["inbounds"]; ok {
		(*jsonConfig)["inbounds"] = othersJson["inbounds"]
	}
	if _, ok := othersJson["experimental"]; ok {
		(*jsonConfig)["experimental"] = othersJson["experimental"]
	}
	if _, ok := othersJson["rule_set"]; ok {
		route["rule_set"] = othersJson["rule_set"]
	}
	j.addHTTPClients(jsonConfig, route, othersJson)
	if settingRules, ok := othersJson["rules"].([]interface{}); ok {
		route["rules"] = settingRules
	}
	if defaultDomainResolver, ok := othersJson["default_domain_resolver"].(string); ok && defaultDomainResolver != "" {
		route["default_domain_resolver"] = defaultDomainResolver
	} else if fallback := fallbackDomainResolver(othersJson); fallback != "" {
		// With more than one DNS server and no resolver named for dial fields,
		// sing-box has to guess which one resolves outbound server domains and
		// reports the guess as deprecated. The template's final server is the
		// one it would have to fall back to anyway.
		route["default_domain_resolver"] = fallback
	}
	if v, ok := othersJson["override_android_vpn"]; ok {
		route["override_android_vpn"] = v
	}
	if final, ok := othersJson["final"].(string); ok && final != "" {
		route["final"] = final
	}
	(*jsonConfig)["route"] = route

	return nil
}

// fallbackDomainResolver returns the DNS server dial fields should resolve
// through when the template names none, or "" when the config has too few
// servers for the choice to matter.
func fallbackDomainResolver(othersJson map[string]interface{}) string {
	dns, ok := othersJson["dns"].(map[string]interface{})
	if !ok {
		return ""
	}
	servers, ok := dns["servers"].([]interface{})
	if !ok || len(servers) < 2 {
		return ""
	}
	if final, ok := dns["final"].(string); ok && final != "" {
		return final
	}
	// No final server either: the first one is what sing-box treats as default.
	if first, ok := servers[0].(map[string]interface{}); ok {
		tag, _ := first["tag"].(string)
		return tag
	}
	return ""
}

// defaultHTTPClientTag names the HTTP client the generated config declares for
// downloading remote rule-sets.
const defaultHTTPClientTag = "default"

// addHTTPClients carries the template's HTTP clients across and, when the
// config downloads remote rule-sets without naming a client for them, declares
// one. Left implicit, sing-box 1.14 falls back to the default outbound and
// reports the fallback as deprecated; an explicit client says the same thing
// and keeps the client's log clean.
func (j *JsonService) addHTTPClients(jsonConfig *map[string]interface{}, route map[string]interface{}, othersJson map[string]interface{}) {
	if clients, ok := othersJson["http_clients"]; ok {
		(*jsonConfig)["http_clients"] = clients
	}
	if defaultClient, ok := othersJson["default_http_client"].(string); ok && defaultClient != "" {
		route["default_http_client"] = defaultClient
		return
	}
	// A template that brings its own clients decides for itself.
	if _, ok := (*jsonConfig)["http_clients"]; ok {
		return
	}
	if !needsDefaultHTTPClient(route) {
		return
	}
	(*jsonConfig)["http_clients"] = []interface{}{
		map[string]interface{}{"tag": defaultHTTPClientTag},
	}
	route["default_http_client"] = defaultHTTPClientTag
}

// needsDefaultHTTPClient reports whether any remote rule-set would fall back to
// the implicit default HTTP client.
func needsDefaultHTTPClient(route map[string]interface{}) bool {
	ruleSets, ok := route["rule_set"].([]interface{})
	if !ok {
		return false
	}
	for _, entry := range ruleSets {
		ruleSet, isObject := entry.(map[string]interface{})
		if !isObject {
			continue
		}
		if ruleSetType, _ := ruleSet["type"].(string); ruleSetType != "remote" {
			continue
		}
		if _, hasClient := ruleSet["http_client"]; !hasClient {
			return true
		}
	}
	return false
}

func (j *JsonService) pushMixed(outbounds *[]map[string]interface{}, outTags *[]string, out map[string]interface{}) {
	socksOut := make(map[string]interface{}, 1)
	httpOut := make(map[string]interface{}, 1)
	for key, value := range out {
		socksOut[key] = value
		httpOut[key] = value
	}
	socksTag := fmt.Sprintf("%s-socks", out["tag"])
	httpTag := fmt.Sprintf("%s-http", out["tag"])
	socksOut["type"] = "socks"
	httpOut["type"] = "http"
	socksOut["tag"] = socksTag
	httpOut["tag"] = httpTag
	*outbounds = append(*outbounds, socksOut, httpOut)
	*outTags = append(*outTags, socksTag, httpTag)
}
