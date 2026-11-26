package nodeagent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestV2RealCoreKeepsRevokeOnRestart(t *testing.T) {
	port := freeV2Port(t)
	state := t.TempDir()
	runtime := NewCoreRuntimeWithState(state)
	m := NewManagerV2(7, FileStoreV2{Directory: state}, runtime)
	if err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	spec, access := vlessState(m.Status().Epoch, port)
	requireNodeTestSuccess(t, m.ApplyAccess(access))
	requireNodeTestSuccess(t, m.ApplyStructure(spec))
	if !m.Status().Running {
		t.Fatal("actual core did not start")
	}
	if !strings.Contains(string(runtime.config), "11111111") {
		t.Fatal("configured user was not applied")
	}
	runtime.Interrupt()
	if runtime.Running() {
		t.Fatal("canceled core reported ready")
	}
	if err := m.Refresh(); err != nil || !runtime.Running() {
		t.Fatal("interrupted startup did not recover a fresh context", err)
	}
	deny := AccessSnapshot{NodeID: 7, Epoch: m.Status().Epoch, Revision: 2, Clients: []Grant{}}
	if err := m.ApplyAccess(deny); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(runtime.config), "11111111") || !m.Status().Running {
		t.Fatal("real hot revoke failed")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	nextRuntime := NewCoreRuntimeWithState(state)
	next := NewManagerV2(7, FileStoreV2{Directory: state}, nextRuntime)
	defer next.Close()
	if err := next.Restore(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(nextRuntime.config), "11111111") || !next.Status().Running {
		t.Fatal("restart restored a revoked ACL")
	}
}

func TestV2FreshCoreStartsAfterAccessBeforeStructure(t *testing.T) {
	state := t.TempDir()
	m := NewManagerV2(7, FileStoreV2{Directory: state}, NewCoreRuntimeWithState(state))
	defer m.Close()
	if err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	deny := AccessSnapshot{NodeID: 7, Epoch: m.Status().Epoch, Revision: 1, Clients: []Grant{}}
	if err := m.ApplyAccess(deny); err != nil {
		t.Fatal(err)
	}
	port := freeV2Port(t)
	s := Structure{NodeID: 7, Epoch: m.Status().Epoch, Revision: 1, Config: json.RawMessage(fmt.Sprintf(`{"inbounds":[{"type":"vless","tag":"vpn","listen":"127.0.0.1","listen_port":%d}],"outbounds":[{"type":"direct","tag":"direct"}]}`, port))}
	if err := m.ApplyStructure(s); err != nil {
		t.Fatal(err)
	}
	if !m.Status().Running {
		t.Fatal("fresh core retained a canceled bootstrap context")
	}
}
