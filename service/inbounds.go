package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
)

type InboundService struct {
	ClientService
}

func (s *InboundService) Get(ids string) (*[]map[string]interface{}, error) {
	if ids == "" {
		return s.GetAll()
	}
	return s.getById(ids)
}

func (s *InboundService) getById(ids string) (*[]map[string]interface{}, error) {
	var inbound []model.Inbound
	var result []map[string]interface{}
	db := database.GetDB()
	err := db.Model(model.Inbound{}).Where("id in ?", strings.Split(ids, ",")).Scan(&inbound).Error
	if err != nil {
		return nil, err
	}
	for _, inb := range inbound {
		inbData, err := inb.MarshalFull()
		if err != nil {
			return nil, err
		}
		result = append(result, *inbData)
	}
	return &result, nil
}

func (s *InboundService) GetAll() (*[]map[string]interface{}, error) {
	db := database.GetDB()
	inbounds := []model.Inbound{}
	err := db.Model(model.Inbound{}).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	var data []map[string]interface{}
	for _, inbound := range inbounds {
		var shadowtls_version uint
		ss_managed := false
		inbData := map[string]interface{}{
			"id":     inbound.Id,
			"type":   inbound.Type,
			"tag":    inbound.Tag,
			"tls_id": inbound.TlsId,
		}
		if inbound.Options != nil {
			var restFields map[string]json.RawMessage
			if err := json.Unmarshal(inbound.Options, &restFields); err != nil {
				return nil, err
			}
			inbData["listen"] = restFields["listen"]
			inbData["listen_port"] = restFields["listen_port"]
			if inbound.Type == "shadowtls" {
				json.Unmarshal(restFields["version"], &shadowtls_version)
			}
			if inbound.Type == "shadowsocks" {
				json.Unmarshal(restFields["managed"], &ss_managed)
			}
		}
		if s.hasUser(inbound.Type) &&
			!(inbound.Type == "shadowtls" && shadowtls_version < 3) &&
			!(inbound.Type == "shadowsocks" && ss_managed) {
			users := []string{}
			err = db.Raw("SELECT clients.name FROM clients, json_each(clients.inbounds) as je WHERE je.value = ?", inbound.Id).Scan(&users).Error
			if err != nil {
				return nil, err
			}
			inbData["users"] = users
		}

		data = append(data, inbData)
	}
	return &data, nil
}

func (s *InboundService) FromIds(ids []uint) ([]*model.Inbound, error) {
	db := database.GetDB()
	inbounds := []*model.Inbound{}
	err := db.Model(model.Inbound{}).Where("id in ?", ids).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) Save(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
	var err error

	switch act {
	case "new", "edit":
		var inbound model.Inbound
		err = inbound.UnmarshalJSON(data)
		if err != nil {
			return err
		}
		if inbound.TlsId > 0 {
			err = tx.Model(model.Tls{}).Where("id = ?", inbound.TlsId).Find(&inbound.Tls).Error
			if err != nil {
				return err
			}
		}
		var oldTag string
		if act == "edit" {
			err = tx.Model(model.Inbound{}).Select("tag").Where("id = ?", inbound.Id).Find(&oldTag).Error
			if err != nil {
				return err
			}
		}

		// Only manage in local core if this is a local inbound (node_id IS NULL)
		if corePtr.IsRunning() && !IsRemoteInbound(&inbound) {
			if act == "edit" {
				err = corePtr.RemoveInbound(oldTag)
				if err != nil && err != os.ErrInvalid {
					return err
				}
			}

			inboundConfig, err := inbound.MarshalJSON()
			if err != nil {
				return err
			}

			if act == "edit" {
				inboundConfig, err = s.addUsers(tx, inboundConfig, inbound.Id, inbound.Type)
			} else {
				inboundConfig, err = s.initUsers(tx, inboundConfig, initUserIds, inbound.Type)
			}
			if err != nil {
				return err
			}

			err = corePtr.AddInbound(inboundConfig)
			if err != nil {
				return err
			}
		}

		err = util.FillOutJson(&inbound, hostname)
		if err != nil {
			return err
		}

		err = tx.Save(&inbound).Error
		if err != nil {
			return err
		}

		// If this is a remote inbound, push it to the node
		if IsRemoteInbound(&inbound) {
			ns := &NodeService{}
			node, err := ns.GetById(*inbound.NodeId)
			if err == nil && node.Enable {
				// Proactively mark dirty before push; clear only on success.
				// This ensures reconcile if the server crashes between DB save and push.
				_ = ns.MarkDirty(node.Id)

				inboundConfig, _ := inbound.MarshalJSON()
				inboundConfig = SanitizeRemoteInboundJSON(inboundConfig)
				users, _ := ns.getUsersForInbound(inbound.Id, inbound.Type)
				req := &NodeApplyRequest{
					Inbound: inboundConfig,
					Users:   users,
					Tag:     inbound.Tag,
					Type:    inbound.Type,
				}
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error("panic recovered in inbound push: ", r)
						}
					}()
					if err := ns.PushInboundToNode(node, req); err != nil {
						logger.Debug("inbound push to node ", node.Name, " failed: ", err)
						// Leave dirty for reconcile
					} else {
						_ = ns.ClearDirty(node.Id)
					}
				}()
			}
		}
		switch act {
		case "new":
			err = s.ClientService.UpdateClientsOnInboundAdd(tx, initUserIds, inbound.Id, hostname)
		case "edit":
			err = s.ClientService.UpdateLinksByInboundChange(tx, &[]model.Inbound{inbound}, hostname, oldTag)
		}
		if err != nil {
			return err
		}
	case "del":
		var tag string
		err = json.Unmarshal(data, &tag)
		if err != nil {
			return err
		}
		var inbound model.Inbound
		_ = tx.Model(model.Inbound{}).Where("tag = ?", tag).First(&inbound).Error

		if corePtr.IsRunning() {
			err = corePtr.RemoveInbound(tag)
			if err != nil && err != os.ErrInvalid {
				return err
			}
		}
		var id uint
		id = inbound.Id
		if id == 0 {
			err = tx.Model(model.Inbound{}).Select("id").Where("tag = ?", tag).Scan(&id).Error
			if err != nil {
				return err
			}
		}

		// If this was a remote inbound, notify the remote node
		if IsRemoteInbound(&inbound) {
			ns := &NodeService{}
			node, err := ns.GetById(*inbound.NodeId)
			if err == nil && node.Enable {
				go func() { _ = ns.DeleteInboundOnNode(node, tag) }()
			}
		}

		err = s.ClientService.UpdateClientsOnInboundDelete(tx, id, tag)
		if err != nil {
			return err
		}
		err = tx.Where("tag = ?", tag).Delete(model.Inbound{}).Error
		if err != nil {
			return err
		}
	default:
		return common.NewErrorf("unknown action: %s", act)
	}
	return nil
}

