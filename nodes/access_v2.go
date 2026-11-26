package nodes

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var accessLocks sync.Map

func effectiveUserName(tx *gorm.DB, node model.Node, client model.Client) string {
	name := userName(client)
	var grant model.ClientNode
	if tx.First(&grant, "client_id = ? AND node_id = ? AND active = ?", client.Id, node.Id, true).Error == nil {
		name += fmt.Sprintf("/n%d", grant.Generation)
	}
	return name
}

// Does not parse Node.Config, templates, TLS or inbound Options. A broken new
// structural draft cannot prevent constructing a complete revocation snapshot.
func compileAccess(tx *gorm.DB, node model.Node, extended, proxies bool) (nodeagent.AccessSnapshot, error) {
	a := nodeagent.AccessSnapshot{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Clients: []nodeagent.Grant{}}
	if !node.Enable {
		return a, nil
	}
	var clients []model.Client
	query := tx.Model(&model.Client{}).Joins("JOIN client_nodes AS binding ON binding.client_id = clients.id").Where("binding.node_id = ? AND binding.active = ?", node.Id, true)
	if err := query.Where("clients.enable = ?", true).Order("clients.id").Find(&clients).Error; err != nil {
		return a, err
	}
	for _, client := range clients {
		grant, include, err := compileClientGrant(tx, node, client, extended, proxies)
		if err != nil {
			return a, err
		}
		if include {
			a.Clients = append(a.Clients, grant)
		}
	}
	return a, nil
}

func newNodeCredential(kind, name string) (json.RawMessage, error) {
	value := map[string]any{"name": name}
	if nodeagent.UsernameAuth(kind) {
		value = map[string]any{"username": name}
	}
	if nodeagent.NeedsUUID(kind) {
		id, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}
		value["uuid"] = id.String()
	} else {
		size := 24
		encoding := base64.RawURLEncoding
		switch kind {
		case "shadowsocks":
			size, encoding = 32, base64.StdEncoding
		case "shadowsocks16":
			size, encoding = 16, base64.StdEncoding
		}
		secret := make([]byte, size)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		value[nodeagent.SecretField(kind)] = encoding.EncodeToString(secret)
	}
	return json.Marshal(value)
}

func prepareAccess(db *gorm.DB, node model.Node, status nodeagent.StatusV2) (nodeagent.AccessSnapshot, error) {
	var desired nodeagent.AccessSnapshot
	err := db.Transaction(func(tx *gorm.DB) error { return prepareAccessTransaction(tx, node, status, &desired) })
	return desired, err
}

