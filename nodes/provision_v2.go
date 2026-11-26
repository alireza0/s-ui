package nodes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type ProvisionDomain struct {
	Mode              string `json:"mode"`
	ProfileID         uint   `json:"profile_id"`
	ZoneID            string `json:"zone_id"`
	Prefix            string `json:"prefix"`
	AutomaticPrefix   bool   `json:"automatic_prefix"`
	FQDN              string `json:"fqdn"`
	CertificateMode   string `json:"certificate_mode"`
	CertificateMethod string `json:"certificate_method"`
	Email             string `json:"email"`
	Certificate       string `json:"certificate"`
	Key               string `json:"key"`
	CertificatePath   string `json:"certificate_path"`
	KeyPath           string `json:"key_path"`
}

type liveProvisionPlan struct {
	Version            int
	Request            ProvisionRequest // Password and uploaded certificate material are cleared.
	Target             model.NodeInstallation
	Previous           model.NodeInstallation
	Control            ControlTLS
	ExpectedGeneration uint64
	Fenced             bool
	Committed          bool
	Listeners          []provisionListener
}

func encodeLivePlan(plan liveProvisionPlan) (string, error) {
	var body bytes.Buffer
	if err := gob.NewEncoder(&body).Encode(plan); err != nil {
		return "", err
	}
	return sealSecret(base64.RawStdEncoding.EncodeToString(body.Bytes()))
}
func saveLivePlan(job *model.NodeJob, plan liveProvisionPlan) error {
	sealed, err := encodeLivePlan(plan)
	if err != nil {
		return err
	}
	job.Plan = sealed
	return database.GetDB().Model(job).Update("plan", sealed).Error
}
func loadLivePlan(job model.NodeJob) (liveProvisionPlan, error) {
	var plan liveProvisionPlan
	plain, err := openSecret(job.Plan)
	if err != nil {
		return plan, errors.New("state_key_unavailable")
	}
	body, err := base64.RawStdEncoding.DecodeString(plain)
	if err != nil {
		return plan, errors.New("invalid_install_plan")
	}
	if err = gob.NewDecoder(bytes.NewReader(body)).Decode(&plan); err != nil || plan.Version != 1 || plan.Target.NodeId != job.NodeId || plan.Request.Password != "" {
		return plan, errors.New("invalid_install_plan")
	}
	return plan, nil
}

func StartProvision(r ProvisionRequest, actor string) (model.NodeJob, error) {
	if r.ModelVersion != 0 && r.ModelVersion != 2 {
		return model.NodeJob{}, errors.New("unsupported_node_model")
	}
	r.ModelVersion = 2
	job, created, err := createLiveProvision(r, actor)
	if err != nil || !created {
		return job, err
	}
	provisionJobs.Store(job.NodeId, true)
	go runLiveProvision(job, r.Password)
	return job, nil
}

