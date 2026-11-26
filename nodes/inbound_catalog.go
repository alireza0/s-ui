package nodes

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

const (
	inboundTemplate    = "inbound"
	deploymentTemplate = "deployment"
)

// A private deployment with neither an owner nor a node using it is historical.
// Keep its saved versions and lineage, but release its current catalog links.
func detachOrphanedPrivateDeployments(tx *gorm.DB) error {
	orphaned := tx.Model(&model.NodeTemplate{}).Select("id").
		Where("kind = ? AND scope = ? AND owner_node_id > 0", deploymentTemplate, "private").
		Where(`NOT EXISTS (
			SELECT 1 FROM nodes AS n
			WHERE n.id = node_templates.owner_node_id OR n.template_id = node_templates.id
		)`)
	return tx.Where("template_id IN (?)", orphaned).Delete(&model.NodeTemplateMember{}).Error
}

func templateSpec(tx *gorm.DB, template model.NodeTemplate) (TemplateSpec, error) {
	return liveVersion(tx, model.Node{TemplateId: template.Id, TemplateTargetVersion: template.Version})
}

func catalogMembers(tx *gorm.DB, templateID uint) ([]uint, error) {
	var ids []uint
	err := tx.Model(&model.NodeTemplateMember{}).Where(whereTemplateID, templateID).Order("position").Pluck("inbound_id", &ids).Error
	return ids, err
}

func selectedInboundEntries(tx *gorm.DB, ids []uint, owner uint) ([]SpecEntry, error) {
	if len(ids) > 32 {
		return nil, errors.New("node_inbound_selection_limit")
	}
	entries := make([]SpecEntry, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		var template model.NodeTemplate
		if err := tx.First(&template, "id = ? AND kind = ?", id, inboundTemplate).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("node_inbound_template_not_found")
			}
			return nil, err
		}
		if template.Scope == "private" && template.OwnerNodeId != owner {
			return nil, errors.New("private_template_belongs_to_another_node")
		}
		spec, err := templateSpec(tx, template)
		if err != nil {
			return nil, err
		}
		if len(spec.Entries) != 1 {
			return nil, errors.New("one_inbound_per_template")
		}
		entry := spec.Entries[0]
		if seen[entry.Key] {
			return nil, errors.New("duplicate_node_inbound_template")
		}
		seen[entry.Key] = true
		entries = append(entries, entry)
	}
	return entries, nil
}

func appendDeploymentSpec(tx *gorm.DB, template *model.NodeTemplate, spec TemplateSpec) error {
	body, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	spec, err = NormalizeSpec(body, nil)
	if err != nil {
		return err
	}
	body, err = json.Marshal(spec)
	if err != nil {
		return err
	}
	return appendTemplateVersion(tx, template, *template, body)
}

func createDeploymentTemplate(tx *gorm.DB, node model.Node) (model.NodeTemplate, error) {
	template := model.NodeTemplate{Name: node.Name, SchemaVersion: 1, Kind: deploymentTemplate, Scope: "private", OwnerNodeId: node.Id, ParentTemplateId: node.TemplateId}
	err := tx.Create(&template).Error
	return template, err
}

func createProvisionTemplate(tx *gorm.DB, node model.Node, ids []uint, protocols []string) (model.NodeTemplate, error) {
	entries, err := selectedInboundEntries(tx, ids, 0)
	if err != nil {
		return model.NodeTemplate{}, err
	}
	if len(ids) == 0 {
		for _, kind := range protocols {
			entry, err := defaultLiveProvisionEntry(kind)
			if err != nil {
				return model.NodeTemplate{}, err
			}
			preset := model.NodeTemplate{Name: entry.Name, Kind: inboundTemplate, Scope: "private", OwnerNodeId: node.Id}
			body, err := json.Marshal(map[string]any{"spec": TemplateSpec{SchemaVersion: 1, Config: json.RawMessage(`{}`), Entries: []SpecEntry{entry}}})
			if err != nil {
				return model.NodeTemplate{}, err
			}
			if err := saveVersionedTemplate(tx, &preset, model.NodeTemplate{}, body); err != nil {
				return model.NodeTemplate{}, err
			}
			ids = append(ids, preset.Id)
			entries = append(entries, entry)
		}
	}
	template, err := createDeploymentTemplate(tx, node)
	if err != nil {
		return template, err
	}
	spec := TemplateSpec{
		SchemaVersion: 1,
		Config:        json.RawMessage(`{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`),
		Entries:       entries,
	}
	err = writeCatalogSelection(tx, &template, ids, spec)
	return template, err
}

