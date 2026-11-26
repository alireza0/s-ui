package nodes

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"gorm.io/gorm"
)

const controlRenewBefore = 30 * 24 * time.Hour
const controlRenewRetry = time.Hour

func controlIdentityValid(node model.Node, install model.NodeInstallation, now time.Time) bool {
	block, _ := pem.Decode([]byte(node.ControlCA))
	if block == nil {
		return false
	}
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !ca.IsCA || !ca.NotAfter.After(now.Add(controlRenewBefore)) {
		return false
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if !controlPairValid(node.ControlCertificate, node.ControlKey, roots, now, x509.ExtKeyUsageClientAuth, "") {
		return false
	}
	certificate, err := openSecret(install.ServerCertificate)
	if err != nil {
		return false
	}
	key, err := openSecret(install.ServerKey)
	return err == nil && controlPairValid(certificate, key, roots, now, x509.ExtKeyUsageServerAuth, "localhost")
}

func controlPairValid(certificate, key string, roots *x509.CertPool, now time.Time, usage x509.ExtKeyUsage, hostname string) bool {
	pair, err := tls.X509KeyPair([]byte(certificate), []byte(key))
	if err != nil || len(pair.Certificate) == 0 {
		return false
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil || !leaf.NotAfter.After(now.Add(controlRenewBefore)) {
		return false
	}
	_, err = leaf.Verify(x509.VerifyOptions{Roots: roots, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{usage}, DNSName: hostname})
	return err == nil
}

// The existing durable installer pins SSH, replaces both mTLS identities and
// preserves client material. Its sealed plan also survives a controller restart.
func queueControlRenewal(node model.Node, install model.NodeInstallation, now time.Time) (model.NodeJob, bool, error) {
	if _, err := openSecret(install.PrivateKey); err != nil {
		return model.NodeJob{}, false, err
	}
	var previous model.NodeJob
	err := database.GetDB().Where("node_id = ? AND request_key LIKE ? AND updated > ?", node.Id, "control-renew-%", now.Add(-controlRenewRetry).Unix()).Order("updated DESC").First(&previous).Error
	if err == nil {
		return previous, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.NodeJob{}, false, err
	}
	sum := sha256.Sum256([]byte(node.ControlCA + node.ControlCertificate + install.ServerCertificate))
	version := node.SettingsVersion
	request := ProvisionRequest{
		ModelVersion: 2, NodeId: node.Id, ExpectedVersion: &version,
		Name: node.Name, IP: install.IP, SSHPort: install.SSHPort, SSHUser: install.SSHUser, Action: "update",
		RequestKey: fmt.Sprintf("control-renew-%d-%x-%d-%d", node.Id, sum[:8], version, now.Unix()/int64(controlRenewRetry/time.Second)),
	}
	return createLiveProvision(request, "ControlPKIRenewal")
}

func renewControlIdentities(ctx context.Context, now time.Time) {
	db := database.GetDB()
	var installations []model.NodeInstallation
	if db.Where("owner_id <> '' AND private_key <> '' AND ready = ?", true).Order("node_id").Find(&installations).Error != nil {
		return
	}
	for _, install := range installations {
		if ctx.Err() != nil {
			return
		}
		var node model.Node
		if db.First(&node, install.NodeId).Error != nil || !managedNodeModel(node) || node.ControlEpoch == "" || jobRunning(node.Id) || controlIdentityValid(node, install, now) {
			continue
		}
		job, created, err := queueControlRenewal(node, install, now)
		if err == nil && created {
			provisionJobs.Store(node.Id, true)
			go runLiveProvision(job, "")
		}
	}
}

func controlRenewalLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		renewControlIdentities(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
