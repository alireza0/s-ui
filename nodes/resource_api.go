package nodes

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodecert"
	"gorm.io/gorm"
)

var resourceCertificateRoots *x509.CertPool // nil uses system trust; tests inject fixture roots.

func ResourcesBundle() (map[string]interface{}, error) {
	db := database.GetDB()
	profiles := []model.NodeDNSProfile{}
	domains := []model.NodeDomainBinding{}
	certificates := []model.NodeCertificateBinding{}
	records := []model.NodeDNSRecord{}
	if err := db.Order("id").Find(&profiles).Error; err != nil {
		return nil, err
	}
	for i := range profiles {
		profiles[i].SecretConfigured = profiles[i].Secret != ""
	}
	if err := db.Order("node_id").Find(&domains).Error; err != nil {
		return nil, err
	}
	if err := db.Order("node_id").Find(&certificates).Error; err != nil {
		return nil, err
	}
	if err := db.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"dnsprofiles": profiles, "domains": domains, "certificates": certificates, "dnsrecords": records}, nil
}

func SaveNodeResource(kind, action string, data json.RawMessage, actor string) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var id uint
		var err error
		switch kind {
		case "dnsprofiles":
			id, err = saveDNSProfile(tx, action, data)
		case "domains":
			id, err = saveDomainBinding(tx, action, data)
		case "certificates":
			id, err = saveCertificateBinding(tx, action, data)
		default:
			return errors.New("unknown_node_resource")
		}
		if err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]uint{"id": id})
		return tx.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: actor, Key: "node-" + kind, Action: action, Obj: body}).Error
	})
}
func saveDNSProfile(tx *gorm.DB, action string, data json.RawMessage) (uint, error) {
	var r dnsProfileRequest
	if err := decodeJSON(data, &r, true); err != nil {
		return 0, err
	}
	old, err := dnsProfileForMutation(tx, r.Id, r.ExpectedVersion)
	if err != nil {
		return r.Id, err
	}
	if action == "del" {
		return r.Id, deleteDNSProfile(tx, old, r.Id)
	}
	if action != "save" || strings.TrimSpace(r.Name) == "" || len(r.Zones) == 0 || len(r.Zones) > 64 {
		return r.Id, errors.New("invalid_dns_profile")
	}
	if r.AuthType != "token" && r.AuthType != "global_key" {
		return r.Id, errors.New("invalid_dns_auth_type")
	}
	if r.AuthType == "global_key" {
		if _, err := mail.ParseAddress(r.Email); err != nil {
			return r.Id, errors.New("dns_email_required")
		}
	}
	if err := normalizeDNSZones(r.Zones); err != nil {
		return r.Id, err
	}
	if err := validateRemovedDNSZones(tx, old, r.Zones); err != nil {
		return r.Id, err
	}
	secret := r.Token
	if r.AuthType == "global_key" {
		secret = r.APIKey
	}
	if strings.ContainsAny(secret, "\r\n\x00") || len(secret) > 4096 {
		return r.Id, errors.New("invalid_dns_credentials")
	}
	value := old
	value.Name, value.Provider, value.AuthType, value.Email = r.Name, "cloudflare", r.AuthType, r.Email
	value.Zones, _ = json.Marshal(r.Zones)
	value.Version = old.Version + 1
	if secret == "" && (old.Secret == "" || old.AuthType != value.AuthType || old.Email != value.Email) {
		return r.Id, errors.New("dns_credentials_required")
	}
	if err := tx.Save(&value).Error; err != nil {
		return r.Id, err
	}
	if secret != "" {
		if err := saveDNSProfileSecret(tx, &value, secret); err != nil {
			return value.Id, err
		}
	}
	var ids []uint
	if err := tx.Model(&model.NodeDomainBinding{}).Where("profile_id = ?", value.Id).Pluck("node_id", &ids).Error; err != nil {
		return value.Id, err
	}
	var certificateNodes []uint
	if err := tx.Model(&model.NodeCertificateBinding{}).Where("dns_profile_id = ?", value.Id).Pluck("node_id", &certificateNodes).Error; err != nil {
		return value.Id, err
	}
	return value.Id, database.QueueNodeTasks(tx, append(ids, certificateNodes...), "resources")
}
func resourceNode(tx *gorm.DB, id uint) (model.Node, error) {
	var n model.Node
	if id == 0 {
		return n, errors.New("invalid_node_id")
	}
	if err := tx.First(&n, id).Error; err != nil {
		return n, err
	}
	if !managedNodeModel(n) {
		return n, errors.New("node_resources_require_converted_v2")
	}
	return n, nil
}
func saveDomainBinding(tx *gorm.DB, action string, data json.RawMessage) (uint, error) {
	var r domainBindingRequest
	if err := decodeJSON(data, &r, true); err != nil {
		return 0, err
	}
	node, err := resourceNode(tx, r.NodeId)
	if err != nil {
		return r.NodeId, err
	}
	var old model.NodeDomainBinding
	err = tx.First(&old, whereNodeID, r.NodeId).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return r.NodeId, err
	}
	if old.Generation != r.ExpectedVersion {
		return r.NodeId, errors.New("domain_version_conflict")
	}
	if action != "save" || r.Mode != "manual" && r.Mode != "managed" {
		return r.NodeId, errors.New("invalid_domain_mode")
	}
	value := old
	value.NodeId, value.Mode, value.ProfileId, value.ZoneId, value.AutomaticPrefix = r.NodeId, r.Mode, r.ProfileId, r.ZoneId, r.AutomaticPrefix
	value.CertificateMode, err = domainCertificateMode(value.CertificateMode, r.CertificateMode)
	if err != nil {
		return r.NodeId, err
	}
	if err := configureDomainName(tx, &value, old, r); err != nil {
		return r.NodeId, err
	}
	var same int64
	if err := tx.Model(&model.NodeDomainBinding{}).Where("node_id <> ? AND fqdn = ?", r.NodeId, value.FQDN).Count(&same).Error; err != nil {
		return r.NodeId, err
	}
	if same > 0 {
		return r.NodeId, errors.New("domain_bound_to_another_node")
	}
	value.ExpectedIP, err = expectedDomainIP(tx, node, old, r.IP)
	if err != nil {
		return r.NodeId, err
	}
	if old.Generation != 0 && old.Mode == value.Mode && old.FQDN == value.FQDN && old.ProfileId == value.ProfileId && old.ZoneId == value.ZoneId && old.Prefix == value.Prefix && old.AutomaticPrefix == value.AutomaticPrefix && old.ExpectedIP == value.ExpectedIP && old.CertificateMode == value.CertificateMode {
		return r.NodeId, nil
	}
	value.Generation = old.Generation + 1
	value.State = "pending"
	value.LastError = ""
	if err := tx.Save(&value).Error; err != nil {
		return r.NodeId, err
	}
	return r.NodeId, database.QueueNodeTasks(tx, []uint{r.NodeId}, "resources")
}
func saveCertificateBinding(tx *gorm.DB, action string, data json.RawMessage) (uint, error) {
	var r certificateBindingRequest
	if err := decodeJSON(data, &r, true); err != nil {
		return 0, err
	}
	if _, err := resourceNode(tx, r.NodeId); err != nil {
		return r.NodeId, err
	}
	var domain model.NodeDomainBinding
	if err := tx.First(&domain, whereNodeID, r.NodeId).Error; err != nil {
		return r.NodeId, errors.New("node_domain_required")
	}
	var old model.NodeCertificateBinding
	err := tx.First(&old, whereNodeID, r.NodeId).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return r.NodeId, err
	}
	if old.Generation != r.ExpectedVersion {
		return r.NodeId, errors.New("certificate_version_conflict")
	}
	if action == "renew" {
		return r.NodeId, renewCertificateBinding(tx, old, r.NodeId)
	}
	if action != "save" {
		return r.NodeId, errors.New("invalid_certificate_action")
	}
	if err := normalizeCertificateRequest(&r, domain); err != nil {
		return r.NodeId, err
	}
	value := old
	value.NodeId, value.Mode, value.CA, value.Email, value.DNSProfileId, value.ZoneId = r.NodeId, r.Mode, r.CA, r.Email, r.DNSProfileId, r.ZoneId
	if err := configureCertificateMode(tx, &value, r, domain); err != nil {
		return r.NodeId, err
	}
	value.Generation = old.Generation + 1
	value.State = "pending"
	value.LastError = ""
	if err := tx.Save(&value).Error; err != nil {
		return r.NodeId, err
	}
	return r.NodeId, database.QueueNodeTasks(tx, []uint{r.NodeId}, "resources")
}

