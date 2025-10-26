package sub

import (
	"strings"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"

	"gopkg.in/yaml.v3"
)

type ClashService struct {
	service.SettingService
	JsonService
	LinkService
}

const basicClashConfig = `mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090
tun:
  enable: true
  stack: system
  auto-route: true
  auto-detect-interface: true
  dns-hijack:
    - any:53
dns:
  enable: true
  ipv6: false
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  default-nameserver:
    - 8.8.8.8
    - 1.1.1.1
  nameserver:
    - https://doh.pub/dns-query
    - https://1.0.0.1/dns-query
  fallback:
    - tcp://9.9.9.9:53
  fake-ip-filter:
    - "*.lan"
    - localhost
    - "*.local"
rules:
  - GEOIP,Private,DIRECT
  - MATCH,Proxy
`

const ProxyGroups = `- name: Proxy
  type: select
  proxies: []
- name: Auto
  type: url-test
  proxies: []
  url: http://www.gstatic.com/generate_204
  interval: 300
  tolerance: 50
`

func (s *ClashService) GetClash(subId string) (*string, []string, error) {

	client, inDatas, err := s.getData(subId)
	if err != nil {
		return nil, nil, err
	}

	outbounds, outTags, err := s.getOutbounds(client.Config, inDatas)
	if err != nil {
		return nil, nil, err
	}

	links := s.LinkService.GetLinks(&client.Links, "external", "")
	tagNumEnable := 0
	if len(links) > 1 {
		tagNumEnable = 1
	}
	for index, link := range links {
		json, tag, err := util.GetOutbound(link, (index+1)*tagNumEnable)
		if err == nil && len(tag) > 0 {
			*outbounds = append(*outbounds, *json)
			*outTags = append(*outTags, tag)
		}
	}

	othersStr, err := s.getClashConfig()
	if err != nil || len(othersStr) == 0 {
		othersStr = basicClashConfig
	}

	result, err := s.ConvertToClashMeta(outbounds)
	if err != nil {
		return nil, nil, err
	}
	resultStr := othersStr + "\n" + string(result)

	updateInterval, _ := s.SettingService.GetSubUpdates()
	headers := util.GetHeaders(client, updateInterval)

	return &resultStr, headers, nil
}

func (s *ClashService) getClashConfig() (string, error) {
	subClashExt, err := s.SettingService.GetSubClashExt()
	if err != nil {
		return "", err
	}

	return subClashExt, nil
}

