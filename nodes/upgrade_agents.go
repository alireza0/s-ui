package nodes

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

//go:embed upgrade_agent.py
var upgradeAgentScript string

var agentDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var activeAgentUpgrades sync.Map

func agentUpgradeRunning(id uint) bool {
	_, active := activeAgentUpgrades.Load(id)
	return active
}

type agentBundle struct {
	SingBox string            `json:"sing_box_version"`
	Agents  map[string]string `json:"agents"`
}

func readAgentBundle() (agentBundle, error) {
	folder, err := agentArtifactsDirectory()
	if err != nil {
		return agentBundle{}, err
	}
	bundle, err := loadAgentBundle(folder)
	if err == nil && bundle.SingBox != nodeagent.SingBoxVersion() {
		return bundle, errors.New("agent bundle does not match the panel sing-box")
	}
	return bundle, err
}

func loadAgentBundle(folder string) (agentBundle, error) {
	var bundle agentBundle
	body, err := os.ReadFile(filepath.Join(folder, "manifest.json"))
	if err != nil {
		return bundle, err
	}
	if json.Unmarshal(body, &bundle) != nil || bundle.SingBox == "" ||
		!agentDigestPattern.MatchString(bundle.Agents["amd64"]) || !agentDigestPattern.MatchString(bundle.Agents["arm64"]) {
		return bundle, errors.New("invalid agent bundle manifest")
	}
	return bundle, nil
}

func upgradeAgentsLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if bundle, err := readAgentBundle(); err == nil {
			upgradeAgents(ctx, bundle)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func upgradeAgents(ctx context.Context, bundle agentBundle) {
	db := database.GetDB()
	var installations []model.NodeInstallation
	if err := db.Where("owner_id <> '' AND private_key <> ''").Order("node_id").Find(&installations).Error; err != nil {
		return
	}
	for _, installation := range installations {
		if ctx.Err() != nil {
			return
		}
		var node model.Node
		if db.First(&node, installation.NodeId).Error != nil || !managedNodeModel(node) || node.ControlEpoch == "" ||
			node.AgentUpgradeRetryAt > time.Now().Unix() {
			continue
		}
		if node.SingBoxVersion == bundle.SingBox && node.AgentDigest == bundle.Agents[installation.Architecture] && node.AgentUpgradeState == "ready" {
			continue
		}
		upgradeManagedAgent(ctx, node.Id, bundle)
	}
}

func upgradeManagedAgent(parent context.Context, id uint, bundle agentBundle) {
	lock, _ := nodeLocks.LoadOrStore(id, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	if parent.Err() != nil {
		return
	}
	operation := provisionOperationLock(id)
	if !operation.TryLock() {
		return
	}
	defer operation.Unlock()
	if jobRunning(id) {
		return
	}
	activeAgentUpgrades.Store(id, true)
	defer activeAgentUpgrades.Delete(id)
	db := database.GetDB()
	var node model.Node
	if db.First(&node, id).Error != nil || !managedNodeModel(node) || node.ControlEpoch == "" {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()
	record := func(state string, retry time.Duration) {
		recordAgentUpgrade(db, node, state, retry)
	}
	ssh, install, err := managedTransport(ctx, id)
	if err != nil {
		record("waiting", time.Minute)
		return
	}
	defer ssh.Close()
	if install.OwnerID == "" || (!install.Ready && !liveProvisionReady(id)) {
		return
	}
	confirm := func(status nodeagent.StatusV2) {
		db.Model(&model.Node{}).Where("id = ? AND management_generation = ? AND control_epoch = ?", id, node.ManagementGeneration, node.ControlEpoch).
			Updates(map[string]interface{}{"agent_upgrade_state": "ready", "agent_upgrade_retry_at": 0, "agent_extended_inbounds": status.ExtendedInbounds, "agent_proxy_inbounds": status.ProxyInbounds})
	}
	params := func(action, digest string) map[string]interface{} {
		return map[string]interface{}{"id": id, "owner": install.OwnerID, "action": action, "sha256": digest}
	}
	call := func(ctx context.Context, action, digest string) ([]byte, error) {
		return remotePython(ctx, ssh, install.SSHUser, upgradeAgentScript, params(action, digest))
	}
	path, digest, err := prepareAgentUpgradeArtifact(ctx, db, install, bundle, call)
	if err != nil {
		recordAgentUpgradePreparationError(db, node, err)
		return
	}
	control, closeControl, err := controlClient(ctx, node)
	if err != nil {
		record("waiting", time.Minute)
		return
	}
	defer closeControl()
	before, err := getStatusV2(ctx, control, node)
	if err != nil || !before.Running || before.BootstrapPending {
		record("waiting", time.Minute)
		return
	}
	if before.SingBoxVersion == bundle.SingBox && before.AgentDigest == digest {
		confirm(before)
		return
	}
	record("updating", time.Minute)
	if err := uploadLiveAgent(ctx, ssh, *install, path, digest); err != nil {
		record("failed", 10*time.Minute)
		return
	}
	activate, stop := context.WithTimeout(ctx, 55*time.Second)
	defer stop()
	if after, confirmed := activateAgentUpgrade(activate, control, node, bundle.SingBox, digest, before, call); confirmed {
		confirm(after)
		return
	}
	// A separate remote deadline restores the working binary even if the
	// controller loses SSH or is restarted before sending this request.
	rollback, done := context.WithTimeout(context.WithoutCancel(parent), 25*time.Second)
	defer done()
	_, rollbackErr := call(rollback, "rollback", digest)
	if rollbackErr != nil {
		record("waiting", 3*time.Minute)
		return
	}
	record("failed", 10*time.Minute)
}

func matchesAgentUpgrade(before, after nodeagent.StatusV2, version, digest string) bool {
	return after.Running && !after.BootstrapPending && before.Epoch == after.Epoch && before.NodeID == after.NodeID &&
		after.SingBoxVersion == version && after.AgentDigest == digest && before.SpecRevision == after.SpecRevision &&
		before.SpecDigest == after.SpecDigest && after.AccessRevision >= before.AccessRevision &&
		after.AccessRevision == after.AppliedAccessRevision && after.AccessDigest == after.AppliedAccessDigest &&
		(after.AccessRevision > before.AccessRevision || after.AccessDigest == before.AccessDigest)
}

func activateAgentUpgrade(ctx context.Context, control *http.Client, node model.Node, version, digest string, before nodeagent.StatusV2, call func(context.Context, string, string) ([]byte, error)) (nodeagent.StatusV2, bool) {
	if _, err := call(ctx, "activate", digest); err == nil {
		for ctx.Err() == nil {
			after, err := getStatusV2(ctx, control, node)
			if err == nil && matchesAgentUpgrade(before, after, version, digest) {
				if _, err := call(ctx, "confirm", digest); err == nil {
					return after, true
				}
				break
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
	return nodeagent.StatusV2{}, false
}

func recordAgentUpgrade(db *gorm.DB, node model.Node, state string, retry time.Duration) {
	var at int64
	if retry > 0 {
		at = time.Now().Add(retry).Unix()
	}
	db.Model(&model.Node{}).Where("id = ? AND management_generation = ? AND control_epoch = ?", node.Id, node.ManagementGeneration, node.ControlEpoch).
		Updates(map[string]interface{}{"agent_upgrade_state": state, "agent_upgrade_retry_at": at})
}

var (
	errAgentUpgradeWaiting = errors.New("agent_upgrade_waiting")
	errAgentUpgradeBundle  = errors.New("agent_upgrade_bundle_unavailable")
)

func prepareAgentUpgradeArtifact(ctx context.Context, db *gorm.DB, install *model.NodeInstallation, bundle agentBundle, call func(context.Context, string, string) ([]byte, error)) (string, string, error) {
	body, err := call(ctx, "inspect", "")
	var result struct {
		Architecture string `json:"arch"`
		Pending      bool   `json:"pending"`
	}
	if err != nil || json.Unmarshal(body, &result) != nil || result.Pending {
		return "", "", errAgentUpgradeWaiting
	}
	path, digest, err := agentArtifact(result.Architecture)
	if err != nil || digest != bundle.Agents[result.Architecture] {
		return "", "", errAgentUpgradeBundle
	}
	if install.Architecture != result.Architecture {
		if err := db.Model(install).Where("owner_id = ?", install.OwnerID).Update("architecture", result.Architecture).Error; err != nil {
			return "", "", err
		}
	}
	return path, digest, nil
}

func recordAgentUpgradePreparationError(db *gorm.DB, node model.Node, err error) {
	switch {
	case errors.Is(err, errAgentUpgradeWaiting):
		recordAgentUpgrade(db, node, "waiting", time.Minute)
	case errors.Is(err, errAgentUpgradeBundle):
		recordAgentUpgrade(db, node, "failed", 10*time.Minute)
	}
}