func (s *InboundService) UpdateOutJsons(tx *gorm.DB, inboundIds []uint, hostname string) error {
	var inbounds []model.Inbound
	err := tx.Model(model.Inbound{}).Preload("Tls").Where("id in ?", inboundIds).Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		err = util.FillOutJson(&inbound, hostname)
		if err != nil {
			return err
		}
		err = tx.Model(model.Inbound{}).Where("tag = ?", inbound.Tag).Update("out_json", inbound.OutJson).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *InboundService) GetAllConfig(db *gorm.DB) ([]json.RawMessage, error) {
	var inboundsJson []json.RawMessage
	var inbounds []*model.Inbound
	// Only load local inbounds (node_id IS NULL) for the local sing-box config.
	err := db.Model(model.Inbound{}).Preload("Tls").Where("node_id IS NULL").Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		inboundJson, err := inbound.MarshalJSON()
		if err != nil {
			return nil, err
		}
		inboundJson, err = s.addUsers(db, inboundJson, inbound.Id, inbound.Type)
		if err != nil {
			return nil, err
		}
		inboundsJson = append(inboundsJson, inboundJson)
	}
	return inboundsJson, nil
}

func (s *InboundService) hasUser(inboundType string) bool {
	switch inboundType {
	case "mixed", "socks", "http", "shadowsocks", "vmess", "trojan", "naive", "hysteria", "shadowtls", "tuic", "hysteria2", "vless", "anytls":
		return true
	}
	return false
}

func (s *InboundService) fetchUsers(db *gorm.DB, inboundType string, condition string, inbound map[string]interface{}) ([]json.RawMessage, error) {
	if inboundType == "shadowtls" {
		version, _ := inbound["version"].(float64)
		if int(version) < 3 {
			return nil, nil
		}
	}
	if inboundType == "shadowsocks" {
		method, _ := inbound["method"].(string)
		inboundType = util.ShadowsocksClientConfigKey(method)
	}

	var users []string

	err := db.Raw(
		fmt.Sprintf(`SELECT json_extract(clients.config, "$.%s")
		FROM clients WHERE enable = true AND %s`,
			inboundType, condition)).Scan(&users).Error
	if err != nil {
		return nil, err
	}
	stripVision := false
	if inboundType == "vless" {
		transportType := ""
		if tr, ok := inbound["transport"].(map[string]interface{}); ok {
			transportType, _ = tr["type"].(string)
		}
		stripVision = inbound["tls"] == nil || transportType != ""
	}

	var usersJson []json.RawMessage
	for _, user := range users {
		if stripVision {
			user = strings.ReplaceAll(user, "xtls-rprx-vision", "")
		}
		usersJson = append(usersJson, json.RawMessage(user))
	}
	return usersJson, nil
}

