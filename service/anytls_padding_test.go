package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"github.com/anytls/sing-anytls/padding"
)

func defaultSchemeLines() []string {
	return strings.Split(string(padding.DefaultPaddingScheme), "\n")
}

// A generated scheme must keep the default's shape exactly -- same lines,
// same segments, same check marks, same fixed sizes -- and move only the
// ranges. Every expectation here is derived from the library default at
// runtime, and the generator's tuning values (jitter, cap, minimum width)
// are deliberately not asserted: changing the default scheme or retuning
// the generator must not break this test.
func TestGeneratePaddingSchemeKeepsShape(t *testing.T) {
	defaults := defaultSchemeLines()
	defaultFactory := padding.NewPaddingFactory(padding.DefaultPaddingScheme)
	if defaultFactory == nil {
		t.Fatal("library rejects its own default scheme")
	}
	seen := map[string]bool{}
	for run := 0; run < 100; run++ {
		scheme, err := generatePaddingScheme()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len(scheme) != len(defaults) {
			t.Fatalf("line count = %d, want %d", len(scheme), len(defaults))
		}
		joined := strings.Join(scheme, "\n")
		if joined == string(padding.DefaultPaddingScheme) {
			t.Fatalf("generated scheme equals the default:\n%s", joined)
		}
		seen[joined] = true
		factory := padding.NewPaddingFactory([]byte(joined))
		if factory == nil {
			t.Fatalf("library rejects generated scheme:\n%s", joined)
		}
		if factory.Stop != defaultFactory.Stop {
			t.Fatalf("stop = %d, want the default's %d", factory.Stop, defaultFactory.Stop)
		}
		for i, line := range scheme {
			defKey, defValue, _ := strings.Cut(defaults[i], "=")
			key, value, _ := strings.Cut(line, "=")
			if key != defKey {
				t.Fatalf("line %d key = %q, want %q", i, key, defKey)
			}
			if defKey == "stop" {
				if line != defaults[i] {
					t.Fatalf("stop line changed: %q", line)
				}
				continue
			}
			segments := strings.Split(value, ",")
			defSegments := strings.Split(defValue, ",")
			if len(segments) != len(defSegments) {
				t.Fatalf("line %d segment count = %d, want %d", i, len(segments), len(defSegments))
			}
			for j, segment := range segments {
				defLo, defHi, defOk := parsePaddingRange(defSegments[j])
				if defSegments[j] == "c" || !defOk || defLo == defHi {
					if segment != defSegments[j] {
						t.Fatalf("line %d segment %d = %q, want fixed %q", i, j, segment, defSegments[j])
					}
					continue
				}
				lo, hi, ok := parsePaddingRange(segment)
				if !ok {
					t.Fatalf("line %d segment %d unparsable: %q", i, j, segment)
				}
				if lo < 10 || hi > paddingMaxBound || lo >= hi {
					t.Fatalf("line %d segment %d out of bounds: %d-%d", i, j, lo, hi)
				}
			}
		}
	}
	if len(seen) < 90 {
		t.Fatalf("only %d distinct schemes in 100 runs", len(seen))
	}
}

func paddingTestInbound(t *testing.T, inboundType string, options map[string]interface{}) *model.Inbound {
	t.Helper()
	raw, err := json.MarshalIndent(options, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return &model.Inbound{Type: inboundType, Options: raw}
}

func paddingOf(t *testing.T, inbound *model.Inbound) []string {
	t.Helper()
	var options map[string]json.RawMessage
	if err := json.Unmarshal(inbound.Options, &options); err != nil {
		t.Fatal(err)
	}
	raw, ok := options["padding_scheme"]
	if !ok {
		return nil
	}
	var lines []string
	if err := json.Unmarshal(raw, &lines); err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestEnsureAnyTLSPaddingScheme(t *testing.T) {
	custom := []string{"stop=4", "0=20-20", "1=200-300", "2=600-900", "3=600-900"}

	cases := []struct {
		name        string
		inboundType string
		options     map[string]interface{}
		wantFilled  bool
		wantSame    bool
	}{
		{
			name: "without scheme is filled", inboundType: "anytls",
			options:    map[string]interface{}{"listen": "::"},
			wantFilled: true,
		},
		{
			name: "with blank lines is filled", inboundType: "anytls",
			options:    map[string]interface{}{"padding_scheme": []string{"", "  "}},
			wantFilled: true,
		},
		{
			name: "with cleared scheme is filled", inboundType: "anytls",
			options:    map[string]interface{}{"padding_scheme": []string{}},
			wantFilled: true,
		},
		{
			name: "with the default scheme is kept", inboundType: "anytls",
			options:  map[string]interface{}{"padding_scheme": defaultSchemeLines()},
			wantSame: true,
		},
		{
			name: "with custom scheme is kept", inboundType: "anytls",
			options:  map[string]interface{}{"padding_scheme": custom},
			wantSame: true,
		},
		{
			name: "other protocols are untouched", inboundType: "vless",
			options:  map[string]interface{}{"listen": "::"},
			wantSame: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inbound := paddingTestInbound(t, tc.inboundType, tc.options)
			before := string(inbound.Options)
			if err := ensureAnyTLSPaddingScheme(inbound); err != nil {
				t.Fatalf("ensure: %v", err)
			}
			if tc.wantSame {
				if string(inbound.Options) != before {
					t.Fatalf("options rewritten:\nbefore: %s\nafter:  %s", before, inbound.Options)
				}
				return
			}
			if tc.wantFilled {
				lines := paddingOf(t, inbound)
				if len(lines) == 0 {
					t.Fatal("padding_scheme still empty")
				}
				joined := strings.Join(lines, "\n")
				if joined == string(padding.DefaultPaddingScheme) {
					t.Fatal("padding_scheme is the library default")
				}
				if padding.NewPaddingFactory([]byte(joined)) == nil {
					t.Fatalf("library rejects filled scheme:\n%s", joined)
				}
				// Other options survive the rewrite.
				var options map[string]json.RawMessage
				if err := json.Unmarshal(inbound.Options, &options); err != nil {
					t.Fatal(err)
				}
				if _, ok := options["padding_scheme"]; !ok {
					t.Fatal("padding_scheme key missing after fill")
				}
			}
		})
	}
}

// The line keys must survive generation in order, or the scheme the
// library parses would describe different packets than the default does.
// The expected keys are the default's own, whatever they are.
func TestGeneratePaddingSchemeLineKeys(t *testing.T) {
	scheme, err := generatePaddingScheme()
	if err != nil {
		t.Fatal(err)
	}
	defaults := defaultSchemeLines()
	if len(scheme) != len(defaults) {
		t.Fatalf("line count = %d, want %d", len(scheme), len(defaults))
	}
	for i, line := range scheme {
		key, _, _ := strings.Cut(line, "=")
		want, _, _ := strings.Cut(defaults[i], "=")
		if key != want {
			t.Fatalf("line %d key = %q, want %q", i, key, want)
		}
	}
}
