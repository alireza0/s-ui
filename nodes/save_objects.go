package nodes

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func parseSaveIdentity(object, action string, data json.RawMessage) (uint, nodeDeleteRequest, error) {
	var id uint
	var deleteRequest nodeDeleteRequest
	if action == "del" {
		if object == "nodes" {
			var err error
			deleteRequest, err = parseNodeDelete(data)
			if err != nil {
				return id, deleteRequest, err
			}
			id = deleteRequest.Id
		} else if err := json.Unmarshal(data, &id); err != nil {
			return id, deleteRequest, err
		}
	} else {
		var meta struct {
			Id uint `json:"id"`
		}
		if err := json.Unmarshal(data, &meta); err != nil {
			return id, deleteRequest, err
		}
		id = meta.Id
		if object == "nodes" && jobRunning(id) {
			return id, deleteRequest, errors.New("node_job_running")
		}
		if !knownNodeSaveAction(object, action) {
			return id, deleteRequest, errors.New("unknown Nodes action")
		}
		if action == "new" && id != 0 {
			return id, deleteRequest, errors.New("new remote records must not reuse supplied IDs")
		}
	}
	return id, deleteRequest, nil
}
func saveTemplateObject(tx *gorm.DB, action string, data json.RawMessage, id uint) (uint, error) {
	if action == "del" {
		return id, deleteTemplateObject(tx, id)
	}
	value := model.NodeTemplate{}
	if action == "edit" {
		if err := tx.First(&value, id).Error; err != nil {
			return id, err
		}
	}
	old := value
	if err := json.Unmarshal(data, &value); err != nil {
		return id, err
	}
	if action == "edit" && old.Kind != inboundTemplate {
		return id, errors.New("edit_node_inbound_selection")
	}
	if action == "new" && value.Kind != inboundTemplate {
		return id, errors.New("invalid_node_template_kind")
	}
	if strings.TrimSpace(value.Name) == "" {
		return id, errors.New("node template needs a name")
	}
	value.ParentTemplateId = old.ParentTemplateId
	if err := saveVersionedTemplate(tx, &value, old, data); err != nil {
		return id, err
	}
	return value.Id, nil
}

func deleteTemplateObject(tx *gorm.DB, id uint) error {
	var template model.NodeTemplate
	if err := tx.First(&template, id).Error; err != nil {
		return err
	}
	if template.Kind == deploymentTemplate {
		return errors.New("edit_node_inbound_selection")
	}
	if err := detachOrphanedPrivateDeployments(tx); err != nil {
		return err
	}
	var members int64
	if err := tx.Model(&model.NodeTemplateMember{}).Where("inbound_id = ?", id).Count(&members).Error; err != nil {
		return err
	}
	if members > 0 {
		return errors.New("node_template_in_use")
	}
	var copies int64
	if err := tx.Model(&model.NodeTemplate{}).Where("parent_template_id = ?", id).Count(&copies).Error; err != nil {
		return err
	}
	if copies > 0 {
		return errors.New("template_is_copy_origin")
	}
	var used int64
	if err := tx.Model(&model.Node{}).Where(whereTemplateID, id).Count(&used).Error; err != nil {
		return err
	}
	if used > 0 {
		return errors.New("node_template_in_use")
	}
	if err := tx.Delete(&model.NodeTemplate{}, id).Error; err != nil {
		return err
	}
	if err := tx.Where(whereTemplateID, id).Delete(&model.NodeTemplateVersion{}).Error; err != nil {
		return err
	}
	return nil
}
func saveServerObject(tx *gorm.DB, action string, data json.RawMessage, id uint, deleteRequest nodeDeleteRequest) (uint, error) {
	var value model.Node
	if err := tx.First(&value, id).Error; err != nil {
		return id, err
	}
	if action != "del" && !managedNodeModel(value) {
		return id, errors.New("unsupported_node_model")
	}
	switch action {
	case "del":
		return id, deleteNodeObject(tx, value, deleteRequest)
	case "retry":
		return id, retryNodeObject(tx, value)
	case "selectinbounds":
		return id, selectNodeObject(tx, value, data)
	default:
		return editNodeObject(tx, value, data)
	}
}
func deleteNodeObject(tx *gorm.DB, value model.Node, deleteRequest nodeDeleteRequest) error {
	id := value.Id
	if jobRunning(id) {
		return errors.New("node_job_running")
	}
	var activeJobs int64
	if err := tx.Model(&model.NodeJob{}).Where("node_id = ? AND state IN ?", id, []string{"queued", "running"}).Count(&activeJobs).Error; err != nil {
		return err
	}
	if activeJobs > 0 {
		return errors.New("node_job_running")
	}
	var resourceTask model.NodeTask
	if err := tx.Where("node_id = ? AND kind = ?", id, "resources").First(&resourceTask).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if resourceTask.LeaseUntil > time.Now().Unix() {
		return errors.New("node_resource_job_running")
	}
	if !disableConfirmed(value) && (value.Enable || !deleteRequest.ServerAlreadyStopped) {
		return errors.New("node_disable_pending_delete")
	}
	if !deleteRequest.DNSAlreadyRemoved {
		var owned int64
		if err := tx.Model(&model.NodeDNSRecord{}).Where("node_id = ? AND retired = ?", id, false).Count(&owned).Error; err != nil {
			return err
		}
		if owned > 0 {
			return errNodeDNSCleanupRequired
		}
	}
	return purgeDeletedNode(tx, id)
}

