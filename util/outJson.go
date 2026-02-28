package util

import (
	"encoding/json"

	"github.com/alireza0/s-ui/util/common"

	"github.com/alireza0/s-ui/database/model"
)

// Fill Inbound's out_json
func FillOutJson(i *model.Inbound, hostname string) error {
	switch i.Type {
	case "direct", "tun", "redirect", "tproxy":
		return nil
	}
	var outJson map[string]interface{}
	err := json.Unmarshal(i.OutJson, &outJson)
	if err != nil {
		return err
	}

	if outJson == nil {
		outJson = make(map[string]interface{})
	}

	if i.TlsId > 0 {
		addTls(&outJson, i.Tls)
	} else {
		delete(outJson, "tls")
	}

	inbound, err := i.MarshalFull()
	if err != nil {
		return err
	}

	outJson["type"] = i.Type
	outJson["tag"] = i.Tag
	outJson["server"] = hostname
	outJson["server_port"] = (*inbound)["listen_port"]

	switch i.Type {
	case "http", "socks", "mixed", "anytls":
	case "naive":
		naiveOut(&outJson, *inbound)
	case "shadowsocks":
		shadowsocksOut(&outJson, *inbound)
	case "shadowtls":
		shadowTlsOut(&outJson, *inbound)
	case "hysteria":
		hysteriaOut(&outJson, *inbound)
	case "hysteria2":
		hysteria2Out(&outJson, *inbound)
	case "tuic":
		tuicOut(&outJson, *inbound)
	case "vless":
		vlessOut(&outJson, *inbound)
	case "trojan":
		trojanOut(&outJson, *inbound)
	case "vmess":
		vmessOut(&outJson, *inbound)
	default:
		for key := range outJson {
			delete(outJson, key)
		}
	}

	i.OutJson, err = json.MarshalIndent(outJson, "", "  ")
	if err != nil {
		return err
	}

	return nil
}

// addTls function
func addTls(out *map[string]interface{}, tls *model.Tls) {
	var tlsServer, tlsConfig map[string]interface{}
	err := json.Unmarshal(tls.Server, &tlsServer)
	if err != nil {
		return
	}
	err = json.Unmarshal(tls.Client, &tlsConfig)
	if err != nil {
		return
	}

	if enabled, ok := tlsServer["enabled"]; ok {
		tlsConfig["enabled"] = enabled
	}
	if serverName, ok := tlsServer["server_name"]; ok {
		tlsConfig["server_name"] = serverName
	}
	if alpn, ok := tlsServer["alpn"]; ok {
		tlsConfig["alpn"] = alpn
	}
	if minVersion, ok := tlsServer["min_version"]; ok {
		tlsConfig["min_version"] = minVersion
	}
	if maxVersion, ok := tlsServer["max_version"]; ok {
		tlsConfig["max_version"] = maxVersion
	}
	if certificate, ok := tlsServer["certificate"]; ok {
		tlsConfig["certificate"] = certificate
	}
	if cipherSuites, ok := tlsServer["cipher_suites"]; ok {
		tlsConfig["cipher_suites"] = cipherSuites
	}
	if reality, ok := tlsServer["reality"].(map[string]interface{}); ok && reality["enabled"].(bool) {
		realityConfig := tlsConfig["reality"].(map[string]interface{})
		realityConfig["enabled"] = true
		if shortIDs, ok := reality["short_id"].([]interface{}); ok && len(shortIDs) > 0 {
			realityConfig["short_id"] = shortIDs[common.RandomInt(len(shortIDs))]
		}
		tlsConfig["reality"] = realityConfig
	}
	if ech, ok := tlsServer["ech"].(map[string]interface{}); ok && ech["enabled"].(bool) {
		echConfig := tlsConfig["ech"].(map[string]interface{})
		echConfig["enabled"] = true
		echConfig["pq_signature_schemes_enabled"] = ech["pq_signature_schemes_enabled"]
		echConfig["dynamic_record_sizing_disabled"] = ech["dynamic_record_sizing_disabled"]
		tlsConfig["ech"] = echConfig
	}

	(*out)["tls"] = tlsConfig
}

func naiveOut(out *map[string]interface{}, inbound map[string]interface{}) {
	if quic_congestion_control, ok := inbound["quic_congestion_control"].(string); ok {
		(*out)["quic"] = true
		switch quic_congestion_control {
		case "bbr_standard":
			(*out)["quic_congestion_control"] = "bbr"
		case "bbr2_variant":
			(*out)["quic_congestion_control"] = "bbr2"
		default:
			(*out)["quic_congestion_control"] = quic_congestion_control
		}
	}

}

func shadowsocksOut(out *map[string]interface{}, inbound map[string]interface{}) {
	if method, ok := inbound["method"].(string); ok {
		(*out)["method"] = method
	}
}

func shadowTlsOut(out *map[string]interface{}, inbound map[string]interface{}) {
	if version, ok := inbound["version"].(float64); ok && int(version) == 3 {
		(*out)["version"] = 3
	} else {
		for key := range *out {
			delete(*out, key)
		}
	}
	(*out)["tls"] = map[string]interface{}{"enabled": true}
}

func hysteriaOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "down_mbps")
	delete(*out, "up_mbps")
	delete(*out, "obfs")
	delete(*out, "recv_window_conn")
	delete(*out, "disable_mtu_discovery")

	if upMbps, ok := inbound["down_mbps"]; ok {
		(*out)["up_mbps"] = upMbps
	}
	if downMbps, ok := inbound["up_mbps"]; ok {
		(*out)["down_mbps"] = downMbps
	}
	if obfs, ok := inbound["obfs"]; ok {
		(*out)["obfs"] = obfs
	}
	if recvWindow, ok := inbound["recv_window_conn"]; ok {
		(*out)["recv_window_conn"] = recvWindow
	}
	if disableMTU, ok := inbound["disable_mtu_discovery"]; ok {
		(*out)["disable_mtu_discovery"] = disableMTU
	}
}

func hysteria2Out(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "down_mbps")
	delete(*out, "up_mbps")
	delete(*out, "obfs")

	if upMbps, ok := inbound["down_mbps"]; ok {
		(*out)["up_mbps"] = upMbps
	}
	if downMbps, ok := inbound["up_mbps"]; ok {
		(*out)["down_mbps"] = downMbps
	}
	if obfs, ok := inbound["obfs"]; ok {
		(*out)["obfs"] = obfs
	}
}

func tuicOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "zero_rtt_handshake")
	delete(*out, "heartbeat")
	if congestionControl, ok := inbound["congestion_control"].(string); ok {
		(*out)["congestion_control"] = congestionControl
	} else {
		(*out)["congestion_control"] = "cubic"
	}
	if zeroRTT, ok := inbound["zero_rtt_handshake"].(bool); ok {
		(*out)["zero_rtt_handshake"] = zeroRTT
	}
	if heartbeat, ok := inbound["heartbeat"]; ok {
		(*out)["heartbeat"] = heartbeat
	}
}

func vlessOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "transport")
	if transport, ok := inbound["transport"]; ok {
		(*out)["transport"] = transport
	}
}

func trojanOut(out *map[string]interface{}, inbound map[string]interface{}) {
	delete(*out, "transport")
	if transport, ok := inbound["transport"]; ok {
		(*out)["transport"] = transport
	}
}

func vmessOut(out *map[string]interface{}, inbound map[string]interface{}) {
	(*out)["alter_id"] = 0
	delete(*out, "transport")
	if transport, ok := inbound["transport"]; ok {
		(*out)["transport"] = transport
	}
}
