package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func setupNodeTestDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/test.db"
	if err := database.InitDB(path); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
}

func TestNodeNormalizeAndURLs(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{
		Name:     " de-fra ",
		Address:  "node.example.com:2095",
		Scheme:   "HTTPS",
		BasePath: "app",
		ApiToken: "tok",
	}
	if err := s.Normalize(n); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if n.Name != "de-fra" {
		t.Fatalf("name=%q", n.Name)
	}
	if n.Address != "node.example.com" || n.Port != 2095 {
		t.Fatalf("address/port = %s %d", n.Address, n.Port)
	}
	if n.Scheme != "https" || n.BasePath != "/app/" {
		t.Fatalf("scheme/basePath = %s %s", n.Scheme, n.BasePath)
	}
	if n.InboundSyncMode != nodeSyncSelected || n.InboundTags != "[]" {
		t.Fatalf("sync defaults = %s %s", n.InboundSyncMode, n.InboundTags)
	}
	base, err := s.BaseURL(n)
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://node.example.com:2095/app/" {
		t.Fatalf("BaseURL=%s", base)
	}
	api, err := s.APIv2URL(n, "status")
	if err != nil {
		t.Fatal(err)
	}
	if api != "https://node.example.com:2095/app/apiv2/status" {
		t.Fatalf("APIv2URL=%s", api)
	}
}

func TestNodeNormalizeRejectsBadPin(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{
		Name:             "x",
		Address:          "1.2.3.4",
		Port:             443,
		Scheme:           "https",
		ApiToken:         "t",
		TlsVerifyMode:    nodeTLSModePin,
		PinnedCertSha256: "not-a-hash",
	}
	if err := s.Normalize(n); err == nil {
		t.Fatal("expected pin validation error")
	}
}

func TestNodeCRUDAndDirty(t *testing.T) {
	setupNodeTestDB(t)
	s := &NodeService{}
	n := &model.Node{
		Name:                "node-a",
		Address:             "10.0.0.1",
		Port:                2095,
		Scheme:              "http",
		ApiToken:            "secret-token",
		AllowPrivateAddress: true,
		PublicHost:          "a.example.com",
		InboundTags:         `["de-vless"," de-vless ",""]`,
	}
	if err := s.Create(n); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n.Id == 0 {
		t.Fatal("expected id")
	}
	if n.InboundTags != `["de-vless"]` {
		t.Fatalf("tags=%s", n.InboundTags)
	}

	got, err := s.GetById(n.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ApiToken != "secret-token" || got.PublicHost != "a.example.com" {
		t.Fatalf("got=%+v", got)
	}

	upd := *got
	upd.Remark = "Frankfurt"
	upd.ApiToken = ""
	if err := s.Update(got.Id, &upd); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := s.GetById(got.Id)
	if got2.ApiToken != "secret-token" || got2.Remark != "Frankfurt" {
		t.Fatalf("update preserve failed: %+v", got2)
	}

	if err := s.MarkDirty(got.Id); err != nil {
		t.Fatal(err)
	}
	got3, _ := s.GetById(got.Id)
	if !got3.ConfigDirty || got3.ConfigDirtyAt == 0 {
		t.Fatalf("dirty not set: %+v", got3)
	}
	if err := s.ClearDirty(got.Id); err != nil {
		t.Fatal(err)
	}
	got4, _ := s.GetById(got.Id)
	if got4.ConfigDirty {
		t.Fatal("dirty not cleared")
	}

	if err := s.SetEnable(got.Id, false); err != nil {
		t.Fatal(err)
	}
	all, err := s.GetAll()
	if err != nil || len(all) != 1 || all[0].Enable {
		t.Fatalf("GetAll/enable failed: %v %+v", err, all)
	}
	if err := s.Delete(got.Id); err != nil {
		t.Fatal(err)
	}
	all, _ = s.GetAll()
	if len(all) != 0 {
		t.Fatalf("expected empty, got %d", len(all))
	}
}

func TestNodeProbeSuccessAndApply(t *testing.T) {
	setupNodeTestDB(t)
	s := &NodeService{}

	mux := http.NewServeMux()
	mux.HandleFunc("/app/apiv2/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Token") != "tok123" {
			http.Error(w, "no", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"msg":     "",
			"obj": map[string]interface{}{
				"sbd": map[string]interface{}{"running": true},
			},
		})
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	n := &model.Node{
		Name:                "local-node",
		Address:             "127.0.0.1",
		Port:                port,
		Scheme:              "http",
		BasePath:            "/app/",
		ApiToken:            "tok123",
		AllowPrivateAddress: true,
		TlsVerifyMode:       nodeTLSModeSkip,
	}
	if err := s.Create(n); err != nil {
		t.Fatal(err)
	}
	res, err := s.Probe(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.CoreRunning || res.StatusCode != 200 {
		t.Fatalf("probe result: %+v", res)
	}
	if err := s.ApplyProbeResult(n.Id, res); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetById(n.Id)
	if got.Status != nodeStatusOnline || !got.CoreRunning || got.LastHeartbeat == 0 {
		t.Fatalf("heartbeat fields: %+v", got)
	}
}

func TestNodeProbeUnauthorized(t *testing.T) {
	s := &NodeService{}
	mux := http.NewServeMux()
	mux.HandleFunc("/apiv2/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte("nope"))
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	n := &model.Node{
		Name:                "x",
		Address:             "127.0.0.1",
		Port:                port,
		Scheme:              "http",
		ApiToken:            "bad",
		AllowPrivateAddress: true,
	}
	res, err := s.Probe(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatalf("expected failure: %+v", res)
	}
}

func TestDecodeCertPinFormats(t *testing.T) {
	sum := sha256.Sum256([]byte("cert"))
	b64 := base64.StdEncoding.EncodeToString(sum[:])
	if _, err := decodeCertPin(b64); err != nil {
		t.Fatal(err)
	}
	hexPin := hex.EncodeToString(sum[:])
	if _, err := decodeCertPin(hexPin); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateAddressBlocked(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{
		Name:                "x",
		Address:             "127.0.0.1",
		Port:                9,
		Scheme:              "http",
		ApiToken:            "t",
		AllowPrivateAddress: false,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := s.Probe(ctx, n)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.OK {
		t.Fatal("private address should be blocked")
	}
}
