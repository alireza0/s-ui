package service

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"
)

// NodeSnapshot is the data a node returns to a master on heartbeat.
// It contains system/core status plus per-user traffic counters.
type NodeSnapshot struct {
	PanelVersion string  `json:"panelVersion"`
	CoreRunning  bool    `json:"coreRunning"`
	CpuPercent   float64 `json:"cpuPercent"`
	MemPercent   float64 `json:"memPercent"`
	Uptime       uint32  `json:"uptime"`

	// Per-user traffic on this node (name -> {up, down}).
	UserTraffic map[string]*UserTraffic `json:"userTraffic,omitempty"`
	// Online user names right now.
	OnlineUsers []string `json:"onlineUsers,omitempty"`
}

// UserTraffic holds cumulative up/down for a single client on a node.
type UserTraffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// BuildNodeSnapshot creates a snapshot from local state.
// Called on the node side when master requests nodeSnapshot.
func (s *NodeService) BuildNodeSnapshot() (*NodeSnapshot, error) {
	db := database.GetDB()

	snap := &NodeSnapshot{}

	// Panel version
	var versionSetting model.Setting
	if err := db.Model(&model.Setting{}).Where("`key` = ?", "version").First(&versionSetting).Error; err == nil {
		snap.PanelVersion = versionSetting.Value
	}

	// Core status
	if corePtr != nil && corePtr.IsRunning() {
		snap.CoreRunning = true
		box := corePtr.GetInstance()
		if box != nil {
			snap.Uptime = box.Uptime()
		}
	}

	// System metrics (best effort)
	snap.CpuPercent = 0
	snap.MemPercent = 0

	// User traffic: read all clients' up/down
	var clients []model.Client
	if err := db.Model(&model.Client{}).Select("name, up, down").Scan(&clients).Error; err == nil {
		snap.UserTraffic = make(map[string]*UserTraffic, len(clients))
		for _, c := range clients {
			snap.UserTraffic[c.Name] = &UserTraffic{Up: c.Up, Down: c.Down}
		}
	}

	// Online users from the stats tracker
	onlines, _ := (&StatsService{}).GetOnlines()
	snap.OnlineUsers = onlines.User

	return snap, nil
}

// ApplySnapshot updates a node's heartbeat/runtime fields from a snapshot.
func (s *NodeService) ApplySnapshot(nodeID uint, snap *NodeSnapshot) error {
	if nodeID == 0 || snap == nil {
		return nil
	}
	updates := map[string]interface{}{
		"status":        nodeStatusOnline,
		"last_heartbeat": timeNowUnix(),
		"panel_version":  snap.PanelVersion,
		"core_running":   snap.CoreRunning,
		"cpu_percent":    snap.CpuPercent,
		"mem_percent":    snap.MemPercent,
		"uptime":         snap.Uptime,
		"last_error":     "",
	}
	return database.GetDB().Model(&model.Node{}).Where("id = ?", nodeID).Updates(updates).Error
}

// MarkNodeOffline sets a node to offline with an error message.
func (s *NodeService) MarkNodeOffline(nodeID uint, errMsg string) error {
	updates := map[string]interface{}{
		"status":        nodeStatusOffline,
		"last_heartbeat": timeNowUnix(),
		"core_running":   false,
		"last_error":     errMsg,
	}
	return database.GetDB().Model(&model.Node{}).Where("id = ?", nodeID).Updates(updates).Error
}

// GetEnabledNodes returns all nodes with enable=true.
func (s *NodeService) GetEnabledNodes() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Model(&model.Node{}).Where("enable = ?", true).Order("id asc").Find(&nodes).Error
	return nodes, err
}

// FetchSnapshot calls the remote node's nodeSnapshot endpoint.
func (s *NodeService) FetchSnapshot(n *model.Node) (*NodeSnapshot, error) {
	target, err := s.APIv2URL(n, "nodeSnapshot")
	if err != nil {
		return nil, err
	}

	body, err := s.doNodeGET(n, target)
	if err != nil {
		return nil, err
	}

	var env struct {
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Obj     json.RawMessage `json:"obj"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if !env.Success {
		return nil, common.NewError("remote nodeSnapshot: " + env.Msg)
	}

	var snap NodeSnapshot
	if err := json.Unmarshal(env.Obj, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// doNodeGET performs an authenticated GET to a node endpoint.
func (s *NodeService) doNodeGET(n *model.Node, targetURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nodeProbeTimeout)
	defer cancel()

	req, err := newNodeRequest(ctx, "GET", targetURL, n.ApiToken)
	if err != nil {
		return nil, err
	}

	client, err := s.httpClientFor(n)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func timeNowUnix() int64 {
	return time.Now().Unix()
}
