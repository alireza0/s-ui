package migration

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

func migrate_dns(db *gorm.DB) error {
	var configStr string
	err := db.Model(model.Setting{}).Select("value").Where("key = ?", "config").First(&configStr).Error
	if err != nil {
		return err
	}
	if configStr == "" {
		return nil
	}
	var config map[string]interface{}
	err = json.Unmarshal([]byte(configStr), &config)
	if err != nil {
		return err
	}
	if dnsConfig, ok := config["dns"].(map[string]interface{}); ok {
		if dnsServers, ok := dnsConfig["servers"].([]interface{}); ok {
			for index, dnsServer := range dnsServers {
				if dnsServer, ok := dnsServer.(map[string]interface{}); ok {
					if addr, ok := dnsServer["address"].(string); ok && addr != "" {
						switch addr {
						case "local":
							delete(dnsServer, "address")
							dnsServer["type"] = "local"
						case "fakeip":
							delete(dnsServer, "address")
							dnsServer["type"] = "fakeip"
						default:
							addrParsed, err := url.Parse(addr)
							if err != nil {
								continue
							}
							switch addrParsed.Scheme {
							case "":
								dnsServer["type"] = "udp"
								dnsServer["server"] = addr
							case "udp", "tcp", "tls", "quic", "https", "h3":
								dnsServer["type"] = addrParsed.Scheme
								dnsServer["server"] = addrParsed.Host
							case "dhcp":
								dnsServer["type"] = addrParsed.Scheme
								if addrParsed.Host != "auto" && addrParsed.Host != "" {
									dnsServer["interface"] = addrParsed.Host
								}
							case "rcode":
								dnsServer["type"] = "predefined"
								dnsServer["responses"] = []map[string]string{
									{
										"rcode": strings.ToUpper(addrParsed.Host),
									},
								}
							}
							delete(dnsServer, "address")
							if addrParsed.Port() != "" {
								port, err := strconv.Atoi(addrParsed.Port())
								if err == nil {
									dnsServer["server_port"] = port
								}
							}
							if address_resolver, ok := dnsServer["address_resolver"].(string); ok && address_resolver != "" {
								delete(dnsServer, "address_resolver")
								dnsServer["domain_resolver"] = address_resolver
							}
							delete(dnsServer, "strategy")
						}
						dnsServers[index] = dnsServer
					}
				}
			}
			dnsConfig["servers"] = dnsServers
		}
		config["dns"] = dnsConfig
	} else {
		return nil
	}

	// save changes
	configs, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return db.Model(model.Setting{}).Where("key = ?", "config").Update("value", string(configs)).Error
}

func remove_outbound_strategy(db *gorm.DB) error {
	var outbounds []model.Outbound
	err := db.Find(&outbounds).Where("json_extract(options, '$.domain_strategy') IS NOT NULL").Error
	if err != nil {
		return err
	}
	for _, outbound := range outbounds {
		var restFields map[string]json.RawMessage
		if err := json.Unmarshal(outbound.Options, &restFields); err != nil {
			return err
		}
		delete(restFields, "domain_strategy")
		outbound.Options, _ = json.MarshalIndent(restFields, "", "  ")
		db.Save(&outbound)
	}
	return nil
}

func anytls_user_config(db *gorm.DB) error {
	var clients []model.Client
	err := db.Model(model.Client{}).Find(&clients).Error
	if err != nil {
		return err
	}
	for index, client := range clients {
		var configs map[string]json.RawMessage
		if err := json.Unmarshal(client.Config, &configs); err != nil {
			return err
		}
		if configs["anytls"] != nil {
			continue
		}
		configs["anytls"] = configs["trojan"]
		configJson, err := json.MarshalIndent(configs, "", "  ")
		if err != nil {
			return err
		}
		clients[index].Config = configJson
		db.Save(&clients[index])
	}
	return nil
}

func to1_3(db *gorm.DB) error {
	err := anytls_user_config(db)
	if err != nil {
		return err
	}
	err = migrate_dns(db)
	if err != nil {
		return err
	}
	err = remove_outbound_strategy(db)
	if err != nil {
		return err
	}
	return nil
}
