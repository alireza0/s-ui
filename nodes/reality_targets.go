package nodes

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
)

var errRealityHandshakeLoop = errors.New("node_reality_handshake_loop")

type realityListener struct {
	Bind string `json:"listen"`
	Port int    `json:"listen_port"`
	TLS  struct {
		Enabled bool `json:"enabled"`
		Reality struct {
			Enabled   bool `json:"enabled"`
			Handshake struct {
				Server string `json:"server"`
				Port   int    `json:"server_port"`
			} `json:"handshake"`
		} `json:"reality"`
	} `json:"tls"`
}

func realityHost(host string) string {
	host = strings.Trim(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), "."), "[]")
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.WithZone("").Unmap().String()
	}
	return host
}

func (listener realityListener) receivesRealityTarget(host string, aliases map[string]bool) bool {
	target := realityHost(host)
	if target == "" {
		return false
	}
	bind := realityHost(listener.Bind)
	if target == bind {
		return true
	}
	ip, _ := netip.ParseAddr(target)
	loopback := ip.IsLoopback() || target == "localhost" || strings.HasSuffix(target, ".localhost")
	if bind != "" && bind != "::" && bind != "0.0.0.0" {
		bound, _ := netip.ParseAddr(bind)
		return !ip.IsValid() && (loopback && bound.IsLoopback() || aliases[target] && aliases[bind])
	}
	return (loopback || ip.IsUnspecified() || aliases[target]) && (bind != "0.0.0.0" || !ip.Is6())
}

// Check the resolved TCP listeners before a revision can be delivered. A
// local TLS service on a different socket is a valid REALITY target.
func validateRealityTargets(entries []SpecEntry, addresses []string) error {
	listeners := make(map[int]realityListener)
	for _, entry := range entries {
		if !entry.Enable || entry.Protocol != "vless" {
			continue
		}
		var listener realityListener
		if err := json.Unmarshal(entry.Options, &listener); err != nil {
			return err
		}
		if listener.TLS.Enabled && listener.TLS.Reality.Enabled {
			listeners[listener.Port] = listener
		}
	}
	aliases := make(map[string]bool, len(addresses))
	for _, address := range addresses {
		aliases[realityHost(address)] = true
	}
	next := make(map[int]int)
	for port, listener := range listeners {
		target := listener.TLS.Reality.Handshake
		if destination, exists := listeners[target.Port]; exists && destination.receivesRealityTarget(target.Server, aliases) {
			next[port] = target.Port
		}
	}
	return validateRealityGraph(next)
}

func validateRealityGraph(next map[int]int) error {
	visited := make(map[int]int)
	for start := range next {
		for port := start; port != 0; port = next[port] {
			if owner, seen := visited[port]; seen {
				if owner == start {
					return errRealityHandshakeLoop
				}
				break
			}
			visited[port] = start
		}
	}
	return nil
}