func (s *InboundService) addUsers(db *gorm.DB, inboundJson []byte, inboundId uint, inboundType string) ([]byte, error) {
	if !s.hasUser(inboundType) {
		return inboundJson, nil
	}

	var inbound map[string]interface{}
	err := json.Unmarshal(inboundJson, &inbound)
	if err != nil {
		return nil, err
	}

	condition := fmt.Sprintf("%d IN (SELECT json_each.value FROM json_each(clients.inbounds))", inboundId)
	inbound["users"], err = s.fetchUsers(db, inboundType, condition, inbound)
	if err != nil {
		return nil, err
	}

	return json.Marshal(inbound)
}

func (s *InboundService) initUsers(db *gorm.DB, inboundJson []byte, clientIds string, inboundType string) ([]byte, error) {
	ClientIds := strings.Split(clientIds, ",")
	if len(ClientIds) == 0 {
		return inboundJson, nil
	}

	if !s.hasUser(inboundType) {
		return inboundJson, nil
	}

	var inbound map[string]interface{}
	err := json.Unmarshal(inboundJson, &inbound)
	if err != nil {
		return nil, err
	}

	condition := fmt.Sprintf("id IN (%s)", strings.Join(ClientIds, ","))
	inbound["users"], err = s.fetchUsers(db, inboundType, condition, inbound)
	if err != nil {
		return nil, err
	}

	return json.Marshal(inbound)
}

func (s *InboundService) enabledClientNames(tx *gorm.DB, inboundId uint) (map[string]struct{}, error) {
	var names []string
	err := tx.Raw(
		"SELECT clients.name FROM clients, json_each(clients.inbounds) AS je WHERE je.value = ? AND clients.enable = true",
		inboundId).Scan(&names).Error
	if err != nil {
		return nil, err
	}
	keep := make(map[string]struct{}, len(names))
	for _, name := range names {
		keep[name] = struct{}{}
	}
	return keep, nil
}

func (s *InboundService) UpdateInboundsUsers(tx *gorm.DB, ids []uint) error {
	if !corePtr.IsRunning() {
		return nil
	}
	var inbounds []*model.Inbound
	// Only update local inbounds in the local core
	err := tx.Model(model.Inbound{}).Preload("Tls").Where("id in ? AND node_id IS NULL", ids).Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		inboundConfig, err := inbound.MarshalJSON()
		if err != nil {
			return err
		}
		inboundConfig, err = s.addUsers(tx, inboundConfig, inbound.Id, inbound.Type)
		if err != nil {
			return err
		}

		handled, err := corePtr.UpdateInboundUsers(inboundConfig)
		if err != nil {
			return err
		}
		if handled {
			// Disconnect only users no longer enabled on this inbound
			keep, err := s.enabledClientNames(tx, inbound.Id)
			if err != nil {
				return err
			}
			closed := corePtr.GetInstance().ConnTracker().CloseConnByInboundUsers(inbound.Tag, keep)
			logger.Debug("updated users of inbound ", inbound.Tag, " in place, closed ", closed, " stale connections")
			continue
		}

		// Fallback: full restart for protocols without in-place user updates
		err = corePtr.RemoveInbound(inbound.Tag)
		if err != nil && err != os.ErrInvalid {
			return err
		}
		corePtr.GetInstance().ConnTracker().CloseConnByInbound(inbound.Tag)
		err = corePtr.AddInbound(inboundConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) RestartInbounds(tx *gorm.DB, ids []uint) error {
	if !corePtr.IsRunning() {
		return nil
	}
	var inbounds []*model.Inbound
	err := tx.Model(model.Inbound{}).Preload("Tls").Where("id in ? AND node_id IS NULL", ids).Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		err = corePtr.RemoveInbound(inbound.Tag)
		if err != nil && err != os.ErrInvalid {
			return err
		}
		// Close all existing connections
		corePtr.GetInstance().ConnTracker().CloseConnByInbound(inbound.Tag)

		inboundConfig, err := inbound.MarshalJSON()
		if err != nil {
			return err
		}
		inboundConfig, err = s.addUsers(tx, inboundConfig, inbound.Id, inbound.Type)
		if err != nil {
			return err
		}
		err = corePtr.AddInbound(inboundConfig)
		if err != nil {
			return err
		}
	}
	return nil
}
