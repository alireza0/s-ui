package migration

import (
	"encoding/json"

	"github.com/alireza0/s-ui/util"

	"gorm.io/gorm"
)

func to1_5_1(tx *gorm.DB) error {
	type tlsRow struct {
		Id     uint
		Server []byte
		Client []byte
	}
	var tlsRows []tlsRow
	if err := tx.Raw("SELECT id, server, client FROM tls").Scan(&tlsRows).Error; err != nil {
		return err
	}

	pinByTlsId := make(map[uint]string, len(tlsRows))
	for _, row := range tlsRows {
		var server map[string]interface{}
		if len(row.Server) > 0 {
			_ = json.Unmarshal(row.Server, &server)
		}

		pin := ""
		isReality := false
		if r, ok := server["reality"].(map[string]interface{}); ok {
			isReality, _ = r["enabled"].(bool)
		}
		// Only self-signed certificates are pinned.
		if !isReality {
			if certPEM := util.CertPEMFromTLS(server); util.CertIsSelfSigned(certPEM) {
				pin = util.CertPublicKeySha256(certPEM)
			}
		}
		pinByTlsId[row.Id] = pin

		var client map[string]interface{}
		if len(row.Client) > 0 {
			if err := json.Unmarshal(row.Client, &client); err != nil {
				continue
			}
		}
		if client == nil {
			client = map[string]interface{}{}
		}
		if applyTlsPin(client, pin) {
			newClient, err := json.MarshalIndent(client, "", "  ")
			if err != nil {
				return err
			}
			if err := tx.Exec("UPDATE tls SET client = ? WHERE id = ?", newClient, row.Id).Error; err != nil {
				return err
			}
		}
	}

	type inboundRow struct {
		Id      uint
		TlsId   uint
		OutJson []byte
	}
	var inbounds []inboundRow
	if err := tx.Raw("SELECT id, tls_id, out_json FROM inbounds WHERE tls_id > 0").Scan(&inbounds).Error; err != nil {
		return err
	}
	for _, in := range inbounds {
		if len(in.OutJson) == 0 {
			continue
		}
		var out map[string]interface{}
		if err := json.Unmarshal(in.OutJson, &out); err != nil {
			continue
		}
		tlsM, ok := out["tls"].(map[string]interface{})
		if !ok {
			continue
		}
		if applyTlsPin(tlsM, pinByTlsId[in.TlsId]) {
			out["tls"] = tlsM
			newOut, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			if err := tx.Exec("UPDATE inbounds SET out_json = ? WHERE id = ?", newOut, in.Id).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func applyTlsPin(tls map[string]interface{}, pin string) bool {
	changed := false
	if pin != "" {
		if cur, _ := tls["certificate_public_key_sha256"].([]interface{}); len(cur) != 1 || cur[0] != pin {
			tls["certificate_public_key_sha256"] = []string{pin}
			changed = true
		}
		if _, ok := tls["certificate"]; ok {
			delete(tls, "certificate")
			changed = true
		}
		if _, ok := tls["certificate_path"]; ok {
			delete(tls, "certificate_path")
			changed = true
		}
	} else if _, ok := tls["certificate_public_key_sha256"]; ok {
		delete(tls, "certificate_public_key_sha256")
		changed = true
	}
	return changed
}
