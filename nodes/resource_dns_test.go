package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

const fixtureZone = "00000000000000000000000000000001"

type cloudFixture struct {
	mu            sync.Mutex
	records       map[string]resourceRecord
	writes, calls int
	failCreateAck bool
	missingStatus int
	rejectReads   bool
	headers       []http.Header
}

func (f *cloudFixture) Writes() int { f.mu.Lock(); defer f.mu.Unlock(); return f.writes }
func (f *cloudFixture) Calls() int  { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }
func (f *cloudFixture) Headers() []http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]http.Header(nil), f.headers...)
}
func (f *cloudFixture) Put(record resourceRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records[record.Id] = record
}
func newCloudFixture(t *testing.T) *cloudFixture {
	t.Helper()
	f := &cloudFixture{records: map[string]resourceRecord{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.handleRequest(t, w, r)
	}))
	t.Cleanup(server.Close)
	oldURL, oldHTTP := cloudflareResourceURL, resourceHTTP
	cloudflareResourceURL = server.URL
	client := server.Client()
	client.CheckRedirect = oldHTTP.CheckRedirect
	resourceHTTP = client
	t.Cleanup(func() { cloudflareResourceURL, resourceHTTP = oldURL, oldHTTP })
	return f
}
func saveResourceFixture(t *testing.T, kind string, value interface{}) {
	t.Helper()
	body, _ := json.Marshal(value)
	requireNodeTestSuccess(t, SaveNodeResource(kind, "save", body, "test"))
}
func profileFixture(t *testing.T, auth string) model.NodeDNSProfile {
	t.Helper()
	request := map[string]interface{}{"name": auth, "auth_type": auth, "zones": []DNSZone{{Id: fixtureZone, Name: "example.com"}}, "token": "fixture-cloudflare-token"}
	if auth == "global_key" {
		request["email"], request["api_key"] = "admin@example.com", "fixture-global-key"
	}
	saveResourceFixture(t, "dnsprofiles", request)
	var profile model.NodeDNSProfile
	requireNodeTestSuccess(t, database.GetDB().Where("name = ?", auth).First(&profile).Error)
	return profile
}

func TestResourceDNSOwnershipAckLossAndCredentialModes(t *testing.T) {
	f := liveSetup(t, 1)
	cloud := newCloudFixture(t)
	profile := profileFixture(t, "token")
	cloud.failCreateAck = true
	if _, err := ensureResourceRecord(context.Background(), 1, profile, fixtureZone, "own.example.com", "A", "192.0.2.1"); err == nil {
		t.Fatal("lost provider ACK accepted")
	}
	record, err := ensureResourceRecord(context.Background(), 1, profile, fixtureZone, "own.example.com", "A", "192.0.2.1")
	requireNodeTestSuccess(t, err)
	if cloud.Writes() != 1 || record.RemoteId == "" {
		t.Fatal("lost ACK duplicated DNS record")
	}
	if _, err := ensureResourceRecord(context.Background(), 1, profile, fixtureZone, "own.example.com", "A", "192.0.2.2"); err != nil {
		t.Fatal(err)
	}
	if cloud.Writes() != 2 {
		t.Fatal("owned record not updated")
	}
	cloud.Put(resourceRecord{Id: record.RemoteId, Name: record.Name, Type: "A", Content: "192.0.2.99", Comment: "foreign"})
	if err := removeResourceRecord(context.Background(), record); err == nil || cloud.Writes() != 2 {
		t.Fatal("foreign record deleted")
	}
	global := profileFixture(t, "global_key")
	if _, err := ensureResourceRecord(context.Background(), 1, global, fixtureZone, "other.example.com", "A", "192.0.2.3"); err != nil {
		t.Fatal(err)
	}
	last := cloud.Headers()[len(cloud.Headers())-1]
	if last.Get("X-Auth-Email") != "admin@example.com" || last.Get("X-Auth-Key") != "fixture-global-key" || last.Get("Authorization") != "" {
		t.Fatal("global key authentication incorrect")
	}
	if cloud.Headers()[0].Get("Authorization") != "Bearer fixture-cloudflare-token" {
		t.Fatal("token auth incorrect")
	}
	public, err := ResourcesBundle()
	requireNodeTestSuccess(t, err)
	body, _ := json.Marshal(public)
	if strings.Contains(string(body), "fixture-cloudflare-token") || strings.Contains(string(body), "fixture-global-key") {
		t.Fatal("DNS credentials leaked")
	}
	f.db.Model(&profile).Update("secret", global.Secret)
	if _, err := profileSecret(profile); err == nil {
		t.Fatal("swapped profile secret accepted")
	}
}

