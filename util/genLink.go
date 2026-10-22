package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

var InboundTypeWithLink = []string{"socks", "http", "mixed", "shadowsocks", "naive", "hysteria", "hysteria2", "anytls", "tuic", "vless", "trojan", "vmess"}

type LinkParam struct {
	Key   string
	Value string
}

// JoinRemark prefixes a node name with the client's remark, so every
// subscriber sees their own alias on the nodes they get.
func JoinRemark(clientRemark, inboundRemark string) string {
	if clientRemark != "" {
		return clientRemark + "-" + inboundRemark
	}
	return inboundRemark
}

func LinkGenerator(clientConfig json.RawMessage, i *model.Inbound, hostname string, clientRemark string) []string {
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
	if err := json.Unmarshal(i.Addrs, &Addrs); err != nil {
		return []string{}
	}
	if len(Addrs) == 0 {
		Addrs = append(Addrs, map[string]interface{}{
			"server":      hostname,
			"server_port": (*inbound)["listen_port"],
			"remark":      JoinRemark(clientRemark, i.Tag),
		})
		if i.TlsId > 0 {
			Addrs[0]["tls"] = tls
		}
	} else {
		for index, addr := range Addrs {
			addrRemark, _ := addr["remark"].(string)
			Addrs[index]["remark"] = JoinRemark(clientRemark, i.Tag+addrRemark)
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

	for index := range Addrs {
		if server, ok := Addrs[index]["server"].(string); ok {
			Addrs[index]["server"] = NormalizeHost(server)
		}
	}

	switch i.Type {
	case "socks":
		return socksLink(userConfig["socks"], Addrs)
	case "http":
		return httpLink(userConfig["http"], Addrs)
	case "mixed":
		return append(
			socksLink(userConfig["socks"], Addrs),
			httpLink(userConfig["http"], Addrs)...,
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
	if err := json.Unmarshal(t.Client, &oTls); err != nil {
		return nil
	}
	if err := json.Unmarshal(t.Server, &iTls); err != nil {
		return nil
	}

	if oTls["certificate_public_key_sha256"] != nil {
		oTls["pinSHA256"] = CertSha256Hex(CertPEMFromTLS(iTls))
	}

	for k, v := range iTls {
		switch k {
		case "enabled", "server_name", "alpn":
			oTls[k] = v
		case "reality":
			reality, okServer := v.(map[string]interface{})
			clientReality, okClient := oTls["reality"].(map[string]interface{})
			if !okServer || !okClient {
				continue
			}
			clientReality["enabled"] = reality["enabled"]
			if shortIDs, hasSIds := reality["short_id"].([]interface{}); hasSIds && len(shortIDs) > 0 {
				clientReality["short_id"] = shortIDs[common.RandomInt(len(shortIDs))]
			}
			oTls["reality"] = clientReality
		}
	}
	return oTls
}

func socksLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	user := stringOr(userConfig["username"], "")
	pass := stringOr(userConfig["password"], "")
	var links []string
	for _, addr := range addrs {
		port, _ := addr["server_port"].(float64)
		// The remark was dropped here, so every socks link arrived unnamed
		// while every other protocol carried one.
		links = append(links, linkURL("socks5", url.UserPassword(user, pass),
			stringOr(addr["server"], ""), port, nil, stringOr(addr["remark"], "")))
	}
	return links
}

func httpLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	user := stringOr(userConfig["username"], "")
	pass := stringOr(userConfig["password"], "")
	var links []string
	for _, addr := range addrs {
		// Decided per address, not carried over: one TLS-enabled address used
		// to make every address after it https, whatever its own setting.
		protocol := "http"
		if addr["tls"] != nil {
			protocol = "https"
		}
		port, _ := addr["server_port"].(float64)
		links = append(links, linkURL(protocol, url.UserPassword(user, pass),
			stringOr(addr["server"], ""), port, nil, stringOr(addr["remark"], "")))
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
	pass, _ = userConfig[ShadowsocksClientConfigKey(method)]["password"].(string)
	userPass = append(userPass, pass)

	// SIP002 specifies base64url without padding for the userinfo. Standard
	// base64 emits + / and =, which several clients reject outright.
	userInfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + strings.Join(userPass, ":")))

	var plugin, pluginOpts string
	if raw, ok := inbound["out_json"].(json.RawMessage); ok {
		var outJson map[string]interface{}
		if json.Unmarshal(raw, &outJson) == nil {
			plugin, _ = outJson["plugin"].(string)
			pluginOpts, _ = outJson["plugin_opts"].(string)
		}
	}

	var links []string
	for _, addr := range addrs {
		port, _ := addr["server_port"].(float64)
		var params []LinkParam
		if plugin != "" {
			pluginVal := plugin
			if pluginOpts != "" {
				pluginVal += ";" + pluginOpts
			}
			params = append(params, LinkParam{"plugin", pluginVal})
		}
		// Through url.URL, so a remark containing a space or a # is escaped
		// rather than concatenated straight into the fragment.
		u := url.URL{
			Scheme:   "ss",
			Host:     fmt.Sprintf("%s:%.0f", HostForURI(stringOr(addr["server"], "")), port),
			Fragment: stringOr(addr["remark"], ""),
		}
		u.RawQuery = encodeParams(params)
		// url.URL would re-escape a pre-encoded userinfo, so the SIP002 blob
		// is spliced in after the scheme.
		links = append(links, strings.Replace(u.String(), "ss://", "ss://"+userInfo+"@", 1))
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
		var params []LinkParam
		params = append(params, LinkParam{"padding", "1"})
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if sni, ok := tls["server_name"].(string); ok {
				params = append(params, LinkParam{"peer", sni})
			}
			if alpn, ok := tls["alpn"].([]interface{}); ok {
				alpnList := make([]string, 0, len(alpn))
				for _, v := range alpn {
					if a, ok := v.(string); ok {
						alpnList = append(alpnList, a)
					}
				}
				params = append(params, LinkParam{"alpn", strings.Join(alpnList, ",")})
			}
			if insecure, ok := tls["insecure"].(bool); ok && insecure {
				params = append(params, LinkParam{"insecure", "1"})
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"tfo", "1"})
		} else {
			params = append(params, LinkParam{"tfo", "0"})
		}

		port, _ := addr["server_port"].(float64)
		uri := baseUri + toBase64([]byte(fmt.Sprintf("%s:%s@%s:%.0f", username, password,
			HostForURI(stringOr(addr["server"], "")), port)))
		links = append(links, addParams(uri, params, stringOr(addr["remark"], "")))

		network, _ := inbound["network"].(string)
		var schemes []string
		switch network {
		case "tcp":
			schemes = []string{"naive+https"}
		case "udp":
			schemes = []string{"naive+quic"}
		default:
			schemes = []string{"naive+https", "naive+quic"}
		}
		for _, scheme := range schemes {
			links = append(links, linkURL(scheme, url.UserPassword(username, password),
				stringOr(addr["server"], ""), port, params, stringOr(addr["remark"], "")))
		}
	}
	return links
}

