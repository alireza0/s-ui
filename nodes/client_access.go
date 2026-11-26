package nodes

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

// Fill only missing protocol credentials before local links/config are rebuilt.
// Existing UUIDs/passwords retain their values when a node is assigned.
func PrepareClientCredentials(tx *gorm.DB, client *model.Client) error {
	if !tx.Migrator().HasTable(&model.ClientNode{}) {
		return nil
	}
	whole, err := wholeNodeSelection(tx, client)
	if err != nil {
		return err
	}
	if len(whole) == 0 {
		return preserveNodeGeneration(tx, client, whole)
	}
	var config map[string]map[string]interface{}
	if err := json.Unmarshal(client.Config, &config); err != nil {
		return err
	}
	if config == nil {
		config = map[string]map[string]interface{}{}
	}
	if client.Id != 0 {
		if err := preserveWholeCredentials(tx, client, config); err != nil {
			return err
		}
	}
	for _, kind := range nodeagent.InboundTypes(true) {
		if nodeagent.ProxyInbound(kind) {
			continue
		}
		if err := ensureInboundCredentials(config, kind, client.Name); err != nil {
			return err
		}
	}
	value, err := json.Marshal(config)
	if err != nil {
		return err
	}
	client.Config = value
	return preserveNodeGeneration(tx, client, whole)
}

func preserveWholeCredentials(tx *gorm.DB, client *model.Client, config map[string]map[string]interface{}) error {
	var old model.Client
	if err := tx.First(&old, client.Id).Error; err != nil {
		return err
	}
	var previous map[string]map[string]interface{}
	if err := json.Unmarshal(old.Config, &previous); err != nil {
		return err
	}
	for kind, user := range previous {
		if !nodeagent.SupportsInbound(kind, true) && kind != "shadowsocks16" {
			continue
		}
		mergePreviousCredentials(config, kind, user, client.Name)
	}
	return nil
}

func mergePreviousCredentials(config map[string]map[string]interface{}, kind string, user map[string]interface{}, name string) {
	if config[kind] == nil {
		config[kind] = map[string]interface{}{}
		for field, value := range user {
			config[kind][field] = value
		}
		config[kind]["name"] = name
	}
	for _, field := range []string{"uuid", "password", "auth_str", "userkey", "username"} {
		if _, exists := config[kind][field]; !exists {
			if value, ok := user[field]; ok {
				config[kind][field] = value
			}
		}
	}
}

func ensureInboundCredentials(config map[string]map[string]interface{}, kind, name string) error {
	user := config[kind]
	newUser := user == nil
	if newUser {
		user = map[string]interface{}{"name": name}
		config[kind] = user
	}
	if nodeagent.NeedsUUID(kind) {
		if err := ensureCredentialUUID(user); err != nil {
			return err
		}
	}
	if nodeagent.NeedsPassword(kind) {
		if err := ensureCredentialPassword(user); err != nil {
			return err
		}
	}
	if kind == "vless" && newUser {
		if _, ok := user["flow"]; !ok {
			user["flow"] = "xtls-rprx-vision"
		}
	}
	return nil
}

func ensureCredentialUUID(user map[string]interface{}) error {
	if value, exists := user["uuid"]; exists && value != nil {
		if _, ok := value.(string); !ok {
			return errors.New("node UUID must be a string")
		}
	}
	value, _ := user["uuid"].(string)
	if value == "" {
		generated, err := uuid.NewV4()
		if err != nil {
			return err
		}
		value = generated.String()
		user["uuid"] = value
	}
	if _, err := uuid.FromString(value); err != nil {
		return errors.New("invalid UUID for selected node protocol")
	}
	return nil
}

func ensureCredentialPassword(user map[string]interface{}) error {
	if value, exists := user["password"]; exists && value != nil {
		if _, ok := value.(string); !ok {
			return errors.New("node password must be a string")
		}
	}
	value, _ := user["password"].(string)
	if value == "" {
		buf := make([]byte, 24)
		if _, err := rand.Read(buf); err != nil {
			return err
		}
		user["password"] = base64.RawURLEncoding.EncodeToString(buf)
	}
	return nil
}

func preserveNodeGeneration(tx *gorm.DB, client *model.Client, selected []uint) error {
	if client.Id == 0 {
		client.NodeAccessChanged = len(selected) > 0
		return nil
	}
	var old model.Client
	if err := tx.First(&old, client.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	client.NodeGeneration = old.NodeGeneration
	_, previous, err := database.NodeProjection(tx, client.Id)
	if err != nil {
		return err
	}
	if len(previous) == 0 && len(selected) == 0 {
		return nil
	}
	var before, after interface{}
	if err := json.Unmarshal(old.Config, &before); err != nil {
		return err
	}
	if err := json.Unmarshal(client.Config, &after); err != nil {
		return err
	}
	if old.Enable != client.Enable || old.Expiry != client.Expiry || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(previous, selected) {
		client.NodeGeneration++
		client.NodeAccessChanged = true
	}
	return nil
}

func HydrateClientAssignments(db *gorm.DB, clients []model.Client) error {
	ids := make([]uint, 0, len(clients))
	for _, client := range clients {
		ids = append(ids, client.Id)
	}
	projections, err := database.NodeProjections(db, ids)
	if err != nil {
		return err
	}
	for i := range clients {
		p := projections[clients[i].Id]
		clients[i].NodeInbounds, _ = json.Marshal(p.Inbounds)
		clients[i].Nodes, _ = json.Marshal(p.Nodes)
	}
	return nil
}

func SaveClientAssignments(tx *gorm.DB, client *model.Client) error {
	if err := saveWholeNodeAssignments(tx, client); err != nil {
		return err
	}
	if client.NodeAccessChanged {
		if err := database.QueueClientNodeAccess(tx, []uint{client.Id}); err != nil {
			return err
		}
	}
	return nil
}

// PrepareClientEnableChange queues reconciliation and adds the access fence to
// the caller's bulk enable update.
func PrepareClientEnableChange(tx *gorm.DB, ids []uint, enable bool, columns map[string]interface{}) error {
	if err := database.QueueClientNodeAccess(tx, ids); err != nil {
		return err
	}
	columns["node_generation"] = gorm.Expr("CASE WHEN enable = ? THEN node_generation + 1 ELSE node_generation END", !enable)
	return nil
}

func PrepareClientResets(tx *gorm.DB, clients []*model.Client) error {
	for _, client := range clients {
		var old model.Client
		if err := tx.Select("id", "enable", "expiry", "node_generation").First(&old, client.Id).Error; err != nil {
			return err
		}
		client.NodeGeneration = old.NodeGeneration
		if old.Enable != client.Enable || old.Expiry != client.Expiry {
			client.NodeGeneration++
			if err := database.QueueClientNodeAccess(tx, []uint{client.Id}); err != nil {
				return err
			}
		}
	}
	return nil
}
