package sub

import (
	"encoding/json"
	"strconv"
	"strings"
)

// xhttpOptsFromTransport converts Xray VLESS URI parameters into Mihomo's
// xhttp-opts. Omitted and null values retain Mihomo defaults.
func xhttpOptsFromTransport(transport map[string]interface{}) map[string]interface{} {
	opts := make(map[string]interface{})
	for _, name := range []string{"host", "path", "mode"} {
		if s, ok := transport[name].(string); ok && s != "" {
			opts[name] = s
		}
	}
	extra, ok := transport["extra"].(map[string]interface{})
	if !ok {
		return opts
	}
	// An explicitly supplied URI parameter wins even when it is empty.
	for _, name := range []string{"host", "path", "mode"} {
		if _, hasTopLevel := transport[name]; !hasTopLevel {
			xhttpPutString(opts, name, extra[name])
		}
	}
	xhttpMapExtra(opts, extra)
	return opts
}

func xhttpPutString(dst map[string]interface{}, key string, raw interface{}) {
	if val, ok := raw.(string); ok && val != "" {
		dst[key] = val
	}
}

func xhttpPutBool(dst map[string]interface{}, key string, raw interface{}) {
	if val, ok := raw.(bool); ok {
		dst[key] = val
	}
}

func xhttpInteger(raw interface{}) (int64, bool) {
	var value string
	switch n := raw.(type) {
	case string:
		value = strings.TrimSpace(n)
	case json.Number:
		value = string(n)
	case float64:
		value = strconv.FormatFloat(n, 'f', -1, 64)
	case int:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil
}

// Mihomo's ranges are strings, including scalar values. Match Xray's int32
// bounds so malformed large ranges cannot overflow the client's random sampler.
func xhttpPutRange(dst map[string]interface{}, key string, raw interface{}, positive bool) {
	value, ok := raw.(string)
	if !ok {
		if number, valid := xhttpInteger(raw); valid {
			value = strconv.FormatInt(number, 10)
		} else {
			return
		}
	}
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 1 || len(parts) > 2 {
		return
	}
	minimum, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || minimum < 0 || minimum > 2147483647 || (positive && minimum == 0) {
		return
	}
	maximum := minimum
	if len(parts) == 2 {
		maximum, err = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || maximum < minimum || maximum > 2147483647 {
			return
		}
	}
	value = strconv.FormatInt(minimum, 10)
	if len(parts) == 2 {
		value += "-" + strconv.FormatInt(maximum, 10)
	}
	dst[key] = value
}

func xhttpPutHeaders(dst map[string]interface{}, raw interface{}) {
	headers, ok := raw.(map[string]interface{})
	if !ok {
		return
	}
	valid := make(map[string]interface{})
	for key, rawValue := range headers {
		if value, ok := rawValue.(string); ok {
			valid[key] = value
		}
	}
	if len(valid) > 0 {
		dst["headers"] = valid
	}
}

// xhttpMapReuse is the Xray xmux -> Mihomo reuse-settings mapping.
func xhttpMapReuse(input map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for from, to := range map[string]string{
		"maxConcurrency":   "max-concurrency",
		"maxConnections":   "max-connections",
		"cMaxReuseTimes":   "c-max-reuse-times",
		"hMaxRequestTimes": "h-max-request-times",
		"hMaxReusableSecs": "h-max-reusable-secs",
	} {
		xhttpPutRange(out, to, input[from], false)
	}
	if value, ok := xhttpInteger(input["hKeepAlivePeriod"]); ok {
		out["h-keep-alive-period"] = value
	}
	return out
}

func xhttpMapExtra(opts map[string]interface{}, extra map[string]interface{}) {
	xhttpPutHeaders(opts, extra["headers"])
	for from, to := range map[string]string{
		"xPaddingKey":         "x-padding-key",
		"xPaddingHeader":      "x-padding-header",
		"xPaddingPlacement":   "x-padding-placement",
		"xPaddingMethod":      "x-padding-method",
		"uplinkHTTPMethod":    "uplink-http-method",
		"sessionIDPlacement":  "session-placement",
		"sessionIDKey":        "session-key",
		"sessionIDTable":      "session-table",
		"seqPlacement":        "seq-placement",
		"seqKey":              "seq-key",
		"uplinkDataPlacement": "uplink-data-placement",
		"uplinkDataKey":       "uplink-data-key",
	} {
		xhttpPutString(opts, to, extra[from])
	}
	for from, to := range map[string]string{
		"noGRPCHeader":     "no-grpc-header",
		"xPaddingObfsMode": "x-padding-obfs-mode",
	} {
		xhttpPutBool(opts, to, extra[from])
	}
	// Xray fills these when obfuscation is enabled; Mihomo leaves them empty.
	if enabled, _ := opts["x-padding-obfs-mode"].(bool); enabled {
		for key, value := range map[string]string{
			"x-padding-key":       "x_padding",
			"x-padding-header":    "X-Padding",
			"x-padding-placement": "queryInHeader",
			"x-padding-method":    "repeat-x",
		} {
			if _, exists := opts[key]; !exists {
				opts[key] = value
			}
		}
	}
	xhttpPutRange(opts, "x-padding-bytes", extra["xPaddingBytes"], true)
	// Xray-generated zeroes mean defaults. Mihomo rejects explicit sc-* zeroes;
	// omission preserves its defaults of 1000000 bytes and 30 milliseconds.
	for from, to := range map[string]string{
		"scMaxEachPostBytes":   "sc-max-each-post-bytes",
		"scMinPostsIntervalMs": "sc-min-posts-interval-ms",
		"sessionIDLength":      "session-length",
		"uplinkChunkSize":      "uplink-chunk-size",
	} {
		xhttpPutRange(opts, to, extra[from], true)
	}
	if xmux, ok := extra["xmux"].(map[string]interface{}); ok && len(xmux) > 0 {
		if mapped := xhttpMapReuse(xmux); len(mapped) > 0 {
			opts["reuse-settings"] = mapped
		}
	}
	// downloadSettings is an Xray StreamConfig, not a flat XHTTP settings map.
	if download, ok := extra["downloadSettings"].(map[string]interface{}); ok && len(download) > 0 {
		if mapped := xhttpMapDownload(download); len(mapped) > 0 {
			opts["download-settings"] = mapped
		}
	}
}

func xhttpMapDownload(download map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	xhttpPutString(out, "server", download["address"])
	if port, ok := xhttpInteger(download["port"]); ok && port > 0 && port <= 65535 {
		out["port"] = port
	}
	settingsName := ""
	if security, ok := download["security"].(string); ok {
		switch security {
		case "tls":
			out["tls"] = true
			settingsName = "tlsSettings"
		case "reality":
			out["tls"] = true
			settingsName = "realitySettings"
		case "", "none":
			out["tls"] = false
		}
	}
	if _, selected := out["tls"]; selected {
		// Mihomo inherits omitted download TLS fields from the uplink. Xray
		// selects an independent security mode, with verification on by default.
		out["skip-cert-verify"] = false
		out["reality-opts"] = map[string]interface{}{}
	}
	// Xray may retain settings for inactive security modes. Export only the
	// selected mode so stale REALITY settings cannot enable it for a TLS link.
	if settings, ok := download[settingsName].(map[string]interface{}); ok && settingsName != "" {
		xhttpPutString(out, "servername", settings["serverName"])
		xhttpPutString(out, "client-fingerprint", settings["fingerprint"])
		xhttpPutBool(out, "skip-cert-verify", settings["allowInsecure"])
		if rawALPN, ok := settings["alpn"].([]interface{}); ok {
			alpn := make([]string, 0, len(rawALPN))
			for _, value := range rawALPN {
				if protocol, ok := value.(string); ok && protocol != "" {
					alpn = append(alpn, protocol)
				}
			}
			if len(alpn) > 0 {
				out["alpn"] = alpn
			}
		}
		if settingsName == "realitySettings" {
			reality := make(map[string]interface{})
			xhttpPutString(reality, "public-key", settings["publicKey"])
			xhttpPutString(reality, "short-id", settings["shortId"])
			if len(reality) > 0 {
				out["reality-opts"] = reality
			}
		}
	}
	if settings, ok := download["xhttpSettings"].(map[string]interface{}); ok {
		for _, name := range []string{"host", "path"} {
			xhttpPutString(out, name, settings[name])
		}
		xhttpPutHeaders(out, settings["headers"])
		if extra, ok := settings["extra"].(map[string]interface{}); ok {
			if xmux, ok := extra["xmux"].(map[string]interface{}); ok {
				if reuse := xhttpMapReuse(xmux); len(reuse) > 0 {
					out["reuse-settings"] = reuse
				}
			}
		}
	}
	return out
}
