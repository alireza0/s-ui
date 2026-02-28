package util

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/alireza0/s-ui/util/common"
)

func GetOutbound(uri string, i int) (*map[string]interface{}, string, error) {
	u, err := url.Parse(uri)
	if err == nil {
		switch u.Scheme {
		case "vmess":
			return vmess(u.Host, i)
		case "vless":
			return vless(u, i)
		case "trojan":
			return trojan(u, i)
		case "hy", "hysteria":
			return hy(u, i)
		case "hy2", "hysteria2":
			return hy2(u, i)
		case "anytls":
			return anytls(u, i)
		case "tuic":
			return tuic(u, i)
		case "ss", "shadowsocks":
			return ss(u, i)
		case "naive+https", "naive+quic", "http2":
			return parseNaiveLink(u, i)
		}
	}
	return nil, "", common.NewError("Unsupported link format")
}

func vmess(data string, i int) (*map[string]interface{}, string, error) {
	dataByte, err := B64StrToByte(data)
	if err != nil {
		return nil, "", err
	}
	var dataJson map[string]interface{}
	err = json.Unmarshal(dataByte, &dataJson)
	if err != nil {
		return nil, "", err
	}
	transport := map[string]interface{}{}
	tp_net, _ := dataJson["net"].(string)
	tp_type, _ := dataJson["type"].(string)
	tp_host, _ := dataJson["host"].(string)
	tp_path, _ := dataJson["path"].(string)
	switch strings.ToLower(tp_net) {
	case "tcp", "":
		if tp_type == "http" {
			transport["type"] = tp_type
			if len(tp_host) > 0 {
				transport["host"] = strings.Split(tp_host, ",")
			}
			transport["path"] = tp_path
		}
	case "http", "h2":
		transport["type"] = "http"
		if len(tp_host) > 0 {
			transport["host"] = strings.Split(tp_host, ",")
		}
		transport["path"] = tp_path
	case "ws":
		transport["type"] = tp_net
		transport["path"] = tp_path
		transport["early_data_header_name"] = "Sec-WebSocket-Protocol"
		if len(tp_host) > 0 {
			transport["headers"] = map[string]interface{}{
				"Host": tp_host,
			}
		}
	case "quic":
		transport["type"] = tp_net
	case "grpc":
		transport["type"] = tp_net
		transport["service_name"] = tp_path
	case "httpupgrade":
		transport["type"] = tp_net
		transport["path"] = tp_path
		transport["host"] = tp_host
	default:
		return nil, "", common.NewError("Invalid vmess")
	}
	tls := map[string]interface{}{}
	vmess_tls, _ := dataJson["tls"].(string)
	if vmess_tls == "tls" {
		tls["enabled"] = true
		tls_sni, _ := dataJson["sni"].(string)
		tls_alpn, _ := dataJson["alpn"].(string)
		_, tls_insecure := dataJson["allowInsecure"]
		tls_fp, _ := dataJson["fp"].(string)
		if len(tls_sni) > 0 {
			tls["server_name"] = tls_sni
		}
		if len(tls_alpn) > 0 {
			tls["alpn"] = strings.Split(tls_alpn, ",")
		}
		if tls_insecure {
			tls["insecure"] = true
		}
		if len(tls_fp) > 0 {
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": tls_fp,
			}
		}
	}
	tag, _ := dataJson["ps"].(string)
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, tag)
	}
	alter_id := 0
	if aid, ok := dataJson["aid"].(float64); ok {
		alter_id = int(aid)
	}
	vmess := map[string]interface{}{
		"type":        "vmess",
		"tag":         tag,
		"server":      dataJson["add"],
		"server_port": dataJson["port"],
		"uuid":        dataJson["id"],
		"security":    "auto",
		"alter_id":    alter_id,
		"tls":         tls,
		"transport":   transport,
	}
	return &vmess, tag, err
}

func vless(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query, _ := url.ParseQuery(u.RawQuery)
	security := query.Get("security")
	host, portStr, _ := net.SplitHostPort(u.Host)
	port := 80
	if len(portStr) > 0 {
		port, _ = strconv.Atoi(portStr)
	} else {
		if security == "tls" || security == "reality" {
			port = 443
		}
	}
	tp_type := query.Get("type")
	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	vless := map[string]interface{}{
		"type":        "vless",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"uuid":        u.User.Username(),
		"flow":        query.Get("flow"),
		"tls":         getTls(security, &query),
		"transport":   getTransport(tp_type, &query),
	}
	return &vless, tag, nil
}

func trojan(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query, _ := url.ParseQuery(u.RawQuery)
	security := query.Get("security")
	host, portStr, _ := net.SplitHostPort(u.Host)
	port := 80
	if len(portStr) > 0 {
		port, _ = strconv.Atoi(portStr)
	} else {
		if security == "tls" || security == "reality" {
			port = 443
		}
	}
	tp_type := query.Get("type")
	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	trojan := map[string]interface{}{
		"type":        "trojan",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    u.User.Username(),
		"tls":         getTls(security, &query),
		"transport":   getTransport(tp_type, &query),
	}
	return &trojan, tag, nil
}

