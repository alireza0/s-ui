package nodes

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func physicalNodeHost(tx *gorm.DB, node model.Node) string {
	var installation model.NodeInstallation
	if tx.First(&installation, whereNodeID, node.Id).Error == nil && installation.IP != "" {
		return strings.ToLower(installation.IP)
	}
	u, _ := url.Parse(node.Address)
	if u != nil {
		return strings.ToLower(u.Hostname())
	}
	return ""
}

type generatedMaterial struct {
	Node         uint
	Key, Purpose string
	Value        json.RawMessage
}

func loadGeneratedValue(tx *gorm.DB, nodeID uint, key, purpose string) (json.RawMessage, error) {
	var row model.NodeGeneratedMaterial
	if err := tx.First(&row, "node_id = ? AND connection_key = ? AND purpose = ?", nodeID, key, purpose).Error; err != nil {
		return nil, err
	}
	plain, err := openSecret(row.EncryptedValue)
	if err != nil {
		return nil, errors.New("generated_material_key_unavailable")
	}
	var stored generatedMaterial
	if json.Unmarshal([]byte(plain), &stored) != nil || stored.Node != nodeID || stored.Key != key || stored.Purpose != purpose {
		return nil, errors.New("generated_material_identity_mismatch")
	}
	return stored.Value, nil
}

func storeGeneratedValue(tx *gorm.DB, nodeID uint, key, purpose string, value json.RawMessage) error {
	plain, err := json.Marshal(generatedMaterial{nodeID, key, purpose, value})
	if err != nil {
		return err
	}
	sealed, err := sealSecret(string(plain))
	if err != nil {
		return err
	}
	return tx.Create(&model.NodeGeneratedMaterial{NodeId: nodeID, ConnectionKey: key, Purpose: purpose, EncryptedValue: sealed}).Error
}

func generatedValue(tx *gorm.DB, node model.Node, entry SpecEntry, purpose string) (json.RawMessage, error) {
	raw, err := loadGeneratedValue(tx, node.Id, entry.Key, purpose)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return raw, err
	}
	var value interface{}
	switch purpose {
	case "reality_private_key":
		key, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		value = base64.RawURLEncoding.EncodeToString(key.Bytes())
	case "reality_short_id":
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		value = []string{hex.EncodeToString(buf)}
	case "hysteria2_obfs_password":
		buf := make([]byte, 24)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		value = base64.RawURLEncoding.EncodeToString(buf)
	case "shadowsocks_key16", "shadowsocks_key32", "snell_psk":
		size := 32
		if purpose == "shadowsocks_key16" {
			size = 16
		}
		buf := make([]byte, size)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		value = base64.StdEncoding.EncodeToString(buf)
	default:
		return nil, errors.New("unknown_generated_purpose")
	}
	raw, _ = json.Marshal(value)
	return raw, storeGeneratedValue(tx, node.Id, entry.Key, purpose, raw)
}

// Caller owns a short database transaction; no network or agent calls here.
func materializeTemplate(tx *gorm.DB, node *model.Node, occupied map[listenerSocket]bool) error {
	if !node.Enable || node.TemplateTargetVersion == 0 {
		return nil
	}
	spec, err := liveVersion(tx, *node)
	if err != nil {
		return err
	}
	profileEntries, err := materializedTLSProfiles(tx, &spec)
	if err != nil {
		return err
	}
	values, err := materializedOverrides(tx, node.Id, spec, profileEntries)
	if err != nil {
		return err
	}
	overrides, err := validateSpecOverrides(spec, values)
	if err != nil {
		return err
	}
	ctx := ResolveContext{Parameters: map[string]json.RawMessage{}, Generated: map[string]json.RawMessage{}, Overrides: values}
	for key, value := range map[string]interface{}{"node_address": node.LinkAddress, "node_name": node.Name, "node_id": node.Id} {
		ctx.Parameters[key], _ = json.Marshal(value)
	}
	domain := node.LinkAddress
	var installation model.NodeInstallation
	if tx.First(&installation, whereNodeID, node.Id).Error == nil && installation.Domain != "" {
		domain = installation.Domain
	}
	if validPublishedAddress(domain) && net.ParseIP(strings.Trim(domain, "[]")) == nil {
		ctx.Parameters["primary_domain"], _ = json.Marshal(domain)
	}
	if err := resourceTLSContext(tx, *node, &ctx); err != nil {
		return err
	}
	host := physicalNodeHost(tx, *node)
	var primaryDomain string
	_ = json.Unmarshal(ctx.Parameters["primary_domain"], &primaryDomain)
	ctx.LocalAddresses = []string{host, node.LinkAddress, domain, primaryDomain}
	requests, err := listenerRequests(tx, *node, spec, overrides)
	if err != nil {
		return err
	}
	listeners, err := reserveListenerPlan(tx, node.Id, host, requests, occupied)
	if err != nil {
		return err
	}
	for _, listener := range listeners {
		ctx.Generated[listener.Key+listenPortPath], _ = json.Marshal(listener.Port)
	}
	if err := materializedSecrets(tx, *node, spec, overrides, &ctx); err != nil {
		return err
	}
	resolved, err := ResolveSpec(spec, ctx)
	if err != nil {
		return err
	}
	if err := persistMaterializedEntries(tx, *node, resolved); err != nil {
		return err
	}
	node.Config, node.MaterializedTemplateVersion = resolved.Config, node.TemplateTargetVersion
	return tx.Model(node).Updates(map[string]interface{}{"config": node.Config, "materialized_template_version": node.MaterializedTemplateVersion}).Error
}