type dnsProfileRequest struct {
	Id              uint      `json:"id"`
	Name            string    `json:"name"`
	AuthType        string    `json:"auth_type"`
	Email           string    `json:"email"`
	Zones           []DNSZone `json:"zones"`
	Token           string    `json:"token"`
	APIKey          string    `json:"api_key"`
	ExpectedVersion uint64    `json:"expected_version"`
}

type domainBindingRequest struct {
	NodeId          uint   `json:"node_id"`
	Mode            string `json:"mode"`
	FQDN            string `json:"fqdn"`
	ProfileId       uint   `json:"profile_id"`
	ZoneId          string `json:"zone_id"`
	Prefix          string `json:"prefix"`
	AutomaticPrefix bool   `json:"automatic_prefix"`
	IP              string `json:"ip"`
	ExpectedVersion uint64 `json:"expected_version"`
	CertificateMode string `json:"certificate_mode"`
}

type certificateBindingRequest struct {
	NodeId          uint   `json:"node_id"`
	Mode            string `json:"mode"`
	CA              string `json:"ca"`
	Email           string `json:"email"`
	DNSProfileId    uint   `json:"dns_profile_id"`
	ZoneId          string `json:"zone_id"`
	CertificatePEM  string `json:"certificate"`
	KeyPEM          string `json:"key"`
	CertificatePath string `json:"certificate_path"`
	KeyPath         string `json:"key_path"`
	ExpectedVersion uint64 `json:"expected_version"`
}

