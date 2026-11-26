package nodes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/alireza0/s-ui/util"
	"gorm.io/gorm"
)

type appliedInboundUsers struct {
	Tag   string                   `json:"tag"`
	Users []map[string]interface{} `json:"users"`
}

type connectionPublisher struct {
	node        model.Node
	snapshot    Snapshot
	applied     []appliedInboundUsers
	credentials map[string]map[string]interface{}
	userName    string
	inboundIDs  map[string]uint
}

func clientConnections(tx *gorm.DB, client model.Client) ([]clientConnection, error) {
	result := []clientConnection{}
	if !client.Enable || (client.Expiry > 0 && client.Expiry <= time.Now().Unix()) {
		return result, nil
	}
	if !tx.Migrator().HasTable(&model.ClientNode{}) {
		return result, nil
	}
	var assigned []uint
	err := tx.Model(&model.ClientNode{}).Where("client_id = ? AND active = ?", client.Id, true).Pluck("node_id", &assigned).Error
	if err != nil {
		return nil, err
	}
	if len(assigned) == 0 {
		return result, nil
	}
	var nodes []model.Node
	if err := tx.Where("id IN ? AND enable = ? AND applied_revision > 0", assigned, true).Order("id").Find(&nodes).Error; err != nil {
		return nil, err
	}
	var credentials map[string]map[string]interface{}
	if err := json.Unmarshal(client.Config, &credentials); err != nil {
		return nil, err
	}
	for _, node := range nodes {
		connections, err := publishNodeConnections(tx, client, node, credentials)
		if err != nil {
			return nil, err
		}
		result = append(result, connections...)
	}
	return result, nil
}

func publicationSnapshot(tx *gorm.DB, node model.Node) (Snapshot, bool) {
	var snapshot Snapshot
	if !managedNodeModel(node) || !node.AgentRunning || json.Unmarshal(node.AppliedSnapshot, &snapshot) != nil || !snapshot.Node.Enable {
		return snapshot, false
	}
	var acl model.NodeAccessState
	if tx.First(&acl, whereNodeID, node.Id).Error != nil || acl.AppliedRevision == 0 {
		return snapshot, false
	}
	var access nodeagent.AccessSnapshot
	if json.Unmarshal(acl.AppliedSnapshot, &access) != nil || access.Epoch != node.ControlEpoch {
		return snapshot, false
	}
	spec := nodeagent.Structure{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Revision: snapshot.Deployment.Revision, Config: snapshot.Deployment.Config}
	body, err := nodeagent.Compose(spec, access, time.Now().Unix())
	if err != nil {
		return snapshot, false
	}
	snapshot.Deployment.Config = body
	return snapshot, true
}

func publishNodeConnections(tx *gorm.DB, client model.Client, node model.Node, credentials map[string]map[string]interface{}) ([]clientConnection, error) {
	result := []clientConnection{}
	snapshot, ok := publicationSnapshot(tx, node)
	if !ok {
		return result, nil
	}
	var applied struct {
		Inbounds []appliedInboundUsers `json:"inbounds"`
	}
	if err := json.Unmarshal(snapshot.Deployment.Config, &applied); err != nil {
		return nil, err
	}
	publisher := connectionPublisher{node: node, snapshot: snapshot, applied: applied.Inbounds, credentials: credentials, userName: effectiveUserName(tx, node, client), inboundIDs: map[string]uint{}}
	for _, inbound := range snapshot.Inbounds {
		publisher.inboundIDs[inbound.ConnectionKey] = inbound.Id
	}
	for _, inbound := range snapshot.Inbounds {
		if !certificatePublished(node, snapshot, inbound) {
			continue
		}
		connection, valid, err := publisher.connection(inbound)
		if err != nil {
			return nil, err
		}
		if valid {
			result = append(result, connection)
		}
	}
	return result, nil
}

