package model

import "encoding/json"

type Setting struct {
	Id    uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

type Tls struct {
	Id     uint            `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name   string          `json:"name" form:"name"`
	Server json.RawMessage `json:"server" form:"server"`
	Client json.RawMessage `json:"client" form:"client"`
}

type User struct {
	Id         uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Username   string `json:"username" form:"username"`
	Password   string `json:"password" form:"password"`
	LastLogins string `json:"lastLogin"`
}

type Client struct {
	Id     uint `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Enable bool `json:"enable" form:"enable"`
	// Name is the join key on every hot path: the stats job and every
	// subscription fetch resolve a client by it. Uniqueness stays in
	// ClientService.validateClientName -- a unique index would need existing
	// duplicates renamed, and a name is the subscription ID, so renaming one
	// breaks that user's link.
	Name     string          `json:"name" form:"name" gorm:"index"`
	Config   json.RawMessage `json:"config,omitempty" form:"config"`
	Inbounds json.RawMessage `json:"inbounds" form:"inbounds"`
	// Read-only projection of applied remote listener IDs.
	NodeInbounds json.RawMessage `json:"nodeinbounds,omitempty" form:"nodeinbounds" gorm:"-"`
	// Whole-node assignments are independent of local and remote inbound IDs.
	Nodes             json.RawMessage `json:"nodes,omitempty" form:"nodes" gorm:"-"`
	NodeAccessChanged bool            `json:"-" gorm:"-"`
	// Persist access changes even if disable/re-enable or unassign/reassign
	// happens between controller polls. A new epoch closes old node sessions.
	NodeGeneration uint64          `json:"-" gorm:"default:0;not null"`
	Links          json.RawMessage `json:"links,omitempty" form:"links"`
	Volume         int64           `json:"volume" form:"volume"`
	Expiry         int64           `json:"expiry" form:"expiry"`
	Down           int64           `json:"down" form:"down"`
	Up             int64           `json:"up" form:"up"`
	Desc           string          `json:"desc" form:"desc"`
	Group          string          `json:"group" form:"group"`
	Remark         string          `json:"remark" form:"remark"`

	// Timestamps (unix seconds): creation time and last time the client had traffic
	CreatedAt int64 `json:"createdAt" form:"createdAt" gorm:"default:0;not null"`
	OnlineAt  int64 `json:"onlineAt" form:"onlineAt" gorm:"default:0;not null"`

	// Delay start and periodic reset
	DelayStart bool  `json:"delayStart" form:"delayStart" gorm:"default:false;not null"`
	AutoReset  bool  `json:"autoReset" form:"autoReset" gorm:"default:false;not null"`
	ResetDays  int   `json:"resetDays" form:"resetDays" gorm:"default:0;not null"`
	NextReset  int64 `json:"nextReset" form:"nextReset" gorm:"default:0;not null"`
	TotalUp    int64 `json:"totalUp" form:"totalUp" gorm:"default:0;not null"`
	TotalDown  int64 `json:"totalDown" form:"totalDown" gorm:"default:0;not null"`
}

type Stats struct {
	Id uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	// date_time sits third in idx_stats_bucket, so that index cannot serve the
	// retention purge, which filters on date_time alone.
	DateTime  int64  `json:"dateTime" gorm:"uniqueIndex:idx_stats_bucket,priority:3;index:idx_stats_date_time"`
	Resource  string `json:"resource" gorm:"uniqueIndex:idx_stats_bucket,priority:1"`
	Tag       string `json:"tag" gorm:"uniqueIndex:idx_stats_bucket,priority:2"`
	Direction bool   `json:"direction" gorm:"uniqueIndex:idx_stats_bucket,priority:4"`
	Traffic   int64  `json:"traffic"`
}

type Changes struct {
	Id       uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	DateTime int64           `json:"dateTime"`
	Actor    string          `json:"actor"`
	Key      string          `json:"key"`
	Action   string          `json:"action"`
	Obj      json.RawMessage `json:"obj"`
}

type Tokens struct {
	Id     uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Desc   string `json:"desc" form:"desc"`
	Token  string `json:"token" form:"token"`
	Expiry int64  `json:"expiry" form:"expiry"`
	UserId uint   `json:"userId" form:"userId"`
	User   *User  `json:"user" gorm:"foreignKey:UserId;references:Id"`
}
