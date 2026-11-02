package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func enableTestService(t *testing.T) *InboundService {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	prev := corePtr
	corePtr = core.NewCore()
	t.Cleanup(func() { corePtr = prev })
	return &InboundService{}
}

func saveInbound(t *testing.T, s *InboundService, act string, data string) {
	t.Helper()
	tx := database.GetDB().Begin()
	if err := s.Save(tx, act, json.RawMessage(data), "", "localhost"); err != nil {
		tx.Rollback()
		t.Fatalf("save %s: %v", act, err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
}

func inboundEnabled(t *testing.T, tag string) bool {
	t.Helper()
	var inbound model.Inbound
	if err := database.GetDB().Where("tag = ?", tag).First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	return inbound.Enable
}

// GORM leaves a false bool out of an INSERT when the column has a default, so
// a new inbound saved as disabled came back enabled.
func TestSaveNewInboundKeepsEnableFlag(t *testing.T) {
	s := enableTestService(t)

	saveInbound(t, s, "new", `{"type":"mixed","tag":"on","listen":"::","listen_port":1080}`)
	saveInbound(t, s, "new", `{"type":"mixed","tag":"off","enable":false,"listen":"::","listen_port":1081}`)

	if !inboundEnabled(t, "on") {
		t.Fatal("an inbound saved without enable must default to enabled")
	}
	if inboundEnabled(t, "off") {
		t.Fatal("an inbound saved with enable=false was stored enabled")
	}
}

func TestDisabledInboundIsLeftOutOfConfig(t *testing.T) {
	s := enableTestService(t)

	saveInbound(t, s, "new", `{"type":"mixed","tag":"on","listen":"::","listen_port":1080}`)
	saveInbound(t, s, "new", `{"type":"mixed","tag":"off","listen":"::","listen_port":1081}`)

	var id uint
	database.GetDB().Model(model.Inbound{}).Select("id").Where("tag = ?", "off").Scan(&id)
	saveInbound(t, s, "edit", `{"id":`+itoa(id)+`,"type":"mixed","tag":"off","enable":false,"listen":"::","listen_port":1081}`)

	configs, err := s.GetAllConfig(database.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	joined := string(json.RawMessage(strings.Join(rawStrings(configs), ",")))
	if !strings.Contains(joined, `"tag":"on"`) {
		t.Fatalf("enabled inbound missing from config: %s", joined)
	}
	if strings.Contains(joined, `"tag":"off"`) {
		t.Fatalf("disabled inbound present in config: %s", joined)
	}
	if strings.Contains(joined, `"enable"`) {
		t.Fatalf("enable is a panel field and must not reach sing-box: %s", joined)
	}
}

func TestInboundEnableRoundTrip(t *testing.T) {
	var inbound model.Inbound
	if err := inbound.UnmarshalJSON([]byte(`{"type":"mixed","tag":"t","enable":false,"listen_port":1}`)); err != nil {
		t.Fatal(err)
	}
	if inbound.Enable {
		t.Fatal("enable=false was not read")
	}
	if strings.Contains(string(inbound.Options), "enable") {
		t.Fatalf("enable leaked into options: %s", inbound.Options)
	}
	full, err := inbound.MarshalFull()
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := (*full)["enable"].(bool); !ok || v {
		t.Fatalf("MarshalFull enable = %v, want false", (*full)["enable"])
	}
}

func itoa(n uint) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func rawStrings(raws []json.RawMessage) []string {
	out := make([]string, len(raws))
	for i, r := range raws {
		out[i] = string(r)
	}
	return out
}
