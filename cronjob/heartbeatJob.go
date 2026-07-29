package cronjob

import (
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
		snap, err := h.NodeService.FetchSnapshot(node)
		if err != nil {
			logger.Debug("heartbeat: node ", node.Name, " offline: ", err)
			_ = h.NodeService.MarkNodeOffline(node.Id, err.Error())
			continue
		}
		if err := h.NodeService.ApplySnapshot(node.Id, snap); err != nil {
			logger.Warning("heartbeat: failed to save snapshot for ", node.Name, ": ", err)
		}

		// Merge traffic from this node into master client counters
		if snap.UserTraffic != nil {
			h.NodeService.MergeNodeTraffic(node.Id, snap.UserTraffic)
		}
	}

	// Reconcile dirty nodes after heartbeat
	h.NodeService.ReconcileDirtyNodes()
}
