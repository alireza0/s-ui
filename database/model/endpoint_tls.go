package model

import "encoding/json"

// OpenConnect and OpenVPN do not use sing-box's standard TLS options. They each
// define their own struct with a different field set, and sing-box rejects
// unknown fields, so a panel TLS config cannot be handed to them as-is: even
// `enabled`, which every stored config carries, fails to parse.
//
// projectEndpointTLS copies across the fields the endpoint actually accepts.
// Whatever the endpoint has no equivalent for is left out rather than failing
// the whole config: an endpoint is one row among many, and refusing to render
// the config would stop the core from starting at all. The UI is what tells the
// operator which TLS configs suit which endpoint, via unsupportedEndpointTLS.
func projectEndpointTLS(endpointType string, tlsConfig *Tls) (json.RawMessage, error) {
	var source map[string]json.RawMessage
	switch endpointType {
	case "openvpn-server":
		// A server presents a certificate, so it reads the server side.
		if len(tlsConfig.Server) == 0 {
			return nil, nil
		}
		if err := json.Unmarshal(tlsConfig.Server, &source); err != nil {
			return nil, err
		}
		projected, err := projectTLSFields(source, openVPNServerTLSFields, openVPNServerTLSRenames)
		if err != nil {
			return nil, err
		}
		return withOpenVPNServerClientAuth(projected, source)
	case "openvpn-client":
		if len(tlsConfig.Client) == 0 {
			return nil, nil
		}
		if err := json.Unmarshal(tlsConfig.Client, &source); err != nil {
			return nil, err
		}
		return projectTLSFields(source, openVPNClientTLSFields, openVPNClientTLSRenames)
	case "openconnect":
		if len(tlsConfig.Client) == 0 {
			return nil, nil
		}
		if err := json.Unmarshal(tlsConfig.Client, &source); err != nil {
			return nil, err
		}
		return projectTLSFields(source, openConnectTLSFields, openConnectTLSRenames)
	default:
		return nil, nil
	}
}

// Fields carried over verbatim, keyed by the name they have in the panel's TLS
// config. Renames map a panel field onto the endpoint's name for it.
var (
	openVPNServerTLSFields = []string{
		"certificate", "certificate_path", "key", "key_path",
		"client_certificate", "peer_fingerprint", "crl_path",
	}
	openVPNServerTLSRenames = map[string]string{
		"min_version": "version_min",
		"max_version": "version_max",
	}

	openVPNClientTLSFields = []string{
		"server_name", "certificate", "certificate_path",
		"client_certificate", "client_certificate_path",
		"client_key", "client_key_path", "peer_fingerprint", "crl_path",
	}
	openVPNClientTLSRenames = map[string]string{
		"min_version": "version_min",
		"max_version": "version_max",
	}

	openConnectTLSFields = []string{
		"server_name", "insecure",
		"client_certificate", "client_certificate_path",
		"client_key", "client_key_path", "peer_fingerprint",
	}
	// OpenConnect names the trust anchor after its role rather than reusing
	// the bare `certificate` the other TLS structs use.
	openConnectTLSRenames = map[string]string{
		"certificate":      "certificate_authority",
		"certificate_path": "certificate_authority_path",
	}
)

// withOpenVPNServerClientAuth folds in the two client-auth fields that need
// more than a rename. The panel stores client CA paths as a list, since
// sing-box inbounds accept several, while OpenVPN takes a single one; and the
// two sides describe verification with different vocabularies.
func withOpenVPNServerClientAuth(projected json.RawMessage, source map[string]json.RawMessage) (json.RawMessage, error) {
	fields := make(map[string]json.RawMessage)
	if projected != nil {
		if err := json.Unmarshal(projected, &fields); err != nil {
			return nil, err
		}
	}

	if raw, ok := source["client_certificate_path"]; ok && !isEmptyJSON(raw) {
		var paths []string
		if err := json.Unmarshal(raw, &paths); err == nil && len(paths) > 0 {
			encoded, err := json.Marshal(paths[0])
			if err != nil {
				return nil, err
			}
			fields["client_certificate_path"] = encoded
		}
	}

	if raw, ok := source["client_authentication"]; ok && !isEmptyJSON(raw) {
		var authType string
		if err := json.Unmarshal(raw, &authType); err == nil {
			if verify := verifyClientCertificateFor(authType); verify != "" {
				encoded, err := json.Marshal(verify)
				if err != nil {
					return nil, err
				}
				fields["verify_client_certificate"] = encoded
			}
		}
	}

	if len(fields) == 0 {
		return nil, nil
	}
	return json.Marshal(fields)
}

// verifyClientCertificateFor maps sing-box's client authentication modes onto
// the three OpenVPN understands. An empty result means leave it unset.
func verifyClientCertificateFor(clientAuthentication string) string {
	switch clientAuthentication {
	case "require-any", "require-and-verify":
		return "require"
	case "request", "verify-if-given":
		return "optional"
	default:
		return ""
	}
}

func projectTLSFields(source map[string]json.RawMessage, fields []string, renames map[string]string) (json.RawMessage, error) {
	projected := make(map[string]json.RawMessage)
	for _, field := range fields {
		if value, ok := source[field]; ok && !isEmptyJSON(value) {
			projected[field] = value
		}
	}
	for field, renamed := range renames {
		if value, ok := source[field]; ok && !isEmptyJSON(value) {
			projected[renamed] = value
		}
	}
	if len(projected) == 0 {
		// sing-box treats an empty object as no TLS options at all, so there is
		// nothing to gain from emitting one.
		return nil, nil
	}
	return json.Marshal(projected)
}

// isEmptyJSON reports whether a value carries nothing worth copying, so an
// unset field in the panel does not become an explicit empty one on the
// endpoint.
func isEmptyJSON(value json.RawMessage) bool {
	switch string(value) {
	case "", "null", `""`, "[]", "{}", "false":
		return true
	}
	return false
}

// unsupportedEndpointTLS lists the panel TLS fields that carry real behaviour
// but have no equivalent on this endpoint type, so the UI can warn before an
// operator picks a config that will not do what they expect. Fields that simply
// do not apply to a VPN tunnel (alpn, utls, fragmentation) are not listed.
func unsupportedEndpointTLS(endpointType string, tlsConfig *Tls) ([]string, error) {
	var (
		source  map[string]json.RawMessage
		raw     json.RawMessage
		checked []string
	)
	switch endpointType {
	case "openvpn-server":
		raw = tlsConfig.Server
		checked = []string{"certificate_provider", "acme", "reality", "ech", "client_certificate_public_key_sha256"}
	case "openvpn-client":
		raw = tlsConfig.Client
		// OpenVPN always verifies its peer, so a config relying on `insecure`
		// will not connect without a certificate authority.
		checked = []string{"insecure", "reality", "ech", "certificate_public_key_sha256"}
	case "openconnect":
		raw = tlsConfig.Client
		checked = []string{"reality", "ech", "certificate_public_key_sha256"}
	default:
		return nil, nil
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	var unsupported []string
	for _, field := range checked {
		if value, ok := source[field]; ok && !isEmptyJSON(value) {
			unsupported = append(unsupported, field)
		}
	}
	return unsupported, nil
}
