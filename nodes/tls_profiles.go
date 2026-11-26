package nodes

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

type tlsProfileChoice struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Reality bool   `json:"reality"`
}

// Shared profiles carry policy. Certificate material and REALITY credentials
// are bound later by the node resolver.
func tlsProfilePolicy(profile model.Tls) (map[string]interface{}, map[string]interface{}, error) {
	server, err := objectJSON(profile.Server)
	if err != nil || server["enabled"] != true {
		return nil, nil, errors.New("node_tls_profile_disabled")
	}
	client := map[string]interface{}{}
	if len(profile.Client) != 0 && string(profile.Client) != "null" {
		client, err = objectJSON(profile.Client)
		if err != nil {
			return nil, nil, errors.New("node_tls_profile_invalid")
		}
	}
	if err := cleanTLSProfileSides(server, client); err != nil {
		return nil, nil, err
	}
	reality, _ := server["reality"].(map[string]interface{})
	if reality["enabled"] == true {
		delete(reality, "private_key")
		delete(reality, "short_id")
		handshake, _ := reality["handshake"].(map[string]interface{})
		target, _ := handshake["server"].(string)
		if !validPublishedAddress(target) {
			return nil, nil, errors.New("node_tls_profile_reality_target_required")
		}
		if name, _ := server["server_name"].(string); name == "" {
			server["server_name"] = target
		}
	} else {
		delete(server, "reality")
		delete(server, "server_name") // Regular TLS uses the node certificate name.
	}
	delete(client, "reality")
	delete(client, "server_name")
	if secretLiteral(server) || secretLiteral(client) || hasPrivateClientTLS(client) {
		return nil, nil, errors.New("node_tls_profile_unsupported")
	}
	return server, client, nil
}

func listTLSProfiles(tx *gorm.DB) ([]tlsProfileChoice, error) {
	var profiles []model.Tls
	if err := tx.Order("name, id").Find(&profiles).Error; err != nil {
		return nil, err
	}
	result := make([]tlsProfileChoice, 0, len(profiles))
	for _, profile := range profiles {
		server, _, err := tlsProfilePolicy(profile)
		if err == nil {
			result = append(result, tlsProfileChoice{profile.Id, profile.Name, server["reality"] != nil})
		}
	}
	return result, nil
}

func applyTLSProfile(tx *gorm.DB, entry *SpecEntry) error {
	var profile model.Tls
	if err := tx.First(&profile, entry.TLSProfileID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("node_tls_profile_not_found")
		}
		return err
	}
	return bindTLSProfile(entry, profile)
}

func validateTLSProfile(tx *gorm.DB, entry SpecEntry) error {
	if entry.TLSProfileID == 0 {
		return nil
	}
	return applyTLSProfile(tx, &entry)
}

func bindTLSProfile(entry *SpecEntry, profile model.Tls) error {
	server, client, err := tlsProfilePolicy(profile)
	if err != nil {
		return err
	}
	options, err := objectJSON(entry.Options)
	if err != nil {
		return err
	}
	transport, _ := options["transport"].(map[string]interface{})
	isReality := server["reality"] != nil
	if isReality && (entry.Protocol != "vless" || transport["type"] == "quic") {
		return errors.New("node_tls_profile_requires_non_quic_vless")
	}
	options["tls"] = server
	entry.Options, err = json.Marshal(options)
	if err != nil {
		return err
	}
	entry.ClientTLS, err = json.Marshal(client)
	if err != nil {
		return err
	}
	sources := make(map[string]ValueSource, len(entry.Sources)+2)
	for path, source := range entry.Sources {
		if path != "/tls" && !strings.HasPrefix(path, "/tls/") {
			sources[path] = source
		}
	}
	if isReality {
		sources["/tls/reality/private_key"] = ValueSource{Kind: "node_generated", Purpose: "reality_private_key"}
		sources["/tls/reality/short_id"] = ValueSource{Kind: "node_generated", Purpose: "reality_short_id"}
	} else {
		sources["/tls/certificate_binding"] = ValueSource{Kind: "node_certificate", Name: "primary"}
	}
	entry.Sources = sources
	return nil
}

// Also consider nodes pinned to a deployment version older than the catalog.
func tlsProfileReferences(tx *gorm.DB, id uint) ([]model.NodeTemplateVersion, error) {
	var rows []model.NodeTemplateVersion
	versions := tx.Model(&model.NodeTemplateVersion{}).Where("schema_version = 1").
		Where(`EXISTS (SELECT 1 FROM json_each(payload, '$.entries') AS entry WHERE json_extract(entry.value, '$.tls_profile_id') = ?)`, id).
		Where(`EXISTS (SELECT 1 FROM node_templates AS t WHERE t.id = node_template_versions.template_id AND t.version = node_template_versions.version)
			OR EXISTS (SELECT 1 FROM nodes AS n WHERE n.template_id = node_template_versions.template_id AND n.template_target_version = node_template_versions.version)`)
	err := versions.Find(&rows).Error
	return rows, err
}

func CheckTLSProfileDeletion(tx *gorm.DB, id uint) error {
	references, err := tlsProfileReferences(tx, id)
	if err == nil && len(references) > 0 {
		return errors.New("tls in use")
	}
	return err
}

func QueueTLSProfileChange(tx *gorm.DB, profile model.Tls) error {
	if profile.Id == 0 {
		return nil
	}
	references, err := tlsProfileReferences(tx, profile.Id)
	if err != nil {
		return err
	}
	var ids []uint
	for _, version := range references {
		var spec TemplateSpec
		if err := json.Unmarshal(version.Payload, &spec); err != nil {
			return err
		}
		for _, entry := range spec.Entries {
			if entry.TLSProfileID == profile.Id {
				if err := bindTLSProfile(&entry, profile); err != nil {
					return err
				}
			}
		}
		var linked []uint
		if err := tx.Model(&model.Node{}).Where("template_id = ? AND template_target_version = ?", version.TemplateId, version.Version).Pluck("id", &linked).Error; err != nil {
			return err
		}
		ids = append(ids, linked...)
	}
	return database.QueueNodeTasks(tx, ids, "structure")
}

func cleanTLSProfileSides(server, client map[string]interface{}) error {
	for _, side := range []map[string]interface{}{server, client} {
		ech, _ := side["ech"].(map[string]interface{})
		if ech["enabled"] == true || side["client_authentication"] != nil || side["client_certificate"] != nil || side["client_certificate_path"] != nil || side["client_certificate_public_key_sha256"] != nil || side["client_key"] != nil || side["client_key_path"] != nil {
			return errors.New("node_tls_profile_unsupported")
		}
		delete(side, "ech")
		for _, field := range []string{"certificate", "key", "certificate_path", "key_path", "certificate_provider", "certificate_public_key_sha256"} {
			delete(side, field)
		}
	}
	return nil
}