func (s *ClashService) ConvertToClashMeta(outbounds *[]map[string]interface{}) ([]byte, error) {
	var proxies []interface{}
	proxyTags := make([]string, 0)
	for _, obMap := range *outbounds {

		t, _ := obMap["type"].(string)
		if t == "selector" || t == "urltest" || t == "direct" {
			continue
		}

		proxy := make(map[string]interface{})
		proxy["name"] = obMap["tag"]
		proxy["type"] = t
		proxy["server"] = obMap["server"]
		proxy["port"] = obMap["server_port"]

		switch t {
		case "vmess", "vless", "tuic":
			proxy["uuid"] = obMap["uuid"]
			if t == "vmess" {
				if alterId, ok := obMap["alter_id"].(float64); ok {
					proxy["alterId"] = int(alterId)
				} else {
					proxy["alterId"] = 0
				}
				proxy["cipher"] = "auto"
			}
			if t == "vless" {
				if flow, ok := obMap["flow"].(string); ok {
					proxy["flow"] = flow
				}
			}
			if t == "tuic" {
				proxy["password"] = obMap["password"]
				if congestion_control, ok := obMap["congestion_control"].(string); ok {
					proxy["congestion-controller"] = congestion_control
				}
			}
		case "trojan":
			proxy["password"] = obMap["password"]
		case "socks", "http":
			if t == "socks" {
				proxy["type"] = "socks5"
			}
			proxy["username"] = obMap["username"]
			proxy["password"] = obMap["password"]
		case "hysteria", "hysteria2":
			if _, ok := obMap["up_mbps"].(float64); ok {
				proxy["up"] = obMap["up_mbps"]
			} else {
				proxy["up"] = 1000
			}
			if _, ok := obMap["down_mbps"].(float64); ok {
				proxy["down"] = obMap["down_mbps"]
			} else {
				proxy["down"] = 1000
			}
			if t == "hysteria" {
				proxy["auth-str"] = obMap["auth_str"]
				if obfs, ok := obMap["obfs"].(string); ok {
					proxy["obfs"] = obfs
				}
			} else {
				proxy["password"] = obMap["password"]
				if obfs, ok := obMap["obfs"].(map[string]interface{}); ok {
					proxy["obfs"] = obfs["type"]
					proxy["obfs-password"] = obfs["password"]
				}
			}

			if portLists, ok := obMap["server_ports"].([]interface{}); ok {
				var ports []string
				for _, portList := range portLists {
					portRange, _ := portList.(string)
					ports = append(ports, strings.ReplaceAll(portRange, ":", "-"))
				}
				proxy["ports"] = strings.Join(ports, ",")
			}
		case "anytls":
			proxy["password"] = obMap["password"]
			if tls, ok := obMap["tls"].(map[string]interface{}); ok {
				proxy["sni"] = tls["server_name"]
				proxy["skip-cert-verify"] = tls["insecure"]
			}
		case "shadowsocks":
			proxy["type"] = "ss"
			proxy["cipher"] = obMap["method"]
			proxy["password"] = obMap["password"]
			if network, ok := obMap["network"].(string); ok && network != "tcp" {
				proxy["udp"] = true
			}
			if uot, ok := obMap["udp_over_tcp"].(bool); ok && uot {
				proxy["udp-over-tcp"] = true
			}
		default:
			continue
		}

		// TLS params
		tls, isTls := obMap["tls"].(map[string]interface{})
		if isTls {
			tlsEnabled, ok := tls["enabled"].(bool)
			if ok && !tlsEnabled {
				isTls = false
			}
		}
		if isTls {
			proxy["tls"] = tls["enabled"]

			// ALPN if exists
			if alpn, ok := tls["alpn"].([]interface{}); ok {
				proxy["alpn"] = alpn
			}

			// Add reality if exists
			if reality, ok := tls["reality"].(map[string]interface{}); ok && reality["enabled"].(bool) {
				reality_opts := make(map[string]interface{})
				if pbk, ok := reality["public_key"].(string); ok {
					reality_opts["public-key"] = pbk
				}
				if sid, ok := reality["short_id"].(string); ok {
					reality_opts["short-id"] = sid
				}
				proxy["reality-opts"] = reality_opts
			}
			if utls, ok := tls["utls"].(map[string]interface{}); ok {
				if enabled, ok := utls["enabled"].(bool); ok && enabled {
					if fp, ok := utls["fingerprint"].(string); ok {
						proxy["client-fingerprint"] = fp
					}
				}
			}
			if sni, ok := tls["server_name"].(string); ok {
				if t == "http" {
					proxy["sni"] = sni
				} else {
					proxy["servername"] = sni
				}
			}
			if insecure, ok := tls["insecure"].(bool); ok && insecure {
				proxy["skip-cert-verify"] = insecure
			}
			// ech outbounds
			if ech, ok := tls["ech"].(interface{}); ok {
				ech_data, _ := ech.(map[string]interface{})
				ech_config, _ := ech_data["config"].([]interface{})
				ech_string := ""
				for i := 1; i < len(ech_config)-1; i++ {
					ech_string += ech_config[i].(string)
				}
				proxy["ech-opts"] = map[string]interface{}{
					"enable": true,
					"config": ech_string,
				}
			}
		}

		// Transport if exist
		if transport, ok := obMap["transport"].(map[string]interface{}); ok {
			tt, _ := transport["type"].(string)
			switch tt {
			case "http":
				httpOpts := make(map[string]interface{})
				if path, ok := transport["path"].([]interface{}); ok {
					httpOpts["path"] = path[0]
				} else if path, ok := transport["path"].(string); ok {
					httpOpts["path"] = path
				}
				if host, ok := transport["host"].([]interface{}); ok {
					httpOpts["host"] = host[0]
				}
				if isTls {
					proxy["network"] = "h2"
					proxy["h2-opts"] = httpOpts
				} else {
					proxy["network"] = "http"
					proxy["http-opts"] = map[string]interface{}{"path": []interface{}{httpOpts["path"]}, "host": httpOpts["host"]}
				}
			case "ws", "httpupgrade":
				proxy["network"] = "ws"
				wsOpts := make(map[string]interface{})
				if path, ok := transport["path"].(string); ok {
					wsOpts["path"] = path
				}
				if headers, ok := transport["headers"].([]interface{}); ok {
					wsOpts["headers"] = headers
				}
				if ed, ok := transport["early_data_header_name"].(string); ok {
					wsOpts["early-data-header-name"] = ed
				}
				if tt == "httpupgrade" {
					wsOpts["v2ray-http-upgrade"] = true
				}
				proxy["ws-opts"] = wsOpts
			case "grpc":
				proxy["network"] = "grpc"
				grpcOpts := make(map[string]interface{})
				if service_name, ok := transport["service_name"].(string); ok {
					grpcOpts["grpc-service-name"] = service_name
				}
				proxy["grpc-opts"] = grpcOpts
			}
		}

		// Multiplex
		if mux, ok := obMap["multiplex"].(map[string]interface{}); ok {
			if enabled, ok := mux["enabled"].(bool); ok && enabled {
				smux := make(map[string]interface{})
				smux["enabled"] = true
				if protocol, ok := mux["protocol"].(string); ok {
					smux["protocol"] = protocol
				}
				if _, ok := mux["max_connections"].(float64); ok {
					smux["max-connections"] = mux["max_connections"]
				}
				if _, ok := mux["min_streams"].(float64); ok {
					smux["min-streams"] = mux["min_streams"]
				}
				if _, ok := mux["max_streams"].(float64); ok {
					smux["max-streams"] = mux["max_streams"]
				}
				if _, ok := mux["padding"].(bool); ok {
					smux["padding"] = mux["padding"]
				}
				if brutal, ok := mux["brutal"].(map[string]interface{}); ok {
					if enabled, ok := brutal["enabled"].(bool); ok && enabled {
						brutalOpts := make(map[string]interface{})
						brutalOpts["enabled"] = true
						if _, ok := brutal["up_mbps"].(float64); ok {
							brutalOpts["up"] = brutal["up_mbps"]
						}
						if _, ok := brutal["down_mbps"].(float64); ok {
							brutalOpts["down"] = brutal["down_mbps"]
						}
						smux["brutal-opts"] = brutalOpts
					}
				}
				proxy["smux"] = smux
			}
		}

		proxies = append(proxies, proxy)
		proxyTags = append(proxyTags, obMap["tag"].(string))
	}

	var proxyGroups []map[string]interface{}
	err := yaml.Unmarshal([]byte(ProxyGroups), &proxyGroups)
	if err != nil {
		logger.Error(err.Error())
	}

	proxyGroups[1]["proxies"] = proxyTags
	proxyGroups[0]["proxies"] = append([]string{proxyGroups[1]["name"].(string)}, proxyTags...)

	output := map[string]interface{}{
		"proxies":      proxies,
		"proxy-groups": proxyGroups,
	}

	return yaml.Marshal(output)
}
