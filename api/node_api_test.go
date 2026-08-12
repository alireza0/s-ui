package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/gin-gonic/gin"
)

func setupAPINodeDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if err := database.InitDB(t.TempDir() + "/api-node.db"); err != nil {
		t.Fatal(err)
	}
}

func TestSaveNodeAndListAPI(t *testing.T) {
	setupAPINodeDB(t)
	a := &ApiService{}

	body := map[string]interface{}{
		"name":                "de1",
		"address":             "203.0.113.10",
		"port":                2095,
		"scheme":              "http",
		"apiToken":            "tok",
		"publicHost":          "de.example.com",
		"allowPrivateAddress": true,
	}
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/saveNode", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	a.SaveNode(c)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var msg Msg
	if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Success {
		t.Fatalf("save failed: %+v", msg)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	a.GetNodes(c)
	if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.Success {
		t.Fatalf("list failed: %+v", msg)
	}
	b, _ := json.Marshal(msg.Obj)
	var nodes []model.Node
	if err := json.Unmarshal(b, &nodes); err != nil || len(nodes) != 1 {
		t.Fatalf("obj=%T %v err=%v", msg.Obj, msg.Obj, err)
	}
	if nodes[0].Name != "de1" || nodes[0].PublicHost != "de.example.com" {
		t.Fatalf("node=%+v", nodes[0])
	}

	id := nodes[0].Id
	form := url.Values{}
	upd := map[string]interface{}{
		"id":                  id,
		"name":                "de1",
		"address":             "203.0.113.10",
		"port":                2095,
		"scheme":              "http",
		"apiToken":            "",
		"remark":              "updated",
		"allowPrivateAddress": true,
	}
	raw, _ = json.Marshal(upd)
	form.Set("data", string(raw))
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPost, "/api/saveNode", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req
	a.SaveNode(c)
	if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil || !msg.Success {
		t.Fatalf("update failed: %s", w.Body.String())
	}
	got, err := a.NodeService.GetById(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Remark != "updated" || got.ApiToken != "tok" {
		t.Fatalf("preserve token/remark failed: %+v", got)
	}

	form = url.Values{}
	form.Set("id", fmt.Sprintf("%d", id))
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPost, "/api/deleteNode", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req
	a.DeleteNode(c)
	if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil || !msg.Success {
		t.Fatalf("delete failed: %s", w.Body.String())
	}
	all, _ := a.NodeService.GetAll()
	if len(all) != 0 {
		t.Fatalf("expected deleted, got %d", len(all))
	}
}

func TestParseIDFormOrQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/node?id=12", nil)
	id, err := parseIDFormOrQuery(c)
	if err != nil || id != 12 {
		t.Fatalf("id=%d err=%v", id, err)
	}
}
