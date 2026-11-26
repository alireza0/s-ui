package nodeagent

import "slices"

var originalInboundTypes = []string{"vless", "tuic", "hysteria2"}
var extraInboundTypes = []string{"vmess", "trojan", "anytls"}
var proxyInboundTypes = []string{"shadowsocks", "snell", "shadowtls", "hysteria", "naive", "http", "socks", "mixed"}

func InboundTypes(extended bool) []string {
	if extended {
		return slices.Concat(originalInboundTypes, extraInboundTypes, proxyInboundTypes)
	}
	return slices.Clone(originalInboundTypes)
}

func SupportsInbound(kind string, extended bool) bool {
	return slices.Contains(originalInboundTypes, kind) || extended && ExtendedInbound(kind)
}

func ExtendedInbound(kind string) bool {
	return slices.Contains(extraInboundTypes, kind) || ProxyInbound(kind)
}
func ProxyInbound(kind string) bool { return slices.Contains(proxyInboundTypes, kind) }

func UsernameAuth(kind string) bool {
	return kind == "http" || kind == "socks" || kind == "mixed" || kind == "naive"
}

func CredentialKey(kind, method string) string {
	if kind == "shadowsocks" && method == "2022-blake3-aes-128-gcm" {
		return "shadowsocks16"
	}
	return kind
}

func SecretField(kind string) string {
	switch kind {
	case "hysteria":
		return "auth_str"
	case "snell":
		return "userkey"
	default:
		if NeedsPassword(kind) || ProxyInbound(kind) || kind == "shadowsocks16" {
			return "password"
		}
		return ""
	}
}

func NeedsUUID(kind string) bool { return kind == "vless" || kind == "tuic" || kind == "vmess" }
func NeedsPassword(kind string) bool {
	return kind == "tuic" || kind == "hysteria2" || kind == "trojan" || kind == "anytls"
}

func SupportsTLS(kind string) bool {
	switch kind {
	case "shadowsocks", "snell", "shadowtls", "socks", "mixed":
		return false
	default:
		return true
	}
}

func RequiresRegularTLS(kind, transport string) bool {
	return kind == "tuic" || kind == "hysteria2" || kind == "hysteria" || kind == "naive" || kind == "anytls" || transport == "quic"
}
