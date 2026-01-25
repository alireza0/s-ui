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
	Id       uint            `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Enable   bool            `json:"enable" form:"enable"`
	Name     string          `json:"name" form:"name"`
	Config   json.RawMessage `json:"config,omitempty" form:"config"`
	Inbounds json.RawMessage `json:"inbounds" form:"inbounds"`
	Links    json.RawMessage `json:"links,omitempty" form:"links"`
	Volume   int64           `json:"volume" form:"volume"`
	Expiry   int64           `json:"expiry" form:"expiry"`
	Down     int64           `json:"down" form:"down"`
	Up       int64           `json:"up" form:"up"`
	Desc     string          `json:"desc" form:"desc"`
	Group    string          `json:"group" form:"group"`
	// Traffic reset fields
	CreatedAt      int64 `json:"createdAt" form:"createdAt"`           // Client creation timestamp
	ResetMode      int   `json:"resetMode" form:"resetMode"`           // 0=disabled, 1=monthly, 2=every N days
	ResetDayOfMonth int  `json:"resetDayOfMonth" form:"resetDayOfMonth"` // Day of month for monthly reset (1-31, 0=use creation day)
	ResetPeriodDays int  `json:"resetPeriodDays" form:"resetPeriodDays"` // Number of days for periodic reset
	LastResetAt    int64 `json:"lastResetAt" form:"lastResetAt"`       // Last reset timestamp
}

// Reset mode constants
const (
	ResetModeDisabled = 0
	ResetModeMonthly  = 1
	ResetModePeriodic = 2 // Every N days
)

// TrafficHistory records traffic usage for each reset period
type TrafficHistory struct {
	Id         uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientId   uint   `json:"clientId" form:"clientId" gorm:"index"`
	ClientName string `json:"clientName" form:"clientName"`
	StartTime  int64  `json:"startTime" form:"startTime"` // Period start timestamp
	EndTime    int64  `json:"endTime" form:"endTime"`     // Period end timestamp (reset time)
	Up         int64  `json:"up" form:"up"`               // Upload traffic in this period
	Down       int64  `json:"down" form:"down"`           // Download traffic in this period
	ResetMode  int    `json:"resetMode" form:"resetMode"` // Reset mode used
}

type Stats struct {
	Id        uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	DateTime  int64  `json:"dateTime"`
	Resource  string `json:"resource"`
	Tag       string `json:"tag"`
	Direction bool   `json:"direction"`
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
