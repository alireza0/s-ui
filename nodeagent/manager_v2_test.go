package nodeagent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeFake struct {
	mu         sync.Mutex
	body       []byte
	fail       bool
	failOnce   bool
	entered    chan struct{}
	cancel     chan struct{}
	once       sync.Once
	cancelOnce sync.Once
}

func (r *safeFake) Validate(b []byte) error {
	if !json.Valid(b) || bytes.Contains(b, []byte(`"broken"`)) {
		return errors.New("synthetic validation failure")
	}
	return nil
}
func (r *safeFake) ApplySafe(b []byte) error {
	if bytes.Contains(b, []byte(`"slow"`)) && r.entered != nil {
		r.once.Do(func() { close(r.entered) })
		<-r.cancel
		return errors.New("startup interrupted")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail || r.failOnce {
		r.failOnce = false
		r.body = nil
		return errors.New("synthetic partial update failure")
	}
	r.body = bytes.Clone(b)
	return nil
}
func (r *safeFake) Close() error  { r.mu.Lock(); defer r.mu.Unlock(); r.body = nil; return nil }
func (r *safeFake) Running() bool { r.mu.Lock(); defer r.mu.Unlock(); return len(r.body) > 0 }
func (r *safeFake) Interrupt() {
	if r.cancel != nil {
		r.cancelOnce.Do(func() { close(r.cancel) })
	}
}
func (r *safeFake) hasUser() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return bytes.Contains(r.body, []byte("11111111"))
}

func v2Fixture(t *testing.T) (*ManagerV2, *safeFake, FileStoreV2, Structure, AccessSnapshot) {
	t.Helper()
	store := FileStoreV2{Directory: t.TempDir()}
	runtime := &safeFake{}
	m := NewManagerV2(7, store, runtime)
	m.clock = func() time.Time { return time.Unix(100, 0) }
	if err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	s, a := vlessState(m.Status().Epoch, 14443)
	a.Revision = 1
	if err := m.ApplyStructure(s); err != nil {
		t.Fatal(err)
	}
	if err := m.ApplyAccess(a); err != nil {
		t.Fatal(err)
	}
	return m, runtime, store, s, a
}

func TestV2RevokeIgnoresRejectedSpecAndSurvivesRestart(t *testing.T) {
	m, r, store, s, a := v2Fixture(t)
	stale := a
	if !r.hasUser() {
		t.Fatal("fixture not authorised")
	}
	bad := s
	bad.Revision = 2
	bad.Config = json.RawMessage(strings.Replace(string(s.Config), `"direct"`, `"broken"`, 1))
	if err := m.ApplyStructure(bad); err == nil {
		t.Fatal("invalid spec accepted")
	}
	a.Revision = 2
	a.Clients = []Grant{}
	if err := m.ApplyAccess(a); err != nil {
		t.Fatal(err)
	}
	if r.hasUser() || m.Status().SpecRevision != 1 || m.Status().AppliedAccessRevision != 2 {
		t.Fatal("failed spec blocked revoke")
	}
	if !errors.Is(m.ApplyAccess(stale), ErrConflict) {
		t.Fatal("old access revision was replayed")
	}
	stale.Revision = 2
	if !errors.Is(m.ApplyAccess(stale), ErrConflict) {
		t.Fatal("conflicting digest reused the latest revision")
	}
	if err := m.ApplyStructure(s); err != nil {
		t.Fatal(err)
	}
	if r.hasUser() {
		t.Fatal("older structure reintroduced access")
	}
	_ = m.Close()
	nextRuntime := &safeFake{}
	next := NewManagerV2(7, store, nextRuntime)
	if err := next.Restore(); err != nil {
		t.Fatal(err)
	}
	if nextRuntime.hasUser() || !next.Status().Running {
		t.Fatal("reboot restored obsolete users or lost last-good")
	}
}

func TestV2AbsoluteExpiryWhileOfflineAndAfterReboot(t *testing.T) {
	m, r, store, _, a := v2Fixture(t)
	now := int64(100)
	m.clock = func() time.Time { return time.Unix(now, 0) }
	a.Revision = 2
	a.Clients[0].Expires = 110
	if err := m.ApplyAccess(a); err != nil || !r.hasUser() {
		t.Fatal("future absolute expiry denied early", err)
	}
	now = 111
	if err := m.Refresh(); err != nil || r.hasUser() || !m.Status().Running {
		t.Fatal("offline expiry did not revoke on last-good structure", err)
	}
	_ = m.Close()
	nextRuntime := &safeFake{}
	next := NewManagerV2(7, store, nextRuntime)
	next.clock = func() time.Time { return time.Unix(now, 0) }
	defer next.Close()
	if err := next.Restore(); err != nil || nextRuntime.hasUser() || !next.Status().Running {
		t.Fatal("offline reboot resurrected an expired client", err)
	}
}

