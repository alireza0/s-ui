package cronjob

import (
	"context"
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

// HeartbeatJob probes all enabled nodes and updates their status.
type HeartbeatJob struct {
	service.NodeService
}

func NewHeartbeatJob() *HeartbeatJob {
	return &HeartbeatJob{}
}

func (h *HeartbeatJob) Run() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic recovered in HeartbeatJob: ", r)
		}
	}()

	nodes, err := h.NodeService.GetEnabledNodes()
	if err != nil {
		logger.Warning("heartbeat: failed to list nodes: ", err)
		return
	}
	for _, node := range nodes {
		// Per-node timeout: 2 minutes max, then move to next node
		done := make(chan struct{})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("panic recovered in heartbeat node loop: ", r)
				}
				close(done)
			}()

			snap, err := h.NodeService.FetchSnapshot(node)
			if err != nil {
				logger.Debug("heartbeat: node ", node.Name, " offline: ", err)
				_ = h.NodeService.MarkNodeOffline(node.Id, err.Error())
				return
			}
			if err := h.NodeService.ApplySnapshot(node.Id, snap); err != nil {
				logger.Warning("heartbeat: failed to save snapshot for ", node.Name, ": ", err)
			}

			// Merge traffic from this node into master client counters
			if snap.UserTraffic != nil {
				h.NodeService.MergeNodeTraffic(node.Id, snap.UserTraffic)
			}
		}()

		select {
		case <-done:
			// Normal completion
		case <-time.After(2 * time.Minute):
			logger.Warning("heartbeat: node ", node.Name, " timed out after 2m, skipping")
			_ = h.NodeService.MarkNodeOffline(node.Id, "heartbeat timeout")
		}
	}

	// Reconcile dirty nodes after heartbeat
	h.NodeService.ReconcileDirtyNodes()
}

// Context for potential future use with per-node cancellation
var _ = context.Background
