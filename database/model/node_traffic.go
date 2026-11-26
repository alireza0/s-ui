package model

// Cursor and client usage are committed together. Each node has an independent
// durable ledger, so simultaneous sessions on different nodes remain additive.
type NodeClientTraffic struct {
	NodeId   uint   `gorm:"primaryKey"`
	ClientId uint   `gorm:"primaryKey"`
	Ledger   string `gorm:"primaryKey"`
	Up       int64  `gorm:"not null;default:0"`
	Down     int64  `gorm:"not null;default:0"`
}
