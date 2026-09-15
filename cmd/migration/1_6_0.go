package migration

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// to1_6_0 brings a database up to what 1.6.0 expects of the objects it stores:
// the options sing-box deprecated in 1.14 are rewritten into their
// replacements, inline certificate providers are hoisted into the shared
// certificate_providers list, rule-set HTTP clients are stripped of the
// settings sing-box refuses, hysteria's own window options are renamed onto the
// shared QUIC fields, and options sing-box no longer acts on are cleared out.
//
// Every step below is written to be re-runnable: it looks for the old shape and
// leaves anything already converted alone.
func to1_6_0(tx *gorm.DB) error {
	if err := migrateSingBox114(tx); err != nil {
		return err
	}
	if err := migrateCertificateProviders(tx); err != nil {
		return err
	}
	if err := repairRuleSetHTTPClients(tx); err != nil {
		return err
	}
	if err := migrateHysteriaQUICFields(tx); err != nil {
		return err
	}
	if err := migrateRemovedOptions(tx); err != nil {
		return err
	}
	reportSingBox114Manual(tx)
	return nil
}

// settingRow reads the settings table by column name, so a database that has
// not been through AutoMigrate yet -- migrations run before it -- is not asked
// for columns the model has grown since.
type settingRow struct {
	Value string
}

// readSetting returns a setting's value and whether the row exists at all,
// which an empty value cannot say on its own.
func readSetting(tx *gorm.DB, key string) (string, bool, error) {
	var rows []settingRow
	if err := tx.Raw("SELECT value FROM settings WHERE key = ?", key).Scan(&rows).Error; err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return rows[0].Value, true, nil
}

func writeSetting(tx *gorm.DB, key, value string) error {
	return tx.Exec("UPDATE settings SET value = ? WHERE key = ?", value, key).Error
}

// migrateSingBox114 rewrites the options sing-box deprecated in 1.14.0 into
// their replacements. Only the lossless rewrites are done automatically:
//
//   - dns.independent_cache is dropped; 1.14 ignores the field entirely
//   - experimental.cache_file.store_rdrc becomes store_dns
//   - route.rule_set[].download_detour becomes http_client
//   - an inline tls.acme block becomes tls.certificate_provider
//
// The two remaining 1.14 deprecations, legacy DNS address filters and the
// `strategy` DNS rule action option, are deliberately left alone: migrating
// them means reordering DNS rules and inserting evaluate actions, which
// changes how names resolve. Those are reported by reportSingBox114Manual so
// the operator can convert them by hand.
func migrateSingBox114(tx *gorm.DB) error {
	changed, err := migrateSingBox114Config(tx)
	if err != nil {
		return err
	}
	tlsChanged, err := migrateSingBox114Tls(tx)
	if err != nil {
		return err
	}
	changed += tlsChanged
	if changed > 0 {
		fmt.Printf("sing-box 1.14: migrated %d deprecated option(s)\n", changed)
	}
	return nil
}

// migrateSingBox114Config migrates the dns, experimental and route sections
// held in the `config` setting, returning how many options it rewrote.
func migrateSingBox114Config(tx *gorm.DB) (int, error) {
	value, exists, err := readSetting(tx, "config")
	if err != nil || !exists {
		return 0, err
	}

	var root map[string]json.RawMessage
	if err = json.Unmarshal([]byte(value), &root); err != nil {
		// A config we cannot parse is left untouched rather than discarded.
		fmt.Printf("sing-box 1.14: skipping config migration, cannot parse: %v\n", err)
		return 0, nil
	}

	changed := 0
	sections := []struct {
		key     string
		migrate func(map[string]any) int
	}{
		{"dns", migrateDNSSection},
		{"experimental", migrateExperimentalSection},
		{"route", migrateRouteSection},
	}
	for _, section := range sections {
		if err = migrateSection(root, section.key, section.migrate, &changed); err != nil {
			return 0, err
		}
	}
	if changed == 0 {
		return 0, nil
	}

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return 0, err
	}
	if err = writeSetting(tx, "config", string(encoded)); err != nil {
		return 0, err
	}
	return changed, nil
}

