package migration

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// to1_6_4 finishes what 1.6.0 started for the ECH options sing-box removed in
// 1.13.0. That step cleared only the client side of each TLS config, but the
// options existed on the server side too, and the panel copied them from there
// into every generated client outbound. sing-box rejects either one when true,
// on the inbound and on the client alike. #1275
func to1_6_4(tx *gorm.DB) error {
	changed, err := clearServerECHOptions(tx)
	if err != nil {
		return err
	}
	outChanged, err := clearOutJsonECHOptions(tx)
	if err != nil {
		return err
	}
	changed += outChanged
	if changed > 0 {
		fmt.Printf("removed options: cleared legacy ECH options from %d object(s)\n", changed)
	}
	return nil
}

func clearServerECHOptions(tx *gorm.DB) (int, error) {
	if !tx.Migrator().HasTable("tls") {
		return 0, nil
	}
	configs, err := readTlsRows(tx)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, tlsConfig := range configs {
		server, ok, err := deleteNestedJSONFields(tlsConfig.Server, "ech", removedECHOptions)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		if err = tx.Exec("UPDATE tls SET server = ? WHERE id = ?", server, tlsConfig.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// The client outbound each inbound hands to subscriptions is stored, so what
// the panel already copied there stays until the inbound is saved again.
func clearOutJsonECHOptions(tx *gorm.DB) (int, error) {
	if !tx.Migrator().HasTable("inbounds") {
		return 0, nil
	}
	var inbounds []inboundRow
	if err := tx.Raw("SELECT id, out_json FROM inbounds WHERE tls_id > 0").Scan(&inbounds).Error; err != nil {
		return 0, err
	}
	changed := 0
	for _, inbound := range inbounds {
		outJson, ok, err := deleteOutJsonECHOptions(inbound.OutJson)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		if err = tx.Exec("UPDATE inbounds SET out_json = ? WHERE id = ?", outJson, inbound.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// deleteOutJsonECHOptions clears the options from tls.ech inside an out_json.
func deleteOutJsonECHOptions(raw json.RawMessage) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return raw, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return raw, false, nil
	}
	tls, ok := fields["tls"]
	if !ok {
		return raw, false, nil
	}
	cleaned, changed, err := deleteNestedJSONFields(tls, "ech", removedECHOptions)
	if err != nil || !changed {
		return raw, false, err
	}
	fields["tls"] = cleaned
	encoded, err := json.Marshal(fields)
	if err != nil {
		return raw, false, err
	}
	return encoded, true, nil
}