func ReconcileAccess(ctx context.Context, id uint) (failure error) {
	lock, _ := accessLocks.LoadOrStore(id, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	db := database.GetDB()
	var node model.Node
	if err := db.First(&node, id).Error; err != nil {
		return err
	}
	if !managedNodeModel(node) {
		return nil
	}
	if err := ensureAccessTask(db, id); err != nil {
		return err
	}
	var task model.NodeTask
	if err := db.First(&task, "node_id = ? AND kind = ?", id, "access").Error; err != nil {
		return err
	}
	if task.NextAttempt > time.Now().Unix() {
		return nil
	}
	token, err := uuid.NewV4()
	if err != nil {
		return err
	}
	lease := token.String()
	result := db.Model(&model.NodeTask{}).Where("id = ? AND lease_until < ?", task.Id, time.Now().Unix()).Updates(map[string]interface{}{"lease_token": lease, "lease_until": time.Now().Unix() + 90})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	defer func() { finishAccessTask(db, node, task, lease, failure) }()
	client, closeClient, err := controlClient(ctx, node)
	if err != nil {
		return err
	}
	defer closeClient()
	status, err := getStatusV2(ctx, client, node)
	if err != nil {
		return err
	}
	desired, err := prepareAccess(db, node, status)
	if err != nil {
		return err
	}
	if status.BootstrapPending || status.AppliedAccessRevision != desired.Revision || status.AppliedAccessDigest != desired.Digest() || !status.Running {
		status, err = putV2(ctx, client, node, "/v2/access", desired)
		if err != nil {
			return err
		}
	}
	if status.AppliedAccessRevision != desired.Revision || status.AppliedAccessDigest != desired.Digest() {
		return errors.New("access_ack_mismatch")
	}
	return acknowledgeAccess(db, node, desired)
}

func accessLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		var nodes []model.Node
		if database.GetDB().Where("agent_protocol = ?", 2).Find(&nodes).Error == nil {
			limit := make(chan struct{}, 4)
			var wait sync.WaitGroup
			for _, node := range nodes {
				if ctx.Err() != nil {
					break
				}
				limit <- struct{}{}
				wait.Add(1)
				go func(id uint) { defer wait.Done(); defer func() { <-limit }(); _ = ReconcileAccess(ctx, id) }(node.Id)
			}
			wait.Wait()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func compileGrantCredentials(g *nodeagent.Grant, config map[string]json.RawMessage, name string, extended, proxies bool) (bool, error) {
	changed := false
	protocols := nodeagent.InboundTypes(extended)
	if proxies {
		protocols = append(protocols, "shadowsocks16")
	}
	for _, kind := range protocols {
		if nodeagent.ProxyInbound(kind) && !proxies {
			continue
		}
		raw := config[kind]
		raw, generated, include, err := grantProtocolCredential(kind, name, raw)
		if err != nil {
			return changed, err
		}
		if generated {
			config[kind] = raw
			changed = true
		}
		if !include {
			continue
		}
		if len(raw) > 0 {
			g.Credentials[kind] = raw
		}
	}
	return changed, nil
}

func compileClientGrant(tx *gorm.DB, node model.Node, client model.Client, extended, proxies bool) (nodeagent.Grant, bool, error) {
	if client.Expiry > 0 && client.Expiry <= time.Now().Unix() {
		return nodeagent.Grant{}, false, nil
	}
	var config map[string]json.RawMessage
	if json.Unmarshal(client.Config, &config) != nil {
		return nodeagent.Grant{}, false, nil
	} // corrupt clients fail closed individually
	g := nodeagent.Grant{Name: effectiveUserName(tx, node, client), TrafficID: clientTrafficID(client), Expires: client.Expiry, All: true, Credentials: map[string]json.RawMessage{}}
	if config == nil {
		config = map[string]json.RawMessage{}
	}
	changed, err := compileGrantCredentials(&g, config, client.Name, extended, proxies)
	if err != nil {
		return g, false, err
	}
	probe := nodeagent.AccessSnapshot{NodeID: uint64(node.Id), Epoch: node.ControlEpoch}
	probe.Revision = 1
	probe.Clients = []nodeagent.Grant{g}
	if probe.Validate() != nil {
		return g, false, nil
	}
	if changed {
		data, err := json.Marshal(config)
		if err != nil {
			return g, false, err
		}
		if err := tx.Model(&client).Update("config", json.RawMessage(data)).Error; err != nil {
			return g, false, err
		}
	}
	return g, true, nil
}

func prepareAccessTransaction(tx *gorm.DB, node model.Node, status nodeagent.StatusV2, desired *nodeagent.AccessSnapshot) error {
	var current model.Node
	if err := tx.First(&current, node.Id).Error; err != nil {
		return err
	}
	if current.ManagementGeneration != node.ManagementGeneration || current.ControlEpoch != status.Epoch || current.Address != node.Address {
		return errors.New("access_target_changed")
	}
	var state model.NodeAccessState
	err := tx.First(&state, whereNodeID, node.Id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	state.NodeId = node.Id
	if current.RestoreAccessPending {
		state.DesiredRevision = max(state.DesiredRevision, status.AccessRevision)
	} else if err := validateAccessCheckpoint(state, status); err != nil {
		return err
	}
	fresh, err := compileAccess(tx, current, extendedInboundsAvailable(tx, current, status), proxyInboundsAvailable(tx, current, status))
	if err != nil {
		return err
	}
	body, _ := json.Marshal(fresh)
	fingerprint := versionDigest(body)
	if current.RestoreAccessPending || state.DesiredRevision == 0 || state.DesiredFingerprint != fingerprint {
		state.DesiredRevision++
		fresh.Revision = state.DesiredRevision
		if err := fresh.Validate(); err != nil {
			return err
		}
		state.DesiredFingerprint = fingerprint
		state.DesiredSnapshot, _ = json.Marshal(fresh)
		if err := tx.Save(&state).Error; err != nil {
			return err
		}
	}
	if err := json.Unmarshal(state.DesiredSnapshot, desired); err != nil {
		return err
	}
	return finishAccessRestore(tx, current)
}

func finishAccessLease(tx *gorm.DB, task model.NodeTask, lease string, failure error) error {
	var latest model.NodeTask
	if err := tx.First(&latest, "id = ? AND lease_token = ?", task.Id, lease).Error; err != nil {
		return err
	}
	updates := map[string]interface{}{"lease_token": "", "lease_until": 0}
	if latest.Generation != task.Generation {
		updates["state"] = "queued"
		updates["next_attempt"] = 0
	} else if failure == nil {
		updates["state"] = "done"
		updates["last_error"] = ""
		updates["attempts"] = 0
		updates["next_attempt"] = 0
	} else {
		delay := int64(1) << min(latest.Attempts, 5)
		updates["state"] = "queued"
		updates["last_error"] = "access_delivery_failed"
		updates["attempts"] = latest.Attempts + 1
		updates["next_attempt"] = time.Now().Unix() + delay
	}
	return tx.Model(&latest).Updates(updates).Error
}

func finishAccessTask(db *gorm.DB, node model.Node, task model.NodeTask, lease string, failure error) {
	id := node.Id
	_ = db.Transaction(func(tx *gorm.DB) error { return finishAccessLease(tx, task, lease, failure) })
	if failure != nil {
		code := "access_delivery_failed"
		switch failure.Error() {
		case "access_state_rollback_detected", "access_state_rollback_or_conflict_detected", "agent_epoch_changed_recovery_required", "traffic_accounting_failed":
			code = failure.Error()
		}
		_ = db.Transaction(func(tx *gorm.DB) error {
			var current model.Node
			if err := tx.First(&current, id).Error; err != nil {
				return err
			}
			if current.ManagementGeneration != node.ManagementGeneration || current.ControlEpoch != node.ControlEpoch {
				return nil
			}
			row := model.NodeAccessState{NodeId: id, LastError: code}
			return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}}, DoUpdates: clause.Assignments(map[string]interface{}{"last_error": code})}).Create(&row).Error
		})
	}
}

