package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"
)

var InboundTypeWithLink = []string{"socks", "http", "mixed", "shadowsocks", "naive", "hysteria", "hysteria2", "anytls", "tuic", "vless", "trojan", "vmess"}

func LinkGenerator(clientConfig json.RawMessage, i *model.Inbound, hostname string) []string {
	inbound, err := i.MarshalFull()
	if err != nil {
		return []string{}
	}

	var tls map[string]interface{}
	if i.TlsId > 0 {
		tls = prepareTls(i.Tls)
	}

	var userConfig map[string]map[string]interface{}
	if err := json.Unmarshal(clientConfig, &userConfig); err != nil {
		return []string{}
	}

	var Addrs []map[string]interface{}
	json.Unmarshal(i.Addrs, &Addrs)
	if len(Addrs) == 0 {
		Addrs = append(Addrs, map[string]interface{}{
			"server":      hostname,
			"server_port": (*inbound)["listen_port"],
			"remark":      i.Tag,
		})
		if i.TlsId > 0 {
			Addrs[0]["tls"] = tls
		}
	} else {
		for index, addr := range Addrs {
			addrRemark, _ := addr["remark"].(string)
			Addrs[index]["remark"] = i.Tag + addrRemark
			if i.TlsId > 0 {
				newTls := map[string]interface{}{}
				for k, v := range tls {
					newTls[k] = v
				}

				// Override tls
				if addrTls, ok := addr["tls"].(map[string]interface{}); ok {
					for k, v := range addrTls {
						newTls[k] = v
					}
				}
				Addrs[index]["tls"] = newTls
			}
		}
	}

	switch i.Type {
	case "socks":
		return socksLink(userConfig["socks"], *inbound, Addrs)
	case "http":
		return httpLink(userConfig["http"], *inbound, Addrs)
	case "mixed":
		return append(
			socksLink(userConfig["socks"], *inbound, Addrs),
			httpLink(userConfig["http"], *inbound, Addrs)...,
		)
	case "shadowsocks":
		return shadowsocksLink(userConfig, *inbound, Addrs)
	case "naive":
		return naiveLink(userConfig["naive"], *inbound, Addrs)
	case "hysteria":
		return hysteriaLink(userConfig["hysteria"], *inbound, Addrs)
	case "hysteria2":
		return hysteria2Link(userConfig["hysteria2"], *inbound, Addrs)
	case "tuic":
		return tuicLink(userConfig["tuic"], *inbound, Addrs)
	case "vless":
		return vlessLink(userConfig["vless"], *inbound, Addrs)
	case "anytls":
		return anytlsLink(userConfig["anytls"], Addrs)
	case "trojan":
		return trojanLink(userConfig["trojan"], *inbound, Addrs)
	case "vmess":
		return vmessLink(userConfig["vmess"], *inbound, Addrs)
	}

	return []string{}
}

func prepareTls(t *model.Tls) map[string]interface{} {
	var iTls, oTls map[string]interface{}
	json.Unmarshal(t.Client, &oTls)
	json.Unmarshal(t.Server, &iTls)

	for k, v := range iTls {
		switch k {
		case "enabled", "server_name", "alpn":
			oTls[k] = v
		case "reality":
			reality := v.(map[string]interface{})
			clientReality := oTls["reality"].(map[string]interface{})
			clientReality["enabled"] = reality["enabled"]
			if short_ids, hasSIds := reality["short_id"].([]interface{}); hasSIds && len(short_ids) > 0 {
				clientReality["short_id"] = short_ids[common.RandomInt(len(short_ids))]
			}
			oTls["reality"] = clientReality
		}
	}
	return oTls
}

func socksLink(userConfig map[string]interface{}, inbound map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	for _, addr := range addrs {
		links = append(links, fmt.Sprintf("socks5://%s:%s@%s:%d", userConfig["username"], userConfig["password"], addr["server"].(string), uint(addr["server_port"].(float64))))
	}
	return links
}

func httpLink(userConfig map[string]interface{}, inbound map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	var protocol string = "http"
	for _, addr := range addrs {
		if addr["tls"] != nil {
			protocol = "https"
		}
		links = append(links, fmt.Sprintf("%s://%s:%s@%s:%d", protocol, userConfig["username"], userConfig["password"], addr["server"].(string), uint(addr["server_port"].(float64))))
	}
	return links
}