func (p connectionPublisher) credentialMatches(inbound model.NodeInbound, options map[string]interface{}) bool {
	method, _ := options["method"].(string)
	expected := p.credentials[nodeagent.CredentialKey(inbound.Type, method)]
	if len(expected) == 0 {
		return false
	}
	userField, userValue := "name", any(p.userName)
	if nodeagent.UsernameAuth(inbound.Type) {
		userField, userValue = "username", expected["username"]
	}
	valid := false
	for _, server := range p.applied {
		if server.Tag != serverTag(p.node.Id, inbound.Id) {
			continue
		}
		for _, user := range server.Users {
			if user[userField] == userValue {
				valid = publicationCredentialsMatch(user, expected, supportsVision(options))
			}
		}
	}
	return valid
}

func publicationCredentialsMatch(user, expected map[string]interface{}, vision bool) bool {
	valid := true
	for key, value := range expected {
		if key == "name" || (key == "flow" && !vision) {
			continue
		}
		if !reflect.DeepEqual(user[key], value) {
			valid = false
		}
	}
	return valid
}

func (p connectionPublisher) label(inbound model.NodeInbound, options map[string]interface{}) string {
	if !p.snapshot.Node.Managed {
		return fmt.Sprintf("%s / %s [node:%d,in:%d]", p.snapshot.Node.Name, inbound.Tag, p.node.Id, inbound.Id)
	}
	label := p.snapshot.Node.Name + " · " + strings.ToUpper(inbound.Type)
	count := 0
	for _, other := range p.snapshot.Inbounds {
		if other.Type == inbound.Type {
			count++
		}
	}
	if count > 1 {
		if inbound.DisplayName != "" {
			label += " · " + inbound.DisplayName
		} else {
			label += fmt.Sprintf(" · %v", options["listen_port"])
		}
	}
	return label
}

func (p connectionPublisher) connection(inbound model.NodeInbound) (clientConnection, bool, error) {
	var options map[string]interface{}
	if err := json.Unmarshal(inbound.Options, &options); err != nil {
		return clientConnection{}, false, err
	}
	if !p.credentialMatches(inbound, options) {
		return clientConnection{}, false, nil
	}
	for _, key := range []string{"tls", "users", "type", "tag"} {
		delete(options, key)
	}
	body, _ := json.Marshal(options)
	addresses, outbound, err := clientPublication(inbound.ClientOptions, p.snapshot.Node.Address, options["listen_port"])
	if err != nil {
		return clientConnection{}, false, err
	}
	// A node's Realm registration credential is never a client default.
	if realm, ok := options["realm"].(map[string]interface{}); ok {
		delete(realm, "token")
		body, _ = json.Marshal(options)
	}
	value := &model.Inbound{Id: inbound.Id, Type: inbound.Type, Tag: p.label(inbound, options), Enable: true, Options: body, Addrs: addresses, OutJson: outbound}
	serverTLS, publicTLS, err := clientTLS(inbound)
	if err != nil {
		return clientConnection{}, false, err
	}
	if serverTLS != nil {
		value.TlsId = inbound.Id
		value.Tls = &model.Tls{Server: serverTLS, Client: publicTLS}
	}
	if err := util.FillOutJson(value, p.snapshot.Node.Address); err != nil {
		return clientConnection{}, false, err
	}
	return p.shadowTLSConnection(value, options)
}

func (p connectionPublisher) shadowTLSConnection(value *model.Inbound, options map[string]interface{}) (clientConnection, bool, error) {
	connection := clientConnection{inbound: value}
	if value.Type != "shadowtls" {
		return connection, true, nil
	}
	detour, _ := options["detour"].(string)
	connection.target = p.inboundIDs[detour]
	handshake, _ := options["handshake"].(map[string]any)
	var clientOut map[string]any
	if err := json.Unmarshal(value.OutJson, &clientOut); err != nil {
		return clientConnection{}, false, err
	}
	if tls, ok := clientOut["tls"].(map[string]any); ok {
		tls["server_name"] = handshake["server"]
	}
	value.OutJson, _ = json.Marshal(clientOut)
	return connection, true, nil
}
