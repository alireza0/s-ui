package service

import (
	"encoding/json"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
)

// FanOutUsersToNodes pushes user updates to all remote nodes that host
// any of the given inbound IDs. Called after client save on master.
func (s *NodeService) FanOutUsersToNodes(inboundIds []uint) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered in FanOutUsersToNodes: ", r)
		}
	}()

	if len(inboundIds) == 0 {
		return
	}

	db := database.GetDB()

	// Find remote inbounds among the changed set
	var remoteInbounds []model.Inbound
	err := db.Model(&model.Inbound{}).
		Where("id IN ? AND node_id IS NOT NULL", inboundIds).
		Find(&remoteInbounds).Error
	if err != nil {
		logger.Warning("fanout: failed to find remote inbounds: ", err)
		return
	}
	if len(remoteInbounds) == 0 {
		return
	}

	// Group by node_id
	nodeInbounds := map[uint][]model.Inbound{}
	for _, ib := range remoteInbounds {
		if ib.NodeId != nil && *ib.NodeId > 0 {
			nodeInbounds[*ib.NodeId] = append(nodeInbounds[*ib.NodeId], ib)
		}
	}

	for nodeID, inbounds := range nodeInbounds {
		node, err := s.GetById(nodeID)
		if err != nil {
			logger.Warning("fanout: node ", nodeID, " not found: ", err)
			continue
		}
		if !node.Enable {
			continue
		}

		for _, ib := range inbounds {
			// Build user list for this inbound
			users, err := s.getUsersForInbound(ib.Id, ib.Type)
			if err != nil {
				logger.Warning("fanout: failed to get users for inbound ", ib.Tag, ": ", err)
				_ = s.MarkDirty(nodeID)
				continue
			}

			req := &NodeApplyUsersRequest{
				Tag:   ib.Tag,
				Type:  ib.Type,
				Users: users,
			}

			if err := s.PushUsersToNode(node, req); err != nil {
				logger.Debug("fanout: node ", node.Name, " offline for ", ib.Tag, ": ", err)
				_ = s.MarkDirty(nodeID)
				continue
			}
		}
	}
}

// getUsersForInbound fetches the user JSON configs for all enabled clients
// attached to the given inbound.
func (s *NodeService) getUsersForInbound(inboundId uint, inboundType string) ([]json.RawMessage, error) {
	gdb := database.GetDB()

	// Get inbound options to determine protocol-specific user config key
	is := &InboundService{}
	if !is.hasUser(inboundType) {
		return nil, nil
	}

	var users []string
	err := gdb.Raw(`SELECT json_extract(clients.config, "$.`+inboundType+`")
		FROM clients, json_each(clients.inbounds) as je
		WHERE clients.enable = true AND je.value = ?`, inboundId).Scan(&users).Error
	if err != nil {
		return nil, err
	}

	var result []json.RawMessage
	for _, u := range users {
		if u != "" {
			result = append(result, json.RawMessage(u))
		}
	}
	return result, nil
}

// ReconcileDirtyNodes pushes full inbound+user state to all dirty nodes.
// Called periodically by the heartbeat job.
func (s *NodeService) ReconcileDirtyNodes() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered in ReconcileDirtyNodes: ", r)
		}
	}()

	db := database.GetDB()

	var dirtyNodes []*model.Node
	err := db.Model(&model.Node{}).
		Where("enable = ? AND config_dirty = ?", true, true).
		Find(&dirtyNodes).Error
	if err != nil {
		logger.Warning("reconcile: failed to find dirty nodes: ", err)
		return
	}

	for _, node := range dirtyNodes {
		// Find all inbounds assigned to this node
		var inbounds []model.Inbound
		err := db.Model(&model.Inbound{}).
			Where("node_id = ?", node.Id).
			Find(&inbounds).Error
		if err != nil {
			logger.Warning("reconcile: failed to find inbounds for node ", node.Name, ": ", err)
			continue
		}

		allOK := true
		for _, ib := range inbounds {
			users, err := s.getUsersForInbound(ib.Id, ib.Type)
			if err != nil {
				logger.Warning("reconcile: failed to get users for ", ib.Tag, ": ", err)
				allOK = false
				continue
			}

			// First push full inbound config to ensure listener exists on node
			ibConfig, err := ib.MarshalJSON()
			if err != nil {
				logger.Warning("reconcile: failed to marshal inbound ", ib.Tag, ": ", err)
				allOK = false
				continue
			}
			ibConfig = SanitizeRemoteInboundJSON(ibConfig)

			applyReq := &NodeApplyRequest{
				Inbound: ibConfig,
				Users:   users,
				Tag:     ib.Tag,
				Type:    ib.Type,
			}

			if err := s.PushInboundToNode(node, applyReq); err != nil {
				logger.Debug("reconcile: node ", node.Name, " push failed for ", ib.Tag, ": ", err)
				allOK = false
				continue // Don't break — continue reconciling remaining inbounds for this node!
			}
		}

		if allOK {
			_ = s.ClearDirty(node.Id)
		}
	}
}