func deleteDNSProfile(tx *gorm.DB, old model.NodeDNSProfile, id uint) error {
	if id == 0 {
		return errors.New("invalid_dns_profile_id")
	}
	for _, table := range []struct {
		row   interface{}
		query string
	}{{&model.NodeDomainBinding{}, "profile_id = ?"}, {&model.NodeCertificateBinding{}, "dns_profile_id = ?"}, {&model.NodeDNSRecord{}, "profile_id = ? AND retired = false"}} {
		var count int64
		if err := tx.Model(table.row).Where(table.query, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("dns_profile_in_use")
		}
	}
	return tx.Delete(&old).Error
}

func normalizeDNSZones(zones []DNSZone) error {
	seen := map[string]bool{}
	for i := range zones {
		zone := &zones[i]
		name, err := canonicalDomain(zone.Name)
		if err != nil || !validZoneId(zone.Id) || seen[zone.Id] {
			return errors.New("invalid_dns_zone")
		}
		zone.Name = name
		seen[zone.Id] = true
	}
	return nil
}

func validateUnusedDNSZone(tx *gorm.DB, id uint, zone DNSZone) error {
	for _, reference := range []struct {
		row   interface{}
		query string
	}{
		{&model.NodeDomainBinding{}, "profile_id = ? AND zone_id = ?"},
		{&model.NodeCertificateBinding{}, "dns_profile_id = ? AND zone_id = ?"},
		{&model.NodeDNSRecord{}, "profile_id = ? AND zone_id = ? AND retired = false"},
	} {
		var count int64
		if err := tx.Model(reference.row).Where(reference.query, id, zone.Id).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errors.New("dns_zone_in_use")
		}
	}
	return nil
}

func validateRemovedDNSZones(tx *gorm.DB, old model.NodeDNSProfile, zones []DNSZone) error {
	if old.Id != 0 {
		var previous []DNSZone
		if err := json.Unmarshal(old.Zones, &previous); err != nil {
			return errors.New("dns_profile_zones_invalid")
		}
		for _, zone := range previous {
			unchanged := false
			for _, next := range zones {
				if next.Id == zone.Id && next.Name == zone.Name {
					unchanged = true
				}
			}
			if unchanged {
				continue
			}
			if err := validateUnusedDNSZone(tx, old.Id, zone); err != nil {
				return err
			}
		}
	}
	return nil
}

func saveDNSProfileSecret(tx *gorm.DB, value *model.NodeDNSProfile, secret string) error {
	body, _ := json.Marshal(struct {
		Id                     uint
		AuthType, Email, Value string
	}{value.Id, value.AuthType, value.Email, secret})
	sealed, err := sealSecret(string(body))
	if err != nil {
		return err
	}
	if err := tx.Model(value).Update("secret", sealed).Error; err != nil {
		return err
	}
	return nil
}

func domainCertificateMode(current, requested string) (string, error) {
	if requested != "" {
		current = requested
	}
	if current == "" {
		current = "auto"
	}
	if current != "auto" && current != "provided" && current != "remote_paths" {
		return current, errors.New("invalid_certificate_mode")
	}
	return current, nil
}

func managedDomainPrefix(old model.NodeDomainBinding, r domainBindingRequest) (string, error) {
	prefix := strings.ToLower(strings.TrimSpace(r.Prefix))
	if r.AutomaticPrefix {
		if old.Mode == r.Mode && old.ProfileId == r.ProfileId && old.ZoneId == r.ZoneId && old.AutomaticPrefix {
			prefix = old.Prefix
		} else {
			buf := make([]byte, 5)
			if _, err := rand.Read(buf); err != nil {
				return "", err
			}
			prefix = "n-" + hex.EncodeToString(buf)
		}
	}
	if prefix == "" || !validPublishedAddress(prefix) || strings.ContainsAny(prefix, "[]:") {
		return "", errors.New("invalid_domain_prefix")
	}
	return prefix, nil
}

