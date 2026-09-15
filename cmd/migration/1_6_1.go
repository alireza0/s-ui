package migration

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// OpenConnect and OpenVPN endpoints used to point at a panel TLS config, which
// the core projected into the shape each one accepts. The projection could only
// ever carry the handful of fields the two vocabularies share, so an OpenVPN
// endpoint set up that way never had enough to build a TLS session; those
// endpoints now hold their own `tls` object written in sing-box's own names.
//
// to1_6_1 runs the old projection one last time and stores the result inline,
// so an endpoint that did work keeps working and one that did not at least
// keeps the certificates the operator chose, ready to be completed in the
// endpoint's own form. It then drops the options each OpenVPN mode rejects and
// settles the client-certificate policy a server left unstated.
func to1_6_1(tx *gorm.DB) error {
	// A database from before endpoints existed has nothing to move.
	if !tx.Migrator().HasTable("endpoints") {
		return nil
	}
	// A database created after the column was dropped has nothing to move.
	if tx.Migrator().HasColumn("endpoints", "tls_id") {
		changed, err := inlineEndpointTls(tx)
		if err != nil {
			return err
		}
		if changed > 0 {
			fmt.Printf("endpoint tls: moved %d shared TLS config(s) into the endpoint(s) that used them\n", changed)
		}
	}
	cleaned, err := dropModeConflictingOpenVPNOptions(tx)
	if err != nil {
		return err
	}
	if cleaned > 0 {
		fmt.Printf("endpoint tls: dropped options from %d openvpn endpoint(s) that do not belong to their mode\n", cleaned)
	}
	settled, err := settleOpenVPNClientCertificatePolicy(tx)
	if err != nil {
		return err
	}
	if settled > 0 {
		fmt.Printf("endpoint tls: %d openvpn server(s) asked for client certificates with no CA to check them against, now set to not ask\n", settled)
	}
	return nil
}

// The two OpenVPN modes each reject the other's options outright, and the panel
// used to offer `cipher` in both. An endpoint carrying one refuses to start, so
// the option that cannot apply is dropped rather than left to fail at runtime.
//
// static_key names a single fixed cipher; a TLS session negotiates one out of
// data_ciphers instead. The remaining names are ones a TLS-mode endpoint has no
// use for either way.
var openVPNModeConflicts = map[string][]string{
	"tls":        {"cipher", "key_direction", "static_key", "static_key_path"},
	"static_key": {"data_ciphers", "data_ciphers_fallback"},
}

// openVPNServerTLSConflicts are rejected on a server in TLS mode on top of the
// shared list: they describe the single peer a static_key tunnel has.
var openVPNServerTLSConflicts = []string{"remote", "remote_port", "peer_address", "peer_address_ipv6"}

func dropModeConflictingOpenVPNOptions(tx *gorm.DB) (int, error) {
	var rows []endpointTlsRow
	err := tx.Raw("SELECT id, type, options FROM endpoints WHERE type in ?",
		[]string{"openvpn-server", "openvpn-client"}).Scan(&rows).Error
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, row := range rows {
		options := make(map[string]json.RawMessage)
		if len(row.Options) > 0 {
			if err = json.Unmarshal(row.Options, &options); err != nil {
				fmt.Printf("endpoint tls: skipping endpoint %d, cannot parse its options: %v\n", row.Id, err)
				continue
			}
		}
		// sing-box reads a missing mode as tls.
		mode := "tls"
		if raw, ok := options["mode"]; ok {
			var stored string
			if err = json.Unmarshal(raw, &stored); err == nil && stored != "" {
				mode = stored
			}
		}
		conflicting := openVPNModeConflicts[mode]
		if mode == "tls" && row.Type == "openvpn-server" {
			conflicting = append(append([]string{}, conflicting...), openVPNServerTLSConflicts...)
		}

		dropped := false
		for _, field := range conflicting {
			if value, ok := options[field]; ok {
				// An empty value is not what sing-box refuses, and removing it
				// would rewrite rows that were fine.
				if isEmptyJSON(value) || string(value) == "0" {
					continue
				}
				delete(options, field)
				dropped = true
			}
		}
		if !dropped {
			continue
		}
		encoded, err := json.MarshalIndent(options, "", "  ")
		if err != nil {
			return 0, err
		}
		if err = tx.Exec("UPDATE endpoints SET options = ? WHERE id = ?", encoded, row.Id).Error; err != nil {
			return 0, err
		}
		cleaned++
	}
	return cleaned, nil
}

// endpointTlsRow reads the columns the migration needs by name, since the
// endpoint model no longer declares tls_id.
type endpointTlsRow struct {
	Id      uint
	Type    string
	TlsId   uint
	Options json.RawMessage
}

