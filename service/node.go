package service

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"
)

const (
	nodeTLSModeVerify = "verify"
	nodeTLSModeSkip   = "skip"
	nodeTLSModePin    = "pin"

	nodeSyncSelected = "selected"
	nodeSyncAll      = "all"

	nodeStatusUnknown = "unknown"
	nodeStatusOnline  = "online"
	nodeStatusOffline = "offline"

	nodeProbeTimeout = 8 * time.Second
)

type NodeService struct{}

// Normalize validates and canonicalizes a node record before persistence.
func (s *NodeService) Normalize(n *model.Node) error {
	if n == nil {
		return common.NewError("node is nil")
	}
	n.Name = strings.TrimSpace(n.Name)
	n.Remark = strings.TrimSpace(n.Remark)
	n.Address = strings.TrimSpace(n.Address)
	n.ApiToken = strings.TrimSpace(n.ApiToken)
	n.PublicHost = strings.TrimSpace(n.PublicHost)
	n.PinnedCertSha256 = strings.TrimSpace(n.PinnedCertSha256)
	n.PanelVersion = strings.TrimSpace(n.PanelVersion)
	n.LastError = strings.TrimSpace(n.LastError)

	if n.Name == "" {
		return common.NewError("node name is required")
	}
	if n.Address == "" {
		return common.NewError("node address is required")
	}

	// Accept accidental host:port in address.
	if host, portStr, err := net.SplitHostPort(n.Address); err == nil {
		n.Address = host
		if n.Port == 0 {
			if p, err := strconv.Atoi(portStr); err == nil {
				n.Port = p
			}
		}
	}

	if n.Port <= 0 || n.Port > 65535 {
		return common.NewError("node port must be 1-65535")
	}

	n.Scheme = strings.ToLower(strings.TrimSpace(n.Scheme))
	if n.Scheme == "" {
		n.Scheme = "https"
	}
	if n.Scheme != "http" && n.Scheme != "https" {
		return common.NewError("node scheme must be http or https")
	}

	n.BasePath = normalizeNodeBasePath(n.BasePath)

	n.TlsVerifyMode = strings.ToLower(strings.TrimSpace(n.TlsVerifyMode))
	if n.TlsVerifyMode == "" {
		n.TlsVerifyMode = nodeTLSModeVerify
	}
	switch n.TlsVerifyMode {
	case nodeTLSModeVerify, nodeTLSModeSkip, nodeTLSModePin:
	default:
		return common.NewError("tlsVerifyMode must be verify, skip, or pin")
	}
	if n.TlsVerifyMode == nodeTLSModePin {
		if n.Scheme != "https" {
			return common.NewError("certificate pinning requires https")
		}
		if _, err := decodeCertPin(n.PinnedCertSha256); err != nil {
			return err
		}
	}

	n.InboundSyncMode = strings.ToLower(strings.TrimSpace(n.InboundSyncMode))
	if n.InboundSyncMode == "" {
		n.InboundSyncMode = nodeSyncSelected
	}
	if n.InboundSyncMode != nodeSyncSelected && n.InboundSyncMode != nodeSyncAll {
		return common.NewError("inboundSyncMode must be selected or all")
	}

	tags, err := normalizeInboundTagsJSON(n.InboundTags)
	if err != nil {
		return err
	}
	n.InboundTags = tags

	if n.Status == "" {
		n.Status = nodeStatusUnknown
	}
	return nil
}

func normalizeNodeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

func normalizeInboundTagsJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "[]", nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return "", common.NewError("inboundTags must be a JSON string array")
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeCertPin(pin string) ([]byte, error) {
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return nil, common.NewError("pinnedCertSha256 is required for pin mode")
	}
	if b, err := base64.StdEncoding.DecodeString(pin); err == nil && len(b) == sha256.Size {
		return b, nil
	}
	if b, err := hex.DecodeString(pin); err == nil && len(b) == sha256.Size {
		return b, nil
	}
	return nil, common.NewError("pinnedCertSha256 must be sha256 as base64 or hex")
}

func (s *NodeService) GetAll() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Model(&model.Node{}).Order("id asc").Find(&nodes).Error
	return nodes, err
}

