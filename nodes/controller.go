package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

var nodeLocks sync.Map

func snapshotFingerprint(snapshot Snapshot) (string, error) {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func Start(parent context.Context, traffic TrafficConfig) context.CancelFunc {
	ctx, cancel := context.WithCancel(parent)
	ctx = context.WithValue(ctx, trafficConfigKey{}, traffic)
	initializeProvisionJobs(ctx)
	go accessLoop(ctx)
	go resourceLoop(ctx)
	go upgradeAgentsLoop(ctx)
	go controlRenewalLoop(ctx)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			ReconcileAll(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return cancel
}

func ReconcileAll(ctx context.Context) {
	var list []model.Node
	if database.GetDB().Order("id").Find(&list).Error != nil {
		return
	}
	limit := make(chan struct{}, 4)
	var wait sync.WaitGroup
	for _, node := range list {
		if jobRunning(node.Id) {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		limit <- struct{}{}
		wait.Add(1)
		go func(id uint) { defer wait.Done(); defer func() { <-limit }(); _ = Reconcile(ctx, id) }(node.Id)
	}
	wait.Wait()
}

func Reconcile(ctx context.Context, id uint) error {
	lock, _ := nodeLocks.LoadOrStore(id, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	db := database.GetDB()
	var node model.Node
	if err := db.First(&node, id).Error; err != nil {
		return err
	}
	if !managedNodeModel(node) {
		return errors.New("unsupported_node_model")
	}
	return reconcileStructureV2(ctx, node)
}

func managedNodeModel(node model.Node) bool {
	return node.AgentProtocol == 2 && node.AssignmentMode == model.NodeAssignmentWhole && node.TemplateTargetVersion > 0
}