func TestV2PersistsDenyBeforeRuntimeAndFailsClosed(t *testing.T) {
	m, r, store, _, a := v2Fixture(t)
	r.fail = true
	a.Revision = 2
	a.Clients = []Grant{}
	if err := m.ApplyAccess(a); err == nil {
		t.Fatal("failed runtime incorrectly acknowledged")
	}
	if m.Status().Running || r.hasUser() {
		t.Fatal("failed revoke returned to old users")
	}
	loaded, err := store.LoadV2()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Access.Revision != 2 || len(loaded.Access.Clients) != 0 {
		t.Fatal("deny intent was not durable")
	}
	nextRuntime := &safeFake{}
	next := NewManagerV2(7, store, nextRuntime)
	if err := next.Restore(); err != nil || nextRuntime.hasUser() {
		t.Fatal("crash recovery lost deny", err)
	}
}

func TestV2PartialUpdateRebuildsWithLatestACL(t *testing.T) {
	m, r, _, _, a := v2Fixture(t)
	r.failOnce = true
	a.Revision = 2
	a.Clients = []Grant{}
	if err := m.ApplyAccess(a); err != nil {
		t.Fatal(err)
	}
	if r.hasUser() || !m.Status().Running || m.Status().AppliedAccessRevision != 2 {
		t.Fatal("fallback did not retain latest permissions")
	}
}

func TestV2AccessPreemptsBlockedStructuralStartup(t *testing.T) {
	m, r, store, s, a := v2Fixture(t)
	r.entered = make(chan struct{})
	r.cancel = make(chan struct{})
	s.Revision = 2
	s.Config = json.RawMessage(strings.Replace(string(s.Config), `"direct"`, `"slow"`, 1))
	done := make(chan error, 1)
	go func() { done <- m.ApplyStructure(s) }()
	select {
	case <-r.entered:
	case <-time.After(time.Second):
		t.Fatal("spec did not begin")
	}
	a.Revision = 2
	a.Clients = []Grant{}
	accessDone := make(chan error, 1)
	go func() { accessDone <- m.ApplyAccess(a) }()
	select {
	case err := <-accessDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("revoke waited for network startup")
	}
	<-done
	state, _ := store.LoadV2()
	if len(state.Access.Clients) != 0 || r.hasUser() {
		t.Fatal("preempted startup restored users")
	}
}

type failingV2Checkpoint struct {
	FileStoreV2
	fail bool
}

func (s *failingV2Checkpoint) SaveV2(state StateV2) error {
	if s.fail && state.AppliedAccess != nil && state.AppliedAccess.Revision >= 2 {
		return errors.New("synthetic checkpoint failure")
	}
	return s.FileStoreV2.SaveV2(state)
}

func TestV2CheckpointFailureDoesNotRollBackDurableDeny(t *testing.T) {
	store := &failingV2Checkpoint{FileStoreV2: FileStoreV2{Directory: t.TempDir()}}
	runtime := &safeFake{}
	m := NewManagerV2(7, store, runtime)
	if err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	s, a := vlessState(m.Status().Epoch, 14443)
	a.Revision = 1
	if err := m.ApplyStructure(s); err != nil {
		t.Fatal(err)
	}
	if err := m.ApplyAccess(a); err != nil {
		t.Fatal(err)
	}
	store.fail = true
	a.Revision = 2
	a.Clients = []Grant{}
	if err := m.ApplyAccess(a); err == nil {
		t.Fatal("failed checkpoint was acknowledged")
	}
	if m.Status().Running {
		t.Fatal("uncertain permission state was left serving")
	}
	saved, err := store.LoadV2()
	if err != nil || saved.Access.Revision != 2 || len(saved.Access.Clients) != 0 {
		t.Fatal("latest deny was rolled back on disk", err)
	}
	nextRuntime := &safeFake{}
	next := NewManagerV2(7, store.FileStoreV2, nextRuntime)
	defer next.Close()
	if err := next.Restore(); err != nil {
		t.Fatal(err)
	}
	if nextRuntime.hasUser() {
		t.Fatal("restart restored pre-revoke permission")
	}
}

func vlessState(epoch string, port int) (Structure, AccessSnapshot) {
	s := Structure{NodeID: 7, Epoch: epoch, Revision: 1, Config: json.RawMessage(fmt.Sprintf(`{"inbounds":[{"type":"vless","tag":"vpn","listen":"127.0.0.1","listen_port":%d}],"outbounds":[{"type":"direct","tag":"direct"}]}`, port))}
	a := AccessSnapshot{NodeID: 7, Epoch: epoch, Revision: 1, Clients: []Grant{{Name: "client/1", All: true, Credentials: map[string]json.RawMessage{"vless": json.RawMessage(`{"uuid":"11111111-1111-4111-8111-111111111111"}`)}}}}
	return s, a
}

func TestCorruptSavedStateDoesNotStartAgent(t *testing.T) {
	m, _, store, _, _ := v2Fixture(t)
	requireNodeTestSuccess(t, m.Close())
	requireNodeTestSuccess(t, os.WriteFile(filepath.Join(store.Directory, "state-v2.json"), []byte("corrupt"), 0o600))
	runtime := &safeFake{}
	restored := NewManagerV2(7, store, runtime)
	if err := restored.Restore(); err == nil || runtime.Running() {
		t.Fatal("agent accepted corrupt persisted state")
	}
}
