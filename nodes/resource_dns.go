package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/net/idna"
	"gorm.io/gorm"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DNSZone struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

var resourceHTTP = &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
var cloudflareResourceURL = "https://api.cloudflare.com/client/v4"
var resourceLookupIP = func(ctx context.Context, name string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, name)
}
var resourceLookupTXT = func(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}
var errDNSRecordMissing = errors.New("dns_record_missing")
var errDNSAddressNotResolved = errors.New("dns_address_not_resolved")
var errDNSLookupFailed = errors.New("dns_lookup_failed")

func canonicalDomain(value string) (string, error) {
	name, err := idna.Lookup.ToASCII(strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), ".")))
	if err != nil || !validPublishedAddress(name) || net.ParseIP(strings.Trim(name, "[]")) != nil || !strings.Contains(name, ".") || strings.Contains(name, "*") {
		return "", errors.New("invalid_node_domain")
	}
	return name, nil
}
func validZoneId(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func profileZone(profile model.NodeDNSProfile, id, domain string) (DNSZone, error) {
	var zones []DNSZone
	if json.Unmarshal(profile.Zones, &zones) != nil {
		return DNSZone{}, errors.New("dns_zones_invalid")
	}
	for _, z := range zones {
		if z.Id == id && (domain == z.Name || strings.HasSuffix(domain, "."+z.Name)) {
			return z, nil
		}
	}
	return DNSZone{}, errors.New("dns_zone_not_allowed")
}
func profileSecret(profile model.NodeDNSProfile) (string, error) {
	value, err := openSecret(profile.Secret)
	if err != nil {
		return "", errors.New("dns_state_key_unavailable")
	}
	var stored struct {
		Id                     uint
		AuthType, Email, Value string
	}
	if json.Unmarshal([]byte(value), &stored) != nil || stored.Id != profile.Id || stored.AuthType != profile.AuthType || stored.Email != profile.Email {
		return "", errors.New("dns_profile_secret_mismatch")
	}
	return stored.Value, nil
}
func cfResource(ctx context.Context, profile model.NodeDNSProfile, method, path string, payload, target interface{}) error {
	secret, err := profileSecret(profile)
	if err != nil {
		return err
	}
	return cfResourceAuth(ctx, profile, secret, method, path, payload, target)
}
func cfResourceAuth(ctx context.Context, profile model.NodeDNSProfile, secret, method, path string, payload, target interface{}) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, cloudflareResourceURL+path, body)
	if err != nil {
		return errors.New("dns_request_invalid")
	}
	if profile.AuthType == "token" {
		request.Header.Set("Authorization", "Bearer "+secret)
	} else if profile.AuthType == "global_key" {
		request.Header.Set("X-Auth-Email", profile.Email)
		request.Header.Set("X-Auth-Key", secret)
	} else {
		return errors.New("dns_auth_type_invalid")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := resourceHTTP.Do(request)
	if err != nil {
		return errors.New("dns_api_unreachable")
	}
	defer response.Body.Close()
	var result cloudflareResourceResult
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result)
	if missingDNSRecordResponse(method, path, response.StatusCode, decodeErr, result) {
		return errDNSRecordMissing
	}
	if decodeErr != nil || !result.Success || response.StatusCode/100 != 2 {
		return errors.New("dns_api_rejected")
	}
	if target != nil && len(result.Result) > 0 {
		return json.Unmarshal(result.Result, target)
	}
	return nil
}

type resourceRecord struct {
	Id, Name, Type, Content, Comment string
	Proxied                          bool
}

func verifyResourceZone(ctx context.Context, p model.NodeDNSProfile, id, domain string) error {
	zone, err := profileZone(p, id, domain)
	if err != nil {
		return err
	}
	var actual DNSZone
	if err := cfResource(ctx, p, "GET", cloudflareZonesPath+id, nil, &actual); err != nil {
		return err
	}
	if actual.Id != zone.Id || actual.Name != zone.Name {
		return errors.New("dns_zone_identity_mismatch")
	}
	return nil
}