func configureManagedDomainName(tx *gorm.DB, value *model.NodeDomainBinding, old model.NodeDomainBinding, r domainBindingRequest) error {
	var profile model.NodeDNSProfile
	if err := tx.First(&profile, r.ProfileId).Error; err != nil {
		return err
	}
	var zones []DNSZone
	if json.Unmarshal(profile.Zones, &zones) != nil {
		return errors.New("invalid_dns_profile")
	}
	suffix := ""
	for _, zone := range zones {
		if zone.Id == r.ZoneId {
			suffix = zone.Name
		}
	}
	if suffix == "" {
		return errors.New("dns_zone_not_allowed")
	}
	prefix, err := managedDomainPrefix(old, r)
	if err != nil {
		return err
	}
	value.Prefix = prefix
	fqdn, err := canonicalDomain(prefix + "." + suffix)
	value.FQDN = fqdn
	return err
}

func configureDomainName(tx *gorm.DB, value *model.NodeDomainBinding, old model.NodeDomainBinding, r domainBindingRequest) error {
	if r.Mode == "managed" {
		return configureManagedDomainName(tx, value, old, r)
	}
	value.ProfileId = 0
	value.ZoneId = ""
	value.Prefix = ""
	value.AutomaticPrefix = false
	fqdn, err := canonicalDomain(r.FQDN)
	value.FQDN = fqdn
	return err
}

func renewCertificateBinding(tx *gorm.DB, old model.NodeCertificateBinding, id uint) error {
	if old.Mode != "dns01" && old.Mode != "http01" {
		return errors.New("certificate_has_no_automatic_issuer")
	}
	if err := tx.Model(&old).Updates(map[string]interface{}{"generation": old.Generation + 1, "renew_requested": true}).Error; err != nil {
		return err
	}
	return database.QueueNodeTasks(tx, []uint{id}, "resources")
}

func normalizeCertificateRequest(r *certificateBindingRequest, domain model.NodeDomainBinding) error {
	if r.Mode == "auto" {
		if r.DNSProfileId == 0 && domain.Mode == "managed" {
			r.DNSProfileId, r.ZoneId = domain.ProfileId, domain.ZoneId
		}
		r.Mode = "http01"
		if r.DNSProfileId != 0 {
			r.Mode = "dns01"
		}
	}
	if r.CA == "" {
		r.CA = "letsencrypt"
	}
	if _, err := nodecert.Directory(r.CA); err != nil {
		return err
	}
	if r.Email != "" {
		if _, err := mail.ParseAddress(r.Email); err != nil {
			return errors.New("invalid_acme_email")
		}
	}
	return nil
}

func configureCertificateMode(tx *gorm.DB, value *model.NodeCertificateBinding, r certificateBindingRequest, domain model.NodeDomainBinding) error {
	var err error
	switch r.Mode {
	case "dns01":
		var profile model.NodeDNSProfile
		if err := tx.First(&profile, r.DNSProfileId).Error; err != nil {
			return err
		}
		var zones []DNSZone
		json.Unmarshal(profile.Zones, &zones)
		allowed := false
		for _, zone := range zones {
			if zone.Id == r.ZoneId && (domain.FQDN == zone.Name || strings.HasSuffix(domain.FQDN, "."+zone.Name)) {
				allowed = true
			}
		}
		if !allowed {
			return errors.New("dns_zone_not_allowed")
		}
	case "http01":
		value.DNSProfileId = 0
		value.ZoneId = ""
	case "provided":
		bundle := nodecert.Bundle{Certificate: r.CertificatePEM, Key: r.KeyPEM}
		if _, err := nodecert.Validate(bundle, domain.FQDN, resourceCertificateRoots, time.Now()); err != nil {
			return err
		}
		body, _ := json.Marshal(struct {
			Node   uint
			Domain string
			Bundle nodecert.Bundle
		}{r.NodeId, domain.FQDN, bundle})
		value.Material, err = sealSecret(string(body))
		if err != nil {
			return err
		}
	case "remote_paths":
		if !filepath.IsAbs(r.CertificatePath) || !filepath.IsAbs(r.KeyPath) {
			return errors.New("certificate_paths_must_be_absolute_on_node")
		}
		value.CertificatePath, value.KeyPath = r.CertificatePath, r.KeyPath
	default:
		return errors.New("invalid_certificate_mode")
	}
	return nil
}

func expectedDomainIP(tx *gorm.DB, node model.Node, old model.NodeDomainBinding, requested string) (string, error) {
	if requested == "" {
		requested = old.ExpectedIP
		if requested == "" {
			requested = physicalNodeHost(tx, node)
		}
	}
	ip := net.ParseIP(requested)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return "", errors.New("node_expected_ip_required")
	}
	return ip.String(), nil
}

func dnsProfileForMutation(tx *gorm.DB, id uint, expected uint64) (model.NodeDNSProfile, error) {
	var old model.NodeDNSProfile
	if id != 0 {
		if err := tx.First(&old, id).Error; err != nil {
			return old, err
		}
		if old.Version != expected {
			return old, errors.New("dns_profile_version_conflict")
		}
	}
	return old, nil
}