func shadowsocksLink(
	userConfig map[string]map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	var userPass []string
	method, _ := inbound["method"].(string)
	if strings.HasPrefix(method, "2022") {
		inbPass, _ := inbound["password"].(string)
		userPass = append(userPass, inbPass)
	}
	var pass string
	if method == "2022-blake3-aes-128-gcm" {
		pass, _ = userConfig["shadowsocks16"]["password"].(string)
	} else {
		pass, _ = userConfig["shadowsocks"]["password"].(string)
	}
	userPass = append(userPass, pass)

	uriBase := fmt.Sprintf("ss://%s", toBase64([]byte(fmt.Sprintf("%s:%s", method, strings.Join(userPass, ":")))))

	var links []string
	for _, addr := range addrs {
		port, _ := addr["server_port"].(float64)
		links = append(links, fmt.Sprintf("%s@%s:%.0f#%s", uriBase, addr["server"].(string), port, addr["remark"].(string)))
	}
	return links
}

func naiveLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	username, _ := userConfig["username"].(string)

	baseUri := "http2://"
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		params["padding"] = "1"
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if sni, ok := tls["server_name"].(string); ok {
				params["peer"] = sni
			}
			if alpn, ok := tls["alpn"].([]interface{}); ok {
				alpnList := make([]string, len(alpn))
				for i, v := range alpn {
					alpnList[i] = v.(string)
				}
				params["alpn"] = strings.Join(alpnList, ",")
			}
			if insecure, ok := tls["insecure"].(bool); ok && insecure {
				params["insecure"] = "1"
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params["tfo"] = "1"
		} else {
			params["tfo"] = "0"
		}

		port, _ := addr["server_port"].(float64)
		uri := baseUri + toBase64([]byte(fmt.Sprintf("%s:%s@%s:%.0f", username, password, addr["server"].(string), port)))
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}
	return links
}

func hysteriaLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	baseUri := "hysteria://"
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params["downmbps"] = fmt.Sprintf("%.0f", upmbps)
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params["upmbps"] = fmt.Sprintf("%.0f", downmbps)
		}
		if auth, ok := userConfig["auth_str"].(string); ok {
			params["auth"] = auth
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(string); ok {
			params["obfs"] = obfs
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params["fastopen"] = "1"
		} else {
			params["fastopen"] = "0"
		}
		var outJson map[string]interface{}
		json.Unmarshal(inbound["out_json"].(json.RawMessage), &outJson)
		if mport, ok := outJson["server_ports"].([]interface{}); ok {
			mportList := make([]string, len(mport))
			for i, v := range mport {
				mportList[i] = v.(string)
			}
			params["mport"] = strings.Join(mportList, ",")
		}

		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addr["server"].(string), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func hysteria2Link(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	baseUri := fmt.Sprintf("%s%s@", "hysteria2://", password)
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params["downmbps"] = fmt.Sprintf("%.0f", upmbps)
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params["upmbps"] = fmt.Sprintf("%.0f", downmbps)
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(map[string]interface{}); ok {
			if obfsType, ok := obfs["type"].(string); ok {
				params["obfs"] = obfsType
			}
			if obfsPassword, ok := obfs["password"].(string); ok {
				params["obfs-password"] = obfsPassword
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params["fastopen"] = "1"
		} else {
			params["fastopen"] = "0"
		}
		var outJson map[string]interface{}
		json.Unmarshal(inbound["out_json"].(json.RawMessage), &outJson)
		if mport, ok := outJson["server_ports"].([]interface{}); ok {
			mportList := make([]string, len(mport))
			for i, v := range mport {
				mportList[i] = v.(string)
			}
			params["mport"] = strings.Join(mportList, ",")
		}

		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addr["server"].(string), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func anytlsLink(
	userConfig map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	baseUri := fmt.Sprintf("%s%s@", "anytls://", password)
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}

		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addr["server"].(string), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func tuicLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	uuid, _ := userConfig["uuid"].(string)
	baseUri := fmt.Sprintf("%s%s:%s@", "tuic://", uuid, password)
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if congestionControl, ok := inbound["congestion_control"].(string); ok {
			params["congestion_control"] = congestionControl
		}

		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, addr["server"].(string), port)
		links = append(links, addParams(uri, params, addr["remark"].(string)))
	}

	return links
}

func vlessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := baseParams
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls["enabled"].(bool) {
			getTlsParams(&params, tls, "allowInsecure")
			if flow, ok := userConfig["flow"].(string); ok {
				params["flow"] = flow
			}
		}
		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("vless://%s@%s:%.0f", uuid, addr["server"].(string), port)
		uri = addParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func trojanLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {
	password, _ := userConfig["password"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := baseParams
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls["enabled"].(bool) {
			getTlsParams(&params, tls, "allowInsecure")
		}
		port, _ := addr["server_port"].(float64)
		uri := fmt.Sprintf("trojan://%s@%s:%.0f", password, addr["server"].(string), port)
		uri = addParams(uri, params, addr["remark"].(string))
		links = append(links, uri)
	}

	return links
}

func vmessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	trasportParams := getTransportParams(inbound["transport"])
	var links []string

	baseParams := map[string]interface{}{
		"v":   2,
		"id":  uuid,
		"aid": 0,
	}
	if trasportParams["type"] == "http" || trasportParams["type"] == "tcp" {
		baseParams["net"] = "tcp"
		if trasportParams["type"] == "http" {
			baseParams["type"] = "http"
		}
	} else {
		baseParams["net"] = trasportParams["type"]
	}

	for _, addr := range addrs {
		obj := baseParams
		obj["add"], _ = addr["server"].(string)
		port, _ := addr["server_port"].(float64)
		obj["port"] = uint(port)
		obj["ps"], _ = addr["remark"].(string)
		if trasportParams["host"] != "" {
			obj["host"] = trasportParams["host"]
		}
		if trasportParams["path"] != "" {
			obj["path"] = trasportParams["path"]
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls["enabled"].(bool) {
			obj["tls"] = "tls"
			if insecure, ok := tls["insecure"].(bool); ok && insecure {
				obj["allowInsecure"] = 1
			}
			if sni, ok := tls["server_name"].(string); ok {
				obj["sni"] = sni
			}
			if alpn, ok := tls["alpn"].([]interface{}); ok {
				alpnList := make([]string, len(alpn))
				for i, v := range alpn {
					alpnList[i] = v.(string)
				}
				obj["alpn"] = strings.Join(alpnList, ",")
			}
			if utls, ok := tls["utls"].(map[string]interface{}); ok {
				obj["fp"], _ = utls["fingerprint"].(string)
			}
		} else {
			obj["tls"] = "none"
		}

		jsonStr, _ := json.MarshalIndent(obj, "", "  ")

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func toBase64(d []byte) string {
	return base64.StdEncoding.EncodeToString([]byte(d))
}

func addParams(uri string, params map[string]string, remark string) string {
	URL, _ := url.Parse(uri)
	var q []string
	for k, v := range params {
		switch k {
		case "mport", "alpn":
			q = append(q, fmt.Sprintf("%s=%s", k, v))
		default:
			q = append(q, fmt.Sprintf("%s=%s", k, url.QueryEscape(v)))
		}
	}
	URL.RawQuery = strings.Join(q, "&")
	URL.Fragment = remark
	return URL.String()
}

func getTransportParams(t interface{}) map[string]string {
	params := map[string]string{}
	trasport, _ := t.(map[string]interface{})
	if transportType, ok := trasport["type"].(string); ok {
		params["type"] = transportType
	} else {
		params["type"] = "tcp"
		return params
	}
	switch params["type"] {
	case "http":
		if host, ok := trasport["host"].([]interface{}); ok {
			var hosts []string
			for _, v := range host {
				hosts = append(hosts, v.(string))
			}
			params["host"] = strings.Join(hosts, ",")
		}
		if path, ok := trasport["path"].(string); ok {
			params["path"] = path
		}
	case "ws":
		if path, ok := trasport["path"].(string); ok {
			params["path"] = path
		}
		if headers, ok := trasport["headers"].(map[string]interface{}); ok {
			if host, ok := headers["Host"].(string); ok {
				params["host"] = host
			}
		}
	case "grpc":
		if serviceName, ok := trasport["service_name"].(string); ok {
			params["serviceName"] = serviceName
		}
	case "httpupgrade":
		if host, ok := trasport["host"].(string); ok {
			params["host"] = host
		}
		if path, ok := trasport["path"].(string); ok {
			params["path"] = path
		}
	}
	return params
}

func getTlsParams(params *map[string]string, tls map[string]interface{}, insecureKey string) {
	if reality, ok := tls["reality"].(map[string]interface{}); ok && reality["enabled"].(bool) {
		(*params)["security"] = "reality"
		if pbk, ok := reality["public_key"].(string); ok {
			(*params)["pbk"] = pbk
		}
		if sid, ok := reality["short_id"].(string); ok {
			(*params)["sid"] = sid
		}
	} else {
		(*params)["security"] = "tls"
		if insecure, ok := tls["insecure"].(bool); ok && insecure {
			(*params)[insecureKey] = "1"
		}
		if disableSni, ok := tls["disable_sni"].(bool); ok && disableSni {
			(*params)["disable_sni"] = "1"
		}
	}
	if utls, ok := tls["utls"].(map[string]interface{}); ok {
		(*params)["fp"], _ = utls["fingerprint"].(string)
	}
	if sni, ok := tls["server_name"].(string); ok {
		(*params)["sni"] = sni
	}
	if alpn, ok := tls["alpn"].([]interface{}); ok {
		alpnList := make([]string, len(alpn))
		for i, v := range alpn {
			alpnList[i] = v.(string)
		}
		(*params)["alpn"] = strings.Join(alpnList, ",")
	}
}
