package service

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/alireza0/s-ui/database/model"

	"github.com/anytls/sing-anytls/padding"
)

// AnyTLS ships one padding scheme that every installation uses verbatim,
// and the AnyTLS FAQ lists that shared scheme as a fingerprint of its own:
// the packet-size distribution is identical everywhere. The scheme is only
// ever a shape, though -- the server pushes it to clients at runtime -- so
// an installation can safely carry its own variant, and the panel form
// offers exactly that choice: keep the default, or roll a generated one.
//
// This is the backstop for saves that bypass the form -- an API call, a
// config import -- and arrive with no scheme at all: generate one from the
// library default and store it with the inbound, where it stays visible
// and editable like any other setting, instead of letting the core fall
// back to the shared default silently at runtime. A scheme that is
// present -- the default included, once the form offers it as a choice --
// is the operator's and is never rewritten.

// paddingJitterPct bounds how far a range bound may move from the default,
// in percent, and paddingMaxBound caps a generated bound so a scheme stays
// in the size territory the default occupies.
const (
	paddingJitterPct = 20
	paddingMaxBound  = 1400
)

// ensureAnyTLSPaddingScheme fills inbound.Options with a generated padding
// scheme when the inbound is AnyTLS and carries no scheme at all.
func ensureAnyTLSPaddingScheme(inbound *model.Inbound) error {
	if inbound.Type != "anytls" || len(inbound.Options) == 0 {
		return nil
	}
	var options map[string]json.RawMessage
	if err := json.Unmarshal(inbound.Options, &options); err != nil {
		return err
	}
	if paddingSchemeState(options["padding_scheme"]) != schemeEmpty {
		return nil
	}
	scheme, err := generatePaddingScheme()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(scheme)
	if err != nil {
		return err
	}
	options["padding_scheme"] = raw
	inbound.Options, err = json.MarshalIndent(options, "", "  ")
	return err
}

const (
	schemeEmpty = iota
	schemeDefault
	schemeCustom
)

// paddingSchemeState classifies a raw padding_scheme value: absent or all
// blank lines is empty; equal to the library default line for line is the
// default; anything else is treated as the operator's own.
func paddingSchemeState(raw json.RawMessage) int {
	if len(raw) == 0 {
		return schemeEmpty
	}
	var lines []string
	if err := json.Unmarshal(raw, &lines); err != nil {
		// Not the array the panel sends; leave malformed values alone.
		return schemeCustom
	}
	kept := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return schemeEmpty
	}
	if strings.Join(kept, "\n") == string(padding.DefaultPaddingScheme) {
		return schemeDefault
	}
	return schemeCustom
}

// generatePaddingScheme derives one installation's scheme from the library
// default. The structure is preserved exactly -- same stop, same packet
// lines, same segment order, same check marks -- because the shape is what
// makes the traffic plausible. Only true ranges move: each bound is
// jittered by up to paddingJitterPct and rounded to the nearest ten, the
// way the default's own round numbers read. Fixed segments (30-30, 9-9)
// stay untouched; they are handshake-sized and are not where the
// fingerprint lives.
func generatePaddingScheme() ([]string, error) {
	lines := strings.Split(string(padding.DefaultPaddingScheme), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		key, value, found := strings.Cut(line, "=")
		if !found || key == "stop" {
			out = append(out, line)
			continue
		}
		segments := strings.Split(value, ",")
		for i, segment := range segments {
			lo, hi, ok := parsePaddingRange(segment)
			if !ok || lo == hi {
				continue
			}
			newLo, newHi, err := jitterPaddingRange(lo, hi)
			if err != nil {
				return nil, err
			}
			segments[i] = fmt.Sprintf("%d-%d", newLo, newHi)
		}
		out = append(out, key+"="+strings.Join(segments, ","))
	}
	return out, nil
}

func parsePaddingRange(segment string) (int, int, bool) {
	loText, hiText, found := strings.Cut(segment, "-")
	if !found {
		return 0, 0, false
	}
	lo, err := strconv.Atoi(loText)
	if err != nil {
		return 0, 0, false
	}
	hi, err := strconv.Atoi(hiText)
	if err != nil {
		return 0, 0, false
	}
	return lo, hi, true
}

// jitterPaddingRange moves both bounds of one range and then repairs the
// result so the range keeps a usable width: at least a quarter of the
// original width, and never less than 40.
func jitterPaddingRange(lo, hi int) (int, int, error) {
	newLo, err := jitterPaddingBound(lo)
	if err != nil {
		return 0, 0, err
	}
	newHi, err := jitterPaddingBound(hi)
	if err != nil {
		return 0, 0, err
	}
	if newHi > paddingMaxBound {
		newHi = paddingMaxBound
	}
	minWidth := (hi - lo) / 4
	if minWidth < 40 {
		minWidth = 40
	}
	if newHi-newLo < minWidth {
		newHi = newLo + minWidth
		if newHi > paddingMaxBound {
			newHi = paddingMaxBound
			newLo = newHi - minWidth
		}
	}
	return newLo, newHi, nil
}

func jitterPaddingBound(bound int) (int, error) {
	// A percentage in [-paddingJitterPct, +paddingJitterPct].
	n, err := rand.Int(rand.Reader, big.NewInt(2*paddingJitterPct+1))
	if err != nil {
		return 0, err
	}
	scaled := bound * (100 + int(n.Int64()) - paddingJitterPct) / 100
	scaled = (scaled + 5) / 10 * 10
	if scaled < 10 {
		scaled = 10
	}
	return scaled, nil
}
