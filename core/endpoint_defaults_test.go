package core

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"github.com/sagernet/sing-box/option"
)

// TestEndpointDefaults builds one endpoint of every type the panel UI can
// create. Unlike inbounds, the OpenConnect and OpenVPN endpoints need details
// only the operator has (a server to dial, certificate paths), so the cases
// below add exactly those and nothing else. Anything a case has to add beyond
// the UI defaults is a field the UI must expose, which is what this guards.
func TestEndpointDefaults(t *testing.T) {
	certPath, keyPath := writeTestCert(t)

	testCases := []struct {
		name    string
		options map[string]any
	}{
		{name: "wireguard", options: map[string]any{
			"type": "wireguard", "address": []string{"10.0.0.2/32"},
			"private_key": "8I9OMDoO5jlvbSraQFxIvUAoluHrM8izP+xuBuT9jFg=",
			"listen_port": 43201, "peers": []any{}}},
		{name: "tailscale", options: map[string]any{
			"type": "tailscale", "state_directory": t.TempDir()}},
		// operator supplies: server
		{name: "openconnect", options: map[string]any{
			"type": "openconnect", "server": "vpn.example.com", "flavor": "anyconnect"}},
		// operator supplies: server, and a tls block (an empty one reads as absent)
		{name: "openvpn-client", options: map[string]any{
			"type": "openvpn-client", "server": "vpn.example.com", "server_port": 1194,
			"mode": "tls", "network": "udp",
			"tls": map[string]any{"certificate_path": certPath}}},
		// static_key mode has no TLS session to negotiate over, so the client
		// carries its own tunnel address, the peer's, and a cipher. It has to
		// be a CBC one: GCM needs the TLS key exchange for IV uniqueness.
		{name: "openvpn-client-static-key", options: map[string]any{
			"type": "openvpn-client", "server": "vpn.example.com", "server_port": 1194,
			"mode": "static_key", "network": "udp", "address": []string{"10.8.0.2/24"},
			"peer_address": "10.8.0.1", "static_key_path": keyPath, "cipher": "AES-256-CBC"}},
		// operator supplies: certificate and key paths
		{name: "openvpn-server", options: map[string]any{
			"type": "openvpn-server", "listen": "127.0.0.1", "listen_port": 43202,
			"mode": "tls", "network": "udp", "address": []string{"10.8.0.1/24"},
			"tls": map[string]any{"certificate_path": certPath, "key_path": keyPath}}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			endpoint := map[string]any{"tag": testCase.name + "-ep"}
			for key, value := range testCase.options {
				endpoint[key] = value
			}
			raw, err := json.Marshal(map[string]any{
				"log":       map[string]any{"level": "error"},
				"endpoints": []any{endpoint},
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

// TestEndpointPanelTLS builds an endpoint the way the panel does: from a stored
// model.Endpoint carrying a tls_id, whose TLS row is projected into whatever
// shape that endpoint type accepts. It proves the projection produces something
// sing-box actually takes, which a unit test over the mapping alone cannot.
func TestEndpointPanelTLS(t *testing.T) {
	certPath, keyPath := writeTestCert(t)

	// A TLS row as the panel stores it, `enabled` and all.
	tlsRow := &model.Tls{
		Name: "panel-cert",
		Server: json.RawMessage(`{
			"enabled": true,
			"server_name": "vpn.example.com",
			"alpn": ["h2"],
			"certificate_path": ` + strconv.Quote(certPath) + `,
			"key_path": ` + strconv.Quote(keyPath) + `
		}`),
		Client: json.RawMessage(`{
			"enabled": true,
			"server_name": "vpn.example.com",
			"utls": {"enabled": true, "fingerprint": "chrome"},
			"certificate_path": ` + strconv.Quote(certPath) + `
		}`),
	}

	testCases := []struct {
		name     string
		endpoint model.Endpoint
	}{
		{name: "openvpn-server", endpoint: model.Endpoint{
			Type: "openvpn-server", Tag: "ovs-tls", TlsId: 1, Tls: tlsRow,
			Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":44101,"mode":"tls","network":"udp","address":["10.8.0.1/24"]}`),
		}},
		{name: "openvpn-client", endpoint: model.Endpoint{
			Type: "openvpn-client", Tag: "ovc-tls", TlsId: 1, Tls: tlsRow,
			Options: json.RawMessage(`{"server":"vpn.example.com","server_port":1194,"mode":"tls","network":"udp"}`),
		}},
		{name: "openconnect", endpoint: model.Endpoint{
			Type: "openconnect", Tag: "oc-tls", TlsId: 1, Tls: tlsRow,
			Options: json.RawMessage(`{"server":"vpn.example.com","flavor":"anyconnect"}`),
		}},
		// mutual TLS: the panel stores client CA paths as a list and names
		// verification differently, so both are converted on the way in.
		{name: "openvpn-server-mtls", endpoint: model.Endpoint{
			Type: "openvpn-server", Tag: "ovs-mtls", TlsId: 2,
			Tls: &model.Tls{Name: "panel-mtls", Server: json.RawMessage(`{
				"enabled": true,
				"certificate_path": ` + strconv.Quote(certPath) + `,
				"key_path": ` + strconv.Quote(keyPath) + `,
				"client_authentication": "require-and-verify",
				"client_certificate_path": [` + strconv.Quote(certPath) + `]
			}`)},
			Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":44102,"mode":"tls","network":"udp","address":["10.8.0.1/24"]}`),
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			endpointJSON, err := testCase.endpoint.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(map[string]any{
				"log":       map[string]any{"level": "error"},
				"endpoints": []any{json.RawMessage(endpointJSON)},
				"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
			})
			if err != nil {
				t.Fatal(err)
			}

			ctx := Context(context.Background(), InboundRegistry(), OutboundRegistry(),
				EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry(), CertificateProviderRegistry())
			var options option.Options
			if err = options.UnmarshalJSONContext(ctx, raw); err != nil {
				t.Fatalf("parse (projected tls was %s): %v", endpointJSON, err)
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
