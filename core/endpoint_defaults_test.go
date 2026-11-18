package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database/model"

	singboxtls "github.com/sagernet/sing-box/common/tls"
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
			skipUnstartedEndpointInShort(t)
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

// TestEndpointInlineTLS starts the TLS-bearing endpoints from the `tls` object
// the panel UI now writes, in sing-box's own field names. It covers the shapes
// the OpenVPN and OpenConnect forms produce, control_wrap among them, which is
// the setting a shared panel TLS config could never express (#1253).
//
// These are started rather than only built, because the checks that made #1253
// hard to pin down -- a server insisting on client certificates it has no CA
// for, a client with nothing to verify its peer against -- all run on start.
func TestEndpointInlineTLS(t *testing.T) {
	certPath, keyPath := writeTestCert(t)
	quotedCert, quotedKey := strconv.Quote(certPath), strconv.Quote(keyPath)
	// The form also takes the PEM itself instead of a path, which is what an
	// operator pasting a .ovpn profile has. sing-box reads it as a list of
	// lines, which is how the panel splits a textarea.
	certLines, keyLines := pemLines(t, certPath), pemLines(t, keyPath)
	generatedKey, generatedCert := generateSelfSigned(t)
	staticKeyPath := writeStaticKey(t)
	quotedStaticKey := strconv.Quote(staticKeyPath)

	testCases := []struct {
		name     string
		endpoint model.Endpoint
	}{
		// The server presents `certificate`/`key` and verifies clients against
		// `client_certificate`, which is the opposite of what those two names
		// mean on the client below.
		{name: "openvpn-server", endpoint: model.Endpoint{
			Type: "openvpn-server", Tag: "ovs-tls",
			Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":44101,"mode":"tls","network":"udp","address":["10.8.0.1/24"],
				"tls":{"certificate_path":` + quotedCert + `,"key_path":` + quotedKey + `,"verify_client_certificate":"none"}}`),
		}},
		{name: "openvpn-server-mtls", endpoint: model.Endpoint{
			Type: "openvpn-server", Tag: "ovs-mtls",
			Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":44102,"mode":"tls","network":"udp","address":["10.8.0.1/24"],
				"tls":{"certificate_path":` + quotedCert + `,"key_path":` + quotedKey + `,
					"client_certificate_path":` + quotedCert + `,"verify_client_certificate":"require"}}`),
		}},
		// tls-crypt wraps the control channel in a pre-shared key. Nearly every
		// real deployment uses one.
		{name: "openvpn-server-control-wrap", endpoint: model.Endpoint{
			Type: "openvpn-server", Tag: "ovs-wrap",
			Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":44103,"mode":"tls","network":"udp","address":["10.8.0.1/24"],
				"tls":{"certificate_path":` + quotedCert + `,"key_path":` + quotedKey + `,
					"verify_client_certificate":"none",
					"control_wrap":{"type":"tls_crypt","key_path":` + quotedStaticKey + `}}}`),
		}},
		// On the client `certificate` is the CA it checks the server against.
		{name: "openvpn-client", endpoint: model.Endpoint{
			Type: "openvpn-client", Tag: "ovc-tls",
			Options: json.RawMessage(`{"server":"127.0.0.1","server_port":1194,"mode":"tls","network":"udp",
				"tls":{"certificate_path":` + quotedCert + `,"version_min":"1.2"}}`),
		}},
		{name: "openvpn-client-mtls", endpoint: model.Endpoint{
			Type: "openvpn-client", Tag: "ovc-mtls",
			Options: json.RawMessage(`{"server":"127.0.0.1","server_port":1194,"mode":"tls","network":"udp",
				"tls":{"certificate_path":` + quotedCert + `,"client_certificate_path":` + quotedCert + `,
					"client_key_path":` + quotedKey + `,"remote_certificate_tls":"server"}}`),
		}},
		// A peer fingerprint stands in for a certificate authority.
		{name: "openvpn-client-fingerprint", endpoint: model.Endpoint{
			Type: "openvpn-client", Tag: "ovc-pin",
			Options: json.RawMessage(`{"server":"127.0.0.1","server_port":1194,"mode":"tls","network":"udp",
				"tls":{"peer_fingerprint":["030a11181f262d343b424950575e656c737a81888f969da4abb2b9c0c7ced5dc"]}}`),
		}},
		{name: "openvpn-server-inline-pem", endpoint: model.Endpoint{
			Type: "openvpn-server", Tag: "ovs-inline",
			Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":44104,"mode":"tls","network":"udp","address":["10.8.0.1/24"],
				"tls":{"certificate":` + certLines + `,"key":` + keyLines + `,"verify_client_certificate":"none"}}`),
		}},
		{name: "openvpn-client-inline-pem", endpoint: model.Endpoint{
			Type: "openvpn-client", Tag: "ovc-inline",
			Options: json.RawMessage(`{"server":"127.0.0.1","server_port":1194,"mode":"tls","network":"udp",
				"tls":{"certificate":` + certLines + `,"control_wrap":{"type":"tls_crypt","key":` + staticKeyLines(t, staticKeyPath) + `}}}`),
		}},
		// What the "generate" button in the OpenVPN server form produces: the
		// same self-signed pair the panel issues for inbounds, held as text
		// because it exists nowhere on disk.
		{name: "openvpn-server-generated-pem", endpoint: model.Endpoint{
			Type: "openvpn-server", Tag: "ovs-gen",
			Options: json.RawMessage(`{"listen":"127.0.0.1","listen_port":44105,"mode":"tls","network":"udp","address":["10.8.0.1/24"],
				"tls":{"certificate":` + generatedCert + `,"key":` + generatedKey + `,"verify_client_certificate":"none"}}`),
		}},
		// OpenConnect names the trust anchor after its role.
		{name: "openconnect", endpoint: model.Endpoint{
			Type: "openconnect", Tag: "oc-tls",
			Options: json.RawMessage(`{"server":"vpn.example.com","flavor":"anyconnect",
				"tls":{"certificate_authority_path":` + quotedCert + `,"server_name":"vpn.example.com"}}`),
		}},
		{name: "openconnect-inline-pem", endpoint: model.Endpoint{
			Type: "openconnect", Tag: "oc-inline",
			Options: json.RawMessage(`{"server":"vpn.example.com","flavor":"anyconnect",
				"tls":{"certificate_authority":` + certLines + `}}`),
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.endpoint.Type != "openvpn-server" {
				skipUnstartedEndpointInShort(t)
			}
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
				t.Fatalf("parse (endpoint was %s): %v", endpointJSON, err)
			}
			instance, err := NewBox(Options{Context: ctx, Options: options})
			skipIfFeatureMissing(t, err)
			if err != nil {
				t.Fatalf("build (endpoint was %s): %v", endpointJSON, err)
			}
			defer instance.Close()
			// A server reads its TLS credentials when it starts listening, so
			// only starting it covers them. A client reads its own when it
			// first dials, which needs a peer that is not there -- and
			// sing-openvpn panics on the failed dial rather than returning --
			// so its credentials are covered by the build above, where
			// validateTLSCredentialSet already rejects having neither a
			// certificate authority nor a peer fingerprint.
			if testCase.endpoint.Type != "openvpn-server" {
				return
			}
			if err = instance.Start(); err != nil {
				t.Fatalf("start (endpoint was %s): %v", endpointJSON, err)
			}
		})
	}
}

