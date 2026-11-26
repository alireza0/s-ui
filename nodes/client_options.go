package nodes

import (
	"encoding/json"
	"errors"
	"strings"
)

var errClientOptions = errors.New("invalid_node_client_options")

func allowedClientOption(protocol, key string) bool {
	switch key {
	case "addresses", "network", "detour", "bind_interface", "inet4_bind_address", "inet6_bind_address",
		"bind_address_no_port", "routing_mark", "reuse_addr", "connect_timeout",
		"tcp_fast_open", "tcp_multi_path", "disable_tcp_keep_alive", "tcp_keep_alive",
		"tcp_keep_alive_interval", "udp_fragment", "domain_resolver":
		return true
	case "packet_encoding":
		return protocol == "vless" || protocol == "vmess"
	case "multiplex":
		return protocol == "vless" || protocol == "vmess" || protocol == "trojan" || protocol == "shadowsocks"
	case "security", "global_padding", "authenticated_length":
		return protocol == "vmess"
	case "idle_session_check_interval", "idle_session_timeout", "min_idle_session":
		return protocol == "anytls"
	case "udp_relay_mode", "udp_over_stream":
		return protocol == "tuic"
	case "server_ports", "hop_interval":
		return protocol == "hysteria2" || protocol == "hysteria"
	case "stream_receive_window", "recv_window_conn", "disable_mtu_discovery":
		return protocol == "hysteria"
	case "realm":
		return protocol == "hysteria2"
	case "insecure_concurrency", "udp_over_tcp":
		return protocol == "naive"
	case "version":
		return protocol == "socks"
	case "headers", "path":
		return protocol == "http"
	}
	return false
}

func validateNodeClientOptions(entry SpecEntry) error {
	if len(entry.ClientOptions) == 0 {
		return nil
	}
	if _, err := objectJSON(entry.ClientOptions); err != nil {
		return errClientOptions
	}
	var options map[string]json.RawMessage
	if err := json.Unmarshal(entry.ClientOptions, &options); err != nil {
		return errClientOptions
	}
	for key, raw := range options {
		if !allowedClientOption(entry.Protocol, key) {
			return errClientOptions
		}
		var value any
		if err := decodeJSON(raw, &value, false); err != nil || secretLiteral(value) || hasPrivateClientTLS(value) {
			return errClientOptions
		}
	}
	if err := validateClientAddresses(options["addresses"]); err != nil {
		return err
	}
	if raw := options["realm"]; len(raw) != 0 {
		var realm struct {
			Token string `json:"token,omitempty"`
		}
		if err := decodeJSON(raw, &realm, true); err != nil {
			return errClientOptions
		}
	}
	return nil
}

func clientPublication(raw json.RawMessage, host string, port any) (json.RawMessage, json.RawMessage, error) {
	options := map[string]json.RawMessage{}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &options); err != nil {
			return nil, nil, err
		}
	}
	addresses := options["addresses"]
	delete(options, "addresses")
	var configured []json.RawMessage
	if len(addresses) == 0 || json.Unmarshal(addresses, &configured) != nil || len(configured) == 0 {
		addresses, _ = json.Marshal([]map[string]any{{"server": host, "server_port": port}})
	}
	outbound, err := json.Marshal(options)
	return addresses, outbound, err
}

// Store the two Realm tokens separately in encrypted template versions. The
// public client token is resolved only when producing a connection's options.
func sealRealmDraft(entry *SpecEntry) error {
	if entry.Protocol != "hysteria2" {
		if entry.RealmServerSecret != "" || entry.RealmClientSecret != "" {
			return errClientOptions
		}
		return nil
	}
	options, err := objectJSON(entry.Options)
	if err != nil {
		return err
	}
	if _, enabled := options["realm"].(map[string]any); !enabled {
		entry.RealmServerSecret, entry.RealmClientSecret = "", ""
		return nil
	}

	if err := storeRealmToken(&entry.Options, &entry.RealmServerSecret, "server", entry.Key); err != nil {
		return err
	}
	return storeRealmToken(&entry.ClientOptions, &entry.RealmClientSecret, "client", entry.Key)
}

func openRealmDraft(entry *SpecEntry) error {
	if entry.RealmServerSecret == "" && entry.RealmClientSecret == "" {
		return nil
	}
	restore := func(raw *json.RawMessage, secret, purpose string) error {
		if secret == "" {
			return nil
		}
		token, err := openSecret(secret)
		if err != nil {
			return errors.New("state_key_unavailable")
		}
		token, found := strings.CutPrefix(token, purpose+"\x00"+entry.Key+"\x00")
		if !found || token == "" {
			return errors.New("invalid_realm_credential")
		}
		options := map[string]any{}
		if len(*raw) != 0 {
			if options, err = objectJSON(*raw); err != nil {
				return err
			}
		}
		realm, ok := options["realm"].(map[string]any)
		if !ok {
			realm = map[string]any{}
			options["realm"] = realm
		}
		realm["token"] = token
		*raw, err = json.Marshal(options)
		return err
	}
	options, err := objectJSON(entry.Options)
	if err != nil {
		return err
	}
	if _, enabled := options["realm"].(map[string]any); !enabled {
		return nil
	}
	if err := restore(&entry.Options, entry.RealmServerSecret, "server"); err != nil {
		return err
	}
	return restore(&entry.ClientOptions, entry.RealmClientSecret, "client")
}

func validateClientAddresses(raw json.RawMessage) error {
	if len(raw) != 0 {
		var addresses []struct {
			Server string `json:"server"`
			Port   int    `json:"server_port"`
			Remark string `json:"remark,omitempty"`
			TLS    *struct {
				Enabled    bool   `json:"enabled"`
				ServerName string `json:"server_name,omitempty"`
				Insecure   bool   `json:"insecure,omitempty"`
			} `json:"tls,omitempty"`
		}
		if err := decodeJSON(raw, &addresses, true); err != nil || len(addresses) > 32 {
			return errClientOptions
		}
		for _, address := range addresses {
			if !validPublishedAddress(address.Server) || address.Port < 1 || address.Port > 65535 ||
				(address.TLS != nil && address.TLS.ServerName != "" && !validPublishedAddress(address.TLS.ServerName)) {
				return errClientOptions
			}
		}
	}
	return nil
}

func storeRealmToken(raw *json.RawMessage, sealed *string, purpose, key string) error {
	if len(*raw) == 0 {
		return nil
	}
	options, err := objectJSON(*raw)
	if err != nil {
		return err
	}
	realm, _ := options["realm"].(map[string]any)
	if value, exists := realm["token"]; exists {
		token, ok := value.(string)
		if !ok || len(token) > 4096 {
			return errClientOptions
		}
		if token == "" {
			*sealed = ""
		} else if *sealed, err = sealSecret(purpose + "\x00" + key + "\x00" + token); err != nil {
			return err
		}
		delete(realm, "token")
		*raw, err = json.Marshal(options)
		return err
	}
	return nil
}
