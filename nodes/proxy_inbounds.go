package nodes

import (
	"errors"
	"fmt"
)

func proxyInboundSources(entry *SpecEntry, options map[string]interface{}) error {
	switch entry.Protocol {
	case "shadowsocks":
		return shadowsocksInboundSources(entry, options)
	case "snell":
		version := fmt.Sprint(options["version"])
		if version != "5" && version != "6" {
			return errors.New("invalid_node_snell_version")
		}
		if entry.Sources == nil {
			entry.Sources = map[string]ValueSource{}
		}
		entry.Sources["/psk"] = ValueSource{Kind: "node_generated", Purpose: "snell_psk"}
	case "shadowtls":
		if fmt.Sprint(options["version"]) != "3" {
			return errors.New("node_shadowtls_requires_v3")
		}
	}
	return nil
}

func shadowsocksInboundSources(entry *SpecEntry, options map[string]interface{}) error {
	if options["managed"] == true || options["destinations"] != nil {
		return errors.New("node_shadowsocks_requires_users")
	}
	purpose := ""
	switch options["method"] {
	case "aes-128-gcm", "aes-192-gcm", "aes-256-gcm", "chacha20-ietf-poly1305", "xchacha20-ietf-poly1305":
	case "2022-blake3-aes-128-gcm":
		purpose = "shadowsocks_key16"
	case "2022-blake3-aes-256-gcm":
		purpose = "shadowsocks_key32"
	default:
		return errors.New("invalid_node_shadowsocks_method")
	}
	delete(entry.Sources, "/password")
	if purpose != "" {
		if entry.Sources == nil {
			entry.Sources = map[string]ValueSource{}
		}
		entry.Sources["/password"] = ValueSource{Kind: "node_generated", Purpose: purpose}
	}
	return nil
}
