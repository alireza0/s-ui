package model

import "encoding/json"

type NodeTemplate struct {
	Id               uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name             string          `json:"name"`
	Config           json.RawMessage `json:"config"`
	Version          uint64          `json:"version" gorm:"not null;default:0"`
	SchemaVersion    int             `json:"schema_version" gorm:"not null;default:0"`
	Scope            string          `json:"scope" gorm:"not null;default:shared"`
	OwnerNodeId      uint            `json:"owner_node_id,omitempty"`
	ParentTemplateId uint            `json:"parent_template_id,omitempty"`
	Kind             string          `json:"kind,omitempty" gorm:"not null;default:''"`
	Protocol         string          `json:"protocol,omitempty" gorm:"not null;default:''"`
}

// Ordered catalog members of a versioned deployment template.
type NodeTemplateMember struct {
	TemplateId uint `gorm:"primaryKey;autoIncrement:false"`
	InboundId  uint `gorm:"primaryKey;autoIncrement:false;index"`
	Position   int
}

type Node struct {
	Id                 uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name               string          `json:"name"`
	Enable             bool            `json:"enable"`
	Address            string          `json:"address"`
	LinkAddress        string          `json:"linkaddress"`
	TemplateId         uint            `json:"template_id"`
	Config             json.RawMessage `json:"config"`
	DesiredRevision    uint64          `json:"desired_revision"`
	AppliedRevision    uint64          `json:"applied_revision"`
	RejectedRevision   uint64          `json:"-"`
	LastSeen           int64           `json:"last_seen"`
	LastError          string          `json:"last_error,omitempty"`
	DesiredFingerprint string          `json:"-"`
	DesiredSnapshot    json.RawMessage `json:"-"`
	AppliedSnapshot    json.RawMessage `json:"-"`
	ControlCA          string          `json:"-"`
	ControlCertificate string          `json:"-"`
	ControlKey         string          `json:"-"`
	Pending            bool            `json:"pending" gorm:"-"`
	DisableConfirmed   bool            `json:"disable_confirmed" gorm:"-"`
	TLSStatus          json.RawMessage `json:"tls_status"`
	// The assignment mode fences agents registered before whole-node access.
	AssignmentMode              string `json:"assignment_mode" gorm:"not null;default:''"`
	AgentProtocol               int    `json:"agent_protocol"`
	AgentExtendedInbounds       bool   `json:"agent_extended_inbounds,omitempty"`
	AgentProxyInbounds          bool   `json:"agent_proxy_inbounds,omitempty"`
	SingBoxVersion              string `json:"sing_box_version,omitempty"`
	AgentDigest                 string `json:"-"`
	AgentUpgradeState           string `json:"agent_upgrade_state,omitempty"`
	AgentUpgradeRetryAt         int64  `json:"-" gorm:"not null;default:0"`
	AgentRunning                bool   `json:"agent_running"`
	AgentObservedAt             int64  `json:"agent_observed_at"`
	TrafficProcess              string `json:"-"`
	TrafficSampleAt             int64  `json:"-" gorm:"not null;default:0"`
	TrafficUp                   int64  `json:"-"`
	TrafficDown                 int64  `json:"-"`
	AgentTrafficAvailable       bool   `json:"agent_traffic_available" gorm:"-"`
	ControlEpoch                string `json:"-"`
	RestoreAccessPending        bool   `json:"-" gorm:"not null;default:false"`
	RestoreCertificatePending   bool   `json:"-" gorm:"not null;default:false"`
	ManagementGeneration        uint64 `json:"management_generation" gorm:"not null;default:1"`
	TemplateTargetVersion       uint64 `json:"template_target_version"`
	MaterializedTemplateVersion uint64 `json:"materialized_template_version"`
	AppliedTemplateVersion      uint64 `json:"applied_template_version"`
	SettingsVersion             uint64 `json:"settings_version"`
	InboundTemplateIds          []uint `json:"inbound_template_ids,omitempty" gorm:"-"`
}

type NodeInbound struct {
	Id                 uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId             uint            `json:"node_id" gorm:"index;uniqueIndex:idx_node_inbound_tag,priority:1;uniqueIndex:idx_node_connection,priority:1,where:connection_key <> ''"`
	Tag                string          `json:"tag" gorm:"uniqueIndex:idx_node_inbound_tag,priority:2"`
	Type               string          `json:"type"`
	Enable             bool            `json:"enable"`
	Options            json.RawMessage `json:"options"`
	ClientTLS          json.RawMessage `json:"client_tls"`
	ClientOptions      json.RawMessage `json:"client_options,omitempty"`
	SourceTemplateId   uint            `json:"source_template_id"`
	TemplateKey        string          `json:"template_key"`
	ConnectionKey      string          `json:"connection_key,omitempty" gorm:"uniqueIndex:idx_node_connection,priority:2,where:connection_key <> ''"`
	DisplayName        string          `json:"display_name,omitempty"`
	Retired            bool            `json:"retired" gorm:"not null;default:false"`
	ManagedCertificate bool            `json:"managed_certificate" gorm:"not null;default:false"`
}

