package model

import (
	"encoding/json"
	"slices"
	"testing"
)

func decode(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	if raw == nil {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// A panel TLS config as the UI writes it: the server side carries `enabled`,
// which is exactly the field the endpoint TLS structs reject.
func panelTLS() *Tls {
	return &Tls{
		Name: "example",
		Server: json.RawMessage(`{
			"enabled": true,
			"server_name": "vpn.example.com",
			"alpn": ["h2"],
			"min_version": "1.2",
			"certificate_path": "/etc/s-ui/cert.pem",
			"key_path": "/etc/s-ui/key.pem",
			"client_certificate_path": ["/etc/s-ui/client-ca.pem"]
		}`),
		Client: json.RawMessage(`{
			"enabled": true,
			"server_name": "vpn.example.com",
			"insecure": true,
			"utls": {"enabled": true, "fingerprint": "chrome"},
			"certificate_path": "/etc/s-ui/ca.pem",
			"client_certificate_path": "/etc/s-ui/client.pem",
			"client_key_path": "/etc/s-ui/client.key"
		}`),
	}
}

func TestProjectEndpointTLSOpenVPNServer(t *testing.T) {
	projected, err := projectEndpointTLS("openvpn-server", panelTLS())
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, projected)

	if got["certificate_path"] != "/etc/s-ui/cert.pem" || got["key_path"] != "/etc/s-ui/key.pem" {
		t.Errorf("certificate and key must carry over, got %v", got)
	}
	if got["version_min"] != "1.2" {
		t.Errorf("min_version should be renamed to version_min, got %v", got["version_min"])
	}
	// These are the ones sing-box would reject outright.
	for _, dropped := range []string{"enabled", "alpn", "min_version", "server_name"} {
		if _, ok := got[dropped]; ok {
			t.Errorf("%q must not reach an openvpn-server, got %v", dropped, got)
		}
	}
}

func TestProjectEndpointTLSOpenVPNClient(t *testing.T) {
	projected, err := projectEndpointTLS("openvpn-client", panelTLS())
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, projected)

	if got["server_name"] != "vpn.example.com" {
		t.Errorf("server_name must carry over, got %v", got)
	}
	if got["certificate_path"] != "/etc/s-ui/ca.pem" {
		t.Errorf("the CA path must carry over, got %v", got)
	}
	if got["client_certificate_path"] != "/etc/s-ui/client.pem" || got["client_key_path"] != "/etc/s-ui/client.key" {
		t.Errorf("client credentials must carry over, got %v", got)
	}
	// OpenVPN has no insecure switch and no use for a utls fingerprint.
	for _, dropped := range []string{"enabled", "insecure", "utls"} {
		if _, ok := got[dropped]; ok {
			t.Errorf("%q must not reach an openvpn-client, got %v", dropped, got)
		}
	}
}

// OpenConnect names the trust anchor differently from the other TLS structs.
func TestProjectEndpointTLSOpenConnect(t *testing.T) {
	projected, err := projectEndpointTLS("openconnect", panelTLS())
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, projected)

	if got["certificate_authority_path"] != "/etc/s-ui/ca.pem" {
		t.Errorf("certificate_path should become certificate_authority_path, got %v", got)
	}
	if _, ok := got["certificate_path"]; ok {
		t.Errorf("the original certificate_path must not survive, got %v", got)
	}
	if got["insecure"] != true {
		t.Errorf("openconnect supports insecure, got %v", got)
	}
	for _, dropped := range []string{"enabled", "utls"} {
		if _, ok := got[dropped]; ok {
			t.Errorf("%q must not reach an openconnect endpoint, got %v", dropped, got)
		}
	}
}

// An endpoint type that has no TLS of its own must not be handed one.
func TestProjectEndpointTLSIgnoresOtherTypes(t *testing.T) {
	for _, endpointType := range []string{"wireguard", "warp", "tailscale"} {
		projected, err := projectEndpointTLS(endpointType, panelTLS())
		if err != nil {
			t.Fatal(err)
		}
		if projected != nil {
			t.Errorf("%s should get no tls block, got %s", endpointType, projected)
		}
	}
}

