package util

import "strings"

// NormalizeHost strips URI brackets from an IPv6 literal ("[::1]" -> "::1").
// Bare hosts, IPv4 addresses and domains are returned unchanged. Config
// formats (sing-box JSON, clash YAML, vmess "add") want the bare form (#1220).
func NormalizeHost(host string) string {
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host[1 : len(host)-1]
	}
	return host
}

// HostForURI returns the host formatted for a URI authority: IPv6 literals
// are wrapped in brackets so "host:port" stays parseable, everything else is
// returned unchanged.
func HostForURI(host string) string {
	h := NormalizeHost(host)
	if strings.Contains(h, ":") {
		return "[" + h + "]"
	}
	return h
}
