package database

import (
	"encoding/json"
	"log"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

// migratedKeyHysteriaQuic marks that stored hysteria objects have been moved
// off the receive-window options hysteria defined for itself.
const migratedKeyHysteriaQuic = "migratedHysteriaQuic"

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

func migrateHysteriaQUICFields() error {
	var flag model.Setting
	err := db.Where("key = ?", migratedKeyHysteriaQuic).First(&flag).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
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
			log.Printf("hysteria: renamed %d deprecated window option(s) to their QUIC equivalents", changed)
		}
		return tx.Create(&model.Setting{Key: migratedKeyHysteriaQuic, Value: "true"}).Error
	})
}

func migrateHysteriaInbounds(tx *gorm.DB) (int, error) {
	var inbounds []model.Inbound
	if err := tx.Where("type = ?", "hysteria").Find(&inbounds).Error; err != nil {
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
		if !optionsChanged && !outChanged {
			continue
		}
		updates := map[string]any{}
		if optionsChanged {
			updates["options"] = options
		}
		if outChanged {
			updates["out_json"] = outJson
		}
		if err = tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
			Updates(updates).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

func migrateHysteriaOutbounds(tx *gorm.DB) (int, error) {
	var outbounds []model.Outbound
	if err := tx.Where("type = ?", "hysteria").Find(&outbounds).Error; err != nil {
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
		if err = tx.Model(&model.Outbound{}).Where("id = ?", outbound.Id).
			Update("options", options).Error; err != nil {
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
