package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sagernet/sing-box/option"
)

// TestInboundDefaults builds one inbound of every type the panel UI can create,
// using the values frontend/src/types/inbounds.ts fills in on protocol change.
//
// It guards two things the UI depends on: a freshly created inbound of any type
// must be startable without further editing, and a type that does not listen
// must not be handed listen options. Both are silent failures otherwise, since
// the UI happily saves a config sing-box then refuses.
// A syntactically valid Cloudflare tunnel token: base64 of the account tag,
// tunnel id and secret. Cloudflare issues the real one.
const cloudflaredTestToken = "eyJhIjogIjAxMjM0NTY3ODlhYmNkZWYwMTIzNDU2Nzg5YWJjZGVmIiwgInQiOiAiMTExMTExMTEtMjIyMi0zMzMzLTQ0NDQtNTU1NTU1NTU1NTU1IiwgInMiOiAiZUhoNGVIaDRlSGg0ZUhoNGVIaDRlSGg0ZUhoNGVIaDRlSGg0ZUhoNGVIZz0ifQ=="

func TestInboundDefaults(t *testing.T) {
	certPath, keyPath := writeTestCert(t)
	tlsOptions := map[string]any{
		"enabled": true, "server_name": "t.example.com",
		"certificate_path": certPath, "key_path": keyPath,
	}
	quicTLSOptions := map[string]any{
		"enabled": true, "server_name": "t.example.com", "alpn": []string{"h3"},
		"certificate_path": certPath, "key_path": keyPath,
	}
	testCases := []struct {
		name string
		// listens is false for the types Inbound.vue keeps out of <Listen>.
		listens bool
		options map[string]any
	}{
		{name: "direct", listens: true, options: map[string]any{"type": "direct"}},
		{name: "mixed", listens: true, options: map[string]any{"type": "mixed"}},
		{name: "socks", listens: true, options: map[string]any{"type": "socks"}},
		{name: "http", listens: true, options: map[string]any{"type": "http"}},
		{name: "redirect", listens: true, options: map[string]any{"type": "redirect"}},
		{name: "tproxy", listens: true, options: map[string]any{"type": "tproxy"}},
		{name: "shadowsocks", listens: true, options: map[string]any{"type": "shadowsocks", "method": "none"}},
		{name: "vmess", listens: true, options: map[string]any{"type": "vmess"}},
		{name: "vless", listens: true, options: map[string]any{"type": "vless"}},
		{name: "trojan", listens: true, options: map[string]any{"type": "trojan"}},
		{name: "anytls", listens: true, options: map[string]any{"type": "anytls", "tls": tlsOptions}},
		{name: "hysteria", listens: true, options: map[string]any{
			"type": "hysteria", "up_mbps": 100, "down_mbps": 100, "tls": quicTLSOptions}},
		{name: "hysteria2", listens: true, options: map[string]any{"type": "hysteria2", "tls": quicTLSOptions}},
		{name: "tuic", listens: true, options: map[string]any{
			"type": "tuic", "congestion_control": "cubic", "tls": quicTLSOptions}},
		// snell needs a psk of 12-255 bytes, which createInbound generates.
		{name: "snell", listens: true, options: map[string]any{
			"type": "snell", "version": 6, "psk": "0123456789abcdefghijklmnopqrstuv"}},
		// naive and shadowtls v3 reject an empty user list, so the client
		// system must attach one; the UI offers no way to save without.
		{name: "naive", listens: true, options: map[string]any{
			"type": "naive", "tls": tlsOptions,
			"users": []any{map[string]any{"username": "u", "password": "p"}}}},
		{name: "shadowtls-v3", listens: true, options: map[string]any{
			"type": "shadowtls", "version": 3,
			"handshake": map[string]any{"server": "example.com", "server_port": 443},
			"users":     []any{map[string]any{"name": "u", "password": "p"}}}},
		// v2 carries an inbound-level password instead of users.
		{name: "shadowtls-v2", listens: true, options: map[string]any{
			"type": "shadowtls", "version": 2, "password": "0123456789abcdef",
			"handshake": map[string]any{"server": "example.com", "server_port": 443}}},
		// cloudflared dials out to the Cloudflare edge, so it has no listen
		// options. Its token is issued by Cloudflare and cannot be generated;
		// the operator has to paste one in before the inbound will start.
		{name: "cloudflared", listens: false, options: map[string]any{
			"type": "cloudflared", "token": cloudflaredTestToken, "protocol": "auto"}},
	}

	port := 43100
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			port++
			inbound := map[string]any{"tag": testCase.name + "-in"}
			for key, value := range testCase.options {
				inbound[key] = value
			}
			if testCase.listens {
				inbound["listen"] = "127.0.0.1"
				inbound["listen_port"] = port
			}

			raw, err := json.Marshal(map[string]any{
				"log":       map[string]any{"level": "error"},
				"inbounds":  []any{inbound},
				"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
			})
			if err != nil {
				t.Fatal(err)
			}

			ctx := Context(context.Background(), InboundRegistry(), OutboundRegistry(),
				EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry(), CertificateProviderRegistry())
			var options option.Options
			if err = options.UnmarshalJSONContext(ctx, raw); err != nil {
				t.Fatalf("parse: %v", err)
			}
			instance, err := NewBox(Options{Context: ctx, Options: options})
			skipIfFeatureMissing(t, err)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			instance.Close()
		})
	}
}
