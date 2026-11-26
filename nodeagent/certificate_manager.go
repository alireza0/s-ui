package nodeagent

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alireza0/s-ui/nodecert"
)

type CertificateRequest struct {
	NodeID            uint64          `json:"node_id"`
	Epoch             string          `json:"epoch"`
	Revision          uint64          `json:"revision"`
	BindingGeneration uint64          `json:"binding_generation,omitempty"`
	DomainGeneration  uint64          `json:"domain_generation,omitempty"`
	Domain            string          `json:"domain"`
	ExpectedIP        string          `json:"ip,omitempty"`
	Method            string          `json:"method"`
	CA                string          `json:"ca,omitempty"`
	Email             string          `json:"email,omitempty"`
	Bundle            nodecert.Bundle `json:"bundle,omitempty"`
	CertificatePath   string          `json:"certificate_path,omitempty"`
	KeyPath           string          `json:"key_path,omitempty"`
}
type CertificateResourceStatus struct {
	Revision        uint64            `json:"revision"`
	Digest          string            `json:"digest"`
	AppliedRevision uint64            `json:"applied_revision"`
	State           string            `json:"state"`
	LastError       string            `json:"last_error,omitempty"`
	Metadata        nodecert.Metadata `json:"metadata"`
}

func (r CertificateRequest) Digest() string { return digestV2(r) }

type certificateState struct {
	Config          *CertificateRequest
	AppliedRevision uint64
	Metadata        nodecert.Metadata
	Active          map[string]nodecert.Metadata
	FailedDigest    string
	RetryAt         int64
	AccountKey      string
	LastError       string
}
type CertificateManager struct {
	worker      sync.Mutex
	lookup      func(context.Context, string) ([]net.IPAddr, error)
	renewNow    func() time.Time
	httpAddress string
	timeout     time.Duration
	mu          sync.Mutex
	directory   string
	nodeID      uint64
	epoch       string
	state       certificateState
	roots       *x509.CertPool
	// Owned background-worker lifetime, never inherited from an HTTP request.
	ctx       context.Context
	cancel    context.CancelFunc
	jobCancel context.CancelFunc
	started   bool
	trial     *nodecert.Metadata
	kick      chan struct{}
	issue     func(context.Context, string, string, string, string, string, *nodecert.HTTP01) (nodecert.Bundle, error)
}

