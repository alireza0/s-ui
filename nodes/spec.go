package nodes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/alireza0/s-ui/nodeagent"
	"github.com/gofrs/uuid/v5"
)

type ValueSource struct {
	Kind    string          `json:"kind"`
	Name    string          `json:"name,omitempty"`
	Purpose string          `json:"purpose,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
}

type SpecEntry struct {
	Key               string                 `json:"connection_key"`
	Name              string                 `json:"name"`
	Protocol          string                 `json:"protocol"`
	Enable            bool                   `json:"enable"`
	Options           json.RawMessage        `json:"options"`
	Sources           map[string]ValueSource `json:"sources,omitempty"`
	TLSProfileID      uint                   `json:"tls_profile_id,omitempty"`
	ClientTLS         json.RawMessage        `json:"client_tls,omitempty"`
	ClientOptions     json.RawMessage        `json:"client_options,omitempty"`
	RealmServerSecret string                 `json:"realm_server_secret,omitempty"`
	RealmClientSecret string                 `json:"realm_client_secret,omitempty"`
}

type TemplateSpec struct {
	SchemaVersion int             `json:"schema_version"`
	Config        json.RawMessage `json:"config"`
	Entries       []SpecEntry     `json:"entries"`
}

type SpecOverride struct {
	ConnectionKey string      `json:"connection_key"`
	Path          string      `json:"path"`
	Operation     string      `json:"operation"`
	Source        ValueSource `json:"source"`
}

// Context contains already selected node resources; resolving never generates
// keys, reads the database or performs DNS/SSH. Generated keys include entry ID.
type ResolveContext struct {
	Parameters     map[string]json.RawMessage
	Generated      map[string]json.RawMessage
	Certificates   map[string]json.RawMessage
	Profiles       map[string]json.RawMessage
	Overrides      []SpecOverride
	LocalAddresses []string
}

func decodeJSON(raw []byte, target interface{}, strict bool) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if strict {
		d.DisallowUnknownFields()
	}
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra interface{}
	if d.Decode(&extra) != io.EOF {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func objectJSON(raw json.RawMessage) (map[string]interface{}, error) {
	var value map[string]interface{}
	if err := decodeJSON(raw, &value, false); err != nil || value == nil {
		return nil, errors.New("configuration must be a JSON object")
	}
	return value, nil
}

func secretLiteral(value interface{}) bool {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			switch key {
			case "users", "uuid", "password", "psk", "private_key", "key", "certificate", "api_token", "api_key", "ssh_password":
				return true
			}
			if secretLiteral(item) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if secretLiteral(item) {
				return true
			}
		}
	case string:
		return strings.Contains(v, "-----BEGIN ")
	}
	return false
}

func sourcePath(path string) ([]string, error) {
	if !strings.HasPrefix(path, "/") || path == "/" {
		return nil, errors.New("source path must identify an options field")
	}
	parts := strings.Split(path[1:], "/")
	for i, part := range parts {
		decoded, err := decodeSourcePart(part)
		if err != nil {
			return nil, err
		}
		part = decoded
		if err := validateSourcePart(part, i); err != nil {
			return nil, err
		}
		parts[i] = part
	}
	return parts, nil
}

func validateSource(path string, source ValueSource) error {
	parts, err := sourcePath(path)
	if err != nil {
		return err
	}
	if path == "/realm/token" {
		return errClientOptions
	}
	switch source.Kind {
	case "literal":
		if err := validateLiteralSource(path, source.Value); err != nil {
			return err
		}
	case "node_parameter":
		switch source.Name {
		case "primary_domain", "node_address", "node_name", "node_id":
		default:
			return errors.New("unknown node parameter")
		}
	case "node_generated":
		if err := validateGeneratedSource(path, source.Purpose); err != nil {
			return err
		}
	case "node_certificate", "profile":
		if strings.TrimSpace(source.Name) == "" {
			return errors.New("resource source needs a name")
		}
	default:
		return errors.New("unknown value source")
	}
	if source.Kind != "literal" && len(source.Value) > 0 {
		return errors.New("resource source must not contain an inline value")
	}
	return validateSecretSource(path, parts[len(parts)-1], source)
}

// NormalizeSpec allocates only missing logical IDs, never per-node material.
func NormalizeSpec(raw json.RawMessage, previous *TemplateSpec) (TemplateSpec, error) {
	var spec TemplateSpec
	if err := decodeJSON(raw, &spec, true); err != nil {
		return spec, err
	}
	if spec.SchemaVersion != 1 {
		return spec, errors.New("unsupported template schema version")
	}
	config, err := objectJSON(spec.Config)
	if err != nil {
		return spec, err
	}
	if _, ok := config["inbounds"]; ok {
		return spec, errors.New("inbounds must be template entries")
	}
	if secretLiteral(config) {
		return spec, errors.New("template config must not contain inline secrets")
	}
	oldTypes := map[string]string{}
	if previous != nil {
		for _, entry := range previous.Entries {
			oldTypes[entry.Key] = entry.Protocol
		}
	}
	keys := map[string]bool{}
	for i := range spec.Entries {
		if err := normalizeSpecEntry(&spec.Entries[i], oldTypes, keys); err != nil {
			return spec, err
		}
	}
	if spec.Entries == nil {
		spec.Entries = []SpecEntry{}
	}
	return spec, nil
}

var errNodeDomainPending = errors.New("node_domain_not_ready")
var errNodeCertificatePending = errors.New("node_certificate_not_ready")

func resolveSource(entry, path string, source ValueSource, ctx ResolveContext) (interface{}, error) {
	var raw json.RawMessage
	switch source.Kind {
	case "literal":
		raw = source.Value
	case "node_parameter":
		raw = ctx.Parameters[source.Name]
	case "node_generated":
		raw = ctx.Generated[entry+"/"+source.Purpose]
	case "node_certificate":
		raw = ctx.Certificates[source.Name]
	case "profile":
		raw = ctx.Profiles[source.Name]
	}
	if len(raw) == 0 {
		if source.Kind == "node_parameter" && source.Name == "primary_domain" {
			return nil, errNodeDomainPending
		}
		if source.Kind == "node_certificate" {
			return nil, errNodeCertificatePending
		}
		return nil, errors.New("required node resource is missing")
	}
	var value interface{}
	if err := decodeJSON(raw, &value, false); err != nil {
		return nil, errors.New("invalid node resource value")
	}
	if source.Kind == "profile" && secretLiteral(value) {
		return nil, errors.New("profile must not inject inline users or server secret objects")
	}
	if source.Kind == "node_certificate" {
		settings, ok := value.(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid_node_certificate_resource")
		}
		switch path {
		case "/tls/certificate_path":
			return settings["certificate_path"], nil
		case "/tls/key_path":
			return settings["key_path"], nil
		case "/tls", "/tls/certificate_binding":
		default:
			return nil, errors.New("invalid_node_certificate_source_path")
		}
	}
	return value, nil
}

func setOption(options map[string]interface{}, path string, value interface{}, remove bool) error {
	parts, err := sourcePath(path)
	if err != nil {
		return err
	}
	parent := options
	for _, part := range parts[:len(parts)-1] {
		next, ok := parent[part]
		if !ok {
			if remove {
				return nil
			}
			child := map[string]interface{}{}
			parent[part] = child
			parent = child
			continue
		}
		child, ok := next.(map[string]interface{})
		if !ok {
			return errors.New("source parent is not an object")
		}
		parent = child
	}
	if remove {
		delete(parent, parts[len(parts)-1])
	} else {
		parent[parts[len(parts)-1]] = value
	}
	return nil
}

func validateSpecOverrides(spec TemplateSpec, values []SpecOverride) (map[string][]SpecOverride, error) {
	overrides := map[string][]SpecOverride{}
	keys := map[string]string{}
	for _, entry := range spec.Entries {
		keys[entry.Key] = entry.Protocol
	}
	seen := map[string]bool{}
	paths := map[string][]string{}
	for _, v := range values {
		key := v.ConnectionKey + "\x00" + v.Path
		if keys[v.ConnectionKey] == "" || seen[key] {
			return nil, errors.New("unknown connection or duplicate override")
		}
		seen[key] = true
		if keys[v.ConnectionKey] == "shadowsocks" && v.Path == "/method" {
			return nil, errors.New("invalid_node_shadowsocks_method")
		}
		if _, err := sourcePath(v.Path); err != nil {
			return nil, err
		}
		if v.Operation != "set" && v.Operation != "remove" {
			return nil, errors.New("invalid override operation")
		}
		if v.Operation == "set" {
			if err := validateSource(v.Path, v.Source); err != nil {
				return nil, err
			}
		}
		paths[v.ConnectionKey] = append(paths[v.ConnectionKey], v.Path)
		overrides[v.ConnectionKey] = append(overrides[v.ConnectionKey], v)
	}
	if err := validateOverridePaths(paths); err != nil {
		return nil, err
	}
	return overrides, nil
}

func ResolveSpec(spec TemplateSpec, ctx ResolveContext) (TemplateSpec, error) {
	for _, entry := range spec.Entries {
		if entry.Key == "" {
			return spec, errors.New("template connections must have persistent keys before resolution")
		}
	}
	body, err := json.Marshal(spec)
	if err != nil {
		return spec, err
	}
	result, err := NormalizeSpec(body, &spec)
	if err != nil {
		return result, err
	}
	overrides, err := validateSpecOverrides(result, ctx.Overrides)
	if err != nil {
		return result, err
	}
	for i := range result.Entries {
		entry := &result.Entries[i]
		// Inactive definitions keep their sources for a later enable, but do not
		// reserve ports or demand certificates/material for an unused listener.
		if !entry.Enable {
			continue
		}
		if err := resolveSpecEntry(entry, ctx, overrides[entry.Key]); err != nil {
			return result, err
		}
	}
	if err := validateRealityTargets(result.Entries, ctx.LocalAddresses); err != nil {
		return result, err
	}
	return result, nil
}

func decodeSourcePart(part string) (string, error) {
	for j := 0; j < len(part); j++ {
		if part[j] == '~' {
			if j+1 == len(part) || (part[j+1] != '0' && part[j+1] != '1') {
				return "", errors.New("invalid JSON pointer escape")
			}
			j++
		}
	}
	part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	return part, nil
}

func validateSourcePart(part string, i int) error {
	if part == "" || part == "-" {
		return errors.New("invalid source path")
	}
	if _, err := strconv.Atoi(part); err == nil {
		return errors.New("array index sources are not supported")
	}
	switch part {
	case "users", "uuid", "api_token", "api_key", "ssh_password":
		return errors.New("source cannot replace connection identity or access")
	case "type", "tag", "id", "node_id", "connection_key", "enable", "enabled_protocols":
		if i == 0 {
			return errors.New("source cannot replace connection identity or access")
		}
	}
	return nil
}

func normalizeSpecEntry(entry *SpecEntry, oldTypes map[string]string, keys map[string]bool) error {
	if entry.Key == "" {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}
		entry.Key = id.String()
	}
	if len(entry.Key) > 128 || strings.TrimSpace(entry.Key) != entry.Key || keys[entry.Key] || strings.TrimSpace(entry.Name) == "" {
		return errors.New("invalid or duplicate connection key/name")
	}
	keys[entry.Key] = true
	if !nodeagent.SupportsInbound(entry.Protocol, true) {
		return errors.New("unsupported node protocol")
	}
	if old, ok := oldTypes[entry.Key]; ok && old != entry.Protocol {
		return errors.New("create a new connection to change protocol")
	}
	options, err := objectJSON(entry.Options)
	if err != nil {
		return err
	}
	return validateEntryOptions(entry, options)
}

func validateEntryPort(options map[string]interface{}, sources map[string]ValueSource) error {
	if port, exists := options["listen_port"]; exists {
		number, ok := port.(json.Number)
		if !ok {
			return errors.New("listen_port must be numeric")
		}
		p, err := number.Int64()
		if err != nil || p < 1 || p > 65535 {
			return errors.New("invalid listen_port")
		}
	} else if _, bound := sources[listenPortPath]; !bound {
		return errors.New("listen_port or its node source is required")
	}
	return nil
}

func validateEntryClientTLS(raw json.RawMessage) error {
	if len(raw) > 0 && string(raw) != "null" {
		client, err := objectJSON(raw)
		if err != nil || secretLiteral(client) || hasPrivateClientTLS(client) {
			return errors.New("invalid_template_client_tls")
		}
	}
	return nil
}

func validateEntrySources(entry SpecEntry) error {
	paths := make([]string, 0, len(entry.Sources))
	for path, source := range entry.Sources {
		if err := validateSource(path, source); err != nil {
			return fmt.Errorf("connection %s source %s: %w", entry.Key, path, err)
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for j := 1; j < len(paths); j++ {
		if strings.HasPrefix(paths[j], paths[j-1]+"/") {
			return errors.New("overlapping value sources")
		}
	}
	return nil
}

func validateEntryOptions(entry *SpecEntry, options map[string]interface{}) error {
	if err := validateEntryPort(options, entry.Sources); err != nil {
		return err
	}
	if secretLiteral(options) {
		return errors.New("template options must not contain inline users or secret material")
	}
	if err := proxyInboundSources(entry, options); err != nil {
		return err
	}
	if err := validateEntryClientTLS(entry.ClientTLS); err != nil {
		return err
	}
	if err := validateNodeClientOptions(*entry); err != nil {
		return err
	}
	for _, field := range []string{"id", "type", "tag", "node_id", "enable", "addrs", "out_json", "tls_id"} {
		if _, ok := options[field]; ok {
			return errors.New("reserved template options field")
		}
	}
	return validateEntrySources(*entry)
}

func validateLiteralSource(path string, raw json.RawMessage) error {
	var value interface{}
	if err := decodeJSON(raw, &value, false); err != nil || secretLiteral(value) {
		return errors.New("invalid or secret template literal")
	}
	if path == "/realm" {
		if realm, ok := value.(map[string]any); ok && realm["token"] != nil {
			return errClientOptions
		}
	}
	return nil
}

func validateGeneratedSource(path, purpose string) error {
	switch purpose {
	case "listen_port":
		if path != listenPortPath {
			return errors.New("invalid_listener_port_source")
		}
	case "hysteria2_obfs_password":
		if path != "/obfs/password" {
			return errors.New("invalid_obfs_password_source")
		}
	case "shadowsocks_key16", "shadowsocks_key32":
		if path != "/password" {
			return errors.New("invalid_shadowsocks_key_source")
		}
	case "snell_psk":
		if path != "/psk" {
			return errors.New("invalid_snell_key_source")
		}
	case "reality_private_key", "reality_short_id":
	default:
		return errors.New("unknown generated material purpose")
	}
	return nil
}

func validateSecretSource(path, last string, source ValueSource) error {
	if last == "password" && source.Kind != "profile" && !(source.Kind == "node_generated" &&
		(source.Purpose == "hysteria2_obfs_password" || source.Purpose == "shadowsocks_key16" || source.Purpose == "shadowsocks_key32")) {
		return errors.New("secret values must use a profile reference")
	}
	if last == "psk" && (source.Kind != "node_generated" || source.Purpose != "snell_psk") {
		return errors.New("Snell PSK must use per-node generated material")
	}
	if last == "private_key" && !(source.Kind == "node_generated" && source.Purpose == "reality_private_key" && path == "/tls/reality/private_key") {
		return errors.New("private key must use per-node generated material")
	}
	if (last == "key" || last == "certificate" || last == "key_path" || last == "certificate_path") && source.Kind != "node_certificate" {
		return errors.New("certificate material must use a node certificate binding")
	}
	return nil
}

func validateOverridePaths(paths map[string][]string) error {
	for _, group := range paths {
		sort.Strings(group)
		for i := 1; i < len(group); i++ {
			if strings.HasPrefix(group[i], group[i-1]+"/") {
				return errors.New("overlapping overrides")
			}
		}
	}
	return nil
}

func resolvedEntrySources(inherited map[string]ValueSource, overrides []SpecOverride) (map[string]ValueSource, []string) {
	sources := map[string]ValueSource{}
	removals := []string{}
	for path, source := range inherited {
		sources[path] = source
	}
	for _, override := range overrides {
		// Replacing an object also replaces inherited descendants.
		for path := range sources {
			if path == override.Path || strings.HasPrefix(path, override.Path+"/") {
				delete(sources, path)
			}
		}
		if override.Operation == "remove" {
			removals = append(removals, override.Path)
		} else {
			sources[override.Path] = override.Source
		}
	}
	return sources, removals
}

func validateResolvedOptions(entry SpecEntry, options map[string]interface{}) error {
	port, ok := options["listen_port"].(json.Number)
	if !ok {
		return errors.New("resolved listen_port is missing or invalid")
	}
	p, err := port.Int64()
	if err != nil || p < 1 || p > 65535 {
		return errors.New("resolved listen_port is invalid")
	}
	transport, _ := options["transport"].(map[string]interface{})
	tls, _ := options["tls"].(map[string]interface{})
	if !nodeagent.SupportsTLS(entry.Protocol) {
		if tls["enabled"] == true || entry.TLSProfileID != 0 {
			return errors.New("this node inbound does not support TLS settings")
		}
		delete(options, "tls")
	}
	reality, _ := tls["reality"].(map[string]interface{})
	transportType, _ := transport["type"].(string)
	if nodeagent.RequiresRegularTLS(entry.Protocol, transportType) {
		if tls["enabled"] != true || reality["enabled"] == true {
			return errors.New("this node inbound needs regular TLS")
		}
	}
	if entry.Protocol != "vless" && reality["enabled"] == true {
		return errors.New("REALITY requires VLESS on this node")
	}
	return nil
}

func applyCertificateFields(options map[string]interface{}, value interface{}) {
	tls, _ := options["tls"].(map[string]interface{})
	if tls == nil {
		tls = map[string]interface{}{}
		options["tls"] = tls
	}
	fields := value.(map[string]interface{})
	for _, key := range []string{"certificate_path", "key_path"} {
		tls[key] = fields[key]
	}
	if tls["server_name"] == nil {
		tls["server_name"] = fields["server_name"]
	}
}

func resolveSpecEntry(entry *SpecEntry, ctx ResolveContext, overrides []SpecOverride) error {
	options, _ := objectJSON(entry.Options)
	sources, removals := resolvedEntrySources(entry.Sources, overrides)
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		value, err := resolveSource(entry.Key, path, sources[path], ctx)
		if err != nil {
			return fmt.Errorf("connection %s field %s: %w", entry.Key, path, err)
		}
		if sources[path].Kind == "node_certificate" && path == "/tls/certificate_binding" {
			applyCertificateFields(options, value)
			continue
		}
		if err := setOption(options, path, value, false); err != nil {
			return err
		}
	}
	for _, path := range removals {
		if err := setOption(options, path, nil, true); err != nil {
			return err
		}
	}
	if err := validateResolvedOptions(*entry, options); err != nil {
		return err
	}
	body, err := json.Marshal(options)
	entry.Options = body
	if err != nil {
		return err
	}
	entry.Sources = nil
	return nil
}
