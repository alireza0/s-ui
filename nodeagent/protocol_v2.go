package nodeagent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofrs/uuid/v5"
)

const ProtocolV2 = 2

type Structure struct {
	NodeID   uint64          `json:"node_id"`
	Epoch    string          `json:"epoch"`
	Revision uint64          `json:"revision"`
	Config   json.RawMessage `json:"config"`
}

type Grant struct {
	Name        string                     `json:"name"`
	TrafficID   string                     `json:"traffic_id,omitempty"`
	Expires     int64                      `json:"expires"`
	All         bool                       `json:"all"`
	Credentials map[string]json.RawMessage `json:"credentials,omitempty"`
}

type AccessSnapshot struct {
	NodeID   uint64  `json:"node_id"`
	Epoch    string  `json:"epoch"`
	Revision uint64  `json:"revision"`
	Clients  []Grant `json:"clients"`
}

func digestV2(value interface{}) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
func (s Structure) Digest() string      { return digestV2(s) }
func (a AccessSnapshot) Digest() string { return digestV2(a) }

func (s Structure) Validate() error {
	if s.NodeID == 0 || s.Epoch == "" || s.Revision == 0 {
		return errors.New("invalid structural identity")
	}
	var config map[string]json.RawMessage
	if json.Unmarshal(s.Config, &config) != nil || config == nil {
		return errors.New("structure must contain an object")
	}
	var inbounds []map[string]json.RawMessage
	if json.Unmarshal(config["inbounds"], &inbounds) != nil || inbounds == nil {
		return errors.New("structure needs an inbound array")
	}
	seen := map[string]bool{}
	for _, in := range inbounds {
		if err := validateStructuralInbound(in, seen); err != nil {
			return err
		}
	}
	return ValidateCertificateReferences(s.Config)
}

func (a AccessSnapshot) Validate() error {
	if a.NodeID == 0 || a.Epoch == "" || a.Revision == 0 || a.Clients == nil {
		return errors.New("invalid access identity or array")
	}
	seen, trafficNames := map[string]bool{}, map[string]string{}
	for _, grant := range a.Clients {
		if grant.Name == "" || seen[grant.Name] || grant.Expires < 0 || !grant.All {
			return errors.New("invalid access grant")
		}
		seen[grant.Name] = true
		if err := validateGrantCredentials(grant, trafficNames); err != nil {
			return err
		}
	}
	return nil
}

func validateGrantCredentials(grant Grant, trafficNames map[string]string) error {
	for protocol, raw := range grant.Credentials {
		if err := ValidateCredential(protocol, raw); err != nil {
			return err
		}
		if !UsernameAuth(protocol) || grant.TrafficID == "" {
			continue
		}
		var auth struct {
			Username string `json:"username"`
		}
		_ = json.Unmarshal(raw, &auth)
		if owner, exists := trafficNames[auth.Username]; exists && owner != grant.TrafficID {
			return errors.New("ambiguous traffic username")
		}
		trafficNames[auth.Username] = grant.TrafficID
	}
	return nil
}

func ValidateCredential(kind string, raw json.RawMessage) error {
	if !SupportsInbound(kind, true) && kind != "shadowsocks16" {
		return errors.New("unsupported credential type")
	}
	var value map[string]interface{}
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return errors.New("invalid credential object")
	}
	if NeedsUUID(kind) {
		id, _ := value["uuid"].(string)
		if _, err := uuid.FromString(id); err != nil {
			return errors.New("invalid credential UUID")
		}
	}
	if field := SecretField(kind); field != "" {
		password, _ := value[field].(string)
		if err := validateSecretCredential(kind, password); err != nil {
			return err
		}
	}
	if UsernameAuth(kind) {
		name, _ := value["username"].(string)
		if name == "" {
			return errors.New("credential username is missing")
		}
	}
	if flow, exists := value["flow"]; kind == "vless" && exists && flow != "" && flow != "xtls-rprx-vision" {
		return errors.New("invalid VLESS flow")
	}
	return nil
}