func inlineEndpointTls(tx *gorm.DB) (int, error) {
	var rows []endpointTlsRow
	err := tx.Raw("SELECT id, type, tls_id, options FROM endpoints WHERE tls_id > 0").Scan(&rows).Error
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	tlsConfigs, err := readTlsRows(tx)
	if err != nil {
		return 0, err
	}
	byId := make(map[uint]*tlsRow, len(tlsConfigs))
	for i := range tlsConfigs {
		byId[tlsConfigs[i].Id] = &tlsConfigs[i]
	}

	changed := 0
	for _, row := range rows {
		tlsConfig, ok := byId[row.TlsId]
		if !ok {
			// The config was deleted out from under the endpoint. Clearing the
			// reference is all that is left to do.
			if err = tx.Exec("UPDATE endpoints SET tls_id = 0 WHERE id = ?", row.Id).Error; err != nil {
				return 0, err
			}
			continue
		}
		projected, err := projectEndpointTLS(row.Type, tlsConfig)
		if err != nil {
			fmt.Printf("endpoint tls: skipping endpoint %d, cannot read TLS config %d: %v\n", row.Id, row.TlsId, err)
			continue
		}

		options := make(map[string]json.RawMessage)
		if len(row.Options) > 0 {
			if err = json.Unmarshal(row.Options, &options); err != nil {
				fmt.Printf("endpoint tls: skipping endpoint %d, cannot parse its options: %v\n", row.Id, err)
				continue
			}
		}
		// An endpoint that already carries its own tls object was written after
		// the change and keeps what it has.
		if _, exists := options["tls"]; !exists && projected != nil {
			options["tls"] = projected
			changed++
		}
		encoded, err := json.MarshalIndent(options, "", "  ")
		if err != nil {
			return 0, err
		}
		if err = tx.Exec("UPDATE endpoints SET options = ?, tls_id = 0 WHERE id = ?", encoded, row.Id).Error; err != nil {
			return 0, err
		}
	}
	return changed, nil
}

// projectEndpointTLS copies a panel TLS config into the field names the
// endpoint type uses. Whatever the endpoint has no equivalent for is left out.
func projectEndpointTLS(endpointType string, tlsConfig *tlsRow) (json.RawMessage, error) {
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

// sing-box reads an unset `verify_client_certificate` as "require", so an
// OpenVPN server that never mentioned client certificates still demands one and
// then refuses to start for want of a CA to check it against:
//
//	tls mode: either certificate-authority or peer-fingerprint must be configured
//
// settleOpenVPNClientCertificatePolicy writes the choice such a server was
// already making by omission. A server that names a CA, pins a fingerprint, or
// states a policy of its own is left alone: only the unstated case is filled in,
// so nothing an operator actually asked for is weakened.
func settleOpenVPNClientCertificatePolicy(tx *gorm.DB) (int, error) {
	var rows []endpointTlsRow
	err := tx.Raw("SELECT id, type, options FROM endpoints WHERE type = ?", "openvpn-server").Scan(&rows).Error
	if err != nil {
		return 0, err
	}

	settled := 0
	for _, row := range rows {
		options := make(map[string]json.RawMessage)
		if len(row.Options) > 0 {
			if err = json.Unmarshal(row.Options, &options); err != nil {
				continue
			}
		}
		if mode, ok := options["mode"]; ok && !isEmptyJSON(mode) && string(mode) != `"tls"` {
			continue
		}
		rawTLS, ok := options["tls"]
		if !ok || isEmptyJSON(rawTLS) {
			// No TLS block at all: the endpoint has no certificate either, and
			// giving it a policy would not make it start.
			continue
		}
		var tlsOptions map[string]json.RawMessage
		if err = json.Unmarshal(rawTLS, &tlsOptions); err != nil {
			continue
		}
		if _, stated := tlsOptions["verify_client_certificate"]; stated {
			continue
		}
		if hasAnyOf(tlsOptions, "client_certificate", "client_certificate_path", "peer_fingerprint") {
			continue
		}

		tlsOptions["verify_client_certificate"] = json.RawMessage(`"none"`)
		encodedTLS, err := json.Marshal(tlsOptions)
		if err != nil {
			return 0, err
		}
		options["tls"] = encodedTLS
		encoded, err := json.MarshalIndent(options, "", "  ")
		if err != nil {
			return 0, err
		}
		if err = tx.Exec("UPDATE endpoints SET options = ? WHERE id = ?", encoded, row.Id).Error; err != nil {
			return 0, err
		}
		settled++
	}
	return settled, nil
}

func hasAnyOf(fields map[string]json.RawMessage, names ...string) bool {
	for _, name := range names {
		if value, ok := fields[name]; ok && !isEmptyJSON(value) {
			return true
		}
	}
	return false
}
