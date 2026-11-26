package nodes

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"sort"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func nodeIDList(raw json.RawMessage) ([]uint, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, errors.New("nodes must be an array, not null")
	}
	var ids []uint
	if len(raw) == 0 || json.Unmarshal(raw, &ids) != nil {
		return nil, errors.New("nodes must contain numeric IDs")
	}
	seen := map[uint]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			return nil, errors.New("nodes contains zero or duplicate IDs")
		}
		seen[id] = true
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// Node inbound IDs are a read-only projection of applied listeners.
func wholeNodeSelection(tx *gorm.DB, client *model.Client) ([]uint, error) {
	projection, previous, err := database.NodeProjection(tx, client.Id)
	if err != nil {
		return nil, err
	}
	selected := previous
	if client.Nodes != nil {
		selected, err = nodeIDList(client.Nodes)
		if err != nil {
			return nil, err
		}
		if err := validateWholeNodeSelection(tx, selected); err != nil {
			return nil, err
		}
	}
	if client.NodeInbounds != nil {
		echo, err := nodeIDList(client.NodeInbounds)
		if err != nil {
			return nil, errors.New("nodeinbounds_read_only_use_nodes")
		}
		if !reflect.DeepEqual(echo, projection) || !reflect.DeepEqual(selected, previous) {
			return nil, errors.New("nodeinbounds_read_only_use_nodes")
		}
	}
	return selected, nil
}

func saveWholeNodeAssignments(tx *gorm.DB, client *model.Client) error {
	if client.Nodes == nil {
		return nil
	}
	ids, err := nodeIDList(client.Nodes)
	if err != nil {
		return err
	}
	var rows []model.ClientNode
	if err := tx.Where("client_id = ?", client.Id).Find(&rows).Error; err != nil {
		return err
	}
	wanted := map[uint]bool{}
	existing := map[uint]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	for _, row := range rows {
		existing[row.NodeId] = true
		if row.Active == wanted[row.NodeId] {
			continue
		}
		row.Active = wanted[row.NodeId]
		row.Generation++
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
	}
	for _, id := range ids {
		if !existing[id] {
			if err := tx.Create(&model.ClientNode{ClientId: client.Id, NodeId: id, Active: true, Generation: 1}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteClientAssignments queues access removal and retains revocation fences.
// The caller deletes the client in the same transaction.
func DeleteClientAssignments(tx *gorm.DB, ids []uint) error {
	if err := tx.Where("client_id IN ?", ids).Delete(&model.NodeClientTraffic{}).Error; err != nil {
		return err
	}
	return database.RevokeClientNodes(tx, ids)
}

func validateWholeNodeSelection(tx *gorm.DB, selected []uint) error {
	if len(selected) > 0 {
		var count int64
		if err := tx.Model(&model.Node{}).Where("id IN ? AND assignment_mode = ? AND agent_protocol = ? AND template_target_version > 0", selected, model.NodeAssignmentWhole, 2).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(selected)) {
			return errors.New("unknown_node")
		}
	}
	return nil
}
