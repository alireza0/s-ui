package database

import (
	"encoding/json"
	"log"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

// migratedKeyRemovedOptions marks that options sing-box has removed have been
// cleared out of the stored objects.
const migratedKeyRemovedOptions = "migratedRemovedOptions"

// Options sing-box still parses but no longer acts on. They cost nothing to
// leave in place, but the panel no longer offers them, so a stored config would
// keep showing settings the UI cannot explain.
var (
	// Removed in sing-box 1.12.0; the tun stack handles this itself now.
	removedTunOptions = []string{"endpoint_independent_nat"}
	// Removed in sing-box 1.13.0.
	removedECHOptions = []string{"pq_signature_schemes_enabled", "dynamic_record_sizing_disabled"}
)

func migrateRemovedOptions() error {
	var flag model.Setting
	err := db.Where("key = ?", migratedKeyRemovedOptions).First(&flag).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
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
			log.Printf("removed options: cleared %d object(s) of settings sing-box no longer acts on", changed)
		}
		return tx.Create(&model.Setting{Key: migratedKeyRemovedOptions, Value: "true"}).Error
	})
}

func clearRemovedTunOptions(tx *gorm.DB) (int, error) {
	var inbounds []model.Inbound
	if err := tx.Where("type = ?", "tun").Find(&inbounds).Error; err != nil {
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
		if err = tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
			Update("options", options).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// The ECH options live on the outbound side of a TLS config, which is what the
// panel hands to clients and share links.
func clearRemovedECHOptions(tx *gorm.DB) (int, error) {
	var configs []model.Tls
	if err := tx.Find(&configs).Error; err != nil {
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
		if err = tx.Model(&model.Tls{}).Where("id = ?", tlsConfig.Id).
			Update("client", client).Error; err != nil {
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
