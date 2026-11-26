package nodes

import (
	"testing"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
)

func TestNodeTrafficDoesNotCountRetriesOrApplyToQuota(t *testing.T) {
	db, node, _ := fixture(t)
	if err := db.AutoMigrate(&model.Stats{}); err != nil {
		t.Fatal(err)
	}
	const first = "11111111-1111-4111-8111-111111111111"
	const second = "22222222-2222-4222-8222-222222222222"
	now := time.Unix(3600, 0)
	for _, reading := range []struct {
		after    time.Duration
		process  string
		up, down int64
	}{
		{time.Second, first, 100, 200},
		{2 * time.Second, first, 100, 200},
		{3 * time.Second, first, 220, 370},
		{5 * time.Second, second, 6, 12},
		{4 * time.Second, first, 230, 390},
	} {
		sample := &nodeagent.TrafficSample{Process: reading.process, Up: reading.up, Down: reading.down}
		if err := recordNodeTraffic(db, node, sample, now.Add(reading.after), 60); err != nil {
			t.Fatal(err)
		}
	}
	var rows []model.Stats
	if err := db.Order("direction").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Traffic != 382 || rows[1].Traffic != 226 || rows[0].Resource != "node" || rows[1].Resource != "node" {
		t.Fatal("wrong per-node totals or traffic recorded as client usage", rows)
	}
}
