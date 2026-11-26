package nodes

import (
	"encoding/json"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"testing"
	"time"
)

func TestProviderEndpointNeedsCertificateReportForConfirmedRevision(t *testing.T) {
	node := model.Node{Id: 1}
	snapshot := Snapshot{Deployment: structureSnapshot{Revision: 7}}
	inbound := model.NodeInbound{Id: 2, Options: json.RawMessage(`{"tls":{"enabled":true,"certificate_provider":"own"}}`)}
	if certificatePublished(node, snapshot, inbound) {
		t.Fatal("published before certificate available")
	}
	status := nodeagent.StatusV2{SpecRevision: 7, Certificates: []nodeagent.CertificateStatus{{Inbound: "node-1-inbound-2", State: "ready", NotAfter: time.Now().Add(time.Hour).Unix()}}}
	node.TLSStatus = certificateReport(status.SpecRevision, status.Certificates)
	if !certificatePublished(node, snapshot, inbound) {
		t.Fatal("ready certificate hidden")
	}
	status.SpecRevision = 6
	node.TLSStatus = certificateReport(status.SpecRevision, status.Certificates)
	if certificatePublished(node, snapshot, inbound) {
		t.Fatal("stale report published")
	}
	status.SpecRevision = 7
	status.Certificates[0].NotAfter = time.Now().Add(-time.Second).Unix()
	node.TLSStatus = certificateReport(status.SpecRevision, status.Certificates)
	if certificatePublished(node, snapshot, inbound) {
		t.Fatal("expired certificate published")
	}
}