func TestResourceManagedPrefixStableAndForeignNameNeverOverwritten(t *testing.T) {
	f := liveSetup(t, 1)
	cloud := newCloudFixture(t)
	profile := profileFixture(t, "token")
	oldLookup := resourceLookupIP
	resourceLookupIP = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	t.Cleanup(func() { resourceLookupIP = oldLookup })
	request := map[string]interface{}{"node_id": 1, "mode": "managed", "profile_id": profile.Id, "zone_id": fixtureZone, "automatic_prefix": true, "ip": "127.0.0.1", "certificate_mode": "provided"}
	saveResourceFixture(t, "domains", request)
	var first model.NodeDomainBinding
	f.db.First(&first, "node_id = ?", 1)
	requireNodeTestSuccess(t, ReconcileResources(context.Background(), 1))
	request["expected_version"] = first.Generation
	saveResourceFixture(t, "domains", request)
	var second model.NodeDomainBinding
	f.db.First(&second, "node_id = ?", 1)
	if second.FQDN != first.FQDN || second.Prefix == "" || second.Generation != first.Generation || second.State != "ready" {
		t.Fatal("repeat regenerated domain prefix")
	}
	requireNodeTestSuccess(t, ReconcileResources(context.Background(), 1))
	if cloud.Writes() != 1 {
		t.Fatal("repeat duplicated record")
	}
	request["expected_version"], request["automatic_prefix"], request["prefix"] = second.Generation, false, "foreign"
	cloud.Put(resourceRecord{Id: "foreign-record", Name: "foreign.example.com", Type: "A", Content: "192.0.2.9", Comment: "another owner"})
	saveResourceFixture(t, "domains", request)
	if err := ReconcileResources(context.Background(), 1); err == nil || err.Error() != "dns_name_occupied" {
		t.Fatal("foreign name accepted", err)
	}
	if cloud.Writes() != 1 {
		t.Fatal("foreign record mutated")
	}
}

func TestResourceUsedZoneCannotBeRemovedAndOmittedIPIsPreserved(t *testing.T) {
	f := liveSetup(t, 1)
	profile := profileFixture(t, "token")
	request := map[string]interface{}{"node_id": 1, "mode": "managed", "profile_id": profile.Id, "zone_id": fixtureZone, "prefix": "stable", "ip": "192.0.2.77", "certificate_mode": "provided"}
	saveResourceFixture(t, "domains", request)
	var before model.NodeDomainBinding
	f.db.First(&before, "node_id = ?", 1)
	delete(request, "ip")
	request["expected_version"] = before.Generation
	saveResourceFixture(t, "domains", request)
	var after model.NodeDomainBinding
	f.db.First(&after, "node_id = ?", 1)
	if after.ExpectedIP != before.ExpectedIP || after.Generation != before.Generation {
		t.Fatal("omitted IP changed desired domain")
	}
	body, _ := json.Marshal(map[string]interface{}{"id": profile.Id, "name": "changed", "auth_type": "token", "expected_version": profile.Version, "zones": []DNSZone{{Id: "00000000000000000000000000000002", Name: "other.example"}}})
	if err := SaveNodeResource("dnsprofiles", "save", body, "test"); err == nil || err.Error() != "dns_zone_in_use" {
		t.Fatal("in-use zone removed", err)
	}
}

func TestResourceManualDNSRejectsWrongAAAAWithoutProviderWrites(t *testing.T) {
	f := liveSetup(t, 1)
	cloud := newCloudFixture(t)
	oldLookup := resourceLookupIP
	wrong := true
	resourceLookupIP = func(context.Context, string) ([]net.IPAddr, error) {
		result := []net.IPAddr{{IP: net.ParseIP("192.0.2.1")}}
		if wrong {
			result = append(result, net.IPAddr{IP: net.ParseIP("2001:db8::9")})
		}
		return result, nil
	}
	t.Cleanup(func() { resourceLookupIP = oldLookup })
	saveResourceFixture(t, "domains", map[string]interface{}{"node_id": 1, "mode": "manual", "fqdn": "manual.example.com", "ip": "192.0.2.1", "certificate_mode": "provided"})
	if err := ReconcileResources(context.Background(), 1); err == nil || err.Error() != "dns_address_mismatch" {
		t.Fatal("bad AAAA ignored", err)
	}
	wrong = false
	requireNodeTestSuccess(t, RetryNodeResources(1, "test"))
	requireNodeTestSuccess(t, ReconcileResources(context.Background(), 1))
	if cloud.Calls() != 0 {
		t.Fatal("manual DNS contacted provider")
	}
	var domain model.NodeDomainBinding
	f.db.First(&domain, "node_id = ?", 1)
	if domain.State != "ready" {
		t.Fatal("correct manual DNS not ready")
	}
}

func TestResourceDNSRecheckPreservesRecentlyVerifiedDomain(t *testing.T) {
	for _, tt := range []dnsRecheckCase{
		{name: "temporary lookup error", lookupErr: &net.DNSError{IsTimeout: true}, code: "dns_lookup_failed", ready: true},
		{name: "temporary missing answer", noAnswer: true, code: "dns_address_not_resolved", ready: true},
		{name: "persistent lookup error", lookupErr: &net.DNSError{IsTimeout: true}, expired: true, code: "dns_lookup_failed"},
		{name: "incorrect address", wrong: true, code: "dns_address_mismatch"},
		{name: "unverified new domain", lookupErr: &net.DNSError{IsNotFound: true}, changed: true, code: "dns_address_not_resolved"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checkDNSRecheck(t, tt)
		})
	}
}

