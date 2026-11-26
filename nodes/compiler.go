// Package nodes owns remote node configuration; it never calls local corePtr.
package nodes

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

type Publication struct {
	Id      uint   `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Enable  bool   `json:"enable"`
	Managed bool   `json:"managed,omitempty"`
}

// Snapshot is private: it includes server configuration and user credentials.
type Snapshot struct {
	TemplateId      uint                `json:"template_id,omitempty"`
	TemplateVersion uint64              `json:"template_version,omitempty"`
	SettingsVersion uint64              `json:"settings_version,omitempty"`
	Node            Publication         `json:"node"`
	Inbounds        []model.NodeInbound `json:"inbounds"`
	Deployment      structureSnapshot   `json:"deployment"`
}

// Keep the persisted snapshot encoding so existing applied revisions and
// fingerprints survive upgrades. Client credentials live in the access state.
type structureSnapshot struct {
	NodeID   uint64          `json:"node_id"`
	Revision uint64          `json:"revision"`
	Config   json.RawMessage `json:"config"`
	Users    []struct{}      `json:"users"`
}

func serverTag(nodeID, inboundID uint) string {
	return fmt.Sprintf("node-%d-inbound-%d", nodeID, inboundID)
}
func userName(client model.Client) string {
	if client.NodeGeneration == 0 {
		return fmt.Sprintf("client/%d", client.Id)
	}
	return fmt.Sprintf("client/%d/g%d", client.Id, client.NodeGeneration)
}

func Compile(tx *gorm.DB, node model.Node) (Snapshot, error) {
	s := Snapshot{Node: Publication{Id: node.Id, Name: node.Name, Address: node.LinkAddress, Enable: node.Enable}, Inbounds: []model.NodeInbound{},
		Deployment: structureSnapshot{NodeID: uint64(node.Id), Users: []struct{}{}}}
	if node.TemplateTargetVersion > 0 {
		s.TemplateId, s.TemplateVersion, s.SettingsVersion = node.TemplateId, node.MaterializedTemplateVersion, node.SettingsVersion
	}
	var installation model.NodeInstallation
	s.Node.Managed = tx.First(&installation, whereNodeID, node.Id).Error == nil || node.TemplateTargetVersion > 0
	config, err := nodeBaseConfig(node)
	if err != nil {
		return s, err
	}
	var inbounds []model.NodeInbound
	if node.Enable {
		if err := tx.Where("node_id = ? AND enable = ? AND retired = ?", node.Id, true, false).Order("id").Find(&inbounds).Error; err != nil {
			return s, err
		}
	}
	serverInbounds := make([]interface{}, 0, len(inbounds))
	detours, err := compileNodeDetours(inbounds)
	if err != nil {
		return s, err
	}
	builder := snapshotCompiler{node: node, detours: detours, snapshot: &s}
	for _, inbound := range inbounds {
		options, err := builder.compileInbound(inbound)
		if err != nil {
			return s, err
		}
		serverInbounds = append(serverInbounds, options)
	}
	config["inbounds"] = serverInbounds
	b, err := json.Marshal(config)
	if err == nil {
		err = nodeagent.ValidateCertificateReferences(b)
	}
	s.Deployment.Config = b
	return s, err
}

func clientTLS(inbound model.NodeInbound) (json.RawMessage, json.RawMessage, error) {
	var options map[string]interface{}
	if err := json.Unmarshal(inbound.Options, &options); err != nil {
		return nil, nil, err
	}
	server, _ := options["tls"].(map[string]interface{})
	if server == nil || server["enabled"] != true {
		return nil, nil, nil
	}
	client := map[string]interface{}{}
	if len(inbound.ClientTLS) > 0 && string(inbound.ClientTLS) != "null" {
		if err := json.Unmarshal(inbound.ClientTLS, &client); err != nil {
			return nil, nil, err
		}
	}
	publicServer := map[string]interface{}{}
	for _, key := range []string{"enabled", "server_name", "alpn", "min_version", "max_version", "cipher_suites", "curve_preferences", "handshake_timeout", "certificate"} {
		if value, ok := server[key]; ok {
			publicServer[key] = value
			if key != "certificate" {
				client[key] = value
			}
		}
	}
	if reality, ok := server["reality"].(map[string]interface{}); ok && reality["enabled"] == true {
		key, _ := reality["private_key"].(string)
		decoded, err := base64.RawURLEncoding.DecodeString(key)
		if err != nil {
			return nil, nil, errors.New("invalid REALITY private key encoding")
		}
		private, err := ecdh.X25519().NewPrivateKey(decoded)
		if err != nil {
			return nil, nil, errors.New("invalid REALITY private key")
		}
		client["reality"] = map[string]interface{}{"enabled": true, "public_key": base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes())}
		if client["utls"] == nil {
			client["utls"] = map[string]interface{}{"enabled": true, "fingerprint": "chrome"}
		}
		publicServer["reality"] = map[string]interface{}{"enabled": true, "short_id": reality["short_id"]}
	}
	if hasPrivateClientTLS(client) {
		return nil, nil, errors.New("private server TLS material must not be published to clients")
	}
	a, err := json.Marshal(publicServer)
	if err != nil {
		return nil, nil, err
	}
	b, err := json.Marshal(client)
	return a, b, err
}

func hasPrivateClientTLS(value interface{}) bool {
	switch x := value.(type) {
	case map[string]interface{}:
		for key, item := range x {
			if key == "private_key" || key == "key" || strings.HasSuffix(key, "_path") {
				return true
			}
			if hasPrivateClientTLS(item) {
				return true
			}
		}
	case []interface{}:
		for _, item := range x {
			if hasPrivateClientTLS(item) {
				return true
			}
		}
	}
	return false
}

func supportsVision(options map[string]interface{}) bool {
	tls, _ := options["tls"].(map[string]interface{})
	transport, _ := options["transport"].(map[string]interface{})
	return tls != nil && tls["enabled"] == true && (transport == nil || transport["type"] == nil || transport["type"] == "")
}

func nodeBaseConfig(node model.Node) (map[string]interface{}, error) {
	config := map[string]interface{}{"log": map[string]interface{}{"level": "error"}, "outbounds": []interface{}{map[string]interface{}{"type": "direct", "tag": "direct"}}, "route": map[string]interface{}{"final": "direct"}}
	// Disable removes the listeners even when the saved configuration cannot be applied.
	var base json.RawMessage
	if node.Enable {
		base = node.Config
	}
	if len(base) > 0 && string(base) != "null" {
		if err := json.Unmarshal(base, &config); err != nil {
			return nil, err
		}
	}
	return config, nil
}

type snapshotCompiler struct {
	node     model.Node
	detours  map[string]string
	snapshot *Snapshot
}

func (builder snapshotCompiler) compileInbound(inbound model.NodeInbound) (map[string]interface{}, error) {
	var options map[string]interface{}
	if err := json.Unmarshal(inbound.Options, &options); err != nil || options == nil {
		return nil, errors.New("invalid node inbound options")
	}
	options["type"], options["tag"] = inbound.Type, serverTag(builder.node.Id, inbound.Id)
	if target := builder.detours[inbound.ConnectionKey]; target != "" {
		options["detour"] = target
	}
	delete(options, "users")
	builder.snapshot.Inbounds = append(builder.snapshot.Inbounds, inbound)
	return options, nil
}