// migrateSection decodes one top-level config section, hands it to migrate and
// writes it back when migrate reports a change.
func migrateSection(root map[string]json.RawMessage, key string, migrate func(map[string]any) int, changed *int) error {
	raw, ok := root[key]
	if !ok || len(raw) == 0 {
		return nil
	}
	var section map[string]any
	if err := json.Unmarshal(raw, &section); err != nil {
		return nil
	}
	count := migrate(section)
	if count == 0 {
		return nil
	}
	encoded, err := json.Marshal(section)
	if err != nil {
		return err
	}
	root[key] = encoded
	*changed += count
	return nil
}

// migrateDNSSection drops independent_cache, which 1.14 reads only to warn on.
func migrateDNSSection(dns map[string]any) int {
	if _, ok := dns["independent_cache"]; !ok {
		return 0
	}
	delete(dns, "independent_cache")
	return 1
}

// migrateExperimentalSection renames cache_file.store_rdrc to store_dns.
func migrateExperimentalSection(experimental map[string]any) int {
	cacheFile, ok := experimental["cache_file"].(map[string]any)
	if !ok {
		return 0
	}
	storeRDRC, ok := cacheFile["store_rdrc"]
	if !ok {
		return 0
	}
	delete(cacheFile, "store_rdrc")
	if enabled, isBool := storeRDRC.(bool); isBool && enabled {
		cacheFile["store_dns"] = true
	}
	return 1
}

// migrateRouteSection converts each remote rule-set's download_detour into the
// equivalent http_client.
//
// A detour to `direct` is the one case that does not carry over as a detour:
// download_detour set the internal disable_empty_direct_check flag, which has
// no JSON field of its own, and without it sing-box rejects a detour to an
// empty direct outbound as pointless. Downloading over the default outbound is
// what that detour meant anyway, so the field is simply dropped.
func migrateRouteSection(route map[string]any) int {
	ruleSets, ok := route["rule_set"].([]any)
	if !ok {
		return 0
	}
	changed := 0
	for _, entry := range ruleSets {
		ruleSet, isObject := entry.(map[string]any)
		if !isObject {
			continue
		}
		detour, hasDetour := ruleSet["download_detour"].(string)
		if !hasDetour {
			continue
		}
		delete(ruleSet, "download_detour")
		// http_client and download_detour conflict in 1.14, so an existing
		// http_client wins and the legacy field is simply dropped.
		if _, hasClient := ruleSet["http_client"]; hasClient {
			changed++
			continue
		}
		if detour != "" && detour != "direct" {
			ruleSet["http_client"] = map[string]any{"detour": detour}
		}
		changed++
	}
	return changed
}

// tlsRow reads the TLS table by column name, for the same reason settingRow
// does: the migration runs before the schema is brought up to date.
type tlsRow struct {
	Id     uint
	Name   string
	Server json.RawMessage
	Client json.RawMessage
}

func readTlsRows(tx *gorm.DB) ([]tlsRow, error) {
	var rows []tlsRow
	err := tx.Raw("SELECT id, name, server, client FROM tls").Scan(&rows).Error
	return rows, err
}