func createLiveProvision(r ProvisionRequest, actor string) (model.NodeJob, bool, error) {
	var job model.NodeJob
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{16,96}$`).MatchString(r.RequestKey) {
		return job, false, errors.New("install_request_key_required")
	}
	digest := liveProvisionDigest(r)
	db := database.GetDB()
	if err := db.First(&job, whereRequestKey, r.RequestKey).Error; err == nil {
		if job.RequestDigest != digest {
			return job, false, errors.New("install_request_conflict")
		}
		return job, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return job, false, err
	}
	if err := prepareLiveProvisionSelection(db, &r); err != nil {
		return job, false, err
	}
	if r.NodeId == 0 && r.Password == "" {
		return job, false, errors.New("ssh_password_required")
	}
	if err := normalizeProvision(&r); err != nil {
		return job, false, err
	}
	creation := liveProvisionCreation{request: r, actor: actor, digest: digest}
	err := db.Transaction(creation.create)
	if err != nil {
		var same model.NodeJob
		if db.First(&same, whereRequestKey, r.RequestKey).Error == nil && same.RequestDigest == digest {
			return same, false, nil
		}
	}
	return creation.job, creation.created, err
}

type liveProvisionCreation struct {
	request       ProvisionRequest
	actor, digest string
	job           model.NodeJob
	created       bool
}

func liveProvisionDigest(r ProvisionRequest) string {
	signature := r
	signature.Password = ""
	if signature.Domain != nil {
		snapshot := *signature.Domain
		snapshot.Certificate = ""
		snapshot.Key = ""
		signature.Domain = &snapshot
	}
	body, _ := json.Marshal(signature)
	return versionDigest(body)
}

func prepareLiveProvisionSelection(db *gorm.DB, r *ProvisionRequest) error {
	if r.NodeId != 0 {
		return prepareExistingLiveSelection(db, r)
	}
	if r.TemplateId != 0 {
		return errors.New("edit_node_inbound_selection")
	}
	if len(r.InboundTemplateIds) > 0 {
		entries, err := selectedInboundEntries(db, r.InboundTemplateIds, 0)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Enable {
				return nil
			}
		}
		return errors.New("choose_node_inbound_templates")
	}
	return nil
}

func (c *liveProvisionCreation) create(tx *gorm.DB) error {
	r, actor, digest := c.request, c.actor, c.digest
	var job model.NodeJob
	defer func() { c.job = job }()
	// The unique request index also fences concurrent creates.
	var same model.NodeJob
	if tx.First(&same, whereRequestKey, r.RequestKey).Error == nil {
		if same.RequestDigest != digest {
			return errors.New("install_request_conflict")
		}
		job = same
		return nil
	}
	var node model.Node
	var installation model.NodeInstallation
	var err error
	if r.NodeId != 0 {
		node, installation, err = existingLiveProvisionTarget(tx, r)
	} else {
		node, installation, err = newLiveProvisionTarget(tx, r)
	}
	if err != nil {
		return err
	}
	if installation.OwnerID == "" {
		return errors.New("node_not_ssh_managed")
	}
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}
	job = model.NodeJob{Id: id.String(), NodeId: node.Id, ModelVersion: 2, RequestKey: r.RequestKey, RequestDigest: digest, Action: r.Action, State: "queued", Stage: "queued", Created: time.Now().Unix(), Updated: time.Now().Unix(), Events: json.RawMessage(`[]`)}
	if err = tx.Create(&job).Error; err != nil {
		return err
	}
	if err := persistLiveProvisionPlan(tx, node, installation, r, &job); err != nil {
		return err
	}
	audit, _ := json.Marshal(map[string]interface{}{"job": job.Id, "node_id": node.Id})
	if err = tx.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: actor, Key: "node-provision", Action: r.Action, Obj: audit}).Error; err != nil {
		return err
	}
	c.created = true
	return nil
}
func provisionLiveResources(tx *gorm.DB, node model.Node, r ProvisionRequest) error {
	certificate, primaryDomain, err := liveResourceNeeds(tx, node)
	if err != nil {
		return err
	}
	needs := r.Domain != nil || certificate || primaryDomain
	if !needs {
		return nil
	}
	domain := r.Domain
	if domain == nil {
		var profiles []model.NodeDNSProfile
		if err = tx.Order("id").Find(&profiles).Error; err != nil {
			return err
		}
		if len(profiles) != 1 {
			return errors.New("dns_profile_required")
		}
		var zones []DNSZone
		if json.Unmarshal(profiles[0].Zones, &zones) != nil || len(zones) != 1 {
			return errors.New("dns_zone_required")
		}
		domain = &ProvisionDomain{Mode: "managed", ProfileID: profiles[0].Id, ZoneID: zones[0].Id, AutomaticPrefix: true}
	}
	value := *domain
	if value.Mode == "" {
		value.Mode = "managed"
	}
	if value.CertificateMode == "" {
		value.CertificateMode = "auto"
	}
	if value.Prefix == "" {
		value.AutomaticPrefix = true
	}
	body, _ := json.Marshal(map[string]interface{}{"node_id": node.Id, "mode": value.Mode, "profile_id": value.ProfileID, "zone_id": value.ZoneID, "prefix": value.Prefix, "automatic_prefix": value.AutomaticPrefix, "fqdn": value.FQDN, "ip": r.IP, "certificate_mode": value.CertificateMode, "expected_version": 0})
	if _, err = saveDomainBinding(tx, "save", body); err != nil {
		return err
	}
	method := value.CertificateMethod
	if method == "" {
		method = value.CertificateMode
	}
	body, _ = json.Marshal(map[string]interface{}{"node_id": node.Id, "mode": method, "ca": "letsencrypt", "email": value.Email, "dns_profile_id": value.ProfileID, "zone_id": value.ZoneID, "certificate": value.Certificate, "key": value.Key, "certificate_path": value.CertificatePath, "key_path": value.KeyPath, "expected_version": 0})
	_, err = saveCertificateBinding(tx, "save", body)
	return err
}

func runLiveProvision(job model.NodeJob, password string) {
	defer provisionJobs.Delete(job.NodeId)
	operation := provisionOperationLock(job.NodeId)
	operation.Lock()
	defer operation.Unlock()
	provisionLifecycle.RLock()
	parent := provisionParent
	provisionLifecycle.RUnlock()
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()
	select {
	case provisionSlots <- struct{}{}:
		defer func() { <-provisionSlots }()
	case <-ctx.Done():
		jobStage(&job, "queued", "interrupted", "install_timeout")
		return
	}
	plan, err := loadLivePlan(job)
	if err != nil {
		jobStage(&job, job.Stage, "failed", "state_key_unavailable")
		return
	}
	if err = executeLiveProvision(ctx, &job, &plan, password); err != nil {
		code := liveInstallError(err)
		state := "failed"
		if ctx.Err() == context.Canceled {
			state = "interrupted"
			code = "panel_restarted"
		} else if ctx.Err() == context.DeadlineExceeded {
			code = "install_timeout"
		}
		jobStage(&job, job.Stage, state, code)
		return
	}
	jobStage(&job, "ready", "succeeded", "")
}

func liveInstallError(err error) string {
	code := err.Error()
	switch code {
	case "ssh_unreachable", "ssh_auth_failed", "ssh_host_changed", "ssh_password_required",
		"state_key_unavailable", "node_listener_port_occupied", "node_listener_port_reserved",
		"node_reality_handshake_loop", "old_server_retirement_pending", "management_target_changed",
		"agent_epoch_changed_recovery_required", "install_timeout", "certificate_request_rejected",
		"structural_revision_rejected", "ssh_key_verification_failed", "installation_database_unavailable",
		errAutoPortsExhausted.Error(), errPortInspection.Error(),
		"node_preflight_failed", "node_preflight_timeout", "node_sudo_required",
		"node_linux_required", "unsupported_architecture", "node_systemd_unavailable", "node_systemd_too_old",
		"node_dependencies_failed", "node_dependencies_timeout", "node_python_bootstrap_unavailable",
		"node_python_version_unsupported", "node_service_install_failed", "node_service_install_timeout",
		"agent_build_unavailable", "agent_build_mismatch", "agent_bootstrap_identity_mismatch",
		"agent_startup_unreachable":
		return code
	}
	return "node_install_failed"
}

func resumeLiveProvisionJobs(ctx context.Context) {
	var jobs []model.NodeJob
	if database.GetDB().Where("model_version = 2 AND state = ?", "interrupted").Find(&jobs).Error != nil {
		return
	}
	for _, job := range jobs {
		plan, err := loadLivePlan(job)
		if err != nil {
			continue
		}
		// No password is saved. A dedicated key in the sealed plan makes later
		// stages resumable; an unfinished first SSH login needs explicit re-entry.
		if plan.Target.PrivateKey == "" {
			continue
		}
		if _, loaded := provisionJobs.LoadOrStore(job.NodeId, true); loaded {
			continue
		}
		go runLiveProvision(job, "")
	}
}

func prepareExistingLiveSelection(db *gorm.DB, r *ProvisionRequest) error {
	if len(r.InboundTemplateIds) > 0 {
		return errors.New("installer_cannot_replace_live_template")
	}
	var node model.Node
	if err := db.First(&node, r.NodeId).Error; err != nil {
		return err
	}
	if !managedNodeModel(node) {
		return errors.New("unsupported_node_model")
	}
	if r.TemplateId != 0 && r.TemplateId != node.TemplateId {
		return errors.New("installer_cannot_replace_live_template")
	}
	if r.ExpectedVersion == nil || *r.ExpectedVersion != node.SettingsVersion {
		return errors.New("node_settings_version_conflict")
	}
	r.TemplateId = node.TemplateId
	return nil
}

func existingLiveProvisionTarget(tx *gorm.DB, r ProvisionRequest) (model.Node, model.NodeInstallation, error) {
	var node model.Node
	var installation model.NodeInstallation
	if err := tx.First(&node, r.NodeId).Error; err != nil {
		return node, installation, err
	}
	if r.ExpectedVersion == nil || node.SettingsVersion != *r.ExpectedVersion {
		return node, installation, errors.New("node_settings_version_conflict")
	}
	if err := tx.First(&installation, whereNodeID, node.Id).Error; err != nil {
		return node, installation, errors.New("node_not_ssh_managed")
	}
	var active int64
	if err := tx.Model(&model.NodeJob{}).Where("node_id = ? AND state IN ?", node.Id, []string{"queued", "running"}).Count(&active).Error; err != nil {
		return node, installation, err
	}
	if active > 0 || jobRunning(node.Id) {
		return node, installation, errors.New("node_job_running")
	}
	return node, installation, nil
}

func newLiveProvisionTarget(tx *gorm.DB, r ProvisionRequest) (model.Node, model.NodeInstallation, error) {
	var node model.Node
	var installation model.NodeInstallation
	var count int64
	if err := tx.Model(&model.Node{}).Where("name = ?", r.Name).Count(&count).Error; err != nil {
		return node, installation, err
	}
	if count > 0 {
		return node, installation, errors.New("node_name_exists")
	}
	node = model.Node{Name: r.Name, LinkAddress: r.IP, Address: controlAddress(16500), Enable: true, AgentProtocol: 2, AssignmentMode: model.NodeAssignmentWhole, Config: json.RawMessage(`{}`)}
	if err := tx.Create(&node).Error; err != nil {
		return node, installation, err
	}
	id, err := uuid.NewV4()
	if err != nil {
		return node, installation, err
	}
	installation = model.NodeInstallation{NodeId: node.Id, OwnerID: strings.ReplaceAll(id.String(), "-", ""), IP: r.IP, SSHPort: r.SSHPort, SSHUser: r.SSHUser}
	if err = tx.Create(&installation).Error; err != nil {
		return node, installation, err
	}
	template, err := createProvisionTemplate(tx, node, r.InboundTemplateIds, r.Protocols)
	if err != nil {
		return node, installation, err
	}
	if err = bindLiveTemplate(tx, &node, template); err != nil {
		return node, installation, err
	}
	if err = provisionLiveResources(tx, node, r); err != nil {
		return node, installation, err
	}
	return node, installation, nil
}

func persistLiveProvisionPlan(tx *gorm.DB, node model.Node, installation model.NodeInstallation, r ProvisionRequest, job *model.NodeJob) error {
	var err error
	clean := r
	clean.NodeId = node.Id
	clean.Password = ""
	clean.Domain = nil
	plan := liveProvisionPlan{Version: 1, Request: clean, Target: installation, Previous: installation, ExpectedGeneration: node.ManagementGeneration, Control: ControlTLS{CA: node.ControlCA, Certificate: node.ControlCertificate, Key: node.ControlKey}}
	plan.Target.IP, plan.Target.SSHPort, plan.Target.SSHUser = r.IP, r.SSHPort, r.SSHUser
	if installation.IP != r.IP || installation.SSHPort != r.SSHPort || installation.SSHUser != r.SSHUser {
		plan.Target.HostKey = ""
	}
	plan.Target.BootstrapID = job.Id
	var encoded bytes.Buffer
	if err = gob.NewEncoder(&encoded).Encode(plan); err != nil {
		return err
	}
	job.Plan, err = sealSecret(base64.RawStdEncoding.EncodeToString(encoded.Bytes()))
	if err != nil {
		return err
	}
	if err = tx.Model(job).Update("plan", job.Plan).Error; err != nil {
		return err
	}
	return nil
}

func defaultLiveProvisionEntry(kind string) (SpecEntry, error) {
	key, err := uuid.NewV4()
	if err != nil {
		return SpecEntry{}, err
	}
	e := SpecEntry{Key: key.String(), Name: strings.ToUpper(kind), Protocol: kind, Enable: true, Sources: map[string]ValueSource{listenPortPath: {Kind: "node_generated", Purpose: "listen_port"}}}
	if kind == "vless" {
		e.Options = json.RawMessage(`{"listen":"::","tls":{"enabled":true,"server_name":"www.cloudflare.com","reality":{"enabled":true,"handshake":{"server":"www.cloudflare.com","server_port":443}}}}`)
		e.Sources["/tls/reality/private_key"] = ValueSource{Kind: "node_generated", Purpose: "reality_private_key"}
		e.Sources["/tls/reality/short_id"] = ValueSource{Kind: "node_generated", Purpose: "reality_short_id"}
	} else if !nodeagent.SupportsTLS(kind) {
		switch kind {
		case "shadowsocks":
			e.Options = json.RawMessage(`{"listen":"::","method":"2022-blake3-aes-128-gcm"}`)
		case "snell":
			e.Options = json.RawMessage(`{"listen":"::","version":6}`)
		case "shadowtls":
			e.Options = json.RawMessage(`{"listen":"::","version":3,"handshake":{"server":"www.cloudflare.com","server_port":443}}`)
		default:
			e.Options = json.RawMessage(`{"listen":"::"}`)
		}
	} else {
		e.Options = json.RawMessage(`{"listen":"::","tls":{"enabled":true}}`)
		if kind == "tuic" || kind == "hysteria2" {
			e.Options = json.RawMessage(`{"listen":"::","tls":{"enabled":true,"alpn":["h3"]}}`)
		}
		if kind == "hysteria" {
			e.Options = json.RawMessage(`{"listen":"::","up_mbps":100,"down_mbps":100,"tls":{"enabled":true,"alpn":["h3"]}}`)
		}
		e.Sources["/tls/server_name"] = ValueSource{Kind: "node_parameter", Name: "primary_domain"}
		e.Sources["/tls/certificate_binding"] = ValueSource{Kind: "node_certificate", Name: "primary"}
	}
	return e, nil
}