// pemLines reads a PEM file back as the JSON list of lines the panel's "use
// text" mode writes.
func pemLines(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return jsonLines(t, string(raw))
}

// generateSelfSigned mirrors ServerService.GenKeypair("tls", ...) behind the
// panel's generate button, and splits the result the way the form does, so the
// test covers what an operator actually ends up with. The service package
// cannot be imported here, since it imports this one.
func generateSelfSigned(t *testing.T) (keyLines string, certLines string) {
	t.Helper()
	privateKeyPem, publicKeyPem, err := singboxtls.GenerateCertificate(nil, nil, time.Now, "ovs-gen", time.Now().AddDate(0, 12, 0))
	if err != nil {
		t.Fatal(err)
	}
	return jsonLines(t, string(privateKeyPem)), jsonLines(t, string(publicKeyPem))
}

func jsonLines(t *testing.T, pem string) string {
	t.Helper()
	encoded, err := json.Marshal(strings.Split(strings.TrimRight(pem, "\n"), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// writeStaticKey writes an OpenVPN static key, which is what tls-auth and
// tls-crypt take: 256 bytes as hex between its own markers, not a PEM.
func writeStaticKey(t *testing.T) string {
	t.Helper()
	material := make([]byte, 256)
	if _, err := rand.Read(material); err != nil {
		t.Fatal(err)
	}
	// Same shape as ServerService.generateOpenVPNStaticKey, comment header and
	// all, so this covers what the panel's generate button hands over.
	var body strings.Builder
	body.WriteString("#\n# 2048 bit OpenVPN static key\n#\n")
	body.WriteString("-----BEGIN OpenVPN Static key V1-----\n")
	encoded := hex.EncodeToString(material)
	for offset := 0; offset < len(encoded); offset += 32 {
		body.WriteString(encoded[offset:offset+32] + "\n")
	}
	body.WriteString("-----END OpenVPN Static key V1-----\n")

	path := filepath.Join(t.TempDir(), "static.key")
	if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func staticKeyLines(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return jsonLines(t, string(raw))
}

// TestOpenVPNServerClientCertificateDefault pins the behaviour that made #1253
// so hard to place: sing-box reads an unset `verify_client_certificate` as
// "require", so a server with a perfectly good certificate and key still
// refuses to start because it has no CA for the client certificates it did not
// know it was asking for. The panel writes the choice explicitly, and the
// migration fills it in for endpoints that predate that.
//
// If this ever stops failing, the default changed upstream and the panel no
// longer has to state it.
func TestOpenVPNServerClientCertificateDefault(t *testing.T) {
	certPath, keyPath := writeTestCert(t)

	start := func(t *testing.T, tlsOptions map[string]any, port int) error {
		t.Helper()
		raw, err := json.Marshal(map[string]any{
			"log": map[string]any{"level": "error"},
			"endpoints": []any{map[string]any{
				"type": "openvpn-server", "tag": "ovs", "listen": "127.0.0.1",
				"listen_port": port, "mode": "tls", "network": "udp",
				"address": []string{"10.8.0.1/24"}, "tls": tlsOptions,
			}},
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
		defer instance.Close()
		return instance.Start()
	}

	t.Run("unstated", func(t *testing.T) {
		err := start(t, map[string]any{"certificate_path": certPath, "key_path": keyPath}, 44201)
		if err == nil {
			t.Fatal("a server with no client CA started; the upstream default may have changed")
		}
		if !strings.Contains(err.Error(), "certificate-authority or peer-fingerprint") {
			t.Fatalf("unexpected failure: %v", err)
		}
	})

	t.Run("stated as none", func(t *testing.T) {
		err := start(t, map[string]any{
			"certificate_path": certPath, "key_path": keyPath,
			"verify_client_certificate": "none",
		}, 44202)
		if err != nil {
			t.Fatalf("start: %v", err)
		}
	})
}