// migrateSingBox114Tls moves an inline acme block on every stored TLS config
// into certificate_provider, which carries the same fields under a type.
func migrateSingBox114Tls(tx *gorm.DB) (int, error) {
	configs, err := readTlsRows(tx)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, tlsConfig := range configs {
		if len(tlsConfig.Server) == 0 {
			continue
		}
		var server map[string]any
		if err := json.Unmarshal(tlsConfig.Server, &server); err != nil {
			continue
		}
		acme, ok := server["acme"].(map[string]any)
		if !ok {
			continue
		}
		delete(server, "acme")
		// certificate_provider and acme cannot both apply, so an existing
		// provider wins and the legacy block is dropped.
		if _, hasProvider := server["certificate_provider"]; !hasProvider {
			provider := make(map[string]any, len(acme)+1)
			for key, value := range acme {
				provider[key] = value
			}
			provider["type"] = "acme"
			server["certificate_provider"] = provider
		}
		encoded, err := json.Marshal(server)
		if err != nil {
			return 0, err
		}
		if err = tx.Exec("UPDATE tls SET server = ? WHERE id = ?", encoded, tlsConfig.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// reportSingBox114Manual prints the 1.14 deprecations that need a human
// decision. Both change how DNS names resolve, so they are reported rather
// than rewritten.
func reportSingBox114Manual(tx *gorm.DB) {
	value, exists, err := readSetting(tx, "config")
	if err != nil || !exists {
		return
	}
	var root struct {
		DNS struct {
			Rules []map[string]any `json:"rules"`
		} `json:"dns"`
	}
	if err := json.Unmarshal([]byte(value), &root); err != nil {
		return
	}

	addressFilter, strategy := 0, 0
	for _, rule := range root.DNS.Rules {
		if dnsRuleUsesAddressFilter(rule) {
			addressFilter++
		}
		if _, ok := rule["strategy"]; ok {
			strategy++
		}
	}
	if addressFilter > 0 {
		fmt.Printf("sing-box 1.14: %d DNS rule(s) use legacy address filters (ip_cidr, ip_is_private, "+
			"ip_accept_any or an IP rule-set). They still work but are removed in 1.16; migrate them to "+
			"match_response: https://sing-box.sagernet.org/migration/#migrate-address-filter-fields-to-response-matching\n",
			addressFilter)
	}
	if strategy > 0 {
		fmt.Printf("sing-box 1.14: %d DNS rule(s) use the deprecated `strategy` action option. "+
			"They still work but are removed in 1.16; migrate them to rule items: "+
			"https://sing-box.sagernet.org/migration/#migrate-dns-rule-action-strategy-to-rule-items\n",
			strategy)
	}
}

// dnsRuleUsesAddressFilter reports whether a DNS rule matches on the resolved
// address without opting into response matching.
func dnsRuleUsesAddressFilter(rule map[string]any) bool {
	if matchResponse, ok := rule["match_response"].(bool); ok && matchResponse {
		return false
	}
	for _, field := range []string{"ip_cidr", "rule_set", "ip_is_private", "ip_accept_any", "rule_set_ip_cidr_accept_empty"} {
		value, ok := rule[field]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			if typed {
				return true
			}
		case []any:
			if len(typed) > 0 {
				return true
			}
		case string:
			if typed != "" {
				return true
			}
		}
	}
	return false
}

// migrateCertificateProviders turns the inline certificate_provider object a
// TLS config may carry into an entry in the config's certificate_providers
// list, leaving the TLS config with a tag reference to it.
//
// sing-box accepts both forms, so this is not a compatibility fix: it is what
// lets the panel edit providers in one place and share them between TLS
// configs, which an inline object cannot do.
func migrateCertificateProviders(tx *gorm.DB) error {
	changed, err := hoistInlineCertificateProviders(tx)
	if err != nil {
		return err
	}
	if changed > 0 {
		fmt.Printf("certificate providers: moved %d inline provider(s) into certificate_providers\n", changed)
	}
	return nil
}

func hoistInlineCertificateProviders(tx *gorm.DB) (int, error) {
	tlsConfigs, err := readTlsRows(tx)
	if err != nil {
		return 0, err
	}

	value, exists, err := readSetting(tx, "config")
	if err != nil {
		return 0, err
	}
	if !exists {
		// Without a base config there is nowhere to put shared providers, so
		// the inline ones are left as they are. They still work.
		return 0, nil
	}
	var root map[string]json.RawMessage
	if err = json.Unmarshal([]byte(value), &root); err != nil {
		fmt.Printf("certificate providers: skipping, cannot parse config: %v\n", err)
		return 0, nil
	}

	providers, takenTags, err := decodeCertificateProviders(root)
	if err != nil {
		fmt.Printf("certificate providers: skipping, cannot parse certificate_providers: %v\n", err)
		return 0, nil
	}

	changed := 0
	for _, tlsConfig := range tlsConfigs {
		if len(tlsConfig.Server) == 0 {
			continue
		}
		var server map[string]json.RawMessage
		if err = json.Unmarshal(tlsConfig.Server, &server); err != nil {
			continue
		}
		inline, ok := inlineProviderObject(server["certificate_provider"])
		if !ok {
			continue
		}

		tag := uniqueProviderTag(providerTagBase(tlsConfig), takenTags)
		takenTags[tag] = struct{}{}
		inline["tag"] = json.RawMessage(strconv.Quote(tag))
		encodedProvider, err := json.Marshal(inline)
		if err != nil {
			return 0, err
		}
		providers = append(providers, encodedProvider)

		server["certificate_provider"] = json.RawMessage(strconv.Quote(tag))
		encodedServer, err := json.Marshal(server)
		if err != nil {
			return 0, err
		}
		if err = tx.Exec("UPDATE tls SET server = ? WHERE id = ?", encodedServer, tlsConfig.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	if changed == 0 {
		return 0, nil
	}

	encodedProviders, err := json.Marshal(providers)
	if err != nil {
		return 0, err
	}
	root["certificate_providers"] = encodedProviders
	encodedRoot, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return 0, err
	}
	if err = writeSetting(tx, "config", string(encodedRoot)); err != nil {
		return 0, err
	}
	return changed, nil
}

// decodeCertificateProviders reads the providers already defined in the config
// and the tags they occupy, so hoisted ones do not collide with them.
func decodeCertificateProviders(root map[string]json.RawMessage) ([]json.RawMessage, map[string]struct{}, error) {
	taken := make(map[string]struct{})
	raw, ok := root["certificate_providers"]
	if !ok || len(raw) == 0 {
		return nil, taken, nil
	}
	var providers []json.RawMessage
	if err := json.Unmarshal(raw, &providers); err != nil {
		return nil, nil, err
	}
	for _, provider := range providers {
		var fields struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(provider, &fields); err != nil {
			continue
		}
		if fields.Tag != "" {
			taken[fields.Tag] = struct{}{}
		}
	}
	return providers, taken, nil
}

// inlineProviderObject reports whether certificate_provider holds an inline
// definition rather than a tag reference to a shared one.
func inlineProviderObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}
	var inline map[string]json.RawMessage
	if err := json.Unmarshal(raw, &inline); err != nil {
		return nil, false
	}
	// A provider with no type cannot be created by sing-box either way, so
	// there is nothing worth moving.
	if _, ok := inline["type"]; !ok {
		return nil, false
	}
	return inline, true
}

// providerTagBase names the hoisted provider after the TLS config it came
// from, which is how the operator will recognise it in the list.
func providerTagBase(tlsConfig tlsRow) string {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, tlsConfig.Name)
	slug = strings.Trim(slug, "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if slug == "" {
		return "cert-" + strconv.FormatUint(uint64(tlsConfig.Id), 10)
	}
	return slug
}