// portHoppingParam reads the multi-port range for hysteria.
//
// It used to assert on inbound["out_json"] and bail out of the whole function
// when the unmarshal failed. An inbound with no out_json -- which is the normal
// state until the operator sets one -- therefore produced no links at all for
// hysteria and hysteria2, silently, and the subscription simply came back
// short. The port entries were asserted to be strings as well, while sing-box
// writes them as numbers when they are single ports.
func portHoppingParam(inbound map[string]interface{}) string {
	raw, ok := inbound["out_json"].(json.RawMessage)
	if !ok || len(raw) == 0 {
		return ""
	}
	var outJson map[string]interface{}
	if err := json.Unmarshal(raw, &outJson); err != nil {
		logger.Warning("sub: unable to read out_json for port hopping: ", err)
		return ""
	}
	ports, ok := outJson["server_ports"].([]interface{})
	if !ok || len(ports) == 0 {
		return ""
	}
	list := make([]string, 0, len(ports))
	for _, v := range ports {
		switch p := v.(type) {
		case string:
			list = append(list, p)
		case float64:
			list = append(list, fmt.Sprintf("%.0f", p))
		}
	}
	return strings.Join(list, ",")
}

func hysteriaLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if auth, ok := userConfig["auth_str"].(string); ok {
			params = append(params, LinkParam{"auth", auth})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "hysteria")
		}
		if obfs, ok := inbound["obfs"].(string); ok {
			params = append(params, LinkParam{"obfs", obfs})
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		if mport := portHoppingParam(inbound); mport != "" {
			params = append(params, LinkParam{"mport", mport})
		}

		port, _ := addr["server_port"].(float64)
		links = append(links, linkURL("hysteria", nil,
			stringOr(addr["server"], ""), port, params, stringOr(addr["remark"], "")))
	}

	return links
}

