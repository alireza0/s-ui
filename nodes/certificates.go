package nodes

import (
	"encoding/json"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"time"
)

type CertificateReport struct {
	Revision     uint64                        `json:"revision"`
	ObservedAt   int64                         `json:"observed_at"`
	Certificates []nodeagent.CertificateStatus `json:"certificates"`
}

func certificateReport(revision uint64, certificates []nodeagent.CertificateStatus) json.RawMessage {
	body, _ := json.Marshal(CertificateReport{revision, time.Now().Unix(), certificates})
	return body
}

func certificatePublished(node model.Node, snapshot Snapshot, inbound model.NodeInbound) bool {
	var options struct {
		TLS struct {
			Enabled  bool   `json:"enabled"`
			Provider string `json:"certificate_provider"`
			Reality  struct {
				Enabled bool `json:"enabled"`
			} `json:"reality"`
		} `json:"tls"`
	}
	if json.Unmarshal(inbound.Options, &options) != nil {
		return false
	}
	if !options.TLS.Enabled || options.TLS.Reality.Enabled {
		return true
	}
	var report CertificateReport
	if json.Unmarshal(node.TLSStatus, &report) != nil || report.Certificates == nil {
		if inbound.ManagedCertificate {
			return false
		}
		// Older agents never supported providers. Do not publish an ACME
		// listener just because the core process reports running.
		return options.TLS.Provider == ""
	}
	if report.Revision != snapshot.Deployment.Revision {
		return false
	}
	for _, status := range report.Certificates {
		if status.Inbound == serverTag(node.Id, inbound.Id) {
			return status.State == "ready" && status.NotAfter > time.Now().Unix()
		}
	}
	return false
}
