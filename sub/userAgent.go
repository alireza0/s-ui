package sub

import "strings"

// formatForUserAgent picks the subscription format a well-known client reads
// when the request names none. It matches only the product token, the part
// before the first "/" or space: Hiddify announces itself "HiddifyNext/2.5
// (android) like ClashMeta v2ray sing-box", and it reads the plain links best.
// An unknown client gets "", the plain links.
func formatForUserAgent(userAgent string) string {
	product := strings.ToLower(strings.TrimSpace(userAgent))
	if i := strings.IndexAny(product, "/ "); i >= 0 {
		product = product[:i]
	}
	switch {
	case product == "happ":
		return "happ"
	// clash-verge, ClashX, ClashMetaForAndroid, ClashforWindows, FlClash,
	// clash-nyanpasu, mihomo and Stash all load a Clash/mihomo config.
	case strings.Contains(product, "clash"), product == "mihomo", product == "stash":
		return "clash"
	// The sing-box CLI and its own apps for Android, iOS, macOS and tvOS.
	case product == "sing-box", product == "sfa", product == "sfi", product == "sfm", product == "sft":
		return "json"
	}
	return ""
}
