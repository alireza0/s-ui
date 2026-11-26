package database

import (
	"encoding/json"
	"sort"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NodeAssignmentProjection struct {
	Inbounds []uint
	Nodes    []uint
}

// NodeProjections batches client DTO hydration; no per-client SQL in GetAll.
func NodeProjections(tx *gorm.DB, clientIDs []uint) (map[uint]NodeAssignmentProjection, error) {
	result := map[uint]NodeAssignmentProjection{}
	for _, id := range clientIDs {
		result[id] = NodeAssignmentProjection{Inbounds: []uint{}, Nodes: []uint{}}
	}
	if len(clientIDs) == 0 {
		return result, nil
	}
	if tx.Migrator().HasTable(&model.ClientNode{}) {
		if err := wholeNodeProjections(tx, clientIDs, result); err != nil {
			return nil, err
		}
	}
	for id, p := range result {
		result[id] = canonicalNodeProjection(p)
	}
	return result, nil
}

func NodeProjection(tx *gorm.DB, clientID uint) ([]uint, []uint, error) {
	if clientID == 0 {
		return []uint{}, []uint{}, nil
	}
	all, err := NodeProjections(tx, []uint{clientID})
	if err != nil {
		return nil, nil, err
	}
	p := all[clientID]
	return p.Inbounds, p.Nodes, nil
}

func QueueNodeTasks(tx *gorm.DB, ids []uint, kind string) error {
	if len(ids) == 0 || !tx.Migrator().HasTable(&model.NodeTask{}) {
		return nil
	}
	seen := map[uint]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		row := model.NodeTask{NodeId: id, Kind: kind, Generation: 1, State: "queued"}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "kind"}}, DoUpdates: clause.Assignments(map[string]interface{}{"generation": gorm.Expr("generation + 1"), "state": "queued", "next_attempt": 0})}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func ClientNodeTargets(tx *gorm.DB, clients []uint) ([]uint, error) {
	ids := []uint{}
	if len(clients) == 0 {
		return ids, nil
	}
	if tx.Migrator().HasTable(&model.ClientNode{}) {
		if err := tx.Model(&model.ClientNode{}).Where("client_id IN ?", clients).Distinct().Pluck("node_id", &ids).Error; err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func QueueClientNodeAccess(tx *gorm.DB, clients []uint) error {
	ids, err := ClientNodeTargets(tx, clients)
	if err != nil {
		return err
	}
	return QueueNodeTasks(tx, ids, "access")
}

func RevokeClientNodes(tx *gorm.DB, clients []uint) error {
	if err := QueueClientNodeAccess(tx, clients); err != nil {
		return err
	}
	if tx.Migrator().HasTable(&model.ClientNode{}) {
		return tx.Model(&model.ClientNode{}).Where("client_id IN ? AND active = ?", clients, true).Updates(map[string]interface{}{"active": false, "generation": gorm.Expr("generation + 1")}).Error
	}
	return nil
}

func appliedNodeInboundIDs(nodes []model.Node) map[uint][]uint {
	byNode := map[uint][]uint{}
	for _, node := range nodes {
		var applied struct {
			Inbounds []struct {
				Id uint `json:"id"`
			} `json:"inbounds"`
		}
		if node.AppliedRevision > 0 && json.Unmarshal(node.AppliedSnapshot, &applied) == nil {
			for _, in := range applied.Inbounds {
				byNode[node.Id] = append(byNode[node.Id], in.Id)
			}
		}
	}
	return byNode
}

func wholeNodeProjections(tx *gorm.DB, clientIDs []uint, result map[uint]NodeAssignmentProjection) error {
	var bindings []model.ClientNode
	if err := tx.Model(&model.ClientNode{}).Select("client_nodes.*").Joins("JOIN nodes ON nodes.id = client_nodes.node_id").Where("client_nodes.client_id IN ? AND client_nodes.active = ? AND nodes.assignment_mode = ? AND nodes.agent_protocol = 2 AND nodes.template_target_version > 0", clientIDs, true, model.NodeAssignmentWhole).Order("client_id, node_id").Find(&bindings).Error; err != nil {
		return err
	}
	nodeIDs := []uint{}
	seenNodes := map[uint]bool{}
	for _, binding := range bindings {
		if !seenNodes[binding.NodeId] {
			seenNodes[binding.NodeId] = true
			nodeIDs = append(nodeIDs, binding.NodeId)
		}
	}
	var nodes []model.Node
	if len(nodeIDs) > 0 {
		if err := tx.Select("id", "applied_revision", "applied_snapshot").Where("id IN ?", nodeIDs).Find(&nodes).Error; err != nil {
			return err
		}
	}
	byNode := appliedNodeInboundIDs(nodes)
	for _, binding := range bindings {
		p := result[binding.ClientId]
		p.Nodes = append(p.Nodes, binding.NodeId)
		p.Inbounds = append(p.Inbounds, byNode[binding.NodeId]...)
		result[binding.ClientId] = p
	}
	return nil
}

func canonicalNodeProjection(p NodeAssignmentProjection) NodeAssignmentProjection {
	sort.Slice(p.Inbounds, func(i, j int) bool { return p.Inbounds[i] < p.Inbounds[j] })
	unique := []uint{}
	for _, in := range p.Inbounds {
		if len(unique) == 0 || unique[len(unique)-1] != in {
			unique = append(unique, in)
		}
	}
	p.Inbounds = unique
	return p
}
