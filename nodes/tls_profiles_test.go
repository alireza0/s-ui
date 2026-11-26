package nodes

import (
	"encoding/json"
	"testing"

	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

func TestNodeTLSProfileAppliesAndKeepsPerNodeKeys(t *testing.T) {
	f := liveSetup(t, 2)
	profile := model.Tls{
		Name:   "Shared REALITY",
		Server: json.RawMessage(`{"enabled":true,"server_name":"target.example","reality":{"enabled":true,"private_key":"local-only","short_id":["00"],"handshake":{"server":"target.example","server_port":443}}}`),
		Client: json.RawMessage(`{"utls":{"enabled":true,"fingerprint":"chrome"}}`),
	}
	if err := f.db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}
	spec := liveSpec()
	spec.Entries[0].TLSProfileID = profile.Id
	f.save(t, spec)
	read := func(index int) (model.NodeInbound, string) {
		t.Helper()
		f.run(t, index)
		var inbound model.NodeInbound
		if err := f.db.First(&inbound, "node_id = ? AND connection_key = ?", f.nodes[index].Id, spec.Entries[0].Key).Error; err != nil {
			t.Fatal(err)
		}
		key, err := loadGeneratedValue(f.db, f.nodes[index].Id, spec.Entries[0].Key, "reality_private_key")
		if err != nil {
			t.Fatal(err)
		}
		return inbound, string(key)
	}
	first, key := read(0)
	_, other := read(1)
	if key == "" || key == other || key == `"local-only"` {
		t.Fatal("expected separate generated keys")
	}
	if err := CheckTLSProfileDeletion(f.db, profile.Id); err == nil {
		t.Fatal("deletion of linked TLS profile was accepted")
	}
	profile.Client = json.RawMessage(`{"utls":{"enabled":true,"fingerprint":"firefox"}}`)
	if err := f.db.Transaction(func(tx *gorm.DB) error {
		if err := QueueTLSProfileChange(tx, profile); err != nil {
			return err
		}
		return tx.Save(&profile).Error
	}); err != nil {
		t.Fatal(err)
	}
	updated, newKey := read(0)
	if first.Id != updated.Id || newKey != key || string(updated.ClientTLS) != string(profile.Client) {
		t.Fatalf("profile edit did not propagate with stable identity: %s", updated.ClientTLS)
	}
}

func TestNodeTLSProfileUsesManagedCertificateAndRejectsAdvancedModes(t *testing.T) {
	entry := liveSpec().Entries[0]
	entry.Protocol = "hysteria2"
	profile := model.Tls{Server: json.RawMessage(`{"enabled":true,"server_name":"local.example","key_path":"/local/key.pem","alpn":["h3"]}`)}
	if err := bindTLSProfile(&entry, profile); err != nil {
		t.Fatal(err)
	}
	if entry.Sources["/tls/certificate_binding"].Name != "primary" ||
		string(entry.Options) != `{"listen":"127.0.0.1","tls":{"alpn":["h3"],"enabled":true}}` {
		t.Fatal("expected node certificate with no paths or name from the local panel")
	}
	profile.Server = json.RawMessage(`{"enabled":true,"ech":{"enabled":true,"key_path":"/local/ech.pem"}}`)
	if err := bindTLSProfile(&entry, profile); err == nil {
		t.Fatal("accepted unsupported ECH policy")
	}
	profile.Server = json.RawMessage(`{"enabled":true,"reality":{"enabled":true,"handshake":{"server":"target.example","server_port":443}}}`)
	if err := bindTLSProfile(&entry, profile); err == nil {
		t.Fatal("accepted a REALITY profile for Hysteria2")
	}
}
