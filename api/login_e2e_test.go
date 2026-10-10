package api

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/middleware"

	"net/http/httptest"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// loginEngine wires the same pieces web.go does: the cookie session store with
// the shared options, and the cookie-authenticated api group behind the
// same-origin check.
func loginEngine(t *testing.T) *gin.Engine {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if db := database.GetDB(); db != nil {
			if sqlDB, err := db.DB(); err == nil {
				sqlDB.Close()
			}
		}
	})

	engine := gin.New()
	store := cookie.NewStore([]byte("test-secret-not-used-anywhere-else"))
	store.Options(BaseSessionOptions(0))
	engine.Use(sessions.Sessions("s-ui", store))

	apiv2 := NewAPIv2Handler(engine.Group("/apiv2"))
	NewAPIHandler(engine.Group("/api", middleware.SameOrigin()), apiv2)
	return engine
}

func findSessionCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == "s-ui" {
			return c
		}
	}
	t.Fatalf("no s-ui cookie in the login response (status %d, cookies %v)", resp.StatusCode, resp.Cookies())
	return nil
}

// The panel has to work on plain HTTP and on HTTPS, so the login path is walked
// in both. InitDB seeds admin/admin.
func TestLoginIssuesAUsableCookieInBothModes(t *testing.T) {
	testCases := []struct {
		name       string
		serve      func(http.Handler) *httptest.Server
		wantSecure bool
	}{
		{"http", httptest.NewServer, false},
		{"https", httptest.NewTLSServer, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := tc.serve(loginEngine(t))
			defer srv.Close()

			jar, _ := cookiejar.New(nil)
			client := srv.Client()
			client.Jar = jar
			if transport, ok := client.Transport.(*http.Transport); ok && transport.TLSClientConfig != nil {
				transport.TLSClientConfig.InsecureSkipVerify = true
			} else {
				client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			}

			form := url.Values{"user": {"admin"}, "pass": {"admin"}}
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/login", strings.NewReader(form.Encode()))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", srv.URL)

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("login request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("login status = %d, want 200", resp.StatusCode)
			}

			c := findSessionCookie(t, resp)
			if !c.HttpOnly {
				t.Error("the session cookie is readable by script")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Errorf("SameSite = %v, want Strict", c.SameSite)
			}
			if c.Secure != tc.wantSecure {
				t.Errorf("Secure = %v, want %v -- on %s a wrong value means the browser never sends the cookie back",
					c.Secure, tc.wantSecure, tc.name)
			}

			// The cookie has to actually authenticate the next request, which
			// is what a Secure flag set in HTTP mode would silently break.
			follow, err := http.NewRequest(http.MethodGet, srv.URL+"/api/settings", nil)
			if err != nil {
				t.Fatal(err)
			}
			follow.Header.Set("X-Requested-With", "XMLHttpRequest")
			resp2, err := client.Do(follow)
			if err != nil {
				t.Fatalf("follow-up request: %v", err)
			}
			defer resp2.Body.Close()

			body := make([]byte, 256)
			n, _ := resp2.Body.Read(body)
			if strings.Contains(string(body[:n]), "Invalid login") {
				t.Errorf("the session did not carry over to the next request: %s", body[:n])
			}
		})
	}
}

// A cross-site page can make the browser post with the session cookie attached.
// What it cannot do is forge the Origin, or set a custom header without the
// panel answering a CORS preflight it never answers.
func TestSameOriginGuardsStateChangingRequests(t *testing.T) {
	srv := httptest.NewServer(loginEngine(t))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")

	testCases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"same origin", map[string]string{"Origin": srv.URL}, http.StatusOK},
		{"same origin via referer", map[string]string{"Referer": srv.URL + "/app/"}, http.StatusOK},
		{"no headers but the xhr marker", map[string]string{"X-Requested-With": "XMLHttpRequest"}, http.StatusOK},
		{"foreign origin", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"foreign referer", map[string]string{"Referer": "https://evil.example/x"}, http.StatusForbidden},
		{"nothing at all", nil, http.StatusForbidden},
		{"origin with the host as a prefix", map[string]string{"Origin": "http://" + host + ".evil.example"}, http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{"user": {"nobody"}, "pass": {"nothing"}}
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/login", strings.NewReader(form.Encode()))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

// Reads are not blocked: the guard only covers requests that change something.
func TestSameOriginAllowsReads(t *testing.T) {
	srv := httptest.NewServer(loginEngine(t))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/settings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		t.Error("a GET was rejected by the same-origin guard")
	}
}