// A TLS config with nothing the endpoint can use should produce no block at
// all, since sing-box reads an empty object as absent anyway.
func TestProjectEndpointTLSEmptyResult(t *testing.T) {
	tlsConfig := &Tls{Client: json.RawMessage(`{"enabled": true, "alpn": ["h2"]}`)}
	projected, err := projectEndpointTLS("openvpn-client", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	if projected != nil {
		t.Errorf("expected no tls block, got %s", projected)
	}
}

func TestUnsupportedEndpointTLS(t *testing.T) {
	realityTLS := &Tls{
		Server: json.RawMessage(`{"enabled": true, "reality": {"enabled": true}, "certificate_path": "/c.pem"}`),
		Client: json.RawMessage(`{"enabled": true, "insecure": true, "utls": {"enabled": true}}`),
	}

	serverUnsupported, err := unsupportedEndpointTLS("openvpn-server", realityTLS)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(serverUnsupported, "reality") {
		t.Errorf("reality should be reported for openvpn-server, got %v", serverUnsupported)
	}

	clientUnsupported, err := unsupportedEndpointTLS("openvpn-client", realityTLS)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(clientUnsupported, "insecure") {
		t.Errorf("insecure should be reported for openvpn-client, got %v", clientUnsupported)
	}

	// openconnect does support insecure, so it must not be flagged there.
	openconnectUnsupported, err := unsupportedEndpointTLS("openconnect", realityTLS)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(openconnectUnsupported, "insecure") {
		t.Errorf("openconnect supports insecure, got %v", openconnectUnsupported)
	}
}

// A referenced TLS config is an explicit choice by the operator, so it must
// replace any TLS fields the endpoint still carries inline rather than being
// quietly overwritten by them.
func TestEndpointMarshalTLSPrecedence(t *testing.T) {
	endpoint := Endpoint{
		Type:    "openconnect",
		Tag:     "oc",
		TlsId:   1,
		Tls:     &Tls{Client: json.RawMessage(`{"enabled": true, "certificate_path": "/from-panel.pem"}`)},
		Options: json.RawMessage(`{"server":"vpn.example.com","tls":{"certificate_authority_path":"/inline.pem"}}`),
	}

	raw, err := endpoint.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, raw)
	tlsBlock, ok := got["tls"].(map[string]any)
	if !ok {
		t.Fatalf("expected a tls block, got %v", got)
	}
	if tlsBlock["certificate_authority_path"] != "/from-panel.pem" {
		t.Errorf("the referenced config must win over the inline one, got %v", tlsBlock)
	}
}

// Without a referenced config the inline TLS the UI wrote must survive intact.
func TestEndpointMarshalKeepsInlineTLS(t *testing.T) {
	endpoint := Endpoint{
		Type:    "openconnect",
		Tag:     "oc",
		Options: json.RawMessage(`{"server":"vpn.example.com","tls":{"certificate_authority_path":"/inline.pem"}}`),
	}

	raw, err := endpoint.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, raw)
	tlsBlock, ok := got["tls"].(map[string]any)
	if !ok {
		t.Fatalf("expected the inline tls block to survive, got %v", got)
	}
	if tlsBlock["certificate_authority_path"] != "/inline.pem" {
		t.Errorf("inline tls must be untouched, got %v", tlsBlock)
	}
}

// The panel keeps client CA paths as a list because sing-box inbounds accept
// several, but OpenVPN takes one, and the two sides name verification modes
// differently. Both need converting, not just copying.
func TestProjectEndpointTLSOpenVPNServerClientAuth(t *testing.T) {
	tlsConfig := &Tls{Server: json.RawMessage(`{
		"enabled": true,
		"certificate_path": "/cert.pem",
		"key_path": "/key.pem",
		"client_authentication": "require-and-verify",
		"client_certificate_path": ["/client-ca.pem", "/second-ca.pem"]
	}`)}

	projected, err := projectEndpointTLS("openvpn-server", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, projected)

	if got["client_certificate_path"] != "/client-ca.pem" {
		t.Errorf("the list should collapse to its first entry, got %v", got["client_certificate_path"])
	}
	if got["verify_client_certificate"] != "require" {
		t.Errorf("require-and-verify should become require, got %v", got["verify_client_certificate"])
	}
	if got["client_authentication"] != nil {
		t.Errorf("the sing-box spelling must not leak through, got %v", got)
	}
}

func TestVerifyClientCertificateFor(t *testing.T) {
	for clientAuthentication, want := range map[string]string{
		"require-and-verify": "require",
		"require-any":        "require",
		"verify-if-given":    "optional",
		"request":            "optional",
		"no":                 "",
		"":                   "",
	} {
		if got := verifyClientCertificateFor(clientAuthentication); got != want {
			t.Errorf("%q: want %q, got %q", clientAuthentication, want, got)
		}
	}
}

// The TLS editor can hold the client credentials as inline PEM instead of
// paths, so the projection has to carry that form across too.
func TestProjectEndpointTLSInlineClientCerts(t *testing.T) {
	tlsConfig := &Tls{
		Server: json.RawMessage(`{
			"enabled": true,
			"certificate": ["-----BEGIN CERTIFICATE-----", "srv", "-----END CERTIFICATE-----"],
			"client_certificate": ["-----BEGIN CERTIFICATE-----", "ca", "-----END CERTIFICATE-----"]
		}`),
		Client: json.RawMessage(`{
			"enabled": true,
			"client_certificate": ["-----BEGIN CERTIFICATE-----", "cli", "-----END CERTIFICATE-----"],
			"client_key": ["-----BEGIN PRIVATE KEY-----", "k", "-----END PRIVATE KEY-----"]
		}`),
	}

	server := decode(t, mustProject(t, "openvpn-server", tlsConfig))
	if server["client_certificate"] == nil {
		t.Errorf("inline client CA must reach an openvpn-server, got %v", server)
	}

	for _, endpointType := range []string{"openvpn-client", "openconnect"} {
		got := decode(t, mustProject(t, endpointType, tlsConfig))
		if got["client_certificate"] == nil || got["client_key"] == nil {
			t.Errorf("%s: inline client credentials must carry over, got %v", endpointType, got)
		}
	}
}

func mustProject(t *testing.T, endpointType string, tlsConfig *Tls) json.RawMessage {
	t.Helper()
	projected, err := projectEndpointTLS(endpointType, tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	if projected == nil {
		t.Fatalf("%s: expected a tls block", endpointType)
	}
	return projected
}