func writeCatalogSelection(tx *gorm.DB, template *model.NodeTemplate, ids []uint, spec TemplateSpec) error {
	if err := tx.Where(whereTemplateID, template.Id).Delete(&model.NodeTemplateMember{}).Error; err != nil {
		return err
	}
	for position, id := range ids {
		if err := tx.Create(&model.NodeTemplateMember{TemplateId: template.Id, InboundId: id, Position: position}).Error; err != nil {
			return err
		}
	}
	return appendDeploymentSpec(tx, template, spec)
}

func selectNodeInbounds(tx *gorm.DB, node *model.Node, ids []uint, expected *uint64) error {
	if !managedNodeModel(*node) {
		return errors.New("unsupported_node_model")
	}
	if expected == nil || node.SettingsVersion != *expected {
		return errors.New("node_settings_version_conflict")
	}
	if ids == nil {
		return errors.New("invalid_node_inbound_selection")
	}
	var template model.NodeTemplate
	if err := tx.First(&template, node.TemplateId).Error; err != nil {
		return err
	}
	old, err := catalogMembers(tx, template.Id)
	if err != nil {
		return err
	}
	if slices.Equal(old, ids) && template.Kind == deploymentTemplate {
		return nil
	}
	entries, err := selectedInboundEntries(tx, ids, node.Id)
	if err != nil {
		return err
	}
	spec, err := liveVersion(tx, *node)
	if err != nil {
		return err
	}
	spec.Entries = entries
	if template.Kind != deploymentTemplate || template.Scope != "private" || template.OwnerNodeId != node.Id {
		template, err = createDeploymentTemplate(tx, *node)
		if err != nil {
			return err
		}
	}
	if err := writeCatalogSelection(tx, &template, ids, spec); err != nil {
		return err
	}
	node.TemplateId = template.Id
	node.TemplateTargetVersion = template.Version
	if err := tx.Model(node).Updates(map[string]interface{}{"template_id": template.Id, "template_target_version": template.Version}).Error; err != nil {
		return err
	}
	return queueNodeStructure(tx, node)
}

func refreshCatalogDeployments(tx *gorm.DB, inboundID uint) error {
	var groups []uint
	if err := tx.Model(&model.NodeTemplateMember{}).Where("inbound_id = ?", inboundID).Pluck("template_id", &groups).Error; err != nil {
		return err
	}
	for _, id := range groups {
		var group model.NodeTemplate
		if err := tx.First(&group, id).Error; err != nil {
			return err
		}
		var nodes []model.Node
		if err := tx.Where(whereTemplateID, id).Find(&nodes).Error; err != nil {
			return err
		}
		for _, node := range nodes {
			if jobRunning(node.Id) {
				return errors.New("node_job_running")
			}
		}
		ids, err := catalogMembers(tx, id)
		if err != nil {
			return err
		}
		entries, err := selectedInboundEntries(tx, ids, group.OwnerNodeId)
		if err != nil {
			return err
		}
		spec, err := templateSpec(tx, group)
		if err != nil {
			return err
		}
		spec.Entries = entries
		if err := appendDeploymentSpec(tx, &group, spec); err != nil {
			return err
		}
	}
	return nil
}
