package nodes

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
)

func TestExtendedNodeProtocolsKeepExistingAccessAndCredentials(t *testing.T) {
	f := liveSetup(t, 1)
	node := f.nodes[0]
	status := f.managers[0].Status()
	install := model.NodeInstallation{NodeId: node.Id, OwnerID: "0123456789abcdef0123456789abcdef"}
	if err := f.db.Create(&install).Error; err != nil {
		t.Fatal(err)
	}
	if extendedInboundsAvailable(f.db, node, status) {
		t.Fatal("unconfirmed trial advertised new credential formats")
	}
	oldAccess, err := compileAccess(f.db, node, false, false)
	if err != nil || len(oldAccess.Clients) != 1 {
		t.Fatalf("baseline access unavailable: %v", err)
	}
	node.AgentExtendedInbounds = true
	if !extendedInboundsAvailable(f.db, node, status) {
		t.Fatal("confirmed agent did not enable new protocols")
	}
	first, err := compileAccess(f.db, node, true, false)
	if err != nil || len(first.Clients) != 1 {
		t.Fatalf("extended access unavailable: %v", err)
	}
	second, err := compileAccess(f.db, node, true, false)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("credentials changed during subsequent reconciliation: %v", err)
	}
	for _, kind := range nodeagent.InboundTypes(false) {
		if string(first.Clients[0].Credentials[kind]) != string(oldAccess.Clients[0].Credentials[kind]) {
			t.Fatalf("%s credential changed", kind)
		}
	}
	for _, kind := range []string{"vmess", "trojan", "anytls"} {
		if err := nodeagent.ValidateCredential(kind, first.Clients[0].Credentials[kind]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExtendedNodeStructureWaitsForAgentAndUsesTCP(t *testing.T) {
	f := liveSetup(t, 1)
	spec := liveSpec()
	entry := SpecEntry{Key: "extra", Name: "VMess", Protocol: "vmess", Enable: true,
		Options:       json.RawMessage(`{"listen":"127.0.0.1","listen_port":8443,"tls":{"enabled":false}}`),
		ClientOptions: json.RawMessage(`{"security":"auto","packet_encoding":"xudp","multiplex":{"enabled":true}}`)}
	spec.Entries = append(spec.Entries, entry)
	f.save(t, spec)
	if err := f.db.First(&f.nodes[0], f.nodes[0].Id).Error; err != nil {
		t.Fatal(err)
	}
	status := f.managers[0].Status()
	status.ExtendedInbounds = false
	_, _, err := prepareStructure(f.db, f.nodes[0], status, nil)
	if !errors.Is(err, errAgentProtocolsPending) {
		t.Fatalf("older agent did not keep its existing spec: %v", err)
	}
	f.run(t, 0)
	var reservation model.NodePortReservation
	if err := f.db.First(&reservation, "node_id = ? AND connection_key = ?", f.nodes[0].Id, entry.Key).Error; err != nil ||
		reservation.Network != "tcp" || reservation.Port != 8443 {
		t.Fatalf("VMess TCP socket was not reserved: %+v %v", reservation, err)
	}
}
