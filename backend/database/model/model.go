package model

import "encoding/json"

type Setting struct {
	Id    uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

type Tls struct {
	Id       uint            `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name     string          `json:"name" form:"name"`
	Inbounds json.RawMessage `json:"inbounds" form:"inbounds"`
	Server   json.RawMessage `json:"server" form:"server"`
	Client   json.RawMessage `json:"client" form:"client"`
}

type User struct {
	Id         uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Username   string `json:"username" form:"username"`
	Password   string `json:"password" form:"password"`
	LastLogins string `json:"lastLogin"`
}

type Client struct {
	Id       uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Enable   bool   `json:"enable" form:"enable"`
	Name     string `json:"name" form:"name"`
	Config   string `json:"config" form:"config"`
	Inbounds string `json:"inbounds" form:"inbounds"`
	Links    string `json:"links" form:"links"`
	Volume   int64  `json:"volume" form:"volume"`
	Expiry   int64  `json:"expiry" form:"expiry"`
	Down     int64  `json:"down" form:"down"`
	Up       int64  `json:"up" form:"up"`
	Desc     string `json:"desc" from:"desc"`
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
	Actor    string          `json:"Actor"`
	Key      string          `json:"key" form:"key"`
	Action   string          `json:"action" form:"action"`
	Index    uint            `json:"index" form:"index"`
	Obj      json.RawMessage `json:"obj" form:"obj"`
}
