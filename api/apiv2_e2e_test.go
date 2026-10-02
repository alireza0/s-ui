package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// apiv2TestServer sets up a minimal engine with a seeded admin user and API token.
func apiv2TestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatal(err)
	}

	// Close database connection pool before temp dir cleanup (critical on Windows)
	t.Cleanup(func() {
		if db := database.GetDB(); db != nil {
			if sqlDB, err := db.DB(); err == nil {
				sqlDB.Close()
			}
		}
	})

	var admin model.User
	if err := database.GetDB().Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("finding seeded admin user: %v", err)
	}

	testToken := "test-valid-api-token-32-chars!!"
	tokenRecord := &model.Tokens{
		Token:  testToken,
		Desc:   "e2e-testing-token",
		Expiry: 0,
		UserId: admin.Id,
	}
	if err := database.GetDB().Create(tokenRecord).Error; err != nil {
		t.Fatalf("seeding test token: %v", err)
	}

	engine := gin.New()
	apiv2 := NewAPIv2Handler(engine.Group("/apiv2"))
	apiv2.ReloadTokens()

	srv := httptest.NewServer(engine)
	t.Cleanup(srv.Close)

	return srv, testToken
}

// Token middleware must reject missing or invalid tokens with HTTP 401,
// and grant access when a valid token is provided.
func TestAPIv2TokenAuthentication(t *testing.T) {
	srv, validToken := apiv2TestServer(t)

	testCases := []struct {
		name       string
		token      string
		sendHeader bool
		wantStatus int
	}{
		{"missing token header", "", false, http.StatusUnauthorized},
		{"empty token", "", true, http.StatusUnauthorized},
		{"invalid token", "wrong-secret-token", true, http.StatusUnauthorized},
		{"valid token", validToken, true, http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/apiv2/system/status", nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.sendHeader {
				req.Header.Set("Token", tc.token)
			}

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

// Legacy RPC-style routes must remain accessible for backward compatibility,
// but advertise their deprecation headers.
func TestAPIv2LegacyDeprecationHeaders(t *testing.T) {
	srv, validToken := apiv2TestServer(t)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/apiv2/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Token", validToken)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("legacy route status = %d, want 200", resp.StatusCode)
	}

	if got := resp.Header.Get("Deprecation"); got != "true" {
		t.Errorf("Deprecation header = %q, want 'true'", got)
	}
	if got := resp.Header.Get("X-API-Warn"); got == "" {
		t.Error("expected X-API-Warn deprecation message in header")
	}
}

// Structured REST routes must answer successfully with valid token authentication.
func TestAPIv2StructuredEndpoints(t *testing.T) {
	srv, validToken := apiv2TestServer(t)

	endpoints := []string{
		"/apiv2/system/status",
		"/apiv2/users",
		"/apiv2/tokens",
		"/apiv2/database/info",
		"/apiv2/inbounds",
		"/apiv2/clients",
		"/apiv2/settings",
	}

	for _, path := range endpoints {
		t.Run(path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Token", validToken)

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("request to %s failed: %v", path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s status = %d, want 200", path, resp.StatusCode)
			}
		})
	}
}

// Token management via v2: create, retrieve and delete tokens.
func TestAPIv2TokenLifecycle(t *testing.T) {
	srv, validToken := apiv2TestServer(t)

	// Create a new token
	form := url.Values{"expiry": {"0"}, "desc": {"bot-token"}}
	addReq, err := http.NewRequest(http.MethodPost, srv.URL+"/apiv2/tokens/add", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	addReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addReq.Header.Set("Token", validToken)

	addResp, err := srv.Client().Do(addReq)
	if err != nil {
		t.Fatalf("add token failed: %v", err)
	}
	defer addResp.Body.Close()

	if addResp.StatusCode != http.StatusOK {
		t.Fatalf("add token status = %d, want 200", addResp.StatusCode)
	}

	var addResult Msg
	if err := json.NewDecoder(addResp.Body).Decode(&addResult); err != nil {
		t.Fatalf("decoding add response: %v", err)
	}
	if !addResult.Success {
		t.Fatalf("token creation returned failure: %s", addResult.Msg)
	}

	// Verify token appears in list
	listReq, err := http.NewRequest(http.MethodGet, srv.URL+"/apiv2/tokens", nil)
	if err != nil {
		t.Fatal(err)
	}
	listReq.Header.Set("Token", validToken)

	listResp, err := srv.Client().Do(listReq)
	if err != nil {
		t.Fatalf("list tokens failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list tokens status = %d, want 200", listResp.StatusCode)
	}
}
