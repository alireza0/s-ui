package model

// NodeClientTraffic tracks per-node traffic baselines for delta merge.
// The master stores the last-seen (up, down) from each node snapshot,
// and only adds the delta to the global client counters.
type NodeClientTraffic struct {
	Id         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId     uint   `json:"nodeId" gorm:"uniqueIndex:idx_nct_node_client"`
	ClientName string `json:"clientName" gorm:"uniqueIndex:idx_nct_node_client;size:255"`
	Up         int64  `json:"up" gorm:"default:0"`
	Down       int64  `json:"down" gorm:"default:0"`
}
