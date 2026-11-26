package nodeagent

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alireza0/s-ui/nodecert"
)

func resourceCertificateFactory(t *testing.T) (func(string, int, time.Time) nodecert.Bundle, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	root := &x509.Certificate{SerialNumber: big.NewInt(100), Subject: pkix.Name{CommonName: "Fixture certificate authority"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(30 * 24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, _ = x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(root)
	return func(domain string, serial int, expiry time.Time) nodecert.Bundle {
		private, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(int64(serial)), DNSNames: []string{domain}, NotBefore: time.Now().Add(-time.Minute), NotAfter: expiry, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		cert, err := x509.CreateCertificate(rand.Reader, leaf, root, &private.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		privateDER, _ := x509.MarshalPKCS8PrivateKey(private)
		return nodecert.Bundle{Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})) + string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), Key: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))}
	}, pool
}
func waitResource(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("certificate operation did not reach expected state")
}
func assertResourceTLS(t *testing.T, port, serial int, roots *x509.CertPool) {
	t.Helper()
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", fmt.Sprintf("127.0.0.1:%d", port), &tls.Config{ServerName: "vpn.example", RootCAs: roots})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if conn.ConnectionState().PeerCertificates[0].SerialNumber.Int64() != int64(serial) {
		t.Fatal("VPN listener did not use expected certificate")
	}
}

