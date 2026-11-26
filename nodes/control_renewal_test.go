package nodes

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database/model"
)

func TestControlRenewalChecksBothKeyPairsAndExpiryWindow(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	var node model.Node
	var install model.NodeInstallation
	if err := newControlIdentity(&node, &install); err != nil {
		t.Fatal(err)
	}
	if !controlIdentityValid(node, install, time.Now()) {
		t.Fatal("fresh identity needs renewal")
	}
	block, _ := pem.Decode([]byte(node.ControlCertificate))
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !controlIdentityValid(node, install, leaf.NotAfter.Add(-31*24*time.Hour)) {
		t.Fatal("rotation starts too early")
	}
	if controlIdentityValid(node, install, leaf.NotAfter.Add(-29*24*time.Hour)) {
		t.Fatal("rotation window ignored")
	}
	if controlIdentityValid(node, install, leaf.NotAfter.Add(time.Hour)) {
		t.Fatal("expired identity accepted")
	}
	bad := install
	bad.ServerKey, err = sealSecret(node.ControlKey)
	if err != nil {
		t.Fatal(err)
	}
	if controlIdentityValid(node, bad, time.Now()) {
		t.Fatal("mismatched server key accepted")
	}
	bad.ServerCertificate = "corrupt"
	if controlIdentityValid(node, bad, time.Now()) {
		t.Fatal("corrupt server identity accepted")
	}
	other := node
	other.ControlKey = "corrupt"
	if controlIdentityValid(other, install, time.Now()) {
		t.Fatal("corrupt controller key accepted")
	}
}

func TestControlRenewalJobIsDurableAndRateLimited(t *testing.T) {
	db, request := liveCreateFixture(t)
	first, _, err := createLiveProvision(request, "unit")
	if err != nil {
		t.Fatal(err)
	}
	db.Model(&first).Update("state", "succeeded")
	var node model.Node
	var install model.NodeInstallation
	db.First(&node, first.NodeId)
	db.First(&install, "node_id = ?", node.Id)
	install.PrivateKey, err = sealSecret("unit-private-ssh-key")
	if err != nil {
		t.Fatal(err)
	}
	if err = newControlIdentity(&node, &install); err != nil {
		t.Fatal(err)
	}
	db.Save(&node)
	db.Save(&install)
	now := time.Unix(1800000000, 0)
	job, created, err := queueControlRenewal(node, install, now)
	if err != nil || !created {
		t.Fatal("renewal was not queued", err)
	}
	plan, err := loadLivePlan(job)
	if err != nil || plan.Request.Password != "" || plan.Request.ReplaceHostKey || plan.Target.PrivateKey != install.PrivateKey || plan.Previous.OwnerID != install.OwnerID {
		t.Fatal("renewal bypassed pinned SSH or lost its durable key", err)
	}
	if plan.Request.TemplateId != node.TemplateId || plan.ExpectedGeneration != node.ManagementGeneration {
		t.Fatal("renewal replaced node settings")
	}
	db.Model(&job).Updates(map[string]interface{}{"state": "failed", "updated": now.Unix()})
	duplicate, created, err := queueControlRenewal(node, install, now.Add(time.Minute))
	if err != nil || created || duplicate.Id != job.Id {
		t.Fatal("renewal retried without backoff", err)
	}
	if _, created, err = queueControlRenewal(node, install, now.Add(controlRenewRetry)); err != nil || !created {
		t.Fatal("renewal did not retry in the next time slot", err)
	}
	var client model.Client
	db.First(&client, 1)
	if !bytes.Contains(client.Config, []byte("11111111")) {
		t.Fatal("renewal changed customer credentials")
	}
}

func TestControlRenewalDoesNotReplaceMissingSecretKey(t *testing.T) {
	_, request := liveCreateFixture(t)
	job, _, err := createLiveProvision(request, "unit")
	if err != nil {
		t.Fatal(err)
	}
	var node model.Node
	var install model.NodeInstallation
	plan, err := loadLivePlan(job)
	if err != nil {
		t.Fatal(err)
	}
	node.Id = job.NodeId
	install.PrivateKey, err = sealSecret("unit-private-ssh-key")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.Getenv("SUI_DB_FOLDER"), "node-provision.key")
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err = queueControlRenewal(node, install, time.Now()); err == nil {
		t.Fatal("missing state key accepted")
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("automatic renewal silently created another key")
	}
	if plan.Request.Password != "" {
		t.Fatal("password in renewal fixture")
	}
}

func TestDisabledNodeRenewalReadinessDoesNotWaitForVPNCertificate(t *testing.T) {
	db, node, _ := fixture(t)
	db.Model(&node).Updates(map[string]interface{}{"enable": false, "desired_revision": 1, "applied_revision": 1, "tls_status": json.RawMessage(`{}`)})
	db.Create(&model.NodeAccessState{NodeId: node.Id, DesiredRevision: 1, AppliedRevision: 1})
	db.Create(&model.NodeDomainBinding{NodeId: node.Id, Mode: "manual", FQDN: "node.example", State: "pending"})
	db.Create(&model.NodeCertificateBinding{NodeId: node.Id, Mode: "http01", State: "pending"})
	if !liveProvisionReady(node.Id) {
		t.Fatal("disabled node cannot finish control renewal")
	}
	db.Model(&node).Update("enable", true)
	if liveProvisionReady(node.Id) {
		t.Fatal("enabled node bypassed VPN readiness")
	}
}
