package nodes

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

var errNodeFirewall = errors.New("node_firewall_failed")
var firewallReceipts sync.Map
var certificateFirewallReceipts sync.Map

//go:embed provision_firewall.py
var provisionFirewallPython string

// The panel uses its existing privileged SSH access. The agent keeps only
// CAP_NET_BIND_SERVICE and never receives firewall administration privileges.
func ensureNodeFirewall(ctx context.Context, db *gorm.DB, node model.Node, snapshot Snapshot, confirmed bool) error {
	var install model.NodeInstallation
	if err := db.First(&install, whereNodeID, node.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if install.PrivateKey == "" {
		return nil
	}
	if !node.Enable {
		if err := ensureCertificateFirewall(ctx, db, node, model.NodeCertificateBinding{}, false); err != nil {
			return err
		}
	}
	name, err := firewallOwnerName(node.Id, install.OwnerID)
	if err != nil {
		return err
	}
	fingerprint, err := snapshotFingerprint(snapshot)
	if err != nil {
		return err
	}
	receipt := fmt.Sprintf("%s/%s/%d/%s/%s", name, install.BootstrapID, node.ManagementGeneration, node.ControlEpoch, fingerprint)
	if previous, exists := firewallReceipts.Load(node.Id); confirmed && exists && previous == receipt {
		return nil
	}
	listeners := []listenerSocket{}
	for _, inbound := range snapshot.Inbounds {
		var options struct {
			Port      int               `json:"listen_port"`
			Transport listenerTransport `json:"transport"`
			Network   string            `json:"network"`
		}
		if err := json.Unmarshal(inbound.Options, &options); err != nil {
			return err
		}
		if options.Port > 0 {
			listeners = append(listeners, expandSocket(listenerSocket{Network: listenerNetwork(inbound.Type, options.Transport, options.Network), Port: options.Port})...)
		}
	}
	if !confirmed {
		firewallReceipts.Delete(node.Id)
	}
	if err := configureManagedFirewall(ctx, node, install, name, listeners, firewallVersion{Revision: snapshot.Deployment.Revision}, confirmed); err != nil {
		return err
	}
	if confirmed {
		firewallReceipts.Store(node.Id, receipt)
	}
	return nil
}

func firewallOwnerName(id uint, owner string) (string, error) {
	name := fmt.Sprintf("s-ui-node-%d", id)
	if owner != "" {
		if len(owner) != 32 {
			return "", errNodeFirewall
		}
		name += "-" + owner[:12]
	}
	return name, nil
}

type firewallVersion struct {
	Revision uint64
	Policy   uint64
}

func configureManagedFirewall(ctx context.Context, node model.Node, install model.NodeInstallation, name string, listeners []listenerSocket, version firewallVersion, confirmed bool) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	client, err := connectSSH(ctx, &install, "", false)
	if err != nil {
		return errNodeFirewall
	}
	defer client.Close()
	script := provisionFirewallPython + `
import json,sys
p=json.load(sys.stdin)
configure_node_firewall(p['listeners'], p['name'], p['confirmed'], version=p['version'])
print(json.dumps({'configured':True}))
`
	_, err = remotePython(ctx, client, install.SSHUser, script, map[string]interface{}{
		"listeners": listeners, "name": name, "confirmed": confirmed,
		"version": map[string]interface{}{"operation": install.BootstrapID, "generation": node.ManagementGeneration, "revision": version.Revision, "policy": version.Policy},
	})
	if err != nil {
		return errNodeFirewall
	}
	return nil
}

func ensureCertificateFirewall(ctx context.Context, db *gorm.DB, node model.Node, binding model.NodeCertificateBinding, enabled bool) error {
	var install model.NodeInstallation
	if err := db.First(&install, whereNodeID, node.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if install.PrivateKey == "" {
		return nil
	}
	name, err := firewallOwnerName(node.Id, install.OwnerID)
	if err != nil {
		return err
	}
	if !node.Enable {
		// A stopped node must close the latest resource generation as well.
		if err := db.Where(whereNodeID, node.Id).Take(&binding).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		enabled = false
	}
	name += "-http01"
	receipt := fmt.Sprintf("%s/%s/%d/%d/%d/%t", name, install.BootstrapID, node.ManagementGeneration, binding.Generation, node.SettingsVersion, enabled)
	if previous, ok := certificateFirewallReceipts.Load(node.Id); ok && previous == receipt {
		return nil
	}
	listeners := []listenerSocket{}
	if enabled {
		listeners = append(listeners, listenerSocket{Port: 80, Network: "tcp"})
	}
	if err := configureManagedFirewall(ctx, node, install, name, listeners, firewallVersion{Revision: binding.Generation, Policy: node.SettingsVersion}, true); err != nil {
		return err
	}
	certificateFirewallReceipts.Store(node.Id, receipt)
	return nil
}

func confirmNodeFirewall(ctx context.Context, db *gorm.DB, node model.Node, status nodeagent.StatusV2) error {
	if !status.Running {
		return ensureNodeFirewall(ctx, db, node, Snapshot{Deployment: structureSnapshot{Revision: status.SpecRevision}}, true)
	}
	if status.SpecRevision == 0 {
		return nil
	}
	var deployment model.NodeDeployment
	if err := db.First(&deployment, "node_id = ? AND revision = ?", node.Id, status.SpecRevision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(deployment.Snapshot, &snapshot); err != nil {
		return err
	}
	spec := nodeagent.Structure{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Revision: status.SpecRevision, Config: snapshot.Deployment.Config}
	if spec.Digest() != status.SpecDigest {
		return errors.New("structural_observation_digest_mismatch")
	}
	return ensureNodeFirewall(ctx, db, node, snapshot, true)
}
