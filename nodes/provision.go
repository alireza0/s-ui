package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
)

type ProvisionRequest struct {
	ModelVersion       int              `json:"model_version,omitempty"`
	RequestKey         string           `json:"request_key,omitempty"`
	ExpectedVersion    *uint64          `json:"expected_version,omitempty"`
	Domain             *ProvisionDomain `json:"domain,omitempty"`
	NodeId             uint             `json:"node_id"`
	Name               string           `json:"name"`
	IP                 string           `json:"ip"`
	SSHPort            int              `json:"ssh_port"`
	SSHUser            string           `json:"ssh_user"`
	Password           string           `json:"password"`
	Protocols          []string         `json:"protocols"`
	TemplateId         uint             `json:"template_id"`
	InboundTemplateIds []uint           `json:"inbound_template_ids,omitempty"`
	Action             string           `json:"action"`
	ReplaceHostKey     bool             `json:"replace_host_key"`
}

var provisionJobs sync.Map
var provisionSlots = make(chan struct{}, 2)
var provisionOperations sync.Map

func provisionOperationLock(id uint) *sync.Mutex {
	value, _ := provisionOperations.LoadOrStore(id, &sync.Mutex{})
	return value.(*sync.Mutex)
}

var provisionLifecycle sync.RWMutex
var provisionParent context.Context = context.Background()

func dbInstall(id uint, i *model.NodeInstallation) error {
	return database.GetDB().First(i, whereNodeID, id).Error
}
func jobRunning(id uint) bool {
	_, ok := provisionJobs.Load(id)
	return ok || agentUpgradeRunning(id)
}
func normalizeProvision(r *ProvisionRequest) error {
	provisionDefaults(r)
	ip := net.ParseIP(r.IP)
	if r.Name == "" || len([]rune(r.Name)) > 80 {
		return errors.New("node_name_required")
	}
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return errors.New("invalid_node_ip")
	}
	if r.SSHPort < 1 || r.SSHPort > 65535 || strings.ContainsAny(r.SSHUser, "\r\n\x00 ") || r.SSHUser == "" {
		return errors.New("invalid_ssh_access")
	}
	if r.Action != "install" && r.Action != "recover" && r.Action != "update" {
		return errors.New("invalid_install_action")
	}
	if r.NodeId == 0 && r.Action != "install" {
		return errors.New("invalid_install_action")
	}
	if r.ReplaceHostKey && (r.Action != "recover" || r.Password == "") {
		return errors.New("recovery_password_required")
	}
	protocols, err := normalizeProvisionProtocols(r.Protocols)
	r.Protocols = protocols
	return err
}

func normalizeProvisionProtocols(protocols []string) ([]string, error) {
	if len(protocols) == 0 {
		return protocols, nil
	}
	var unique []string
	for _, kind := range protocols {
		if !nodeagent.SupportsInbound(kind, true) {
			return nil, errors.New("unsupported_node_protocol")
		}
		if !slices.Contains(unique, kind) {
			unique = append(unique, kind)
		}
	}
	return unique, nil
}

func jobStage(job *model.NodeJob, stage, state, code string) {
	type event struct {
		Time  int64  `json:"time"`
		Stage string `json:"stage"`
		State string `json:"state"`
		Error string `json:"error,omitempty"`
	}
	var events []event
	_ = json.Unmarshal(job.Events, &events)
	events = append(events, event{time.Now().Unix(), stage, state, code})
	job.Events, _ = json.Marshal(events)
	job.Stage, job.State, job.Error, job.Updated = stage, state, code, time.Now().Unix()
	database.GetDB().Save(job)
}

func initializeProvisionJobs(ctx context.Context) {
	provisionLifecycle.Lock()
	provisionParent = ctx
	provisionLifecycle.Unlock()
	database.GetDB().Model(&model.NodeJob{}).Where("state IN ?", []string{"running", "queued"}).Updates(map[string]interface{}{"state": "interrupted", "error": "panel_restarted", "updated": time.Now().Unix()})
	resumeLiveProvisionJobs(ctx)
}

func provisionDefaults(r *ProvisionRequest) {
	r.Name = strings.TrimSpace(r.Name)
	r.IP = strings.TrimSpace(r.IP)
	r.SSHUser = strings.TrimSpace(r.SSHUser)
	if r.SSHPort == 0 {
		r.SSHPort = 22
	}
	if r.SSHUser == "" {
		r.SSHUser = "root"
	}
	if r.Action == "" {
		r.Action = "install"
	}
}
