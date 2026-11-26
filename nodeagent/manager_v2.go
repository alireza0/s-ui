package nodeagent

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/alireza0/s-ui/nodecert"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
)

var ErrConflict = errors.New("deployment revision or node identity conflict")

type SafeRuntime interface {
	Validate([]byte) error
	Close() error
	Running() bool
	ApplySafe([]byte) error
	Interrupt()
}

type StatusV2 struct {
	BootstrapID           string                     `json:"bootstrap_id,omitempty"`
	BootstrapPending      bool                       `json:"bootstrap_pending,omitempty"`
	Protocol              int                        `json:"protocol"`
	ExtendedInbounds      bool                       `json:"extended_inbounds,omitempty"`
	ProxyInbounds         bool                       `json:"proxy_inbounds,omitempty"`
	SingBoxVersion        string                     `json:"sing_box_version,omitempty"`
	AgentDigest           string                     `json:"agent_digest,omitempty"`
	NodeID                uint64                     `json:"node_id"`
	Epoch                 string                     `json:"epoch"`
	SpecRevision          uint64                     `json:"spec_revision"`
	SpecDigest            string                     `json:"spec_digest,omitempty"`
	AccessRevision        uint64                     `json:"access_revision"`
	AccessDigest          string                     `json:"access_digest"`
	AppliedAccessRevision uint64                     `json:"applied_access_revision"`
	AppliedAccessDigest   string                     `json:"applied_access_digest,omitempty"`
	Running               bool                       `json:"running"`
	Error                 string                     `json:"error,omitempty"`
	Certificates          []CertificateStatus        `json:"certificates,omitempty"`
	CertificateResource   *CertificateResourceStatus `json:"certificate_resource,omitempty"`
	Traffic               *TrafficSample             `json:"traffic,omitempty"`
}

type ManagerV2 struct {
	bootstrapID      string
	bootstrapPending bool
	mu               sync.Mutex
	gate             sync.Mutex
	nodeID           uint64
	store            StateStoreV2
	runtime          SafeRuntime
	state            StateV2
	clock            func() time.Time
	applyTimeout     time.Duration
	busySpec         bool
	running          bool
	lastError        string
	effective        []byte
	certificates     []CertificateStatus
	resources        *CertificateManager
}

func NewManagerV2(id uint64, store StateStoreV2, runtime SafeRuntime) *ManagerV2 {
	return &ManagerV2{nodeID: id, store: store, runtime: runtime, clock: time.Now, applyTimeout: 4 * time.Second}
}

// RequireBootstrap must be called before Restore. A new installation operation
// cannot start a restored VPN with permissions cached before that operation.
func (m *ManagerV2) RequireBootstrap(id string) { m.bootstrapID = id }
func cloneState(s StateV2) StateV2 {
	body, _ := json.Marshal(s)
	var result StateV2
	_ = json.Unmarshal(body, &result)
	return result
}

// Restore never falls back to a v1 ACL if a v2 state file exists but is broken.
func (m *ManagerV2) Restore() error {
	m.gate.Lock()
	defer m.gate.Unlock()
	state, err := m.store.LoadV2()
	if err != nil {
		return err
	}
	if state == nil {
		state, err = m.createInitialState()
		if err != nil {
			return err
		}
	}
	if err := m.validateRestoredState(state); err != nil {
		return err
	}
	m.mu.Lock()
	m.state = cloneState(*state)
	m.mu.Unlock()
	if store, ok := m.store.(FileStoreV2); ok {
		resources, err := NewCertificateManager(m.nodeID, state.Epoch, store.Directory)
		if err != nil {
			return err
		}
		m.resources = resources
		if runtime, ok := m.runtime.(interface {
			SetCertificateResolver(func(string) (nodecert.Metadata, error))
		}); ok {
			runtime.SetCertificateResolver(resources.Resolve)
		}
	}
	m.bootstrapPending = m.bootstrapID != "" && state.BootstrapID != m.bootstrapID
	if m.bootstrapPending {
		return nil
	}
	if err := m.applyCurrent(nil); err != nil {
		m.fail("restore_failed")
		return nil
	}
	return nil
}

func (m *ManagerV2) fail(code string) {
	_ = m.runtime.Close()
	m.mu.Lock()
	m.running = false
	m.lastError = code
	m.mu.Unlock()
}