// Compose does not read a desired structure or roll access back with a spec.
func Compose(s Structure, a AccessSnapshot, now int64) ([]byte, error) {
	if s.NodeID != a.NodeID || s.Epoch != a.Epoch {
		return nil, ErrConflict
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(s.Config, &config); err != nil {
		return nil, err
	}
	var inbounds []map[string]json.RawMessage
	if err := json.Unmarshal(config["inbounds"], &inbounds); err != nil {
		return nil, err
	}
	for _, in := range inbounds {
		users, err := composeInboundUsers(in, a.Clients, now)
		if err != nil {
			return nil, err
		}
		in["users"], _ = json.Marshal(users)
	}
	config["inbounds"], _ = json.Marshal(inbounds)
	return json.Marshal(config)
}

var deniedProxyKey = func() []byte {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return key
}()

func deniedProxyUser(kind, credential string) map[string]interface{} {
	key := deniedProxyKey
	if credential == "shadowsocks16" {
		key = key[:16]
	}
	secret := base64.StdEncoding.EncodeToString(key)
	name := "unassigned"
	if UsernameAuth(kind) {
		return map[string]interface{}{"username": name, "password": secret}
	}
	return map[string]interface{}{"name": name, SecretField(kind): secret}
}

func validateStructuralInbound(in map[string]json.RawMessage, seen map[string]bool) error {
	if _, exists := in["users"]; exists {
		return errors.New("structural spec must not contain users")
	}
	var tag, kind string
	if json.Unmarshal(in["tag"], &tag) != nil || tag == "" || seen[tag] {
		return errors.New("invalid structural inbound tag")
	}
	seen[tag] = true
	if json.Unmarshal(in["type"], &kind) != nil {
		return errors.New("invalid structural protocol")
	}
	if !SupportsInbound(kind, true) {
		return errors.New("unsupported structural protocol")
	}
	return nil
}

func validateSecretCredential(kind, password string) error {
	if password == "" {
		return errors.New("credential password is missing")
	}
	if kind == "shadowsocks" || kind == "shadowsocks16" {
		want := 32
		if kind == "shadowsocks16" {
			want = 16
		}
		if key, err := base64.StdEncoding.DecodeString(password); err != nil || len(key) != want {
			return errors.New("invalid Shadowsocks user key")
		}
	}
	return nil
}

func composeInboundUsers(in map[string]json.RawMessage, grants []Grant, now int64) ([]map[string]interface{}, error) {
	var kind, method string
	_ = json.Unmarshal(in["type"], &kind)
	_ = json.Unmarshal(in["method"], &method)
	credential := CredentialKey(kind, method)
	users := []map[string]interface{}{}
	for _, grant := range grants {
		if !grant.All || (grant.Expires > 0 && grant.Expires <= now) {
			continue
		}
		user, err := composeGrantUser(in, grant, kind, credential)
		if err != nil {
			return nil, err
		}
		if user != nil {
			users = append(users, user)
		}
	}
	if len(users) == 0 && ProxyInbound(kind) {
		// Empty native proxy user arrays can allow anonymous access.
		users = append(users, deniedProxyUser(kind, credential))
	}
	return users, nil
}

func composeGrantUser(in map[string]json.RawMessage, grant Grant, kind, credential string) (map[string]interface{}, error) {
	raw := grant.Credentials[credential]
	if len(raw) == 0 && ExtendedInbound(kind) {
		return nil, nil
	}
	var user map[string]interface{}
	if json.Unmarshal(raw, &user) != nil || user == nil {
		return nil, fmt.Errorf("missing credentials for granted protocol %s", kind)
	}
	if UsernameAuth(kind) {
		delete(user, "name")
	} else {
		user["name"] = grant.Name
	}
	if kind == "vless" {
		stripUnsupportedFlow(user, in)
	}
	return user, nil
}

func stripUnsupportedFlow(user map[string]interface{}, in map[string]json.RawMessage) {
	var tls, transport map[string]interface{}
	_ = json.Unmarshal(in["tls"], &tls)
	_ = json.Unmarshal(in["transport"], &transport)
	if tls["enabled"] != true || (transport["type"] != nil && transport["type"] != "") {
		delete(user, "flow")
	}
}
