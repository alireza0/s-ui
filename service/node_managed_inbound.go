package service

import (
	"encoding/json"
	"os"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

// ApplyNodeManagedInbound runs on the NODE when master pushes a managed inbound.
// It saves the inbound into node's DB (marked as managed) and starts it in the node's local sing-box core.
func (s *InboundService) ApplyNodeManagedInbound(req *NodeApplyRequest) error {
	if req == nil || len(req.Inbound) == 0 {
		return common.NewError("empty inbound payload")
	}

	var inbound model.Inbound
	if err := inbound.UnmarshalJSON(req.Inbound); err != nil {
		return err
	}

	db := database.GetDB()

	// Check if already exists by tag
	var existing model.Inbound
	err := db.Model(&model.Inbound{}).Where("tag = ?", inbound.Tag).First(&existing).Error
	if err == nil {
		inbound.Id = existing.Id
	}

	// Save to node DB
	if err := db.Save(&inbound).Error; err != nil {
		return err
	}

	// Apply to local sing-box core on the node
	if corePtr != nil && corePtr.IsRunning() {
		// Build full config including pushed users
		fullConfig, err := s.buildConfigWithUsers(req.Inbound, req.Users)
		if err != nil {
			return err
		}

		// Remove old if re-applying
		_ = corePtr.RemoveInbound(inbound.Tag)

		if err := corePtr.AddInbound(fullConfig); err != nil {
			logger.Error("node apply inbound error: ", err)
			return err
		}
		logger.Info("node applied managed inbound: ", inbound.Tag)
	}

	return nil
}

// DeleteNodeManagedInbound removes a master-managed inbound from the node's DB and core.
func (s *InboundService) DeleteNodeManagedInbound(tag string) error {
	if tag == "" {
		return common.NewError("empty tag")
	}

	db := database.GetDB()

	if corePtr != nil && corePtr.IsRunning() {
		err := corePtr.RemoveInbound(tag)
		if err != nil && err != os.ErrInvalid {
			logger.Warning("node delete inbound core remove error: ", err)
		}
		corePtr.GetInstance().ConnTracker().CloseConnByInbound(tag)
	}

	return db.Where("tag = ?", tag).Delete(&model.Inbound{}).Error
}

// ApplyNodeManagedUsers updates user credentials on a managed inbound in the node's core.
func (s *InboundService) ApplyNodeManagedUsers(req *NodeApplyUsersRequest) error {
	if req == nil || req.Tag == "" {
		return common.NewError("empty tag")
	}

	db := database.GetDB()

	var inbound model.Inbound
	if err := db.Model(&model.Inbound{}).Where("tag = ?", req.Tag).First(&inbound).Error; err != nil {
		return err
	}

	inboundConfig, err := inbound.MarshalJSON()
	if err != nil {
		return err
	}

	fullConfig, err := s.buildConfigWithUsers(inboundConfig, req.Users)
	if err != nil {
		return err
	}

	if corePtr != nil && corePtr.IsRunning() {
		handled, err := corePtr.UpdateInboundUsers(fullConfig)
		if err != nil {
			logger.Warning("node update users error: ", err)
		}
		if !handled {
			// Fallback: restart inbound with new users
			_ = corePtr.RemoveInbound(req.Tag)
			corePtr.GetInstance().ConnTracker().CloseConnByInbound(req.Tag)
			if err := corePtr.AddInbound(fullConfig); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildConfigWithUsers embeds pushed user JSON arrays into inbound JSON options.
func (s *InboundService) buildConfigWithUsers(inboundConfig json.RawMessage, users []json.RawMessage) ([]byte, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(inboundConfig, &parsed); err != nil {
		return nil, err
	}
	if len(users) > 0 {
		parsed["users"] = users
	} else {
		parsed["users"] = []interface{}{}
	}
	return json.Marshal(parsed)
}
