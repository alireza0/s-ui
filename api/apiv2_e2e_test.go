package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func apiv2TestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatal(err)
	}

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

func TestAPIv2TokenAuthentication(t *testing.T) {
	srv, validToken := apiv2TestServer(t)

	testCases := []struct {
		name        string
		token       string
		sendHeader  bool
		wantSuccess bool
	}{
		{"missing token header", "", false, false},
		{"empty token", "", true, false},
		{"invalid token", "wrong-secret-token", true, false},
		{"valid token", validToken, true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/apiv2/status", nil)
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

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}

			var body Msg
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}

			if body.Success != tc.wantSuccess {
				t.Errorf("body.Success = %v, want %v (msg: %s)", body.Success, tc.wantSuccess, body.Msg)
			}
		})
	}
}

func TestAPIv2UnknownActionHandling(t *testing.T) {
	srv, validToken := apiv2TestServer(t)

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{"unknown GET action", http.MethodGet, "/apiv2/unknownAction123"},
		{"unknown POST action", http.MethodPost, "/apiv2/unknownAction123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, srv.URL+tc.path, nil)
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
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}

			var body Msg
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response: %v", err)
			}

			if body.Success {
				t.Error("expected success: false for unknown action")
			}
			if !strings.Contains(body.Msg, "unknown action") {
				t.Errorf("expected error message to mention 'unknown action', got %q", body.Msg)
			}
		})
	}
}

func TestAPIv2CoreGetActions(t *testing.T) {
	srv, validToken := apiv2TestServer(t)

	actions := []string{
		"/apiv2/status",
		"/apiv2/clients",
		"/apiv2/config",
		"/apiv2/inbounds",
		"/apiv2/outbounds",
		"/apiv2/tls",
		"/apiv2/users",
		"/apiv2/settings",
		"/apiv2/stats",
	}

	for _, path := range actions {
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
				t.Fatalf("%s HTTP status = %d, want 200", path, resp.StatusCode)
			}

			var body Msg
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decoding %s response: %v", path, err)
			}

			if !body.Success {
				t.Errorf("%s returned success: false, msg: %s", path, body.Msg)
			}
		})
	}
}
