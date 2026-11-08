package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDomainValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		domain, host string
		code         int
	}{
		{"panel.example", "panel.example", http.StatusOK},
		{"panel.example", "panel.example:2095", http.StatusOK},
		{"Panel.Example", "panel.example:2095", http.StatusOK},
		{"panel.example", "other.example:2095", http.StatusForbidden},
		{"192.0.2.1", "192.0.2.1:2095", http.StatusOK},
		{"2001:db8::1", "[2001:db8::1]:2095", http.StatusOK},
		// On the default port the IPv6 host has no port, only brackets.
		{"2001:db8::1", "[2001:db8::1]", http.StatusOK},
		{"2001:db8::1", "[2001:db8::2]", http.StatusForbidden},
	} {
		engine := gin.New()
		engine.Use(DomainValidator(tc.domain))
		engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = tc.host
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Errorf("domain %q, Host %q: got %d, want %d", tc.domain, tc.host, w.Code, tc.code)
		}
	}
}
