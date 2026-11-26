package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNodeRoutesUsePanelAuthentication(t *testing.T) {
	engine := loginEngine(t)

	session := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	engine.ServeHTTP(session, request)
	if session.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous session request returned %d", session.Code)
	}

	token := httptest.NewRecorder()
	engine.ServeHTTP(token, httptest.NewRequest(http.MethodGet, "/apiv2/nodes", nil))
	if !strings.Contains(token.Body.String(), "invalid token") {
		t.Fatal("anonymous token request did not hit the token check")
	}

	login := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("user=admin&pass=admin"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	engine.ServeHTTP(login, request)

	request = httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	request.AddCookie(findSessionCookie(t, login.Result()))
	authenticated := httptest.NewRecorder()
	engine.ServeHTTP(authenticated, request)
	if authenticated.Code != http.StatusOK || !strings.Contains(authenticated.Body.String(), "nodetemplates") {
		t.Fatalf("authenticated node request returned %d: %s", authenticated.Code, authenticated.Body.String())
	}
}