func NewCertificateManager(id uint64, epoch, directory string) (*CertificateManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &CertificateManager{directory: directory, nodeID: id, epoch: epoch, ctx: ctx, cancel: cancel, kick: make(chan struct{}, 1)}
	r.lookup = net.DefaultResolver.LookupIPAddr
	r.renewNow = time.Now
	r.timeout = 5 * time.Minute
	r.issue = func(ctx context.Context, ca, email, key, domain, method string, solver *nodecert.HTTP01) (nodecert.Bundle, error) {
		return nodecert.Issue(ctx, ca, email, key, domain, method, solver)
	}
	body, err := os.ReadFile(filepath.Join(directory, "certificate-state.json"))
	if err == nil {
		var envelope struct {
			State  certificateState
			Digest string
		}
		if json.Unmarshal(body, &envelope) != nil || digestV2(envelope.State) != envelope.Digest {
			cancel()
			return nil, errors.New("certificate_state_corrupt")
		}
		r.state = envelope.State
		if r.state.Config != nil && (r.state.Config.NodeID != id || r.state.Config.Epoch != epoch) {
			cancel()
			return nil, ErrConflict
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		cancel()
		return nil, err
	}
	if r.state.Config != nil {
		r.start()
	}
	return r, nil
}
func (r *CertificateManager) save(next certificateState) error {
	return nodecert.WriteJSON(r.directory, "certificate-state.json", struct {
		State  certificateState
		Digest string
	}{next, digestV2(next)})
}
func (r *CertificateManager) start() {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.mu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if r.ctx.Err() != nil {
				return
			}
			r.reconcile()
			select {
			case <-r.ctx.Done():
				return
			case <-r.kick:
			case <-ticker.C:
			}
		}
	}()
}
func (r *CertificateManager) Configure(request CertificateRequest) error {
	if request.NodeID != r.nodeID || request.Epoch != r.epoch || request.Revision == 0 {
		return ErrConflict
	}
	if err := r.validateRequest(request); err != nil {
		return err
	}
	r.mu.Lock()
	current := r.state.Config
	if current != nil && (request.Revision < current.Revision || request.Revision == current.Revision && digestV2(request) != digestV2(current)) {
		r.mu.Unlock()
		return ErrConflict
	}
	if current != nil && request.Revision == current.Revision {
		r.mu.Unlock()
		select {
		case r.kick <- struct{}{}:
		default:
		}
		return nil
	}
	next := r.state
	snapshot := request
	next.Config = &snapshot
	next.LastError = ""
	next.FailedDigest = ""
	next.RetryAt = 0
	if err := r.save(next); err != nil {
		r.mu.Unlock()
		return err
	}
	r.state = next
	if r.jobCancel != nil {
		r.jobCancel()
	}
	r.mu.Unlock()
	r.start()
	select {
	case r.kick <- struct{}{}:
	default:
	}
	return nil
}
func (r *CertificateManager) Status() CertificateResourceStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := CertificateResourceStatus{State: "unconfigured", Metadata: r.state.Metadata, AppliedRevision: r.state.AppliedRevision, LastError: r.state.LastError}
	if r.state.Config != nil {
		status.Revision = r.state.Config.Revision
		status.Digest = digestV2(r.state.Config)
		status.State = "pending"
		if r.state.AppliedRevision == status.Revision && r.state.Metadata.Domain == r.state.Config.Domain {
			status.State = "ready"
			if r.state.Metadata.NotAfter <= time.Now().Unix() {
				status.State = "expired"
			}
		}
		if status.State != "ready" && status.LastError != "" {
			status.State = "failed"
		}
	}
	return status
}
func (r *CertificateManager) Resolve(domain string) (nodecert.Metadata, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if domain == "" && r.state.Config != nil {
		domain = r.state.Config.Domain
	}
	if r.trial != nil && r.trial.Domain == domain {
		return *r.trial, nil
	}
	meta, ok := r.state.Active[domain]
	if !ok {
		candidates := []nodecert.Metadata{}
		if r.trial != nil {
			candidates = append(candidates, *r.trial)
		}
		for _, candidate := range r.state.Active {
			candidates = append(candidates, candidate)
		}
		for _, candidate := range candidates {
			body, err := readCertificateFile(filepath.Join(r.directory, filepath.FromSlash(candidate.CertificatePath)))
			if err != nil {
				continue
			}
			leaf, err := nodecert.LeafPEM(nodecert.Bundle{Certificate: string(body)})
			if err == nil && leaf.VerifyHostname(domain) == nil {
				return candidate, nil
			}
		}
		return meta, errors.New("certificate_not_installed_for_domain")
	}
	return meta, nil
}

