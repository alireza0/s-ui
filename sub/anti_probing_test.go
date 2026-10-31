package sub

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"

	"github.com/gin-gonic/gin"
)

// The tests in this file pin the subscription side of the anti-probing
// posture discussed in #1316: this HTTP surface stays silent to
// unauthenticated probing. They lock in current behaviour only -- if one
// of them starts failing, a change has altered what an unauthenticated
// scanner can learn from this box, and that change should be deliberate,
// not accidental. The panel API side of the same posture lives in
// api/anti_probing_test.go.

func antiProbingDB(t *testing.T) {
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
}

// antiProbingSubEngine builds the subscription server's real router, the
// same engine sub.Server.Start serves.
func antiProbingSubEngine(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	engine, err := NewServer().initRouter()
	if err != nil {
		t.Fatal(err)
	}
	settings := &service.SettingService{}
	subPath, err := settings.GetSubPath()
	if err != nil {
		t.Fatal(err)
	}
	return engine, subPath
}

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

// The subscription server must not identify the software in a Server
// header: that string is a fingerprint scanners index on, and nothing in
// the stack needs to send it. The panel API is checked the same way in
// the api package.
func TestAntiProbingNoServerHeader(t *testing.T) {
	antiProbingDB(t)
	antiProbingSeedClients(t)

	subEngine, subPath := antiProbingSubEngine(t)
	subSrv := httptest.NewServer(subEngine)
	defer subSrv.Close()

	urls := []string{
		subSrv.URL + subPath + "regression-real", // subscription, 200
		subSrv.URL + subPath + "no-such-client",  // subscription, 400
	}
	for _, url := range urls {
		_, header, _ := antiProbingGet(t, url)
		if values, ok := header["Server"]; ok {
			t.Errorf("GET %s: Server header present (%v), want none", url, values)
		}
	}
}

// A subscription fetch for a name that cannot be served -- nonexistent,
// disabled, or in a format that fails -- must always get the same generic
// 400. If different failures answered differently, a prober could tell
// which client names exist on the box without ever authenticating.
func TestAntiProbingSubInvalidNameGeneric400(t *testing.T) {
	antiProbingDB(t)
	antiProbingSeedClients(t)

	engine, subPath := antiProbingSubEngine(t)
	srv := httptest.NewServer(engine)
	defer srv.Close()

	bad := []string{
		"no-such-client",
		"no-such-client?format=json",
		"no-such-client?format=clash",
		"regression-off", // exists but disabled: must answer like a missing one
	}
	var wantBody string
	for i, name := range bad {
		status, _, body := antiProbingGet(t, srv.URL+subPath+name)
		if status != http.StatusBadRequest {
			t.Errorf("GET %q: status %d, want 400", name, status)
		}
		if i == 0 {
			wantBody = string(body)
			if wantBody != "Error!" {
				t.Errorf("GET %q: body %q, want the generic %q", name, wantBody, "Error!")
			}
		} else if string(body) != wantBody {
			t.Errorf("GET %q: body %q, want it indistinguishable from %q", name, body, wantBody)
		}
	}

	// The HEAD variant of the same probe must fail the same way.
	resp, err := http.Head(srv.URL + subPath + "no-such-client")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("HEAD missing client: status %d, want 400", resp.StatusCode)
	}

	// Control: the enabled client still serves, so the identical 400s
	// above are the failure path, not a broken server.
	status, _, _ := antiProbingGet(t, srv.URL+subPath+"regression-real")
	if status != http.StatusOK {
		t.Errorf("GET enabled client: status %d, want 200", status)
	}
}
