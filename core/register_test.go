package core

import (
	"context"
	"testing"

	sb "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
)

func TestSingBox114ConfigSurfaceUnmarshals(t *testing.T) {
	ctx := sb.Context(context.Background(), InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry(), CertificateProviderRegistry())

	config := []byte(`{
		"log": {"disabled": true},
		"dns": {
			"servers": [
				{"tag": "mdns", "type": "mdns", "interface": ["eth0"]}
			]
		},
		"certificate_providers": [
			{"tag": "origin", "type": "cloudflare-origin-ca", "domain": ["example.com"], "api_token": "token"}
		],
		"http_clients": [
			{"tag": "h3", "version": 3, "headers": {"User-Agent": ["s-ui-test"]}}
		],
		"inbounds": [
			{"tag": "snell-in", "type": "snell", "listen": "127.0.0.1", "listen_port": 23456, "version": 5, "psk": "secret", "obfs_mode": "http"}
		],
		"outbounds": [
			{"tag": "snell-out", "type": "snell", "server": "example.com", "server_port": 443, "version": 4, "psk": "secret", "obfs_mode": "tls", "obfs_host": "example.com"},
			{"tag": "bridge", "type": "bridge", "interface": "eth0", "bridge_name": "br0"},
			{"tag": "trojan-spoof", "type": "trojan", "server": "example.com", "server_port": 443, "password": "secret", "tls": {"enabled": true, "server_name": "example.com", "spoof": "cdn.example.com", "spoof_method": "default"}}
		],
		"services": [
			{"tag": "api", "type": "api", "listen": "127.0.0.1", "listen_port": 9090, "secret": "secret"},
			{"tag": "realm", "type": "hysteria-realm", "listen": "127.0.0.1", "listen_port": 8443, "users": [{"name": "user", "token": "token"}], "tls": {"enabled": true, "certificate_provider": "origin"}}
		],
		"route": {
			"rules": [
				{"domain_suffix": ["example.com"], "action": "route-options", "tls_spoof": "cdn.example.com", "tls_spoof_method": "default"}
			],
			"final": "snell-out",
			"default_http_client": "h3"
		}
	}`)

	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, config); err != nil {
		t.Fatalf("unmarshal sing-box 1.14 config surface: %v", err)
	}

	if len(options.Inbounds) != 1 || options.Inbounds[0].Type != "snell" {
		t.Fatalf("snell inbound was not parsed: %#v", options.Inbounds)
	}
	if len(options.Outbounds) != 3 || options.Outbounds[1].Type != "bridge" {
		t.Fatalf("bridge outbound was not parsed: %#v", options.Outbounds)
	}
	if len(options.Services) != 2 || options.Services[1].Type != "hysteria-realm" {
		t.Fatalf("hysteria realm service was not parsed: %#v", options.Services)
	}
	if len(options.CertificateProviders) != 1 || options.CertificateProviders[0].Type != "cloudflare-origin-ca" {
		t.Fatalf("certificate provider was not parsed: %#v", options.CertificateProviders)
	}
	if len(options.HTTPClients) != 1 || options.HTTPClients[0].Tag != "h3" {
		t.Fatalf("HTTP client was not parsed: %#v", options.HTTPClients)
	}
}
