package nodes

import (
	"encoding/json"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/alireza0/s-ui/nodecert"
	"testing"
	"time"
)

func TestCertificateDeliveryAckRequeuesStructureWithoutChangingLeaf(t *testing.T) {
	db, node, _ := fixture(t)
	node.ControlEpoch = "registered"
	node.ManagementGeneration = 2
	if err := db.Save(&node).Error; err != nil {
		t.Fatal(err)
	}
	meta := nodecert.Metadata{Domain: "node.example", Digest: "stored-leaf", NotAfter: time.Now().Add(time.Hour).Unix(), CertificatePath: "certificates/leaf.pem", KeyPath: "certificates/key.pem"}
	body, _ := json.Marshal(meta)
	domain := model.NodeDomainBinding{NodeId: node.Id, FQDN: meta.Domain, AppliedFQDN: meta.Domain, State: "ready"}
	binding := model.NodeCertificateBinding{NodeId: node.Id, Generation: 1, State: "pending", DesiredRevision: 7, Metadata: body}
	task := model.NodeTask{NodeId: node.Id, Kind: "structure", Generation: 1, State: "rejected"}
	for _, v := range []interface{}{&domain, &binding, &task} {
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	context := ResolveContext{Parameters: map[string]json.RawMessage{}}
	if err := resourceTLSContext(db, node, &context); err != nil || context.Certificates["primary"] != nil {
		t.Fatal("unacknowledged leaf used", err)
	}
	request := nodeagent.CertificateRequest{Revision: 7}
	if err := acknowledgeCertificate(node, binding, request, meta, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&task, "node_id = ? AND kind = ?", node.Id, "structure").Error; err != nil {
		t.Fatal(err)
	}
	if task.State != "queued" || task.Generation != 2 {
		t.Fatal("certificate receipt did not release failed structure", task)
	}
	if err := resourceTLSContext(db, node, &context); err != nil || context.Certificates["primary"] == nil {
		t.Fatal("acknowledged leaf unavailable", err)
	}
	if err := acknowledgeCertificate(node, binding, request, meta, ""); err != nil {
		t.Fatal(err)
	}
	db.First(&task, "node_id = ? AND kind = ?", node.Id, "structure")
	if task.Generation != 2 {
		t.Fatal("unchanged periodic receipt requeued structure")
	}
	old := node
	old.ManagementGeneration--
	if err := acknowledgeCertificate(old, binding, request, meta, ""); err == nil {
		t.Fatal("old target certificate receipt accepted")
	}
}