func TestResourceOfflineHTTPRenewalKeepsAccessIndependentAndSurvivesRestart(t *testing.T) {
	factory, roots := resourceCertificateFactory(t)
	now := time.Now()
	first, second := factory("vpn.example", 1, now.Add(12*time.Hour)), factory("vpn.example", 2, now.Add(24*time.Hour))
	state := t.TempDir()
	runtime := NewCoreRuntimeWithState(state)
	manager := NewManagerV2(7, FileStoreV2{Directory: state}, runtime)
	if err := manager.Restore(); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	resource := manager.resources
	resource.roots = roots
	resource.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	resource.httpAddress = fmt.Sprintf("127.0.0.1:%d", freeV2Port(t))
	var delta atomic.Int64
	resource.renewNow = func() time.Time { return now.Add(time.Duration(delta.Load()) * time.Hour) }
	entered, release := make(chan struct{}), make(chan struct{})
	var count atomic.Int64
	resource.issue = func(ctx context.Context, _, _, _, _, _ string, _ *nodecert.HTTP01) (nodecert.Bundle, error) {
		n := count.Add(1)
		if n == 1 {
			return first, nil
		}
		if n == 2 {
			close(entered)
			select {
			case <-release:
				return second, nil
			case <-ctx.Done():
				return nodecert.Bundle{}, ctx.Err()
			}
		}
		return nodecert.Bundle{}, errors.New("acme_order_failed")
	}
	request := CertificateRequest{NodeID: 7, Epoch: manager.Status().Epoch, Revision: 1, Domain: "vpn.example", ExpectedIP: "127.0.0.1", Method: "http01", CA: "letsencrypt"}
	if err := resource.Configure(request); err != nil {
		t.Fatal(err)
	}
	waitResource(t, func() bool { return resource.Status().State == "ready" })
	port := freeV2Port(t)
	spec := Structure{NodeID: 7, Epoch: request.Epoch, Revision: 1, Config: json.RawMessage(fmt.Sprintf(`{"inbounds":[{"type":"vless","tag":"vpn","listen":"127.0.0.1","listen_port":%d,"tls":{"enabled":true,"server_name":"vpn.example","certificate_path":"managed:primary","key_path":"managed:primary"}}],"outbounds":[{"type":"direct","tag":"direct"}]}`, port))}
	access := AccessSnapshot{NodeID: 7, Epoch: request.Epoch, Revision: 1, Clients: []Grant{{Name: "client/1", All: true, Credentials: map[string]json.RawMessage{"vless": json.RawMessage(`{"uuid":"11111111-1111-4111-8111-111111111111"}`)}}}}
	if err := manager.ApplyAccess(access); err != nil {
		t.Fatal(err)
	}
	if err := manager.ApplyStructure(spec); err != nil {
		t.Fatal(err)
	}
	assertResourceTLS(t, port, 1, roots)
	delta.Store(9)
	select {
	case resource.kick <- struct{}{}:
	default:
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("offline renewal did not start")
	}
	access.Revision = 2
	access.Clients = []Grant{}
	done := make(chan error, 1)
	go func() { done <- manager.ApplyAccess(access) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("ACME blocked access revocation")
	}
	assertResourceTLS(t, port, 1, roots)
	close(release)
	waitResource(t, func() bool { return resource.Status().Metadata.SHA256 != manager.Status().Certificates[0].SHA256 })
	if err := manager.Refresh(); err != nil {
		t.Fatal(err)
	}
	assertResourceTLS(t, port, 2, roots)
	if bytes.Contains(runtime.config, []byte("11111111")) {
		t.Fatal("renewal rolled permissions back")
	}
	delta.Store(22)
	select {
	case resource.kick <- struct{}{}:
	default:
	}
	waitResource(t, func() bool { return resource.Status().LastError != "" })
	if err := manager.Refresh(); err != nil {
		t.Fatal(err)
	}
	assertResourceTLS(t, port, 2, roots)
	public, _ := json.Marshal(manager.Status())
	if bytes.Contains(public, []byte("PRIVATE KEY")) || bytes.Contains(public, []byte("certificate_path\":\"/")) {
		t.Fatal("public status leaked certificate secrets")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	next := NewManagerV2(7, FileStoreV2{Directory: state}, NewCoreRuntimeWithState(state))
	defer next.Close()
	if err := next.Restore(); err != nil {
		t.Fatal(err)
	}
	assertResourceTLS(t, port, 2, roots)
	if next.Status().AccessRevision != 2 {
		t.Fatal("restart lost latest access")
	}
}

func TestResourceCertificateValidationReplayAndRemotePaths(t *testing.T) {
	factory, roots := resourceCertificateFactory(t)
	bundle := factory("vpn.example", 1, time.Now().Add(12*time.Hour))
	resource, err := NewCertificateManager(7, "fixture-epoch", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer resource.Close()
	resource.roots = roots
	request := CertificateRequest{NodeID: 7, Epoch: "fixture-epoch", Revision: 1, Domain: "vpn.example", Method: "install", Bundle: bundle}
	wrong := request
	wrong.Domain = "wrong.example"
	if err := resource.Configure(wrong); err == nil {
		t.Fatal("wrong host accepted")
	}
	wrong = request
	wrong.Bundle.Key = "not a key"
	if err := resource.Configure(wrong); err == nil {
		t.Fatal("bad key accepted")
	}
	if err := resource.Configure(request); err != nil {
		t.Fatal(err)
	}
	waitResource(t, func() bool { return resource.Status().State == "ready" })
	if err := resource.Configure(request); err != nil {
		t.Fatal("idempotent retry rejected", err)
	}
	wrong = request
	wrong.Domain = "other.example"
	wrong.Bundle = factory("other.example", 2, time.Now().Add(12*time.Hour))
	if !errors.Is(resource.Configure(wrong), ErrConflict) {
		t.Fatal("same revision replaced certificate")
	}
	meta, err := nodecert.Install(t.TempDir(), bundle, "vpn.example", roots)
	if err != nil {
		t.Fatal(err)
	}
	wrong = request
	wrong.Revision = 2
	wrong.Method = "remote_paths"
	wrong.Bundle = nodecert.Bundle{}
	wrong.CertificatePath = meta.CertificatePath
	wrong.KeyPath = meta.KeyPath
	if err := resource.Configure(wrong); err == nil {
		t.Fatal("relative path interpreted as panel path")
	}
	var fileRequest = wrong
	fileRequest.CertificatePath = filepath.Join(t.TempDir(), "missing.pem")
	fileRequest.KeyPath = filepath.Join(t.TempDir(), "missing-key.pem")
	if err := resource.Configure(fileRequest); err != nil {
		t.Fatal(err)
	}
	waitResource(t, func() bool { return resource.Status().State == "failed" })
	if resource.Status().Metadata.SHA256 == "" {
		t.Fatal("missing remote file erased old material")
	}
}

func TestResourceRejectedActivationKeepsDurableActiveCertificate(t *testing.T) {
	factory, roots := resourceCertificateFactory(t)
	directory := t.TempDir()
	resource, err := NewCertificateManager(7, "fixture-epoch", directory)
	if err != nil {
		t.Fatal(err)
	}
	resource.roots = roots
	defer resource.Close()
	request := CertificateRequest{NodeID: 7, Epoch: "fixture-epoch", Revision: 1, Domain: "vpn.example", Method: "install", Bundle: factory("vpn.example", 1, time.Now().Add(12*time.Hour))}
	if err := resource.Configure(request); err != nil {
		t.Fatal(err)
	}
	waitResource(t, func() bool { return resource.Status().State == "ready" })
	first := resource.Status().Metadata
	if !resource.BeginTrial() {
		t.Fatal("initial activation missing")
	}
	if err := resource.FinishTrial(true); err != nil {
		t.Fatal(err)
	}
	request.Revision = 2
	request.Bundle = factory("vpn.example", 2, time.Now().Add(24*time.Hour))
	if err := resource.Configure(request); err != nil {
		t.Fatal(err)
	}
	waitResource(t, func() bool { return resource.Status().AppliedRevision == 2 })
	if !resource.BeginTrial() {
		t.Fatal("replacement activation missing")
	}
	candidate, err := resource.Resolve("vpn.example")
	if err != nil || candidate.Digest == first.Digest {
		t.Fatal("trial did not resolve replacement", err)
	}
	if err := resource.FinishTrial(false); err != nil {
		t.Fatal(err)
	}
	active, err := resource.Resolve("vpn.example")
	if err != nil || active.Digest != first.Digest || resource.BeginTrial() {
		t.Fatal("failed trial replaced active certificate or ignored backoff", err)
	}
	resource.Close()
	restored, err := NewCertificateManager(7, "fixture-epoch", directory)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	active, err = restored.Resolve("vpn.example")
	if err != nil || active.Digest != first.Digest || restored.BeginTrial() {
		t.Fatal("restart lost last active certificate/backoff", err)
	}
	restored.mu.Lock()
	restored.state.RetryAt = 0
	restored.mu.Unlock()
	if !restored.BeginTrial() {
		t.Fatal("replacement did not retry after backoff")
	}
	if err := restored.FinishTrial(true); err != nil {
		t.Fatal(err)
	}
	active, err = restored.Resolve("vpn.example")
	if err != nil || active.Digest != candidate.Digest || restored.Status().LastError != "" {
		t.Fatal("successful retry retained the activation error or old certificate", err)
	}
	restored.Close()
	confirmed, err := NewCertificateManager(7, "fixture-epoch", directory)
	if err != nil {
		t.Fatal(err)
	}
	defer confirmed.Close()
	active, err = confirmed.Resolve("vpn.example")
	if err != nil || active.Digest != candidate.Digest || confirmed.Status().LastError != "" {
		t.Fatal("successful retry was not durable", err)
	}
}

func TestSuccessfulOldCertificateTrialKeepsNewResourceError(t *testing.T) {
	for _, code := range []string{"certificate_activation_failed", "certificate_issue_failed"} {
		t.Run(code, func(t *testing.T) {
			resource, err := NewCertificateManager(7, "fixture-epoch", t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer resource.Close()
			resource.state.Metadata = nodecert.Metadata{Domain: "vpn.example", Digest: "new-candidate"}
			resource.state.LastError = code
			resource.trial = &nodecert.Metadata{Domain: "vpn.example", Digest: "old-candidate"}
			if err := resource.FinishTrial(true); err != nil {
				t.Fatal(err)
			}
			if resource.Status().LastError != code {
				t.Fatal("old activation hid the newer resource error")
			}
		})
	}
}

func TestResourceIssuerTimeoutIsVisibleAndKeepsActiveMaterial(t *testing.T) {
	factory, roots := resourceCertificateFactory(t)
	r, err := NewCertificateManager(7, "fixture-epoch", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.roots, r.timeout, r.httpAddress = roots, 40*time.Millisecond, "127.0.0.1:0"
	r.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	r.issue = func(ctx context.Context, _, _, _, _, _ string, _ *nodecert.HTTP01) (nodecert.Bundle, error) {
		<-ctx.Done()
		return nodecert.Bundle{}, ctx.Err()
	}
	request := CertificateRequest{NodeID: 7, Epoch: "fixture-epoch", Revision: 1, Domain: "vpn.example", Method: "install", Bundle: factory("vpn.example", 1, time.Now().Add(12*time.Hour))}
	if err = r.Configure(request); err != nil {
		t.Fatal(err)
	}
	waitResource(t, func() bool { return r.Status().State == "ready" })
	if !r.BeginTrial() {
		t.Fatal("initial material not available")
	}
	if err = r.FinishTrial(true); err != nil {
		t.Fatal(err)
	}
	old := r.Status().Metadata.Digest
	request.Method, request.Revision, request.Bundle, request.ExpectedIP = "http01", 2, nodecert.Bundle{}, "127.0.0.1"
	if err = r.Configure(request); err != nil {
		t.Fatal(err)
	}
	waitResource(t, func() bool { return r.Status().LastError == "certificate_operation_timed_out" })
	meta, err := r.Resolve("vpn.example")
	if err != nil || meta.Digest != old || r.Status().Metadata.Digest != old {
		t.Fatal("timeout discarded working certificate", err)
	}
}