func uniqueProviderTag(base string, taken map[string]struct{}) string {
	tag := base
	for i := 2; ; i++ {
		if _, exists := taken[tag]; !exists {
			return tag
		}
		tag = base + "-" + strconv.Itoa(i)
	}
}

// repairRuleSetHTTPClients removes the two http_client settings that stop a
// config from loading:
//
//   - disable_empty_direct_check, which is an internal flag with no JSON field
//     of its own. sing-box rejects the whole config over the unknown key.
//   - a detour to `direct`, which sing-box refuses as pointless because it
//     dials exactly what it would have dialled anyway.
//
// Both come from an earlier rewrite of the deprecated download_detour option,
// which set the internal flag rather than exposing it.
func repairRuleSetHTTPClients(tx *gorm.DB) error {
	changed, err := repairRuleSetHTTPClientsIn(tx)
	if err != nil {
		return err
	}
	if changed > 0 {
		fmt.Printf("http clients: repaired %d rule-set http_client option(s)\n", changed)
	}
	return nil
}

func repairRuleSetHTTPClientsIn(tx *gorm.DB) (int, error) {
	value, exists, err := readSetting(tx, "config")
	if err != nil || !exists {
		return 0, err
	}
	var root map[string]json.RawMessage
	if err = json.Unmarshal([]byte(value), &root); err != nil {
		fmt.Printf("http clients: skipping, cannot parse config: %v\n", err)
		return 0, nil
	}
	rawRoute, ok := root["route"]
	if !ok || len(rawRoute) == 0 {
		return 0, nil
	}
	var route map[string]any
	if err = json.Unmarshal(rawRoute, &route); err != nil {
		return 0, nil
	}

	changed := repairRuleSetList(route)
	if changed == 0 {
		return 0, nil
	}

	encodedRoute, err := json.Marshal(route)
	if err != nil {
		return 0, err
	}
	root["route"] = encodedRoute
	encodedRoot, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return 0, err
	}
	if err = writeSetting(tx, "config", string(encodedRoot)); err != nil {
		return 0, err
	}
	return changed, nil
}

