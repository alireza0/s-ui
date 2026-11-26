package nodes

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func queueLiveTemplate(tx *gorm.DB, template model.NodeTemplate) error {
	var ids []uint
	if err := tx.Model(&model.Node{}).Where("template_id = ? AND template_target_version > 0", template.Id).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Model(&model.Node{}).Where("id IN ?", ids).Update("template_target_version", template.Version).Error; err != nil {
		return err
	}
	return database.QueueNodeTasks(tx, ids, "structure")
}

func queueNodeStructure(tx *gorm.DB, node *model.Node) error {
	node.SettingsVersion++
	if err := tx.Model(node).Update("settings_version", node.SettingsVersion).Error; err != nil {
		return err
	}
	return database.QueueNodeTasks(tx, []uint{node.Id}, "structure")
}

func bindLiveTemplate(tx *gorm.DB, node *model.Node, template model.NodeTemplate) error {
	if node.AgentProtocol != 2 || node.AssignmentMode != model.NodeAssignmentWhole {
		return errors.New("unsupported_node_model")
	}
	if template.Scope == "private" && template.OwnerNodeId != node.Id {
		return errors.New("private_template_belongs_to_another_node")
	}
	node.TemplateId, node.TemplateTargetVersion = template.Id, template.Version
	if err := tx.Save(node).Error; err != nil {
		return err
	}
	return queueNodeStructure(tx, node)
}

func liveVersion(tx *gorm.DB, node model.Node) (TemplateSpec, error) {
	var version model.NodeTemplateVersion
	var spec TemplateSpec
	if err := tx.First(&version, whereTemplateVersion, node.TemplateId, node.TemplateTargetVersion).Error; err != nil {
		return spec, err
	}
	if version.SchemaVersion != 1 || versionDigest(version.Payload) != version.Digest {
		return spec, errors.New("invalid_live_template_version")
	}
	return NormalizeSpec(version.Payload, nil)
}

type liveActionRequest struct {
	Id              uint           `json:"id"`
	Version         uint64         `json:"version"`
	ExpectedVersion uint64         `json:"expected_version"`
	Overrides       []SpecOverride `json:"overrides"`
}

func saveLiveAction(tx *gorm.DB, object, action string, data json.RawMessage, actor string) error {
	var request liveActionRequest
	if err := decodeJSON(data, &request, true); err != nil {
		return err
	}
	if request.Id == 0 {
		return errors.New("invalid_live_template_action")
	}
	if action == "restoreversion" && object == "nodetemplates" {
		if err := restoreLiveTemplateVersion(tx, request); err != nil {
			return err
		}
	} else if action == "setoverrides" && object == "nodes" {
		if err := saveLiveNodeAction(tx, request); err != nil {
			return err
		}
	} else {
		return errors.New("invalid_live_template_action")
	}
	obj, _ := json.Marshal(map[string]uint{"id": request.Id})
	return tx.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: actor, Key: object, Action: action, Obj: obj}).Error
}

func restoreLiveTemplateVersion(tx *gorm.DB, request liveActionRequest) error {
	var template model.NodeTemplate
	var version model.NodeTemplateVersion
	if err := tx.First(&template, request.Id).Error; err != nil {
		return err
	}
	if template.Kind == deploymentTemplate {
		return errors.New("edit_node_inbound_selection")
	}
	if err := tx.First(&version, whereTemplateVersion, request.Id, request.Version).Error; err != nil {
		return err
	}
	if version.SchemaVersion != 1 {
		return errors.New("unsupported_template_version")
	}
	body, _ := json.Marshal(map[string]interface{}{"spec": version.Payload, "expected_version": request.ExpectedVersion})
	return saveVersionedTemplate(tx, &template, template, body)
}

func saveLiveNodeAction(tx *gorm.DB, request liveActionRequest) error {
	var node model.Node
	if err := tx.First(&node, request.Id).Error; err != nil {
		return err
	}
	if !managedNodeModel(node) {
		return errors.New("unsupported_node_model")
	}
	if node.SettingsVersion != request.ExpectedVersion {
		return errors.New("node_settings_version_conflict")
	}
	spec, err := liveVersion(tx, node)
	if err != nil {
		return err
	}
	if err := replaceLiveOverrides(tx, node.Id, spec, request.Overrides); err != nil {
		return err
	}
	return queueNodeStructure(tx, &node)
}

func replaceLiveOverrides(tx *gorm.DB, id uint, spec TemplateSpec, overrides []SpecOverride) error {
	if _, err := validateSpecOverrides(spec, overrides); err != nil {
		return err
	}
	// Archived records from removed keys do not become active implicitly.
	if err := tx.Where("node_id = ? AND archived = ?", id, false).Delete(&model.NodeOverride{}).Error; err != nil {
		return err
	}
	for _, override := range overrides {
		body, _ := json.Marshal(override.Source)
		row := model.NodeOverride{NodeId: id, ConnectionKey: override.ConnectionKey, Path: override.Path, Operation: override.Operation, Source: body}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
