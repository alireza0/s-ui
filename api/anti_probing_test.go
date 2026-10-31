package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// The tests in this file pin the panel API side of the anti-probing
// posture discussed in #1316: this HTTP surface stays silent to
// unauthenticated probing. They lock in current behaviour only -- if one
// of them starts failing, a change has altered what an unauthenticated
// scanner can learn from this box, and that change should be deliberate,
// not accidental. The subscription side of the same posture lives in
// sub/anti_probing_test.go.
//
// Both tests run on loginEngine from login_e2e_test.go, the panel's own
// test harness, so no router wiring is copied here.

// antiProbingSeedClients inserts the two clients the unauthenticated-API
// test probes for. Call it after loginEngine, which initialises the
// database.
func antiProbingSeedClients(t *testing.T) {
	t.Helper()
	db := database.GetDB()
	clients := []model.Client{
		{Name: "regression-real", Enable: true},
		{Name: "regression-off", Enable: false},
	}
	for i := range clients {
		clients[i].Config = json.RawMessage(`{}`)
		clients[i].Inbounds = json.RawMessage(`[]`)
		clients[i].Links = json.RawMessage(`[]`)
		if err := db.Create(&clients[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func antiProbingGet(t *testing.T, url string) (int, http.Header, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, body
}

// The panel API must not identify the software in a Server header: that
// string is a fingerprint scanners index on, and nothing in the stack
// needs to send it. The subscription server is checked the same way in
// the sub package.
func TestAntiProbingNoServerHeader(t *testing.T) {
	srv := httptest.NewServer(loginEngine(t))
	defer srv.Close()

	_, header, _ := antiProbingGet(t, srv.URL+"/api/clients") // panel API, unauthenticated
	if values, ok := header["Server"]; ok {
		t.Errorf("GET /api/clients: Server header present (%v), want none", values)
	}
}

// Panel data actions refuse unauthenticated callers without giving
// anything away: an XHR caller (the panel frontend's shape) gets a bare
// 401 with no success flag and no data object, and a plain request is
// bounced to the login page. Neither answer may carry data -- the seeded
// client name must not appear anywhere in either body.
func TestAntiProbingUnauthenticatedAPI(t *testing.T) {
	engine := loginEngine(t)
	antiProbingSeedClients(t)

	srv := httptest.NewServer(engine)
	defer srv.Close()

	for _, action := range []string{"clients", "load"} {
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/"+action, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("XHR GET /api/%s unauthenticated: status %d, want 401", action, resp.StatusCode)
		}
		var msg struct {
			Success bool            `json:"success"`
			Msg     string          `json:"msg"`
			Obj     json.RawMessage `json:"obj"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			t.Fatalf("XHR GET /api/%s unauthenticated: undecodable body %q: %v", action, body, err)
		}
		if msg.Success {
			t.Errorf("XHR GET /api/%s unauthenticated: success=true, want false", action)
		}
		if msg.Msg != "Invalid login" {
			t.Errorf("XHR GET /api/%s unauthenticated: msg %q, want %q", action, msg.Msg, "Invalid login")
		}
		if string(msg.Obj) != "null" {
			t.Errorf("XHR GET /api/%s unauthenticated: obj %s, want null (no data)", action, msg.Obj)
		}
	}

	// The non-XHR shape: a bare redirect to the login page, no data.
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := noFollow.Get(srv.URL + "/api/clients")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("plain GET /api/clients unauthenticated: status %d, want 307", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("plain GET /api/clients unauthenticated: Location %q, want /login", loc)
	}
	if strings.Contains(string(body), "regression-real") {
		t.Errorf("plain GET /api/clients unauthenticated: body leaks data: %q", body)
	}
}