func (s *NodeService) GetById(id uint) (*model.Node, error) {
	db := database.GetDB()
	n := &model.Node{}
	err := db.Model(&model.Node{}).Where("id = ?", id).First(n).Error
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (s *NodeService) Create(n *model.Node) error {
	if err := s.Normalize(n); err != nil {
		return err
	}
	if strings.TrimSpace(n.ApiToken) == "" {
		return common.NewError("apiToken is required")
	}
	n.Id = 0
	if n.Status == "" {
		n.Status = nodeStatusUnknown
	}
	return database.GetDB().Create(n).Error
}

func (s *NodeService) Update(id uint, in *model.Node) error {
	if id == 0 {
		return common.NewError("node id is required")
	}
	existing, err := s.GetById(id)
	if err != nil {
		return err
	}
	// Keep token if caller sends blank (UI edit without re-entering secret).
	if strings.TrimSpace(in.ApiToken) == "" {
		in.ApiToken = existing.ApiToken
	}
	if err := s.Normalize(in); err != nil {
		return err
	}
	in.Id = id
	// Preserve heartbeat/runtime fields; those are written by probe/heartbeat paths.
	in.Status = existing.Status
	in.LastHeartbeat = existing.LastHeartbeat
	in.LatencyMs = existing.LatencyMs
	in.PanelVersion = existing.PanelVersion
	in.CoreRunning = existing.CoreRunning
	in.CpuPercent = existing.CpuPercent
	in.MemPercent = existing.MemPercent
	in.Uptime = existing.Uptime
	in.LastError = existing.LastError
	in.ConfigDirty = existing.ConfigDirty
	in.ConfigDirtyAt = existing.ConfigDirtyAt
	return database.GetDB().Save(in).Error
}

func (s *NodeService) Delete(id uint) error {
	if id == 0 {
		return common.NewError("node id is required")
	}
	db := database.GetDB()

	// Block deletion if inbounds are still assigned to this node
	var count int64
	if err := db.Model(&model.Inbound{}).Where("node_id = ?", id).Count(&count).Error; err == nil && count > 0 {
		return common.NewErrorf("cannot delete node: %d inbound(s) are still assigned to this node", count)
	}

	res := db.Where("id = ?", id).Delete(&model.Node{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return common.NewError("node not found")
	}
	// Cleanup traffic baselines
	s.CleanupNodeTraffic(id)
	return nil
}

func (s *NodeService) SetEnable(id uint, enable bool) error {
	res := database.GetDB().Model(&model.Node{}).Where("id = ?", id).Update("enable", enable)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return common.NewError("node not found")
	}
	return nil
}

func (s *NodeService) MarkDirty(id uint) error {
	now := time.Now().Unix()
	res := database.GetDB().Model(&model.Node{}).Where("id = ?", id).Updates(map[string]interface{}{
		"config_dirty":    true,
		"config_dirty_at": now,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return common.NewError("node not found")
	}
	return nil
}

func (s *NodeService) ClearDirty(id uint) error {
	res := database.GetDB().Model(&model.Node{}).Where("id = ?", id).Updates(map[string]interface{}{
		"config_dirty":    false,
		"config_dirty_at": 0,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return common.NewError("node not found")
	}
	return nil
}

// BaseURL builds the remote panel root including base path.
func (s *NodeService) BaseURL(n *model.Node) (string, error) {
	if n == nil {
		return "", common.NewError("node is nil")
	}
	if strings.TrimSpace(n.Address) == "" {
		return "", common.NewError("node address is required")
	}
	if n.Port <= 0 || n.Port > 65535 {
		return "", common.NewError("node port must be 1-65535")
	}
	scheme := strings.ToLower(strings.TrimSpace(n.Scheme))
	if scheme == "" {
		scheme = "https"
	}
	if scheme != "http" && scheme != "https" {
		return "", common.NewError("node scheme must be http or https")
	}
	basePath := normalizeNodeBasePath(n.BasePath)
	host := net.JoinHostPort(n.Address, strconv.Itoa(n.Port))
	return scheme + "://" + host + basePath, nil
}

// APIv2URL returns absolute URL for an apiv2 action path segment.
func (s *NodeService) APIv2URL(n *model.Node, action string) (string, error) {
	base, err := s.BaseURL(n)
	if err != nil {
		return "", err
	}
	action = strings.Trim(action, "/")
	return base + "apiv2/" + action, nil
}

type NodeProbeResult struct {
	OK            bool                   `json:"ok"`
	LatencyMs     int64                  `json:"latencyMs"`
	StatusCode    int                    `json:"statusCode"`
	CoreRunning   bool                   `json:"coreRunning"`
	Raw           map[string]interface{} `json:"raw,omitempty"`
	Error         string                 `json:"error,omitempty"`
	CertSHA256B64 string                 `json:"certSha256B64,omitempty"`
}

// Probe checks connectivity to a full remote s-ui using existing apiv2/status.
// M2 will switch to dedicated nodeSnapshot; this is enough for registry validation.
func (s *NodeService) Probe(ctx context.Context, n *model.Node) (*NodeProbeResult, error) {
	if n == nil {
		return nil, common.NewError("node is nil")
	}
	tmp := *n
	if err := s.Normalize(&tmp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tmp.ApiToken) == "" {
		return nil, common.NewError("apiToken is required")
	}

	target, err := s.APIv2URL(&tmp, "status")
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if q.Get("r") == "" {
		q.Set("r", "sbd,sys")
	}
	u.RawQuery = q.Encode()

	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithTimeout(ctx, nodeProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", tmp.ApiToken)
	req.Header.Set("Accept", "application/json")

	client, err := s.httpClientFor(&tmp)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	result := &NodeProbeResult{LatencyMs: latency}
	if err != nil {
		result.OK = false
		result.Error = err.Error()
		return result, nil
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
		result.CertSHA256B64 = base64.StdEncoding.EncodeToString(sum[:])
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		result.OK = false
		result.Error = err.Error()
		return result, nil
	}
	if resp.StatusCode != http.StatusOK {
		result.OK = false
		result.Error = fmt.Sprintf("http %d", resp.StatusCode)
		return result, nil
	}

	var env struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		result.OK = false
		result.Error = "invalid json envelope"
		return result, nil
	}
	if !env.Success {
		result.OK = false
		result.Error = env.Msg
		if result.Error == "" {
			result.Error = "remote success=false"
		}
		return result, nil
	}
	result.OK = true
	result.Raw = env.Obj
	if env.Obj != nil {
		if sbd, ok := env.Obj["sbd"].(map[string]interface{}); ok {
			if running, ok := sbd["running"].(bool); ok {
				result.CoreRunning = running
			}
		}
	}
	return result, nil
}

func (s *NodeService) httpClientFor(n *model.Node) (*http.Client, error) {
	tlsConf := &tls.Config{MinVersion: tls.VersionTLS12}
	switch n.TlsVerifyMode {
	case nodeTLSModeSkip:
		tlsConf.InsecureSkipVerify = true
	case nodeTLSModePin:
		pin, err := decodeCertPin(n.PinnedCertSha256)
		if err != nil {
			return nil, err
		}
		tlsConf.InsecureSkipVerify = true
		tlsConf.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return common.NewError("node presented no certificate")
			}
			sum := sha256.Sum256(rawCerts[0])
			if subtleConstantTimeCompare(sum[:], pin) {
				return nil
			}
			return common.NewError("node certificate pin mismatch")
		}
	}

	dialer := &net.Dialer{Timeout: nodeProbeTimeout}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if ip := net.ParseIP(host); ip != nil {
				if !n.AllowPrivateAddress && isPrivateIP(ip) {
					return nil, common.NewError("private node address blocked (set allowPrivateAddress)")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
		TLSClientConfig: tlsConf,
	}
	return &http.Client{Transport: transport, Timeout: nodeProbeTimeout}, nil
}

func subtleConstantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate()
}

// ApplyProbeResult writes heartbeat fields for an existing node id.
func (s *NodeService) ApplyProbeResult(id uint, res *NodeProbeResult) error {
	if id == 0 {
		return common.NewError("node id is required")
	}
	if res == nil {
		return common.NewError("probe result is nil")
	}
	updates := map[string]interface{}{
		"latency_ms":     res.LatencyMs,
		"last_heartbeat": time.Now().Unix(),
		"core_running":   res.CoreRunning,
	}
	if res.OK {
		updates["status"] = nodeStatusOnline
		updates["last_error"] = ""
	} else {
		updates["status"] = nodeStatusOffline
		updates["last_error"] = res.Error
	}
	return database.GetDB().Model(&model.Node{}).Where("id = ?", id).Updates(updates).Error
}

// newNodeRequest creates an HTTP request with the node's API token.
func newNodeRequest(ctx context.Context, method, url, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}
