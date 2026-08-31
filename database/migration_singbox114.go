package database

import (
	"encoding/json"
	"log"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

// migratedKeySingBox114 marks that the one-shot sing-box 1.14 option migration
// has already run, so it is not attempted again on every start.
const migratedKeySingBox114 = "migratedSingBox114"

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
func migrateSingBox114() error {
	var flag model.Setting
	err := db.Where("key = ?", migratedKeySingBox114).First(&flag).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
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
			log.Printf("sing-box 1.14: migrated %d deprecated option(s)", changed)
		}
		return tx.Create(&model.Setting{Key: migratedKeySingBox114, Value: "true"}).Error
	})
}

// migrateSingBox114Config migrates the dns, experimental and route sections
// held in the `config` setting, returning how many options it rewrote.
func migrateSingBox114Config(tx *gorm.DB) (int, error) {
	var setting model.Setting
	err := tx.Where("key = ?", "config").First(&setting).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	var root map[string]json.RawMessage
	if err = json.Unmarshal([]byte(setting.Value), &root); err != nil {
		// A config we cannot parse is left untouched rather than discarded.
		log.Printf("sing-box 1.14: skipping config migration, cannot parse: %v", err)
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
	setting.Value = string(encoded)
	if err = tx.Save(&setting).Error; err != nil {
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

// migrateSingBox114Tls moves an inline acme block on every stored TLS config
// into certificate_provider, which carries the same fields under a type.
func migrateSingBox114Tls(tx *gorm.DB) (int, error) {
	var configs []model.Tls
	if err := tx.Find(&configs).Error; err != nil {
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
		if err = tx.Model(&model.Tls{}).Where("id = ?", tlsConfig.Id).
			Update("server", json.RawMessage(encoded)).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// reportSingBox114Manual logs the 1.14 deprecations that need a human decision.
// Both change how DNS names resolve, so they are reported rather than rewritten.
func reportSingBox114Manual() {
	var setting model.Setting
	if err := db.Where("key = ?", "config").First(&setting).Error; err != nil {
		return
	}
	var root struct {
		DNS struct {
			Rules []map[string]any `json:"rules"`
		} `json:"dns"`
	}
	if err := json.Unmarshal([]byte(setting.Value), &root); err != nil {
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
		log.Printf("sing-box 1.14: %d DNS rule(s) use legacy address filters (ip_cidr, ip_is_private, "+
			"ip_accept_any or an IP rule-set). They still work but are removed in 1.16; migrate them to "+
			"match_response: https://sing-box.sagernet.org/migration/#migrate-address-filter-fields-to-response-matching",
			addressFilter)
	}
	if strategy > 0 {
		log.Printf("sing-box 1.14: %d DNS rule(s) use the deprecated `strategy` action option. "+
			"They still work but are removed in 1.16; migrate them to rule items: "+
			"https://sing-box.sagernet.org/migration/#migrate-dns-rule-action-strategy-to-rule-items",
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