func (m *ManagerV2) applySafe(body []byte, access AccessSnapshot) error {
	done := make(chan struct{})
	exited := make(chan struct{})
	timer := time.NewTimer(m.applyTimeout)
	defer timer.Stop()
	go func() {
		defer close(exited)
		select {
		case <-done:
		case <-timer.C:
			m.runtime.Interrupt()
		}
	}()
	var err error
	if accounting, ok := m.runtime.(interface {
		ApplyWithAccess([]byte, AccessSnapshot) error
	}); ok {
		err = accounting.ApplyWithAccess(body, access)
	} else {
		err = m.runtime.ApplySafe(body)
	}
	close(done)
	<-exited // A late watchdog must never cancel the next runtime operation.
	return err
}

// Caller holds gate; intent writes use only mu+fsync, never this runtime gate.
func (m *ManagerV2) applyCurrent(candidate *Structure) error {
	trial := m.resources != nil && m.resources.BeginTrial()
	if trial {
		m.effective = nil
	}
	defer func() {
		if trial {
			_ = m.resources.FinishTrial(false)
		}
	}()
	for {
		retry, err := m.applyCurrentRevision(candidate, &trial)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
}

func (m *ManagerV2) ApplyAccess(a AccessSnapshot) error {
	if err := a.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	if a.NodeID != m.nodeID || a.Epoch != m.state.Epoch || a.Revision < m.state.Access.Revision || (a.Revision == m.state.Access.Revision && a.Digest() != m.state.Access.Digest()) {
		m.mu.Unlock()
		return ErrConflict
	}
	if a.Revision > m.state.Access.Revision {
		next := cloneState(m.state)
		next.Access = a
		next = cloneState(next)
		if err := m.store.SaveV2(next); err != nil {
			m.mu.Unlock()
			return err
		}
		m.state = next
	}
	interrupt := m.busySpec
	m.mu.Unlock()
	if interrupt {
		m.runtime.Interrupt()
	}
	m.gate.Lock()
	defer m.gate.Unlock()
	if err := m.applyCurrent(nil); err != nil {
		if retry := m.applyCurrent(nil); retry != nil {
			m.fail("access_apply_failed")
			return errors.Join(err, retry)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bootstrapPending {
		next := cloneState(m.state)
		next.BootstrapID = m.bootstrapID
		if err := m.store.SaveV2(next); err != nil {
			return err
		}
		m.state = next
		m.bootstrapPending = false
	}
	return nil
}

func (m *ManagerV2) ApplyStructure(s Structure) error {
	body, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return err
	}
	if err := s.Validate(); err != nil {
		return err
	}
	m.gate.Lock()
	defer m.gate.Unlock()
	m.mu.Lock()
	state := cloneState(m.state)
	if m.bootstrapPending {
		m.mu.Unlock()
		return errors.New("bootstrap_access_required")
	}
	if s.NodeID != m.nodeID || s.Epoch != state.Epoch || (state.Spec != nil && (s.Revision < state.Spec.Revision || (s.Revision == state.Spec.Revision && s.Digest() != state.Spec.Digest()))) {
		m.mu.Unlock()
		return ErrConflict
	}
	m.busySpec = true
	m.mu.Unlock()
	defer func() { m.mu.Lock(); m.busySpec = false; m.mu.Unlock() }()
	if err := m.applyCurrent(&s); err != nil {
		// Rebuild only the old structure; permissions come from latest intent.
		if rollback := m.applyCurrent(nil); rollback != nil {
			m.fail("spec_recovery_failed")
			return errors.Join(err, rollback)
		}
		m.mu.Lock()
		m.lastError = "spec_rejected"
		m.mu.Unlock()
		return err
	}
	return nil
}

func (m *ManagerV2) Refresh() error {
	m.gate.Lock()
	defer m.gate.Unlock()
	m.mu.Lock()
	pending := m.bootstrapPending
	m.mu.Unlock()
	if pending {
		return nil
	}
	if err := m.applyCurrent(nil); err != nil {
		if retry := m.applyCurrent(nil); retry != nil {
			m.fail("expiry_or_recovery_failed")
			return errors.Join(err, retry)
		}
		return err
	}
	return nil
}

func (m *ManagerV2) Status() StatusV2 {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := StatusV2{Protocol: ProtocolV2, NodeID: m.nodeID, Epoch: m.state.Epoch, AccessRevision: m.state.Access.Revision, AccessDigest: m.state.Access.Digest(), Running: m.running, Error: m.lastError, Certificates: append([]CertificateStatus(nil), m.certificates...)}
	s.BootstrapID, s.BootstrapPending = m.bootstrapID, m.bootstrapPending
	s.SingBoxVersion, s.AgentDigest = SingBoxVersion(), ExecutableDigest()
	s.ExtendedInbounds = true
	s.ProxyInbounds = true
	if m.resources != nil {
		status := m.resources.Status()
		s.CertificateResource = &status
	}
	if m.state.Spec != nil {
		s.SpecRevision = m.state.Spec.Revision
		s.SpecDigest = m.state.Spec.Digest()
	}
	if m.state.AppliedAccess != nil {
		s.AppliedAccessRevision = m.state.AppliedAccess.Revision
		s.AppliedAccessDigest = m.state.AppliedAccess.Digest()
	}
	return s
}

func (m *ManagerV2) Close() error {
	if m.resources != nil {
		m.resources.Close()
	}
	m.runtime.Interrupt()
	m.gate.Lock()
	defer m.gate.Unlock()
	err := m.runtime.Close()
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	return err
}

func (m *ManagerV2) createInitialState() (*StateV2, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	created := StateV2{NodeID: m.nodeID, Epoch: id.String()}
	created.Access = AccessSnapshot{NodeID: m.nodeID, Epoch: created.Epoch, Clients: []Grant{}}
	if err := m.store.SaveV2(created); err != nil {
		return nil, err
	}
	return &created, nil
}

func (m *ManagerV2) validateRestoredState(state *StateV2) error {
	if state.NodeID != m.nodeID || state.Epoch == "" || state.Access.NodeID != m.nodeID || state.Access.Epoch != state.Epoch {
		return ErrConflict
	}
	if state.Spec != nil {
		if err := state.Spec.Validate(); err != nil {
			return err
		}
		if state.Spec.NodeID != m.nodeID || state.Spec.Epoch != state.Epoch {
			return ErrConflict
		}
	}
	if state.Access.Revision > 0 {
		if err := state.Access.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (m *ManagerV2) applyStateRuntime(spec *Structure, access AccessSnapshot) ([]byte, error) {
	var body []byte
	var err error
	if spec == nil {
		err = m.runtime.Close()
	} else {
		body, err = Compose(*spec, access, m.clock().Unix())
		if err == nil {
			err = m.runtime.Validate(body)
		}
		if err == nil && (!bytes.Equal(body, m.effective) || !m.runtime.Running()) {
			err = m.applySafe(body, access)
		}
	}
	return body, err
}

func (m *ManagerV2) applyCurrentRevision(candidate *Structure, trial *bool) (bool, error) {
	m.mu.Lock()
	state := cloneState(m.state)
	m.mu.Unlock()
	spec := state.Spec
	if candidate != nil {
		spec = candidate
	}
	body, err := m.applyStateRuntime(spec, state.Access)
	if err != nil {
		return false, err
	}
	if accounting, ok := m.runtime.(interface{ SetTrafficAccess(AccessSnapshot) }); ok {
		accounting.SetTrafficAccess(state.Access)
	}
	if *trial {
		if err := m.resources.FinishTrial(true); err != nil {
			return false, err
		}
		*trial = false
	}
	m.effective = append([]byte(nil), body...)
	m.mu.Lock()
	if state.Access.Digest() != m.state.Access.Digest() {
		m.mu.Unlock()
		return true, nil
	}
	next := cloneState(m.state)
	next.Spec = spec
	applied := next.Access
	next.AppliedAccess = &applied
	if digestV2(next) != digestV2(m.state) {
		if err := m.store.SaveV2(next); err != nil {
			m.mu.Unlock()
			return false, err
		}
	}
	m.state = next
	m.running = spec != nil && m.runtime.Running()
	m.effective = append([]byte(nil), body...)
	m.lastError = ""
	m.mu.Unlock()
	if reporter, ok := m.runtime.(interface{ Certificates() []CertificateStatus }); ok {
		certs := reporter.Certificates()
		m.mu.Lock()
		m.certificates = certs
		m.mu.Unlock()
	}
	return false, nil
}