func grantProtocolCredential(kind, name string, raw json.RawMessage) (json.RawMessage, bool, bool, error) {
	generated := false
	if nodeagent.ExtendedInbound(kind) || kind == "shadowsocks16" {
		if len(raw) == 0 || string(raw) == "null" {
			var err error
			raw, err = newNodeCredential(kind, name)
			if err != nil {
				return raw, generated, false, err
			}
			generated = true
		}
		if nodeagent.ValidateCredential(kind, raw) != nil {
			return raw, generated, false, nil
		}
	}
	return raw, generated, true, nil
}

func ensureAccessTask(db *gorm.DB, id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		if err := tx.First(&current, id).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "kind"}}, DoNothing: true}).Create(&model.NodeTask{NodeId: id, Kind: "access", Generation: 1, State: "queued"}).Error
	})
}

func acknowledgeAccess(db *gorm.DB, node model.Node, desired nodeagent.AccessSnapshot) error {
	body, _ := json.Marshal(desired)
	return db.Transaction(func(tx *gorm.DB) error {
		var target model.Node
		if err := tx.First(&target, node.Id).Error; err != nil {
			return err
		}
		if target.ManagementGeneration != node.ManagementGeneration || target.ControlEpoch != node.ControlEpoch {
			return errors.New("management_target_changed")
		}
		return tx.Model(&model.NodeAccessState{}).Where("node_id = ? AND applied_revision <= ?", node.Id, desired.Revision).Updates(map[string]interface{}{"applied_revision": desired.Revision, "applied_snapshot": json.RawMessage(body), "applied_digest": desired.Digest(), "last_error": ""}).Error
	})
}
