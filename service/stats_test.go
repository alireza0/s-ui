package service

import (
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// Retried cycles fold into one row per (resource, tag, direction) instead of
// piling up (#1294).
func TestMergeStatsFoldsRetries(t *testing.T) {
	var rows []model.Stats
	for i := 0; i < 100; i++ {
		rows = append(rows,
			model.Stats{Resource: "user", Tag: "alice", Direction: true, Traffic: 1},
			model.Stats{Resource: "user", Tag: "alice", Direction: false, Traffic: 2},
			model.Stats{Resource: "inbound", Tag: "alice", Direction: true, Traffic: 3},
		)
	}
	merged := mergeStats(rows)
	if len(merged) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(merged))
	}
	for i, want := range []int64{100, 200, 300} {
		if merged[i].Traffic != want {
			t.Fatalf("row %d: expected traffic %d, got %d", i, want, merged[i].Traffic)
		}
	}
}
