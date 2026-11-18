package sub

import "strings"

// Happ accepts socks:// for SOCKS5 and has no URI format for HTTP proxies.
func happLinks(links []string) []string {
	result := links[:0]
	for _, link := range links {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		if scheme, rest, ok := strings.Cut(link, "://"); ok {
			switch strings.ToLower(scheme) {
			case "http", "https":
				continue
			case "socks5":
				link = "socks://" + rest
			}
		}
		result = append(result, link)
	}
	return result
}