const NodeAssignmentWhole = "node"

type ClientNode struct {
	ClientId   uint   `json:"client_id" gorm:"primaryKey;autoIncrement:false"`
	NodeId     uint   `json:"node_id" gorm:"primaryKey;autoIncrement:false;index"`
	Active     bool   `json:"active"`
	Generation uint64 `json:"generation" gorm:"not null;default:1"`
}

// Versions are append-only through the service API. Payloads are not Bundle data.
type NodeTemplateVersion struct {
	TemplateId    uint            `json:"template_id" gorm:"primaryKey;autoIncrement:false"`
	Version       uint64          `json:"version" gorm:"primaryKey;autoIncrement:false"`
	SchemaVersion int             `json:"schema_version"`
	Digest        string          `json:"digest"`
	Payload       json.RawMessage `json:"-"`
	Created       int64           `json:"created"`
}

type NodeOverride struct {
	NodeId        uint            `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	ConnectionKey string          `json:"connection_key" gorm:"primaryKey"`
	Path          string          `json:"path" gorm:"primaryKey"`
	Operation     string          `json:"operation"`
	Source        json.RawMessage `json:"source"`
	Archived      bool            `json:"archived" gorm:"not null;default:false"`
}

// Immutable deliveries recover an ACK lost before panel commit.
type NodeDeployment struct {
	NodeId      uint            `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	Revision    uint64          `json:"revision" gorm:"primaryKey;autoIncrement:false"`
	Fingerprint string          `json:"fingerprint"`
	Snapshot    json.RawMessage `json:"-"`
	Created     int64           `json:"created"`
}

type NodePortReservation struct {
	Id            uint `gorm:"primaryKey;autoIncrement"`
	NodeId        uint `gorm:"index"`
	ConnectionKey string
	Host          string `gorm:"uniqueIndex:idx_node_host_port,priority:1"`
	Network       string `gorm:"uniqueIndex:idx_node_host_port,priority:2"`
	Port          int    `gorm:"uniqueIndex:idx_node_host_port,priority:3"`
}

type NodeGeneratedMaterial struct {
	NodeId         uint   `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	ConnectionKey  string `json:"connection_key" gorm:"primaryKey"`
	Purpose        string `json:"purpose" gorm:"primaryKey"`
	EncryptedValue string `json:"-"`
}

// One coalescing durable intent per node/kind.
type NodeTask struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId      uint   `json:"node_id" gorm:"uniqueIndex:idx_node_task_kind,priority:1"`
	Kind        string `json:"kind" gorm:"uniqueIndex:idx_node_task_kind,priority:2"`
	Generation  uint64 `json:"generation"`
	State       string `json:"state"`
	Attempts    int    `json:"attempts"`
	NextAttempt int64  `json:"next_attempt"`
	LeaseUntil  int64  `json:"-"`
	LeaseToken  string `json:"-"`
	LastError   string `json:"last_error,omitempty"`
}

// Credentials are write-only and encrypted with the panel state key. An SSH
// password is never stored: bootstrap installs a dedicated key for this node.
type NodeInstallation struct {
	OwnerID           string `json:"-"`
	Architecture      string `json:"-"`
	BootstrapID       string `json:"-"`
	NodeId            uint   `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	IP                string `json:"ip"`
	SSHPort           int    `json:"ssh_port"`
	SSHUser           string `json:"ssh_user"`
	HostKey           string `json:"-"`
	HostFingerprint   string `json:"host_fingerprint" gorm:"-"`
	PrivateKey        string `json:"-"`
	ServerCertificate string `json:"-"`
	ServerKey         string `json:"-"`
	Domain            string `json:"domain"`
	ControlPort       int    `json:"control_port"`
	Ready             bool   `json:"ready"`
}

type NodeJob struct {
	ModelVersion  int             `json:"model_version,omitempty"`
	RequestKey    string          `json:"-" gorm:"uniqueIndex:idx_node_provision_request,where:request_key <> ''"`
	RequestDigest string          `json:"-"`
	Plan          string          `json:"-"`
	Id            string          `json:"id" gorm:"primaryKey"`
	NodeId        uint            `json:"node_id" gorm:"index"`
	Action        string          `json:"action"`
	State         string          `json:"state"`
	Stage         string          `json:"stage"`
	Error         string          `json:"error"`
	Created       int64           `json:"created"`
	Updated       int64           `json:"updated"`
	Events        json.RawMessage `json:"events"`
}

// Payloads contain client credentials and are never returned by the admin bundle.
type NodeAccessState struct {
	NodeId             uint            `json:"node_id" gorm:"primaryKey;autoIncrement:false"`
	DesiredRevision    uint64          `json:"desired_revision"`
	AppliedRevision    uint64          `json:"applied_revision"`
	DesiredFingerprint string          `json:"-"`
	DesiredSnapshot    json.RawMessage `json:"-"`
	AppliedSnapshot    json.RawMessage `json:"-"`
	AppliedDigest      string          `json:"applied_digest"`
	LastError          string          `json:"last_error,omitempty"`
}