func hysteria2Link(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "hysteria2")
		}
		if obfs, ok := inbound["obfs"].(map[string]interface{}); ok {
			if obfsType, ok := obfs["type"].(string); ok {
				params = append(params, LinkParam{"obfs", obfsType})
			}
			if obfsPassword, ok := obfs["password"].(string); ok {
				params = append(params, LinkParam{"obfs-password", obfsPassword})
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		if mport := portHoppingParam(inbound); mport != "" {
			params = append(params, LinkParam{"mport", mport})
		}

		port, _ := addr["server_port"].(float64)
		links = append(links, linkURL("hysteria2", url.User(password),
			stringOr(addr["server"], ""), port, params, stringOr(addr["remark"], "")))
	}

	return links
}

func anytlsLink(
	userConfig map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	var links []string

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "anytls")
		}

		port, _ := addr["server_port"].(float64)
		links = append(links, linkURL("anytls", url.User(password),
			stringOr(addr["server"], ""), port, params, stringOr(addr["remark"], "")))
	}

	return links
}

func tuicLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	uuid, _ := userConfig["uuid"].(string)

	var links []string

	// udp_relay_mode is a client-side (outbound) param and lives in out_json
	var outJson map[string]interface{}
	if raw, ok := inbound["out_json"].(json.RawMessage); ok {
		_ = json.Unmarshal(raw, &outJson)
	}

	for _, addr := range addrs {
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "tuic")
		}
		if congestionControl, ok := inbound["congestion_control"].(string); ok {
			params = append(params, LinkParam{"congestion_control", congestionControl})
		}
		if udpRelayMode, ok := outJson["udp_relay_mode"].(string); ok && udpRelayMode != "" {
			params = append(params, LinkParam{"udp_relay_mode", udpRelayMode})
		}

		port, _ := addr["server_port"].(float64)
		links = append(links, linkURL("tuic", url.UserPassword(uuid, password),
			stringOr(addr["server"], ""), port, params, stringOr(addr["remark"], "")))
	}

	return links
}

func vlessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	baseParams := getTransportParams(inbound["transport"])
	isTcp := false
	if len(baseParams) == 1 && baseParams[0].Value == "tcp" {
		isTcp = true
	}
	var links []string

	for _, addr := range addrs {
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok && boolOr(tls["enabled"]) {
			getTlsParams(&params, tls, "vless")
			if flow, ok := userConfig["flow"].(string); ok && isTcp {
				params = append(params, LinkParam{"flow", flow})
			}
		}
		port, _ := addr["server_port"].(float64)
		links = append(links, linkURL("vless", url.User(uuid),
			stringOr(addr["server"], ""), port, params, stringOr(addr["remark"], "")))
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
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok && boolOr(tls["enabled"]) {
			getTlsParams(&params, tls, "trojan")
		}
		port, _ := addr["server_port"].(float64)
		links = append(links, linkURL("trojan", url.User(password),
			stringOr(addr["server"], ""), port, params, stringOr(addr["remark"], "")))
	}

	return links
}

func vmessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	transportParams := getTransportParams(inbound["transport"])
	var links []string

	baseParams := map[string]interface{}{
		"v":   "2",
		"id":  uuid,
		"aid": 0,
	}

	var net, typ, host, path string
	for _, p := range transportParams {
		switch p.Key {
		case "type":
			net = p.Value
		case "host":
			host = p.Value
		case "path":
			path = p.Value
		case "serviceName":
			// The vmess JSON has no serviceName field; grpc carries the
			// service name in "path". Dropping it meant every grpc vmess link
			// pointed at the default service and simply did not connect.
			if path == "" {
				path = p.Value
			}
		}
	}

	if net == "http" || net == "tcp" {
		baseParams["net"] = "tcp"
		if net == "http" {
			typ = "http"
		}
	} else {
		baseParams["net"] = net
	}

	for _, addr := range addrs {
		obj := make(map[string]interface{})
		for k, v := range baseParams {
			obj[k] = v
		}

		obj["add"], _ = addr["server"].(string)
		port, _ := addr["server_port"].(float64)
		obj["port"] = fmt.Sprintf("%.0f", port)
		obj["ps"], _ = addr["remark"].(string)
		if typ != "" {
			obj["type"] = typ
		}
		if host != "" {
			obj["host"] = host
		}
		if path != "" {
			obj["path"] = path
		}
		populateVmessTlsParams(obj, addr["tls"])

		jsonStr, _ := json.Marshal(obj)

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func populateVmessTlsParams(obj map[string]interface{}, tlsConfig interface{}) {
	if tlsMap, ok := tlsConfig.(map[string]interface{}); ok && boolOr(tlsMap["enabled"]) {
		obj["tls"] = "tls"
		var tlsParams []LinkParam
		getTlsParams(&tlsParams, tlsMap, "vmess")
		for _, p := range tlsParams {
			switch p.Key {
			case "security":
				// ignore, as "tls" is already set
			case "allowInsecure":
				obj["allowInsecure"] = 1
			case "sni":
				obj["sni"] = p.Value
			case "fp":
				obj["fp"] = p.Value
			case "alpn":
				obj["alpn"] = p.Value
			}
		}
	} else {
		obj["tls"] = "none"
	}
}

func toBase64(d []byte) string {
	return base64.StdEncoding.EncodeToString(d)
}

// encodeParams renders the query. mport and alpn keep their commas, which the
// client parsers expect unescaped.
func encodeParams(params []LinkParam) string {
	var q []string
	for _, p := range params {
		switch p.Key {
		case "mport", "alpn":
			q = append(q, fmt.Sprintf("%s=%s", p.Key, p.Value))
		default:
			q = append(q, fmt.Sprintf("%s=%s", p.Key, url.QueryEscape(p.Value)))
		}
	}
	return strings.Join(q, "&")
}

// linkURL assembles a link from its parts rather than formatting a string and
// parsing it back.
//
// This is the fix for the whole family of crashes: a password with a space, a
// %, a #, a /, a : or any non-ASCII character produced a string url.Parse
// refused, addParams dropped that error, and the nil *URL was dereferenced on
// the next line. One client whose password contained a space took link
// generation down for every client on the panel. url.URL escapes the userinfo
// itself, so there is nothing left to get wrong.
func linkURL(scheme string, user *url.Userinfo, host string, port float64, params []LinkParam, remark string) string {
	u := url.URL{
		Scheme:   scheme,
		User:     user,
		Host:     fmt.Sprintf("%s:%.0f", HostForURI(host), port),
		Fragment: remark,
	}
	u.RawQuery = encodeParams(params)
	return u.String()
}

// addParams is the path for links whose authority is already encoded, such as
// the base64 payload of ss:// and http2://.
func addParams(uri string, params []LinkParam, remark string) string {
	URL, err := url.Parse(uri)
	if err != nil || URL == nil {
		// Reported rather than dereferenced. Every caller that could reach
		// here with an unparseable string now builds through linkURL instead,
		// so this is a backstop -- but it used to be the crash itself.
		logger.Warning("sub: unable to parse a generated link, returning it unadorned: ", err)
		if q := encodeParams(params); q != "" {
			uri += "?" + q
		}
		if remark != "" {
			uri += "#" + url.PathEscape(remark)
		}
		return uri
	}
	URL.RawQuery = encodeParams(params)
	URL.Fragment = remark
	return URL.String()
}

// boolOr reads a map value the schema says is a bool. A config written by hand
// or carried over from an older version can leave it absent, and the bare
// assertion these replaced panicked on that.
func boolOr(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

// stringOr reads a map value that the database says is a string but may not be.
func stringOr(v interface{}, fallback string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

func getTransportParams(t interface{}) []LinkParam {
	var params []LinkParam
	trasport, _ := t.(map[string]interface{})
	var transportType string
	if tt, ok := trasport["type"].(string); ok {
		transportType = tt
	} else {
		transportType = "tcp"
	}
	params = append(params, LinkParam{"type", transportType})
	if transportType == "tcp" {
		return params
	}

	switch transportType {
	case "http":
		if host, ok := trasport["host"].([]interface{}); ok {
			var hosts []string
			for _, v := range host {
				if h, ok := v.(string); ok {
					hosts = append(hosts, h)
				}
			}
			params = append(params, LinkParam{"host", strings.Join(hosts, ",")})
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	case "ws":
		if path, ok := trasport["path"].(string); ok {
			if maxED, ok := trasport["max_early_data"].(float64); ok && maxED > 0 {
				if edName, _ := trasport["early_data_header_name"].(string); edName == "Sec-WebSocket-Protocol" {
					sep := "?"
					if strings.Contains(path, "?") {
						sep = "&"
					}
					path = fmt.Sprintf("%s%sed=%d", path, sep, int(maxED))
				}
			}
			params = append(params, LinkParam{"path", path})
		}
		if headers, ok := trasport["headers"].(map[string]interface{}); ok {
			if host, ok := headers["Host"].(string); ok {
				params = append(params, LinkParam{"host", host})
			}
		}
	case "grpc":
		if serviceName, ok := trasport["service_name"].(string); ok {
			params = append(params, LinkParam{"serviceName", serviceName})
		}
	case "httpupgrade":
		if host, ok := trasport["host"].(string); ok {
			params = append(params, LinkParam{"host", host})
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	}
	return params
}

func getTlsParams(params *[]LinkParam, tls map[string]interface{}, protocol string) {
	if reality, ok := tls["reality"].(map[string]interface{}); ok && boolOr(reality["enabled"]) {
		*params = append(*params, LinkParam{"security", "reality"})
		if pbk, ok := reality["public_key"].(string); ok {
			*params = append(*params, LinkParam{"pbk", pbk})
		}
		if sid, ok := reality["short_id"].(string); ok {
			*params = append(*params, LinkParam{"sid", sid})
		}
	} else {
		*params = append(*params, LinkParam{"security", "tls"})
		if insecure, ok := tls["insecure"].(bool); ok && insecure {
			*params = append(*params, LinkParam{insecureKeyFor(protocol), "1"})
		}
		if pin, ok := tls["pinSHA256"].(string); ok && pin != "" {
			*params = append(*params, LinkParam{pcsKeyFor(protocol), pin})
		}
		if disableSni, ok := tls["disable_sni"].(bool); ok && disableSni {
			*params = append(*params, LinkParam{"disable_sni", "1"})
		}
	}
	if utls, ok := tls["utls"].(map[string]interface{}); ok {
		if fingerprint, ok := utls["fingerprint"].(string); ok {
			*params = append(*params, LinkParam{"fp", fingerprint})
		}
	}
	if sni, ok := tls["server_name"].(string); ok {
		*params = append(*params, LinkParam{"sni", sni})
	}
	if alpn, ok := tls["alpn"].([]interface{}); ok {
		alpnList := make([]string, 0, len(alpn))
		for _, v := range alpn {
			if a, ok := v.(string); ok {
				alpnList = append(alpnList, a)
			}
		}
		*params = append(*params, LinkParam{"alpn", strings.Join(alpnList, ",")})
	}
}

func insecureKeyFor(protocol string) string {
	switch protocol {
	case "vless", "trojan", "vmess":
		return "allowInsecure"
	}
	return "insecure"
}

func pcsKeyFor(protocol string) string {
	switch protocol {
	case "hysteria", "hysteria2":
		return "pinSHA256"
	}
	return "pcs"
}