func hy(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query, _ := url.ParseQuery(u.RawQuery)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port := 443
	if len(portStr) > 0 {
		port, _ = strconv.Atoi(portStr)
	}

	security := query.Get("security")
	if len(security) == 0 {
		security = "tls"
	}

	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	hy := map[string]interface{}{
		"type":        "hysteria",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"obfs":        query.Get("obfsParam"),
		"auth_str":    query.Get("auth"),
		"tls":         getTls(security, &query),
	}
	down, _ := strconv.Atoi(query.Get("downmbps"))
	up, _ := strconv.Atoi(query.Get("upmbps"))
	recv_window_conn, _ := strconv.Atoi(query.Get("recv_window_conn"))
	recv_window, _ := strconv.Atoi(query.Get("recv_window"))
	if down > 0 {
		hy["down_mbps"] = down
	}
	if up > 0 {
		hy["up_mbps"] = up
	}
	if recv_window_conn > 0 {
		hy["recv_window_conn"] = recv_window_conn
	}
	if recv_window > 0 {
		hy["recv_window"] = recv_window
	}
	return &hy, tag, nil
}

func hy2(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query, _ := url.ParseQuery(u.RawQuery)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port := 443
	if len(portStr) > 0 {
		port, _ = strconv.Atoi(portStr)
	}

	security := query.Get("security")
	if len(security) == 0 {
		security = "tls"
	}

	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	hy2 := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    u.User.Username(),
		"tls":         getTls(security, &query),
	}
	down, _ := strconv.Atoi(query.Get("downmbps"))
	up, _ := strconv.Atoi(query.Get("upmbps"))
	obfs := query.Get("obfs")
	mport := query.Get("mport")
	fastopen := query.Get("fastopen")
	if down > 0 {
		hy2["down_mbps"] = down
	}
	if up > 0 {
		hy2["up_mbps"] = up
	}
	if obfs == "salamander" {
		hy2["obfs"] = map[string]interface{}{
			"type":     "salamander",
			"password": query.Get("obfs-password"),
		}
	}
	if len(mport) > 0 {
		hy2["server_ports"] = strings.Split(mport, ",")
	}
	if fastopen == "1" || fastopen == "true" {
		hy2["fastopen"] = true
	}
	return &hy2, tag, nil
}

func anytls(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query, _ := url.ParseQuery(u.RawQuery)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port := 443
	if len(portStr) > 0 {
		port, _ = strconv.Atoi(portStr)
	}

	security := query.Get("security")
	if len(security) == 0 {
		security = "tls"
	}

	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	anytls := map[string]interface{}{
		"type":        "anytls",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"password":    u.User.Username(),
		"tls":         getTls(security, &query),
	}
	return &anytls, tag, nil
}

func tuic(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query, _ := url.ParseQuery(u.RawQuery)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port := 443
	if len(portStr) > 0 {
		port, _ = strconv.Atoi(portStr)
	}

	security := query.Get("security")
	if len(security) == 0 {
		security = "tls"
	}

	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	password, _ := u.User.Password()
	tuic := map[string]interface{}{
		"type":               "tuic",
		"tag":                tag,
		"server":             host,
		"server_port":        port,
		"uuid":               u.User.Username(),
		"password":           password,
		"congestion_control": query.Get("congestion_control"),
		"udp_relay_mode":     query.Get("udp_relay_mode"),
		"tls":                getTls(security, &query),
	}
	return &tuic, tag, nil
}

func ss(u *url.URL, i int) (*map[string]interface{}, string, error) {
	query, _ := url.ParseQuery(u.RawQuery)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port := 443
	if len(portStr) > 0 {
		port, _ = strconv.Atoi(portStr)
	}
	method := u.User.Username()
	password, ok := u.User.Password()
	if !ok {
		decrypted := StrOrBase64Encoded(method)
		decrypted_arr := strings.Split(decrypted, ":")
		if len(decrypted_arr) > 1 {
			method = decrypted_arr[0]
			password = strings.Join(decrypted_arr[1:], ":")
		} else {
			return nil, "", common.NewError("Unsupported shadowsocks")
		}
	}

	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	ss := map[string]interface{}{
		"type":        "shadowsocks",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"method":      method,
		"password":    password,
	}

	v2ray_type := query.Get("type")
	if len(v2ray_type) > 0 {
		pl_arr := []string{}
		host_header := query.Get("host")
		if query.Get("security") == "tls" {
			pl_arr = append(pl_arr, "tls")
		}
		if v2ray_type == "quic" {
			pl_arr = append(pl_arr, "mode=quic")
		}
		if len(host_header) > 0 {
			pl_arr = append(pl_arr, "host="+host_header)
		}
		ss["plugin"] = "v2ray-plugin"
		ss["plugin_opts"] = strings.Join(pl_arr, ";")
	}
	plugin := query.Get("plugin")
	if len(plugin) > 0 {
		pl_arr := strings.Split(plugin, ";")
		if len(pl_arr) > 0 {
			ss["plugin"] = pl_arr[0]
			ss["plugin_opts"] = strings.Join(pl_arr[1:], ";")
		}
	}
	return &ss, tag, nil
}

