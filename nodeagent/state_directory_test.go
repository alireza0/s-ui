package nodeagent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCanonicalSystemdStateRootAllowsPrivateCertificateCheckpoint(t *testing.T) {
	parent := t.TempDir()
	statePath := filepath.Join(parent, "private-state")
	alias := filepath.Join(parent, "state-alias")
	if err := os.Mkdir(statePath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(statePath, alias); err != nil {
		t.Skip("symlink not available")
	}
	directory, err := CanonicalStateDirectory(alias)
	if err != nil || directory != statePath {
		t.Fatal("not canonical", directory, err)
	}
	manager := NewManagerV2(7, FileStoreV2{Directory: directory}, &safeFake{})
	if err = manager.Restore(); err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	factory, roots := resourceCertificateFactory(t)
	manager.resources.roots = roots
	request := CertificateRequest{NodeID: 7, Epoch: manager.Status().Epoch, Revision: 1, Domain: "node.example", Method: "install", Bundle: factory("node.example", 1, time.Now().Add(24*time.Hour))}
	if err = manager.resources.Configure(request); err != nil {
		t.Fatal("systemd alias blocked certificate checkpoint", err)
	}
	if _, err = os.Stat(filepath.Join(statePath, "certificate-state.json")); err != nil {
		t.Fatal(err)
	}
}