type dnsRecheckCase struct {
	name      string
	lookupErr error
	noAnswer  bool
	wrong     bool
	expired   bool
	changed   bool
	code      string
	ready     bool
}

func checkDNSRecheck(t *testing.T, tt dnsRecheckCase) {
	t.Helper()
	f := liveSetup(t, 1)
	oldLookup := resourceLookupIP
	t.Cleanup(func() { resourceLookupIP = oldLookup })
	request := map[string]interface{}{"node_id": 1, "mode": "manual", "fqdn": "manual.example.com", "ip": "192.0.2.1", "certificate_mode": "provided"}
	lookupName := ""
	resourceLookupIP = func(_ context.Context, name string) ([]net.IPAddr, error) {
		lookupName = name
		return []net.IPAddr{{IP: net.ParseIP("192.0.2.1")}}, nil
	}
	saveResourceFixture(t, "domains", request)
	if err := ReconcileResources(t.Context(), 1); err != nil || lookupName != "manual.example.com." {
		t.Fatalf("expected an absolute DNS lookup and a ready domain: %v %q", err, lookupName)
	}
	var before model.NodeDomainBinding
	requireNodeTestSuccess(t, f.db.First(&before, 1).Error)
	if tt.expired {
		requireNodeTestSuccess(t, f.db.Model(&before).Update("checked_at", time.Now().Add(-2*dnsRecheckGrace).Unix()).Error)
	}
	if tt.changed {
		request["expected_version"] = before.Generation
		request["fqdn"] = "replacement.example.com"
		saveResourceFixture(t, "domains", request)
	}
	resourceLookupIP = func(context.Context, string) ([]net.IPAddr, error) {
		if tt.wrong {
			return []net.IPAddr{{IP: net.ParseIP("192.0.2.2")}}, nil
		}
		return nil, tt.lookupErr
	}
	if err := ReconcileResources(t.Context(), 1); err == nil || err.Error() != tt.code {
		t.Fatalf("unexpected DNS check result: %v", err)
	}
	var after model.NodeDomainBinding
	requireNodeTestSuccess(t, f.db.First(&after, 1).Error)
	assertDNSRecheckState(t, tt, before, after)
}

func assertDNSRecheckState(t *testing.T, tt dnsRecheckCase, before, after model.NodeDomainBinding) {
	t.Helper()
	if tt.ready {
		if after.State != "ready" || after.LastError != "" || after.CheckedAt != before.CheckedAt {
			t.Fatalf("temporary DNS error replaced the last verified state: %+v", after)
		}
	} else if after.State != "failed" || after.LastError != tt.code {
		t.Fatalf("genuine DNS error was not reported: %+v", after)
	}
}

func (f *cloudFixture) handleRequest(t *testing.T, w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.headers = append(f.headers, r.Header.Clone())
	w.Header().Set("Content-Type", "application/json")
	result := func(value interface{}) {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "result": value})
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 2 && parts[0] == "zones" {
		result(DNSZone{Id: parts[1], Name: "example.com"})
		return
	}
	if len(parts) < 3 {
		t.Error("unexpected Cloudflare route", r.URL.Path)
		w.WriteHeader(404)
		return
	}
	id := ""
	if len(parts) > 3 {
		id = parts[3]
	}
	switch r.Method {
	case "GET":
		f.readRecord(w, r, id, result)
	case "POST", "PATCH":
		f.writeRecord(t, w, r, id, result)
	case "DELETE":
		delete(f.records, id)
		f.writes++
		result(map[string]string{"id": id})
	default:
		t.Error("unexpected CF method", r.Method)
	}
}

func (f *cloudFixture) readRecord(w http.ResponseWriter, r *http.Request, id string, result func(interface{})) {
	if f.rejectReads {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"success":false,"errors":[{"code":10000}]}`)
		return
	}
	if id != "" {
		record, ok := f.records[id]
		if !ok {
			if f.missingStatus != 0 {
				w.WriteHeader(f.missingStatus)
				fmt.Fprint(w, `{"success":false,"errors":[{"code":81044}]}`)
				return
			}
			w.WriteHeader(404)
			return
		}
		result(record)
	} else {
		records := []resourceRecord{}
		for _, record := range f.records {
			if record.Name == r.URL.Query().Get("name") {
				records = append(records, record)
			}
		}
		result(records)
	}
}

func (f *cloudFixture) writeRecord(t *testing.T, w http.ResponseWriter, r *http.Request, id string, result func(interface{})) {
	var record resourceRecord
	if json.NewDecoder(r.Body).Decode(&record) != nil {
		t.Error("bad CF body")
	}
	f.writes++
	if id == "" {
		id = fmt.Sprintf("%032x", f.writes+100)
	}
	record.Id = id
	f.records[id] = record
	if f.failCreateAck && r.Method == "POST" {
		f.failCreateAck = false
		w.WriteHeader(503)
		return
	}
	result(record)
}