// Called only under the VPN manager's runtime gate. A failed candidate never
// overwrites the active certificate used when rebuilding with latest access.
func (r *CertificateManager) BeginTrial() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	meta := r.state.Metadata
	if meta.Digest == "" || meta.NotAfter <= time.Now().Unix() || r.state.Active[meta.Domain].Digest == meta.Digest || r.state.FailedDigest == meta.Digest && r.state.RetryAt > time.Now().Unix() {
		return false
	}
	snapshot := meta
	r.trial = &snapshot
	return true
}
func (r *CertificateManager) FinishTrial(success bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.trial == nil {
		return nil
	}
	meta := *r.trial
	r.trial = nil
	next := r.state
	if success {
		active := map[string]nodecert.Metadata{}
		for domain, value := range next.Active {
			active[domain] = value
		}
		active[meta.Domain] = meta
		next.Active = active
		next.FailedDigest = ""
		next.RetryAt = 0
		if next.LastError == "certificate_activation_failed" && next.Metadata.Digest == meta.Digest {
			next.LastError = ""
		}
	} else {
		next.FailedDigest = meta.Digest
		next.RetryAt = time.Now().Unix() + 60
		next.LastError = "certificate_activation_failed"
	}
	if err := r.save(next); err != nil {
		return err
	}
	r.state = next
	return nil
}
func (r *CertificateManager) reconcile() {
	r.worker.Lock()
	defer r.worker.Unlock()
	if r.ctx.Err() != nil {
		return
	}
	r.mu.Lock()
	if r.state.Config == nil {
		r.mu.Unlock()
		return
	}
	request := *r.state.Config
	meta := r.state.Metadata
	if r.state.AppliedRevision == request.Revision && request.Method != "remote_paths" && (request.Method != "http01" || !nodecert.NeedsRenewal(meta, r.renewNow())) {
		r.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(r.ctx, r.timeout)
	r.jobCancel = cancel
	if err := r.ensureAccountKey(request); err != nil {
		r.mu.Unlock()
		cancel()
		return
	}
	account := r.state.AccountKey
	r.mu.Unlock()
	defer cancel()
	bundle := request.Bundle
	var err error
	if request.Method == "remote_paths" {
		var cert, key []byte
		cert, err = readCertificateFile(request.CertificatePath)
		if err == nil {
			key, err = readCertificateFile(request.KeyPath)
		}
		bundle = nodecert.Bundle{Certificate: string(cert), Key: string(key)}
	}
	if request.Method == "http01" {
		err = r.verifyHTTP01Address(ctx, request)
		if err == nil {
			solver := &nodecert.HTTP01{Address: r.httpAddress}
			defer solver.Close()
			err = solver.Reserve()
			if err == nil {
				bundle, err = r.issue(ctx, request.CA, request.Email, account, request.Domain, "http-01", solver)
			}
		}
	}
	var installed nodecert.Metadata
	if err == nil {
		installed, err = nodecert.Install(r.directory, bundle, request.Domain, r.roots)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.Config == nil || r.state.Config.Revision != request.Revision || r.ctx.Err() != nil || errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	next := r.state
	if err != nil {
		next.LastError = certificateErrorCode(err)
	} else {
		next.Metadata = installed
		next.AppliedRevision = request.Revision
		next.LastError = ""
	}
	if r.save(next) == nil {
		r.state = next
	}
}
func readCertificateFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.New("node_certificate_file_unreadable")
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("node_certificate_file_invalid")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("node_certificate_file_unreadable")
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("node_certificate_file_invalid")
	}
	body, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return nil, errors.New("node_certificate_file_invalid")
	}
	return body, nil
}
func certificateErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "certificate_operation_timed_out"
	}
	switch err.Error() {
	case "http01_port_occupied", "http01_dns_not_resolved", "http01_dns_address_mismatch", "certificate_key_pair_invalid", "certificate_hostname_mismatch", "certificate_expired_or_not_yet_valid", "certificate_chain_untrusted", "certificate_material_too_large", "node_certificate_file_unreadable", "node_certificate_file_invalid", "acme_order_failed", "acme_account_failed":
		return err.Error()
	}
	return "certificate_operation_failed"
}
func (r *CertificateManager) Close() {
	r.cancel()
	r.mu.Lock()
	if r.jobCancel != nil {
		r.jobCancel()
	}
	r.mu.Unlock()
}

func (r *CertificateManager) validateRequest(request CertificateRequest) error {
	if request.Domain == "" || !strings.Contains(request.Domain, ".") || strings.ContainsAny(request.Domain, "/*\\ \t\r\n") || net.ParseIP(request.Domain) != nil {
		return errors.New("invalid_certificate_domain")
	}
	switch request.Method {
	case "install":
		if _, err := nodecert.Validate(request.Bundle, request.Domain, r.roots, time.Now()); err != nil {
			return err
		}
	case "http01":
		if _, err := nodecert.Directory(request.CA); err != nil {
			return err
		}
		if net.ParseIP(request.ExpectedIP) == nil {
			return errors.New("http01_expected_ip_required")
		}
	case "remote_paths":
		if !filepath.IsAbs(request.CertificatePath) || !filepath.IsAbs(request.KeyPath) {
			return errors.New("certificate_paths_must_be_absolute_on_node")
		}
	default:
		return errors.New("unsupported_certificate_method")
	}
	return nil
}

// The caller holds r.mu while updating durable account state.
func (r *CertificateManager) ensureAccountKey(request CertificateRequest) error {
	if request.Method == "http01" && r.state.AccountKey == "" {
		key, err := nodecert.NewAccountKey()
		if err != nil {
			return err
		}
		next := r.state
		next.AccountKey = key
		if err := r.save(next); err != nil {
			return err
		}
		r.state = next
	}
	return nil
}

func (r *CertificateManager) verifyHTTP01Address(ctx context.Context, request CertificateRequest) error {
	addresses, lookupErr := r.lookup(ctx, request.Domain)
	expected := net.ParseIP(request.ExpectedIP)
	if lookupErr != nil || len(addresses) == 0 {
		return errors.New("http01_dns_not_resolved")
	} else {
		for _, address := range addresses {
			if !address.IP.Equal(expected) {
				return errors.New("http01_dns_address_mismatch")
			}
		}
	}
	return nil
}
