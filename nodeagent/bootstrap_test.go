package nodeagent

import "testing"

func TestInstallationBootstrapHoldsOldAccessUntilFreshRevoke(t *testing.T) {
	m, old, store, s, a := v2Fixture(t)
	if !old.hasUser() {
		t.Fatal("fixture not running")
	}
	m.Close()
	runtime := &safeFake{}
	next := NewManagerV2(7, store, runtime)
	next.RequireBootstrap("operation-two")
	if err := next.Restore(); err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if err := next.Refresh(); err != nil {
		t.Fatal(err)
	}
	if runtime.Running() || !next.Status().BootstrapPending {
		t.Fatal("restored stale ACL started")
	}
	if err := next.ApplyStructure(s); err == nil {
		t.Fatal("spec bypassed fresh access")
	}
	a.Revision++
	a.Clients = []Grant{}
	if err := next.ApplyAccess(a); err != nil {
		t.Fatal(err)
	}
	if runtime.hasUser() || next.Status().BootstrapPending {
		t.Fatal("revoke/bootstrap not durable")
	}
	next.Close()
	restarted := NewManagerV2(7, store, &safeFake{})
	restarted.RequireBootstrap("operation-two")
	if err := restarted.Restore(); err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if !restarted.Status().Running || restarted.Status().BootstrapPending {
		t.Fatal("normal offline restart blocked")
	}
}

func TestNewRegistrationBootstrapDoesNotInventOrResetAccess(t *testing.T) {
	store := FileStoreV2{Directory: t.TempDir()}
	runtime := &safeFake{}
	m := NewManagerV2(7, store, runtime)
	m.RequireBootstrap("operation-new")
	if err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	status := m.Status()
	if status.Epoch == "" || status.AccessRevision != 0 || status.Running || !status.BootstrapPending {
		t.Fatal("invalid initial bootstrap")
	}
	_, a := vlessState(status.Epoch, 14443)
	a.Revision = 100
	a.Clients = []Grant{}
	if err := m.ApplyAccess(a); err != nil {
		t.Fatal(err)
	}
	a.Revision = 99
	if err := m.ApplyAccess(a); err == nil {
		t.Fatal("rollback accepted")
	}
}
