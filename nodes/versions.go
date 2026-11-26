package nodes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func versionDigest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func appendTemplateVersion(tx *gorm.DB, value *model.NodeTemplate, old model.NodeTemplate, spec json.RawMessage) error {
	current := old.Version
	if current > 0 {
		var previous model.NodeTemplateVersion
		if err := tx.First(&previous, whereTemplateVersion, value.Id, current).Error; err != nil {
			return err
		}
		if previous.SchemaVersion == value.SchemaVersion && previous.Digest == versionDigest(spec) {
			value.Version = current
			return tx.Model(value).Update("version", current).Error
		}
	}
	current++
	row := model.NodeTemplateVersion{TemplateId: value.Id, Version: current, SchemaVersion: 1, Digest: versionDigest(spec), Payload: spec, Created: time.Now().Unix()}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	value.Version = current
	if err := tx.Model(value).Updates(map[string]interface{}{"version": current, "schema_version": value.SchemaVersion}).Error; err != nil {
		return err
	}
	if value.Kind == inboundTemplate {
		return refreshCatalogDeployments(tx, value.Id)
	}
	return queueLiveTemplate(tx, *value)
}

func saveVersionedTemplate(tx *gorm.DB, value *model.NodeTemplate, old model.NodeTemplate, data json.RawMessage) error {
	var request struct {
		Spec            json.RawMessage `json:"spec"`
		ExpectedVersion *uint64         `json:"expected_version"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}
	if request.ExpectedVersion != nil && *request.ExpectedVersion != old.Version {
		return errors.New("template_version_conflict")
	}
	if len(request.Spec) == 0 {
		return errors.New("one_inbound_per_template")
	}
	if old.Id != 0 && request.ExpectedVersion == nil {
		return errors.New("expected_template_version_required")
	}
	// A retired key keeps its protocol identity even after a version in which
	// the entry is absent. Restoring history must not reuse a VLESS key for TUIC.
	prior, err := priorTemplateProtocols(tx, old)
	if err != nil {
		return err
	}
	spec, err := NormalizeSpec(request.Spec, prior)
	if err != nil {
		return err
	}
	for i := range spec.Entries {
		if err := validateTLSProfile(tx, spec.Entries[i]); err != nil {
			return err
		}
		if err := sealRealmDraft(&spec.Entries[i]); err != nil {
			return err
		}
	}
	if err := updateInboundTemplateIdentity(tx, value, old, spec); err != nil {
		return err
	}
	value.SchemaVersion = 1
	value.Version = old.Version
	if err := updateTemplateScope(tx, value, old); err != nil {
		return err
	}
	if err := tx.Save(value).Error; err != nil {
		return err
	}
	payload, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	return appendTemplateVersion(tx, value, old, payload)
}

func TemplateHistory(id uint) ([]model.NodeTemplateVersion, error) {
	versions := []model.NodeTemplateVersion{}
	err := database.GetDB().Order("version").Where(whereTemplateID, id).Find(&versions).Error
	return versions, err
}

type TemplateVersionView struct {
	TemplateId    uint            `json:"template_id"`
	Version       uint64          `json:"version"`
	SchemaVersion int             `json:"schema_version"`
	Digest        string          `json:"digest"`
	Spec          json.RawMessage `json:"spec,omitempty"`
}

func ReadTemplateVersion(id uint, version uint64) (TemplateVersionView, error) {
	var view TemplateVersionView
	if version == 0 {
		var template model.NodeTemplate
		if err := database.GetDB().First(&template, id).Error; err != nil {
			return view, err
		}
		version = template.Version
	}
	var row model.NodeTemplateVersion
	if err := database.GetDB().First(&row, whereTemplateVersion, id, version).Error; err != nil {
		return view, err
	}
	view = TemplateVersionView{TemplateId: id, Version: row.Version, SchemaVersion: row.SchemaVersion, Digest: row.Digest}
	if row.SchemaVersion == 1 {
		view.Spec = append(json.RawMessage(nil), row.Payload...)
	}
	return view, nil
}

func priorTemplateProtocols(tx *gorm.DB, old model.NodeTemplate) (*TemplateSpec, error) {
	prior := &TemplateSpec{}
	if old.Id != 0 {
		var history []model.NodeTemplateVersion
		if err := tx.Where(whereTemplateID, old.Id).Order("version").Find(&history).Error; err != nil {
			return nil, err
		}
		for _, version := range history {
			if version.SchemaVersion != 1 {
				return nil, errors.New("unsupported_template_version")
			}
			var previous TemplateSpec
			if err := json.Unmarshal(version.Payload, &previous); err != nil {
				return nil, err
			}
			prior.Entries = append(prior.Entries, previous.Entries...)
		}
	}
	return prior, nil
}

func updateInboundTemplateIdentity(tx *gorm.DB, value *model.NodeTemplate, old model.NodeTemplate, spec TemplateSpec) error {
	if old.Id != 0 {
		value.Kind = old.Kind
	}
	if value.Kind == inboundTemplate {
		if len(spec.Entries) != 1 {
			return errors.New("one_inbound_per_template")
		}
		if old.Id != 0 {
			previous, err := templateSpec(tx, old)
			if err != nil {
				return err
			}
			if len(previous.Entries) != 1 || previous.Entries[0].Key != spec.Entries[0].Key {
				return errors.New("node_inbound_template_key_immutable")
			}
		}
		value.Name, value.Protocol = spec.Entries[0].Name, spec.Entries[0].Protocol
	}
	return nil
}

func updateTemplateScope(tx *gorm.DB, value *model.NodeTemplate, old model.NodeTemplate) error {
	if old.Id != 0 {
		value.Scope, value.OwnerNodeId, value.ParentTemplateId = old.Scope, old.OwnerNodeId, old.ParentTemplateId
	}
	if value.Scope == "" {
		value.Scope = "shared"
	}
	if value.Scope != "shared" && value.Scope != "private" {
		return errors.New("invalid_template_scope")
	}
	if value.Scope == "private" {
		var owner model.Node
		if value.OwnerNodeId == 0 {
			return errors.New("private_template_owner_required")
		}
		if err := tx.First(&owner, value.OwnerNodeId).Error; err != nil {
			return err
		}
	} else {
		value.OwnerNodeId = 0
	}
	return nil
}
