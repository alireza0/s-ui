package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/alireza0/s-ui/nodecert"
	"github.com/mholt/acmez/v3"
	"github.com/mholt/acmez/v3/acme"
	"gorm.io/gorm"
)

var resourceIssueDNS = func(ctx context.Context, ca, email, key, domain string, solver acmez.Solver) (nodecert.Bundle, error) {
	return nodecert.Issue(ctx, ca, email, key, domain, "dns-01", solver)
}

type resourceDNSSolver struct {
	nodeID  uint
	profile model.NodeDNSProfile
	zone    string
	records map[string]model.NodeDNSRecord
}

func (s *resourceDNSSolver) Present(ctx context.Context, c acme.Challenge) error {
	record, err := ensureResourceRecord(ctx, s.nodeID, s.profile, s.zone, c.DNS01TXTRecordName(), "TXT", c.DNS01KeyAuthorization())
	if record.Id != 0 {
		if s.records == nil {
			s.records = map[string]model.NodeDNSRecord{}
		}
		s.records[c.Token] = record
	}
	return err
}
func (s *resourceDNSSolver) Wait(ctx context.Context, c acme.Challenge) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		values, err := resourceLookupTXT(ctx, c.DNS01TXTRecordName())
		if err == nil {
			for _, value := range values {
				if value == c.DNS01KeyAuthorization() {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (s *resourceDNSSolver) CleanUp(ctx context.Context, c acme.Challenge) error {
	record, ok := s.records[c.Token]
	if !ok {
		return nil
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	return removeResourceRecord(cleanup, record)
}

func openCertificateVersion(version model.NodeCertificateVersion) (nodecert.Bundle, error) {
	plain, err := openSecret(version.Bundle)
	if err != nil {
		return nodecert.Bundle{}, errors.New("certificate_state_key_unavailable")
	}
	var stored struct {
		Node    uint
		Version uint64
		Bundle  nodecert.Bundle
	}
	if json.Unmarshal([]byte(plain), &stored) != nil || stored.Node != version.NodeId || stored.Version != version.Version {
		return nodecert.Bundle{}, errors.New("certificate_material_identity_mismatch")
	}
	return stored.Bundle, nil
}
func persistCertificateVersion(nodeID uint, generation uint64, domain model.NodeDomainBinding, bundle nodecert.Bundle) (model.NodeCertificateVersion, error) {
	var result model.NodeCertificateVersion
	meta, err := nodecert.Validate(bundle, domain.FQDN, resourceCertificateRoots, time.Now())
	if err != nil {
		return result, err
	}
	err = database.GetDB().Transaction(func(tx *gorm.DB) error {
		var binding model.NodeCertificateBinding
		var current model.NodeDomainBinding
		if err := tx.First(&binding, whereNodeID, nodeID).Error; err != nil {
			return err
		}
		if err := tx.First(&current, whereNodeID, nodeID).Error; err != nil {
			return err
		}
		if binding.Generation != generation || current.Generation != domain.Generation {
			return errors.New("certificate_target_changed")
		}
		if binding.CurrentVersion > 0 {
			var previous model.NodeCertificateVersion
			if err := tx.First(&previous, whereNodeVersion, nodeID, binding.CurrentVersion).Error; err != nil {
				return err
			}
			var old nodecert.Metadata
			json.Unmarshal(previous.Metadata, &old)
			if old.Digest == meta.Digest && previous.DomainGeneration == domain.Generation {
				result = previous
				return nil
			}
		}
		version := binding.CurrentVersion + 1
		plain, _ := json.Marshal(struct {
			Node    uint
			Version uint64
			Bundle  nodecert.Bundle
		}{nodeID, version, bundle})
		encrypted, err := sealSecret(string(plain))
		if err != nil {
			return err
		}
		metadata, _ := json.Marshal(meta)
		result = model.NodeCertificateVersion{NodeId: nodeID, Version: version, DomainGeneration: domain.Generation, Bundle: encrypted, Metadata: metadata, Created: time.Now().Unix()}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return tx.Model(&binding).Updates(map[string]interface{}{"current_version": version, "renew_requested": false}).Error
	})
	return result, err
}

func prepareCertificateRequest(node model.Node, domain model.NodeDomainBinding, binding model.NodeCertificateBinding, remote *nodeagent.CertificateResourceStatus) (nodeagent.CertificateRequest, error) {
	request := nodeagent.CertificateRequest{NodeID: uint64(node.Id), Epoch: node.ControlEpoch, Domain: domain.FQDN, ExpectedIP: domain.ExpectedIP, CA: binding.CA, Email: binding.Email, BindingGeneration: binding.Generation, DomainGeneration: domain.Generation}
	switch binding.Mode {
	case "dns01", "provided":
		var version model.NodeCertificateVersion
		if err := database.GetDB().First(&version, whereNodeVersion, node.Id, binding.CurrentVersion).Error; err != nil {
			return request, err
		}
		bundle, err := openCertificateVersion(version)
		if err != nil {
			return request, err
		}
		if _, err := nodecert.Validate(bundle, domain.FQDN, resourceCertificateRoots, time.Now()); err != nil {
			return request, err
		}
		request.Method, request.Bundle = "install", bundle
	case "http01":
		request.Method = "http01"
	case "remote_paths":
		request.Method, request.CertificatePath, request.KeyPath = "remote_paths", binding.CertificatePath, binding.KeyPath
	default:
		return request, errors.New("certificate_mode_invalid")
	}
	body, _ := json.Marshal(request)
	fingerprint := versionDigest(body)
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return prepareCertificateTransaction(tx, node, domain, binding, remote, fingerprint, &request)
	})
	return request, err
}
func reconcileCertificate(ctx context.Context, node model.Node, domain model.NodeDomainBinding) error {
	db := database.GetDB()
	var binding model.NodeCertificateBinding
	err := db.First(&binding, whereNodeID, node.Id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		prepared, prepareErr := prepareAutomaticCertificate(db, node, domain, &binding)
		if prepareErr != nil || !prepared {
			return prepareErr
		}
	} else if err != nil {
		return err
	}
	if binding.Mode == "http01" {
		if err := ensureCertificateFirewall(ctx, db, node, binding, true); err != nil {
			return err
		}
	}
	if binding.Mode == "dns01" {
		if err := prepareDNSCertificate(ctx, db, node, domain, &binding); err != nil {
			return err
		}
	} else if binding.Mode == "provided" {
		if err := prepareProvidedCertificate(node, domain, binding); err != nil {
			return err
		}
	}
	return deliverResourceCertificate(ctx, db, node, domain, binding)
}

func acknowledgeCertificate(node model.Node, binding model.NodeCertificateBinding, request nodeagent.CertificateRequest, meta nodecert.Metadata, lastError string) error {
	db := database.GetDB()
	body, _ := json.Marshal(meta)
	return db.Transaction(func(tx *gorm.DB) error {
		var liveTarget model.Node
		if err := tx.First(&liveTarget, node.Id).Error; err != nil {
			return err
		}
		if liveTarget.ManagementGeneration != node.ManagementGeneration || liveTarget.ControlEpoch != node.ControlEpoch {
			return errors.New("management_target_changed")
		}
		var current model.NodeCertificateBinding
		if err := tx.First(&current, whereNodeID, node.Id).Error; err != nil {
			return err
		}
		if current.Generation != binding.Generation || current.DesiredRevision != request.Revision {
			return errors.New("certificate_target_changed")
		}
		changed := string(current.Metadata) != string(body) || current.AppliedRevision != request.Revision || current.State != "installed"
		if err := tx.Model(&current).Updates(map[string]interface{}{"applied_revision": request.Revision, "metadata": json.RawMessage(body), "state": "installed", "last_error": lastError, "renew_requested": false}).Error; err != nil {
			return err
		}
		if changed {
			var target model.Node
			if err := tx.First(&target, node.Id).Error; err != nil {
				return err
			}
			return queueNodeStructure(tx, &target)
		}
		return nil
	})
}

func resourceTLSContext(tx *gorm.DB, node model.Node, ctx *ResolveContext) error {
	var domain model.NodeDomainBinding
	err := tx.First(&domain, whereNodeID, node.Id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if domain.State != "ready" || domain.AppliedFQDN != domain.FQDN {
		delete(ctx.Parameters, "primary_domain")
		return nil
	}
	ctx.Parameters["primary_domain"], _ = json.Marshal(domain.AppliedFQDN)
	var certificate model.NodeCertificateBinding
	err = tx.First(&certificate, whereNodeID, node.Id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if certificate.State != "installed" {
		// Omit an undelivered resource. Only entries that reference it must wait;
		// unrelated REALITY listeners do not depend on this certificate.
		return nil
	}
	var meta nodecert.Metadata
	if json.Unmarshal(certificate.Metadata, &meta) != nil || meta.Domain != domain.FQDN || meta.NotAfter <= time.Now().Unix() {
		return nil
	}
	certPath, keyPath := meta.CertificatePath, meta.KeyPath
	if certificate.Mode == "http01" || certificate.Mode == "remote_paths" {
		certPath, keyPath = "managed:primary", "managed:primary"
	}
	value := map[string]interface{}{"enabled": true, "server_name": domain.AppliedFQDN, "certificate_path": certPath, "key_path": keyPath}
	ctx.Certificates = map[string]json.RawMessage{"primary": nil}
	ctx.Certificates["primary"], _ = json.Marshal(value)
	return nil
}

func safeResourceError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "node_resource_timed_out"
	}
	value := err.Error()
	if len(value) > 100 || strings.ContainsAny(value, " \n\r\t:/{") {
		return "node_resource_failed"
	}
	return value
}

func validateRemoteCertificateRevision(current model.NodeCertificateBinding, remote *nodeagent.CertificateResourceStatus) error {
	if remote != nil && remote.Revision > current.DesiredRevision {
		return errors.New("certificate_state_rollback_detected")
	}
	if current.DesiredRevision > 0 && remote != nil && remote.Revision == current.DesiredRevision {
		plain, err := openSecret(current.DesiredRequest)
		if err != nil {
			return errors.New("certificate_state_key_unavailable")
		}
		var planned nodeagent.CertificateRequest
		if json.Unmarshal([]byte(plain), &planned) != nil || planned.Digest() != remote.Digest {
			return errors.New("certificate_state_conflict_detected")
		}
	}
	return nil
}

func prepareCertificateTransaction(tx *gorm.DB, node model.Node, domain model.NodeDomainBinding, binding model.NodeCertificateBinding, remote *nodeagent.CertificateResourceStatus, fingerprint string, request *nodeagent.CertificateRequest) error {
	var current model.NodeCertificateBinding
	if err := tx.First(&current, whereNodeID, node.Id).Error; err != nil {
		return err
	}
	var target model.NodeDomainBinding
	if err := tx.First(&target, whereNodeID, node.Id).Error; err != nil {
		return err
	}
	if current.Generation != binding.Generation || target.Generation != domain.Generation {
		return errors.New("certificate_target_changed")
	}
	restoring, err := restoredCertificateCheckpoint(tx, node, &current, remote)
	if err != nil {
		return err
	}
	if restoring || current.DesiredRevision == 0 || current.DesiredFingerprint != fingerprint {
		current.DesiredRevision++
		request.Revision = current.DesiredRevision
		if err := storeCertificateRequest(tx, current, *request, fingerprint); err != nil {
			return err
		}
	} else {
		plain, err := openSecret(current.DesiredRequest)
		if err != nil {
			return errors.New("certificate_state_key_unavailable")
		}
		if json.Unmarshal([]byte(plain), request) != nil {
			return errors.New("certificate_request_corrupt")
		}
	}
	if restoring {
		return tx.Model(&model.Node{}).Where("id = ?", node.Id).Update("restore_certificate_pending", false).Error
	}
	return nil
}

func storeCertificateRequest(tx *gorm.DB, current model.NodeCertificateBinding, request nodeagent.CertificateRequest, fingerprint string) error {
	encoded, _ := json.Marshal(request)
	sealed, err := sealSecret(string(encoded))
	if err != nil {
		return err
	}
	return tx.Model(&current).Updates(map[string]interface{}{"desired_revision": request.Revision, "desired_fingerprint": fingerprint, "desired_request": sealed}).Error
}

func liveResourceNeeds(db *gorm.DB, node model.Node) (certificate, domain bool, err error) {
	spec, err := liveVersion(db, node)
	if err != nil {
		return false, false, err
	}
	for _, entry := range spec.Entries {
		if !entry.Enable {
			continue
		}
		cert, primary, err := entryResourceNeeds(db, entry)
		if err != nil {
			return false, false, err
		}
		certificate = certificate || cert
		domain = domain || primary
	}
	return certificate, domain, nil
}

func prepareAutomaticCertificate(db *gorm.DB, node model.Node, domain model.NodeDomainBinding, binding *model.NodeCertificateBinding) (bool, error) {
	if domain.CertificateMode != "auto" || node.TemplateTargetVersion == 0 {
		return false, nil
	}
	needed, _, err := liveResourceNeeds(db, node)
	if err != nil {
		return false, err
	}
	if !needed {
		return false, nil
	}
	request, _ := json.Marshal(map[string]interface{}{"node_id": node.Id, "mode": "auto"})
	if err := db.Transaction(func(tx *gorm.DB) error { _, err := saveCertificateBinding(tx, "save", request); return err }); err != nil {
		return false, err
	}
	if err := db.First(binding, whereNodeID, node.Id).Error; err != nil {
		return false, err
	}
	return true, nil
}

func prepareDNSCertificate(ctx context.Context, db *gorm.DB, node model.Node, domain model.NodeDomainBinding, binding *model.NodeCertificateBinding) error {
	var version model.NodeCertificateVersion
	var meta nodecert.Metadata
	if binding.CurrentVersion > 0 {
		if err := db.First(&version, whereNodeVersion, node.Id, binding.CurrentVersion).Error; err != nil {
			return err
		}
		json.Unmarshal(version.Metadata, &meta)
	}
	if binding.CurrentVersion == 0 || meta.Domain != domain.FQDN || nodecert.NeedsRenewal(meta, time.Now()) || binding.RenewRequested {
		if err := ensureResourceAccountKey(db, node.Id, binding); err != nil {
			return err
		}
		if err := renewResourceDNSCertificate(ctx, db, node, domain, *binding); err != nil {
			return err
		}
	}
	return nil
}

func ensureResourceAccountKey(db *gorm.DB, nodeID uint, binding *model.NodeCertificateBinding) error {
	if binding.AccountKey == "" {
		key, err := nodecert.NewAccountKey()
		if err != nil {
			return err
		}
		body, _ := json.Marshal(struct {
			Node uint
			Key  string
		}{nodeID, key})
		sealed, err := sealSecret(string(body))
		if err != nil {
			return err
		}
		if err := db.Model(&model.NodeCertificateBinding{}).Where("node_id = ? AND generation = ?", nodeID, binding.Generation).Update("account_key", sealed).Error; err != nil {
			return err
		}
		binding.AccountKey = sealed
	}
	return nil
}

func renewResourceDNSCertificate(ctx context.Context, db *gorm.DB, node model.Node, domain model.NodeDomainBinding, binding model.NodeCertificateBinding) error {
	plain, err := openSecret(binding.AccountKey)
	if err != nil {
		return errors.New("certificate_state_key_unavailable")
	}
	var account struct {
		Node uint
		Key  string
	}
	if json.Unmarshal([]byte(plain), &account) != nil || account.Node != node.Id {
		return errors.New("certificate_account_identity_mismatch")
	}
	var profile model.NodeDNSProfile
	if err := db.First(&profile, binding.DNSProfileId).Error; err != nil {
		return err
	}
	if err := verifyResourceZone(ctx, profile, binding.ZoneId, domain.FQDN); err != nil {
		return err
	}
	solver := &resourceDNSSolver{nodeID: node.Id, profile: profile, zone: binding.ZoneId}
	bundle, err := resourceIssueDNS(ctx, binding.CA, binding.Email, account.Key, domain.FQDN, solver)
	if err != nil {
		return err
	}
	if _, err := persistCertificateVersion(node.Id, binding.Generation, domain, bundle); err != nil {
		return err
	}
	return nil
}

func prepareProvidedCertificate(node model.Node, domain model.NodeDomainBinding, binding model.NodeCertificateBinding) error {
	plain, err := openSecret(binding.Material)
	if err != nil {
		return errors.New("certificate_state_key_unavailable")
	}
	var stored struct {
		Node   uint
		Domain string
		Bundle nodecert.Bundle
	}
	if json.Unmarshal([]byte(plain), &stored) != nil || stored.Node != node.Id || stored.Domain != domain.FQDN {
		return errors.New("certificate_material_invalid")
	}
	if _, err := persistCertificateVersion(node.Id, binding.Generation, domain, stored.Bundle); err != nil {
		return err
	}
	return nil
}

func validateCertificateAcknowledgement(domain model.NodeDomainBinding, request nodeagent.CertificateRequest, observed *nodeagent.CertificateResourceStatus) (nodecert.Metadata, error) {
	if observed == nil || observed.Revision != request.Revision || observed.Digest != request.Digest() {
		return nodecert.Metadata{}, errors.New("certificate_ack_mismatch")
	}
	if observed.State != "ready" || observed.AppliedRevision != request.Revision {
		if observed.LastError != "" {
			return nodecert.Metadata{}, errors.New("agent_" + observed.LastError)
		}
		return nodecert.Metadata{}, errors.New("certificate_pending")
	}
	meta := observed.Metadata
	if meta.Domain != domain.FQDN || meta.NotAfter <= time.Now().Unix() || len(meta.Digest) != 64 || meta.CertificatePath != "certificates/managed/"+meta.Digest+"/cert.pem" || meta.KeyPath != "certificates/managed/"+meta.Digest+"/key.pem" {
		return nodecert.Metadata{}, errors.New("certificate_artifact_ack_invalid")
	}
	if request.Method == "install" && meta.Digest != nodecert.Digest(request.Bundle) {
		return nodecert.Metadata{}, errors.New("certificate_artifact_digest_mismatch")
	}
	return meta, nil
}

func deliverResourceCertificate(ctx context.Context, db *gorm.DB, node model.Node, domain model.NodeDomainBinding, binding model.NodeCertificateBinding) error {
	if err := db.First(&binding, whereNodeID, node.Id).Error; err != nil {
		return err
	}
	control, closeControl, err := controlClient(ctx, node)
	if err != nil {
		return err
	}
	defer closeControl()
	status, err := getStatusV2(ctx, control, node)
	if err != nil {
		return err
	}
	if status.CertificateResource == nil {
		return errors.New("agent_certificate_upgrade_required")
	}
	request, err := prepareCertificateRequest(node, domain, binding, status.CertificateResource)
	if err != nil {
		return err
	}
	if status.CertificateResource.Revision != request.Revision || status.CertificateResource.Digest != request.Digest() || status.CertificateResource.State == "failed" {
		status, err = putV2(ctx, control, node, "/v2/certificate", request)
		if err != nil {
			return err
		}
	}
	observed := status.CertificateResource
	meta, err := validateCertificateAcknowledgement(domain, request, observed)
	if err != nil {
		return err
	}
	if err := acknowledgeCertificate(node, binding, request, meta, observed.LastError); err != nil {
		return err
	}
	if binding.Mode != "http01" {
		return ensureCertificateFirewall(ctx, db, node, binding, false)
	}
	return nil
}

func entryResourceNeeds(db *gorm.DB, entry SpecEntry) (certificate, domain bool, err error) {
	if entry.TLSProfileID > 0 {
		if err := applyTLSProfile(db, &entry); err != nil {
			return false, false, err
		}
	}
	for _, source := range entry.Sources {
		if source.Kind == "node_certificate" {
			certificate = true
		}
		if source.Kind == "node_parameter" && source.Name == "primary_domain" {
			domain = true
		}
	}
	return certificate, domain, nil
}
