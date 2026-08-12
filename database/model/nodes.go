package model

// Node is a full remote s-ui panel managed by this master.
// Master owns clients/subscription; the node keeps its own DNS/route/outbounds/settings.
type Node struct {
	Id     uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name   string `json:"name" form:"name" gorm:"uniqueIndex;size:128;not null"`
	Remark string `json:"remark" form:"remark" gorm:"size:255"`

	// Connection to the remote full s-ui apiv2.
	Scheme   string `json:"scheme" form:"scheme" gorm:"size:16;default:https"`
	Address  string `json:"address" form:"address" gorm:"size:255;not null"`
	Port     int    `json:"port" form:"port" gorm:"not null"`
	BasePath string `json:"basePath" form:"basePath" gorm:"size:255;default:/"`
	ApiToken string `json:"apiToken" form:"apiToken" gorm:"size:255"`

	Enable              bool   `json:"enable" form:"enable" gorm:"default:true;not null"`
	AllowPrivateAddress bool   `json:"allowPrivateAddress" form:"allowPrivateAddress" gorm:"default:false;not null"`
	TlsVerifyMode       string `json:"tlsVerifyMode" form:"tlsVerifyMode" gorm:"size:16;default:verify"`
	PinnedCertSha256    string `json:"pinnedCertSha256" form:"pinnedCertSha256" gorm:"size:128"`

	// public_host is used when generating multi-location subscription links.
	PublicHost string `json:"publicHost" form:"publicHost" gorm:"size:255"`

	// selected = only master-managed tags; all = master is inbound authority on node.
	InboundSyncMode string `json:"inboundSyncMode" form:"inboundSyncMode" gorm:"size:16;default:selected"`
	InboundTags     string `json:"inboundTags" form:"inboundTags" gorm:"type:text"` // JSON array of tags

	// Runtime / heartbeat fields (master-written).
	Status        string `json:"status" form:"status" gorm:"size:32;default:unknown"`
	LastHeartbeat int64  `json:"lastHeartbeat" form:"lastHeartbeat" gorm:"default:0"`
	LatencyMs     int64  `json:"latencyMs" form:"latencyMs" gorm:"default:0"`
	PanelVersion  string `json:"panelVersion" form:"panelVersion" gorm:"size:64"`
	CoreRunning   bool   `json:"coreRunning" form:"coreRunning" gorm:"default:false"`
	CpuPercent    float64 `json:"cpuPercent" form:"cpuPercent" gorm:"default:0"`
	MemPercent    float64 `json:"memPercent" form:"memPercent" gorm:"default:0"`
	Uptime        uint32  `json:"uptime" form:"uptime" gorm:"default:0"`
	LastError     string  `json:"lastError" form:"lastError" gorm:"type:text"`

	ConfigDirty   bool  `json:"configDirty" form:"configDirty" gorm:"default:false;not null"`
	ConfigDirtyAt int64 `json:"configDirtyAt" form:"configDirtyAt" gorm:"default:0"`

	CreatedAt int64 `json:"createdAt" form:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt int64 `json:"updatedAt" form:"updatedAt" gorm:"autoUpdateTime"`
}
