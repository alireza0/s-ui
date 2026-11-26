package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/alireza0/s-ui/database/model"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkspaceReadIsRedactedAndDoesNotMaterialize(t *testing.T) {
	f := liveSetup(t, 1)
	var before int64
	f.db.Model(&model.NodeInbound{}).Count(&before)
	view, err := ReadWorkspace(1)
	if err != nil {
		t.Fatal(err)
	}
	if view["spec"] == nil {
		t.Fatal("missing editor spec")
	}
	var after int64
	f.db.Model(&model.NodeInbound{}).Count(&after)
	if before != after {
		t.Fatal("reading materialized a node")
	}
	if err := f.db.Create(&model.NodeInbound{NodeId: 1, Tag: "fixture-sensitive", Type: "vless", Options: json.RawMessage(`{"tls":{"reality":{"private_key":"private-fixture"}},"users":[{"uuid":"private-user"}]}`)}).Error; err != nil {
		t.Fatal(err)
	}
	view, err = ReadWorkspace(1)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(view)
	if bytes.Contains(body, []byte("private-fixture")) || bytes.Contains(body, []byte("private-user")) {
		t.Fatal("workspace leaked server material")
	}
}
func TestDNSZoneDiscoveryUsesWriteOnlyCredentialWithoutPersistingProfile(t *testing.T) {
	f := liveSetup(t, 1)
	var before int64
	f.db.Model(&model.NodeDNSProfile{}).Count(&before)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/zones" || r.Header.Get("Authorization") != "Bearer fixture-discovery-token" {
			t.Error("invalid discovery request")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"result":[{"id":"00000000000000000000000000000001","name":"example.com"}]}`))
	}))
	defer server.Close()
	old := cloudflareResourceURL
	cloudflareResourceURL = server.URL
	defer func() { cloudflareResourceURL = old }()
	zones, err := DiscoverDNSZones(context.Background(), json.RawMessage(`{"auth_type":"token","token":"fixture-discovery-token"}`))
	if err != nil || len(zones) != 1 || zones[0].Name != "example.com" {
		t.Fatal("zone discovery failed", err)
	}
	var after int64
	f.db.Model(&model.NodeDNSProfile{}).Count(&after)
	if before != after {
		t.Fatal("discovery persisted a credential/profile")
	}
	body, _ := json.Marshal(zones)
	if bytes.Contains(body, []byte("fixture-discovery-token")) {
		t.Fatal("discovery returned credential")
	}
}
func TestLiveTemplateClientTLSMaterializesAndRejectsSecret(t *testing.T) {
	f := liveSetup(t, 1)
	spec := liveSpec()
	spec.Entries[0].ClientTLS = json.RawMessage(`{"utls":{"enabled":true,"fingerprint":"firefox"}}`)
	f.save(t, spec)
	if err := Reconcile(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	var inbound model.NodeInbound
	f.db.First(&inbound)
	if !bytes.Contains(inbound.ClientTLS, []byte("firefox")) {
		t.Fatal("client TLS lost during materialization")
	}
	spec.Entries[0].ClientTLS = json.RawMessage(`{"client_key_path":"/private/key.pem"}`)
	body, _ := json.Marshal(spec)
	if _, err := NormalizeSpec(body, nil); err == nil {
		t.Fatal("private client file allowed in shared template")
	}
}
