package database

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

// migratedKeyCertProviders marks that inline certificate providers have already
// been hoisted into the shared certificate_providers list.
const migratedKeyCertProviders = "migratedCertProviders"

// migrateCertificateProviders turns the inline certificate_provider object a
// TLS config may carry into an entry in the config's certificate_providers
// list, leaving the TLS config with a tag reference to it.
//
// sing-box accepts both forms, so this is not a compatibility fix: it is what
// lets the panel edit providers in one place and share them between TLS
// configs, which an inline object cannot do.
func migrateCertificateProviders() error {
	var flag model.Setting
	err := db.Where("key = ?", migratedKeyCertProviders).First(&flag).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		changed, err := hoistInlineCertificateProviders(tx)
		if err != nil {
			return err
		}
		if changed > 0 {
			log.Printf("certificate providers: moved %d inline provider(s) into certificate_providers", changed)
		}
		return tx.Create(&model.Setting{Key: migratedKeyCertProviders, Value: "true"}).Error
	})
}

func hoistInlineCertificateProviders(tx *gorm.DB) (int, error) {
	var tlsConfigs []model.Tls
	if err := tx.Find(&tlsConfigs).Error; err != nil {
		return 0, err
	}

	var setting model.Setting
	err := tx.Where("key = ?", "config").First(&setting).Error
	if err == gorm.ErrRecordNotFound {
		// Without a base config there is nowhere to put shared providers, so
		// the inline ones are left as they are. They still work.
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var root map[string]json.RawMessage
	if err = json.Unmarshal([]byte(setting.Value), &root); err != nil {
		log.Printf("certificate providers: skipping, cannot parse config: %v", err)
		return 0, nil
	}

	providers, takenTags, err := decodeCertificateProviders(root)
	if err != nil {
		log.Printf("certificate providers: skipping, cannot parse certificate_providers: %v", err)
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
		if err = tx.Model(&model.Tls{}).Where("id = ?", tlsConfig.Id).
			Update("server", json.RawMessage(encodedServer)).Error; err != nil {
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
	setting.Value = string(encodedRoot)
	if err = tx.Save(&setting).Error; err != nil {
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
func providerTagBase(tlsConfig model.Tls) string {
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
