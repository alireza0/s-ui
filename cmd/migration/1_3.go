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
	// Where before Find. Chained the other way round the filter never applied,
	// so this loaded every outbound and rewrote each one -- reindenting rows
	// that had nothing to migrate.
	err := db.Where("json_extract(CAST(options AS TEXT), '$.domain_strategy') IS NOT NULL").Find(&outbounds).Error
	if err != nil {
		return err
	}
	for _, outbound := range outbounds {
		var restFields map[string]json.RawMessage
		if err := json.Unmarshal(outbound.Options, &restFields); err != nil {
			return err
		}
		delete(restFields, "domain_strategy")
		options, err := json.MarshalIndent(restFields, "", "  ")
		if err != nil {
			return err
		}
		outbound.Options = options
		// The error was discarded, so a row that failed to save left the set
		// partly migrated behind a migration that reported success.
		if err := db.Save(&outbound).Error; err != nil {
			return err
		}
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
		if len(configs["anytls"]) > 0 {
			continue
		}
		// A client with no trojan block gave a nil RawMessage, which marshals
		// as the literal `null`. That was then written to the column, and the
		// guard above treated RawMessage("null") as present on every later
		// run, so it could never be corrected.
		trojan, ok := configs["trojan"]
		if !ok || len(trojan) == 0 {
			continue
		}
		configs["anytls"] = trojan
		configJson, err := json.MarshalIndent(configs, "", "  ")
		if err != nil {
			return err
		}
		clients[index].Config = configJson
		if err := db.Save(&clients[index]).Error; err != nil {
			return err
		}
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