func repairRuleSetList(route map[string]any) int {
	ruleSets, ok := route["rule_set"].([]any)
	if !ok {
		return 0
	}
	changed := 0
	for _, entry := range ruleSets {
		ruleSet, isObject := entry.(map[string]any)
		if !isObject {
			continue
		}
		httpClient, isObject := ruleSet["http_client"].(map[string]any)
		if !isObject {
			continue
		}
		fixed := false
		if _, ok := httpClient["disable_empty_direct_check"]; ok {
			delete(httpClient, "disable_empty_direct_check")
			fixed = true
		}
		if detour, isString := httpClient["detour"].(string); isString && detour == "direct" {
			delete(httpClient, "detour")
			fixed = true
		}
		if !fixed {
			continue
		}
		// An http_client with nothing left in it means the defaults, which is
		// what leaving it out says more clearly.
		if len(httpClient) == 0 {
			delete(ruleSet, "http_client")
		}
		changed++
	}
	return changed
}

// Hysteria's own window options are deprecated in favour of the QUIC fields
// every QUIC protocol now shares. sing-box still reads the old names, but only
// as a fallback when the QUIC field is unset, so the rename is exact.
//
// The two sides disagree on which window is which, so the maps are separate:
// what an inbound calls recv_window_client an outbound calls recv_window, and
// both mean the stream window.
var (
	hysteriaInboundQUICFields = map[string]string{
		"recv_window_conn":      "connection_receive_window",
		"recv_window_client":    "stream_receive_window",
		"max_conn_client":       "max_concurrent_streams",
		"disable_mtu_discovery": "disable_path_mtu_discovery",
	}
	hysteriaOutboundQUICFields = map[string]string{
		"recv_window_conn":      "connection_receive_window",
		"recv_window":           "stream_receive_window",
		"disable_mtu_discovery": "disable_path_mtu_discovery",
	}
)

func migrateHysteriaQUICFields(tx *gorm.DB) error {
	changed, err := migrateHysteriaInbounds(tx)
	if err != nil {
		return err
	}
	outboundsChanged, err := migrateHysteriaOutbounds(tx)
	if err != nil {
		return err
	}
	changed += outboundsChanged
	if changed > 0 {
		fmt.Printf("hysteria: renamed %d deprecated window option(s) to their QUIC equivalents\n", changed)
	}
	return nil
}

// inboundRow and outboundRow read only the columns the migration touches.
type inboundRow struct {
	Id      uint
	Options json.RawMessage
	OutJson json.RawMessage
}

type outboundRow struct {
	Id      uint
	Options json.RawMessage
}