func retryNodeObject(tx *gorm.DB, value model.Node) error {
	id := value.Id
	if err := tx.Model(&value).Updates(map[string]interface{}{"desired_fingerprint": "", "rejected_revision": 0, "last_error": ""}).Error; err != nil {
		return err
	}
	if err := database.QueueNodeTasks(tx, []uint{id}, "structure"); err != nil {
		return err
	}
	return database.QueueNodeTasks(tx, []uint{id}, "access")
}

func selectNodeObject(tx *gorm.DB, value model.Node, data json.RawMessage) error {
	var request struct {
		Ids      []uint  `json:"inbound_template_ids"`
		Expected *uint64 `json:"expected_version"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}
	if err := selectNodeInbounds(tx, &value, request.Ids, request.Expected); err != nil {
		return err
	}

	return nil
}

func editNodeObject(tx *gorm.DB, value model.Node, data json.RawMessage) (uint, error) {
	id := value.Id
	var request struct {
		Id     uint    `json:"id"`
		Name   *string `json:"name"`
		Enable *bool   `json:"enable"`
	}
	if err := decodeJSON(data, &request, true); err != nil {
		return id, err
	}
	if request.Name != nil {
		value.Name = *request.Name
	}
	if strings.TrimSpace(value.Name) == "" {
		return id, errors.New("node needs a name")
	}
	if request.Enable != nil {
		value.Enable = *request.Enable
	}
	if err := tx.Save(&value).Error; err != nil {
		return id, err
	}
	return id, queueNodeStructure(tx, &value)
}

func finishNodeSave(tx *gorm.DB, object, action string, id uint, deleteRequest nodeDeleteRequest, actor string) error {
	obj, _ := json.Marshal(map[string]uint{"id": id})
	if deleteRequest.DNSAlreadyRemoved || deleteRequest.ServerAlreadyStopped {
		obj, _ = json.Marshal(deleteRequest)
	}
	return tx.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: actor, Key: object, Action: action, Obj: obj}).Error
}

func knownNodeSaveAction(object, action string) bool {
	switch action {
	case "new":
		return object == "nodetemplates"
	case "edit":
		return true
	case "selectinbounds", "retry":
		return object == "nodes"
	default:
		return false
	}
}

func purgeDeletedNode(tx *gorm.DB, id uint) error {
	if err := tx.Where(whereNodeID, id).Delete(&model.NodeInbound{}).Error; err != nil {
		return err
	}
	// No grants or control state may survive a logical node deletion.
	// A restored database can otherwise reuse the deleted numeric ID.
	for _, row := range []interface{}{&model.ClientNode{}, &model.NodeAccessState{}, &model.NodeTask{}, &model.NodeOverride{}, &model.NodeGeneratedMaterial{}, &model.NodeInstallation{}, &model.NodeJob{}, &model.NodeDeployment{}, &model.NodePortReservation{}, &model.NodeDomainBinding{}, &model.NodeCertificateBinding{}, &model.NodeCertificateVersion{}, &model.NodeDNSRecord{}} {
		if err := tx.Where(whereNodeID, id).Delete(row).Error; err != nil {
			return err
		}
	}
	if err := tx.Delete(&model.Node{}, id).Error; err != nil {
		return err
	}
	if err := tx.Where(whereNodeID, id).Delete(&model.NodeClientTraffic{}).Error; err != nil {
		return err
	}
	if err := detachOrphanedPrivateDeployments(tx); err != nil {
		return err
	}

	return nil
}