func parseNaiveLink(u *url.URL, i int) (*map[string]interface{}, string, error) {
	var host, portStr, username, password string
	var port int

	switch u.Scheme {
	case "http2":
		decoded := StrOrBase64Encoded(u.Hostname())
		if idx := strings.Index(decoded, "@"); idx != -1 {
			userInfo := decoded[:idx]
			hostPort := decoded[idx+1:]
			if idx2 := strings.Index(userInfo, ":"); idx2 != -1 {
				username = userInfo[:idx2]
				password = userInfo[idx2+1:]
			} else {
				username = userInfo
			}
			host, portStr, _ = net.SplitHostPort(hostPort)
			if portStr != "" {
				port, _ = strconv.Atoi(portStr)
			} else {
				port = 443
			}
		} else {
			return nil, "", common.NewError("Invalid naive link (http2)")
		}
	case "naive+https", "naive+quic":
		host, portStr, _ = net.SplitHostPort(u.Host)
		if portStr != "" {
			port, _ = strconv.Atoi(portStr)
		} else {
			port = 443
		}
		if u.User != nil {
			username = u.User.Username()
			password, _ = u.User.Password()
		}
	default:
		return nil, "", common.NewError("Unsupported naive scheme")
	}

	tag := u.Fragment
	if i > 0 {
		tag = fmt.Sprintf("%d.%s", i, u.Fragment)
	}
	if tag == "" {
		tag = fmt.Sprintf("naive-%d", i)
	}

	naive := map[string]interface{}{
		"type":        "naive",
		"tag":         tag,
		"server":      host,
		"server_port": port,
		"username":    username,
		"password":    password,
		"tls":         map[string]interface{}{"enabled": true},
	}

	query := u.Query()
	if peer := query.Get("peer"); peer != "" {
		if tls, ok := naive["tls"].(map[string]interface{}); ok {
			tls["server_name"] = peer
		}
	}
	if insecure := query.Get("insecure"); insecure == "1" || insecure == "true" {
		if tls, ok := naive["tls"].(map[string]interface{}); ok {
			tls["insecure"] = true
		}
	}
	if alpn := query.Get("alpn"); alpn != "" {
		if tls, ok := naive["tls"].(map[string]interface{}); ok {
			tls["alpn"] = strings.Split(alpn, ",")
		}
	}
	if u.Scheme == "naive+quic" {
		naive["quic"] = true
	}

	return &naive, tag, nil
}

func getTransport(tp_type string, q *url.Values) map[string]interface{} {
	transport := map[string]interface{}{}
	tp_host := q.Get("host")
	tp_path := q.Get("path")
	switch strings.ToLower(tp_type) {
	case "tcp", "":
		if q.Get("headerType") == "http" {
			transport["type"] = "http"
			if len(tp_host) > 0 {
				transport["host"] = strings.Split(tp_host, ",")
			}
			transport["path"] = tp_path
		}
	case "http", "h2":
		transport["type"] = "http"
		if len(tp_host) > 0 {
			transport["host"] = strings.Split(tp_host, ",")
		}
		transport["path"] = tp_path
	case "ws":
		transport["type"] = "ws"
		transport["path"] = tp_path
		if len(tp_host) > 0 {
			transport["headers"] = map[string]interface{}{
				"Host": tp_host,
			}
		}
	case "quic":
		transport["type"] = "quic"
	case "grpc":
		transport["type"] = "grpc"
		transport["service_name"] = q.Get("serviceName")
	case "httpupgrade":
		transport["type"] = "httpupgrade"
		transport["path"] = tp_path
		transport["host"] = tp_host
	}
	return transport
}

func getTls(security string, q *url.Values) map[string]interface{} {
	tls := map[string]interface{}{}
	tls_fp := q.Get("fp")
	tls_sni := q.Get("sni")
	tls_allow_insecure := q.Get("allowInsecure")
	tls_insecure := q.Get("insecure")
	tls_alpn := q.Get("alpn")
	tls_ech := q.Get("ech")
	disable_sni := q.Get("disable_sni")
	switch security {
	case "tls":
		tls["enabled"] = true
	case "reality":
		tls["enabled"] = true
		tls["reality"] = map[string]interface{}{
			"enabled":    true,
			"public_key": q.Get("pbk"),
			"short_id":   q.Get("sid"),
		}
	}
	if len(tls_sni) > 0 {
		tls["server_name"] = tls_sni
	}
	if len(tls_alpn) > 0 {
		tls["alpn"] = strings.Split(tls_alpn, ",")
	}
	if tls_insecure == "1" || tls_insecure == "true" || tls_allow_insecure == "1" || tls_allow_insecure == "true" {
		tls["insecure"] = true
	}
	if len(tls_fp) > 0 {
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": tls_fp,
		}
	}
	if len(tls_ech) > 0 {
		tls["ech"] = map[string]interface{}{
			"enabled": true,
			"config": []string{
				tls_ech,
			},
		}
	}
	if disable_sni == "1" || disable_sni == "true" {
		tls["disable_sni"] = true
	}
	return tls
}