func materializedTLSProfiles(tx *gorm.DB, spec *TemplateSpec) (map[string]bool, error) {
	profileEntries := map[string]bool{}
	for i := range spec.Entries {
		entry := &spec.Entries[i]
		if entry.TLSProfileID == 0 {
			continue
		}
		profileEntries[entry.Key] = true
		if err := applyTLSProfile(tx, entry); err != nil {
			return nil, err
		}
	}
	return profileEntries, nil
}

func materializedOverrides(tx *gorm.DB, nodeID uint, spec TemplateSpec, profileEntries map[string]bool) ([]SpecOverride, error) {
	var values []SpecOverride
	var storedOverrides []model.NodeOverride
	if err := tx.Where("node_id = ? AND archived = ?", nodeID, false).Order("connection_key, path").Find(&storedOverrides).Error; err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, entry := range spec.Entries {
		keys[entry.Key] = true
	}
	for _, row := range storedOverrides {
		if !keys[row.ConnectionKey] {
			if err := tx.Model(&row).Update("archived", true).Error; err != nil {
				return nil, err
			}
			continue
		}
		if profileEntries[row.ConnectionKey] && (row.Path == "/tls" || strings.HasPrefix(row.Path, "/tls/")) {
			continue
		}
		var source ValueSource
		if json.Unmarshal(row.Source, &source) != nil {
			return nil, errors.New("invalid_stored_node_override")
		}
		values = append(values, SpecOverride{ConnectionKey: row.ConnectionKey, Path: row.Path, Operation: row.Operation, Source: source})
	}
	return values, nil
}

func materializedSecrets(tx *gorm.DB, node model.Node, spec TemplateSpec, overrides map[string][]SpecOverride, ctx *ResolveContext) error {
	for _, entry := range spec.Entries {
		if !entry.Enable {
			continue
		}
		sources, _ := resolvedEntrySources(entry.Sources, overrides[entry.Key])
		paths := make([]string, 0, len(sources))
		for path := range sources {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			source := sources[path]
			if source.Kind == "node_generated" && source.Purpose != "listen_port" {
				value, err := generatedValue(tx, node, entry, source.Purpose)
				if err != nil {
					return err
				}
				ctx.Generated[entry.Key+"/"+source.Purpose] = value
			}
		}
	}
	return nil
}

func persistMaterializedEntries(tx *gorm.DB, node model.Node, resolved TemplateSpec) error {
	var existing []model.NodeInbound
	if err := tx.Where(whereNodeID, node.Id).Find(&existing).Error; err != nil {
		return err
	}
	byKey, err := materializedEntryIndex(existing)
	if err != nil {
		return err
	}
	kept := map[uint]bool{}
	for _, entry := range resolved.Entries {
		in, found := byKey[entry.Key]
		id, err := saveMaterializedEntry(tx, node, entry, in, found)
		if err != nil {
			return err
		}
		kept[id] = true
	}
	for _, in := range existing {
		if !kept[in.Id] {
			if err := tx.Model(&in).Updates(map[string]interface{}{"enable": false, "retired": true}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func saveMaterializedEntry(tx *gorm.DB, node model.Node, entry SpecEntry, in model.NodeInbound, found bool) (uint, error) {
	if entry.Enable {
		if err := openRealmDraft(&entry); err != nil {
			return 0, err
		}
	}
	if found && in.Type != entry.Protocol {
		return 0, errors.New("node_connection_protocol_is_immutable")
	}
	if !found {
		in = model.NodeInbound{NodeId: node.Id, Type: entry.Protocol, ConnectionKey: entry.Key, Tag: "entry-" + versionDigest([]byte(entry.Key))}
	}
	in.SourceTemplateId, in.TemplateKey, in.DisplayName, in.Enable, in.Retired = node.TemplateId, entry.Key, entry.Name, entry.Enable, false
	in.ClientTLS = entry.ClientTLS
	in.ClientOptions = entry.ClientOptions
	if entry.Enable {
		in.Options = entry.Options
		var managed struct {
			TLS struct {
				CertificatePath string `json:"certificate_path"`
			} `json:"tls"`
		}
		_ = json.Unmarshal(in.Options, &managed)
		in.ManagedCertificate = strings.HasPrefix(managed.TLS.CertificatePath, "certificates/managed/") || managed.TLS.CertificatePath == "managed:primary"
		if err := validateInbound(&in, true); err != nil {
			return 0, fmt.Errorf("invalid_materialized_connection: %w", err)
		}
	} else if !found {
		in.Options = entry.Options
	}
	if err := tx.Save(&in).Error; err != nil {
		return 0, err
	}
	return in.Id, nil
}

func materializedEntryIndex(existing []model.NodeInbound) (map[string]model.NodeInbound, error) {
	byKey := map[string]model.NodeInbound{}
	for _, in := range existing {
		if in.ConnectionKey != "" {
			if _, duplicate := byKey[in.ConnectionKey]; duplicate {
				return nil, errors.New("duplicate_node_connection_identity")
			}
			byKey[in.ConnectionKey] = in
		}
	}
	return byKey, nil
}