func ensureResourceRecord(ctx context.Context, nodeID uint, profile model.NodeDNSProfile, zone, name, kind, content string) (model.NodeDNSRecord, error) {
	db := database.GetDB()
	var record model.NodeDNSRecord
	if err := verifyResourceZone(ctx, profile, zone, strings.TrimPrefix(name, "_acme-challenge.")); err != nil {
		return record, err
	}
	target := dnsRecordTarget{Name: name, Kind: kind, Content: content}
	err := db.Transaction(func(tx *gorm.DB) error { return prepareDNSRecordRow(tx, nodeID, profile.Id, zone, target, &record) })
	if err != nil {
		return record, err
	}
	path := cloudflareZonesPath + zone + "/dns_records"
	var list []resourceRecord
	if err := cfResource(ctx, profile, "GET", path+"?name="+url.QueryEscape(name)+"&per_page=100", nil, &list); err != nil {
		return record, err
	}
	if len(list) >= 100 {
		return record, errors.New("dns_record_inventory_too_large")
	}
	chosen, err := chooseOwnedDNSRecord(list, record, name, kind)
	if err != nil {
		return record, err
	}
	if record.RemoteId != "" && chosen == nil {
		return record, errors.New("dns_owned_record_changed")
	}
	body := map[string]interface{}{"type": kind, "name": name, "content": content, "ttl": 60, "proxied": false, "comment": record.Owner}
	var ack resourceRecord
	if chosen == nil {
		err = cfResource(ctx, profile, "POST", path, body, &ack)
	} else if chosen.Content != content {
		err = cfResource(ctx, profile, "PATCH", path+"/"+chosen.Id, body, &ack)
	} else {
		ack = *chosen
	}
	if err != nil {
		return record, err
	}
	if ack.Id == "" || ack.Name != name || ack.Type != kind || ack.Content != content || ack.Comment != record.Owner || ack.Proxied {
		return record, errors.New("dns_record_ack_mismatch")
	}
	record.RemoteId, record.Content = ack.Id, content
	return record, db.Model(&record).Updates(map[string]interface{}{"remote_id": ack.Id, "content": content, "target_content": content}).Error
}
func removeResourceRecord(ctx context.Context, record model.NodeDNSRecord) error {
	var profile model.NodeDNSProfile
	if err := database.GetDB().First(&profile, record.ProfileId).Error; err != nil {
		return err
	}
	path := cloudflareZonesPath + record.ZoneId + "/dns_records"
	if err := findDNSRecordForRemoval(ctx, profile, &record, path); err != nil {
		return err
	}
	if record.RemoteId != "" {
		path += "/" + record.RemoteId
		var current resourceRecord
		err := cfResource(ctx, profile, "GET", path, nil, &current)
		if err != nil && !errors.Is(err, errDNSRecordMissing) {
			return err
		}
		if err == nil {
			if current.Comment != record.Owner || !strings.EqualFold(current.Name, record.Name) || current.Type != record.Type || (current.Content != record.Content && current.Content != record.TargetContent) {
				return errors.New("dns_record_ownership_changed")
			}
			if err := cfResource(ctx, profile, "DELETE", path, nil, nil); err != nil && !errors.Is(err, errDNSRecordMissing) {
				return err
			}
		}
	}
	return database.GetDB().Model(&record).Update("retired", true).Error
}
func verifyDomainAddress(ctx context.Context, domain, expected string) error {
	ip := net.ParseIP(expected)
	if ip == nil {
		return errors.New("node_expected_ip_required")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addresses, err := resourceLookupIP(lookupCtx, strings.TrimSuffix(domain, ".")+".")
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return errDNSAddressNotResolved
		}
		return errDNSLookupFailed
	}
	if len(addresses) == 0 {
		return errDNSAddressNotResolved
	}
	for _, address := range addresses {
		if !address.IP.Equal(ip) {
			return errors.New("dns_address_mismatch")
		}
	}
	return nil
}

type cloudflareResourceResult struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
	Errors  []struct {
		Code int `json:"code"`
	} `json:"errors"`
}

func missingDNSRecordResponse(method, path string, status int, decodeErr error, result cloudflareResourceResult) bool {
	if status == http.StatusNotFound || status == http.StatusBadRequest {
		parts := strings.Split(path, "/")
		if (method == http.MethodGet || method == http.MethodDelete) && len(parts) == 5 && parts[1] == "zones" && parts[3] == "dns_records" && validZoneId(parts[2]) && validZoneId(parts[4]) {
			// Cloudflare also reports a missing record as HTTP 400 with error 81044.
			if decodeErr == nil && len(result.Errors) == 1 && result.Errors[0].Code == 81044 ||
				status == http.StatusNotFound && errors.Is(decodeErr, io.EOF) {
				return true
			}
		}
	}
	return false
}

type dnsRecordTarget struct{ Name, Kind, Content string }

func prepareDNSRecordRow(tx *gorm.DB, nodeID, profileID uint, zone string, target dnsRecordTarget, record *model.NodeDNSRecord) error {
	name, kind, content := target.Name, target.Kind, target.Content
	var node model.Node
	if err := tx.First(&node, nodeID).Error; err != nil {
		return err
	}
	query := tx.Where("node_id = ? AND profile_id = ? AND zone_id = ? AND name = ? AND type = ? AND retired = ?", nodeID, profileID, zone, name, kind, false)
	if kind == "TXT" {
		query = query.Where("content = ?", content)
	}
	err := query.First(record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		*record = model.NodeDNSRecord{NodeId: nodeID, ProfileId: profileID, ZoneId: zone, Name: name, Type: kind, Content: content, TargetContent: content, Owner: "S-UI " + id.String(), Challenge: kind == "TXT"}
		return tx.Create(record).Error
	}
	record.TargetContent = content
	return tx.Model(record).Update("target_content", content).Error
}

func chooseOwnedDNSRecord(list []resourceRecord, record model.NodeDNSRecord, name, kind string) (*resourceRecord, error) {
	var chosen *resourceRecord
	for _, candidate := range list {
		if !strings.EqualFold(candidate.Name, name) {
			continue
		}
		owned := candidate.Comment == record.Owner && candidate.Type == kind && (candidate.Id == record.RemoteId || record.RemoteId == "") && (candidate.Content == record.Content || candidate.Content == record.TargetContent) && !candidate.Proxied
		if owned {
			if chosen != nil {
				return nil, errors.New("dns_record_duplicate_owner")
			}
			snapshot := candidate
			chosen = &snapshot
			continue
		}
		if kind != "TXT" || candidate.Type == "CNAME" {
			return nil, errors.New("dns_name_occupied")
		}
	}
	return chosen, nil
}

func findDNSRecordForRemoval(ctx context.Context, profile model.NodeDNSProfile, record *model.NodeDNSRecord, path string) error {
	if record.RemoteId == "" {
		var list []resourceRecord
		if err := cfResource(ctx, profile, "GET", path+"?name="+url.QueryEscape(record.Name)+"&per_page=100", nil, &list); err != nil {
			return err
		}
		for _, candidate := range list {
			if candidate.Comment == record.Owner && candidate.Name == record.Name && candidate.Type == record.Type {
				record.RemoteId = candidate.Id
				break
			}
		}
	}
	return nil
}