func migrateHysteriaInbounds(tx *gorm.DB) (int, error) {
	var inbounds []inboundRow
	err := tx.Raw("SELECT id, options, out_json FROM inbounds WHERE type = ?", "hysteria").Scan(&inbounds).Error
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, inbound := range inbounds {
		options, optionsChanged, err := renameQUICFields(inbound.Options, hysteriaInboundQUICFields)
		if err != nil {
			return 0, err
		}
		// The client-side copy is an outbound, so it uses the outbound names.
		outJson, outChanged, err := renameQUICFields(inbound.OutJson, hysteriaOutboundQUICFields)
		if err != nil {
			return 0, err
		}
		switch {
		case optionsChanged && outChanged:
			err = tx.Exec("UPDATE inbounds SET options = ?, out_json = ? WHERE id = ?", options, outJson, inbound.Id).Error
		case optionsChanged:
			err = tx.Exec("UPDATE inbounds SET options = ? WHERE id = ?", options, inbound.Id).Error
		case outChanged:
			err = tx.Exec("UPDATE inbounds SET out_json = ? WHERE id = ?", outJson, inbound.Id).Error
		default:
			continue
		}
		if err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

func migrateHysteriaOutbounds(tx *gorm.DB) (int, error) {
	var outbounds []outboundRow
	err := tx.Raw("SELECT id, options FROM outbounds WHERE type = ?", "hysteria").Scan(&outbounds).Error
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, outbound := range outbounds {
		options, optionsChanged, err := renameQUICFields(outbound.Options, hysteriaOutboundQUICFields)
		if err != nil {
			return 0, err
		}
		if !optionsChanged {
			continue
		}
		if err = tx.Exec("UPDATE outbounds SET options = ? WHERE id = ?", options, outbound.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// renameQUICFields moves each deprecated option onto its QUIC name. A QUIC
// field that is already set wins, matching sing-box, which reads the old name
// only when the new one is unset.
func renameQUICFields(raw json.RawMessage, names map[string]string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return raw, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return raw, false, nil
	}
	changed := false
	for deprecated, quic := range names {
		value, ok := fields[deprecated]
		if !ok {
			continue
		}
		delete(fields, deprecated)
		changed = true
		if existing, taken := fields[quic]; taken && !isEmptyOptionValue(existing) {
			continue
		}
		if isEmptyOptionValue(value) {
			continue
		}
		fields[quic] = value
	}
	if !changed {
		return raw, false, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return raw, false, err
	}
	return encoded, true, nil
}

// isEmptyOptionValue reports whether a value carries nothing, so an unset
// deprecated option does not become an explicit zero on the QUIC side.
func isEmptyOptionValue(value json.RawMessage) bool {
	switch string(value) {
	case "", "null", "0", "false", `""`:
		return true
	}
	return false
}

// Options sing-box still parses but no longer acts on. They cost nothing to
// leave in place, but the panel no longer offers them, so a stored config would
// keep showing settings the UI cannot explain.
var (
	// Removed in sing-box 1.12.0; the tun stack handles this itself now.
	removedTunOptions = []string{"endpoint_independent_nat"}
	// Removed in sing-box 1.13.0.
	removedECHOptions = []string{"pq_signature_schemes_enabled", "dynamic_record_sizing_disabled"}
)

func migrateRemovedOptions(tx *gorm.DB) error {
	changed, err := clearRemovedTunOptions(tx)
	if err != nil {
		return err
	}
	echChanged, err := clearRemovedECHOptions(tx)
	if err != nil {
		return err
	}
	changed += echChanged
	if changed > 0 {
		fmt.Printf("removed options: cleared %d object(s) of settings sing-box no longer acts on\n", changed)
	}
	return nil
}

func clearRemovedTunOptions(tx *gorm.DB) (int, error) {
	var inbounds []inboundRow
	err := tx.Raw("SELECT id, options FROM inbounds WHERE type = ?", "tun").Scan(&inbounds).Error
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, inbound := range inbounds {
		options, ok, err := deleteJSONFields(inbound.Options, removedTunOptions)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		if err = tx.Exec("UPDATE inbounds SET options = ? WHERE id = ?", options, inbound.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// The ECH options live on the outbound side of a TLS config, which is what the
// panel hands to clients and share links.
func clearRemovedECHOptions(tx *gorm.DB) (int, error) {
	configs, err := readTlsRows(tx)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, tlsConfig := range configs {
		client, ok, err := deleteNestedJSONFields(tlsConfig.Client, "ech", removedECHOptions)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		if err = tx.Exec("UPDATE tls SET client = ? WHERE id = ?", client, tlsConfig.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

func deleteJSONFields(raw json.RawMessage, names []string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return raw, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return raw, false, nil
	}
	changed := false
	for _, name := range names {
		if _, ok := fields[name]; ok {
			delete(fields, name)
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return raw, false, err
	}
	return encoded, true, nil
}

// deleteNestedJSONFields clears fields from one object nested inside raw.
func deleteNestedJSONFields(raw json.RawMessage, key string, names []string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return raw, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return raw, false, nil
	}
	nested, ok := fields[key]
	if !ok {
		return raw, false, nil
	}
	cleaned, changed, err := deleteJSONFields(nested, names)
	if err != nil || !changed {
		return raw, false, err
	}
	fields[key] = cleaned
	encoded, err := json.Marshal(fields)
	if err != nil {
		return raw, false, err
	}
	return encoded, true, nil
}
