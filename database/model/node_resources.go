package model

import "encoding/json"

type NodeDNSProfile struct {
	Id               uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name             string          `json:"name"`
	Provider         string          `json:"provider"`
	AuthType         string          `json:"auth_type"`
	Email            string          `json:"email,omitempty"`
	Zones            json.RawMessage `json:"zones"`
	Version          uint64          `json:"version"`
	Secret           string          `json:"-"`
	SecretConfigured bool            `json:"secret_configured" gorm:"-"`
}
type NodeDomainBinding struct {
	NodeId          uint   `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	Mode            string `json:"mode"`
	FQDN            string `json:"fqdn"`
	ProfileId       uint   `json:"profile_id,omitempty"`
	ZoneId          string `json:"zone_id,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	AutomaticPrefix bool   `json:"automatic_prefix"`
	ExpectedIP      string `json:"ip"`
	Generation      uint64 `json:"generation"`
	State           string `json:"state"`
	AppliedFQDN     string `json:"applied_fqdn,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	CheckedAt       int64  `json:"checked_at"`
	CertificateMode string `json:"certificate_mode"`
}

// An intent row, including its owner nonce, exists before a DNS mutation.
type NodeDNSRecord struct {
	Id            uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId        uint   `json:"node_id" gorm:"index"`
	ProfileId     uint   `json:"profile_id"`
	ZoneId        string `json:"zone_id"`
	RemoteId      string `json:"-"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Content       string `json:"-"`
	TargetContent string `json:"-"`
	Owner         string `json:"-"`
	Challenge     bool   `json:"challenge"`
	Retired       bool   `json:"retired"`
}

type NodeCertificateBinding struct {
	NodeId             uint            `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	Mode               string          `json:"mode"`
	CA                 string          `json:"ca"`
	Email              string          `json:"email,omitempty"`
	DNSProfileId       uint            `json:"dns_profile_id,omitempty"`
	ZoneId             string          `json:"zone_id,omitempty"`
	Generation         uint64          `json:"generation"`
	State              string          `json:"state"`
	LastError          string          `json:"last_error,omitempty"`
	CurrentVersion     uint64          `json:"current_version"`
	RenewRequested     bool            `json:"renew_requested"`
	Material           string          `json:"-"`
	AccountKey         string          `json:"-"`
	DesiredRevision    uint64          `json:"desired_revision"`
	DesiredFingerprint string          `json:"-"`
	DesiredRequest     string          `json:"-"`
	AppliedRevision    uint64          `json:"applied_revision"`
	Metadata           json.RawMessage `json:"metadata"`
	CertificatePath    string          `json:"certificate_path,omitempty"`
	KeyPath            string          `json:"key_path,omitempty"`
}
type NodeCertificateVersion struct {
	NodeId           uint            `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	Version          uint64          `json:"version" gorm:"primaryKey;autoIncrement:false"`
	DomainGeneration uint64          `json:"domain_generation"`
	Bundle           string          `json:"-"`
	Metadata         json.RawMessage `json:"metadata"`
	Created          int64           `json:"created"`
}
