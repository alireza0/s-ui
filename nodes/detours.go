package nodes

import (
	"encoding/json"
	"errors"

	"github.com/alireza0/s-ui/database/model"
)

var errNodeDetour = errors.New("invalid_node_detour")

func compileNodeDetours(inbounds []model.NodeInbound) (map[string]string, error) {
	type entry struct {
		tag       string
		tcpVLESS  bool
		shadowTLS bool
		detour    string
	}
	entries := map[string]entry{}
	for _, inbound := range inbounds {
		var options struct {
			Transport listenerTransport `json:"transport"`
			Detour    string            `json:"detour"`
		}
		if err := json.Unmarshal(inbound.Options, &options); err != nil {
			return nil, err
		}
		entries[inbound.ConnectionKey] = entry{
			tag:       serverTag(inbound.NodeId, inbound.Id),
			tcpVLESS:  inbound.Type == "vless" && options.Transport.Type != "quic",
			shadowTLS: inbound.Type == "shadowtls",
			detour:    options.Detour,
		}
	}
	resolved := map[string]string{}
	for key, e := range entries {
		if e.detour == "" {
			continue
		}
		next := entries[e.detour]
		if !(e.tcpVLESS || e.shadowTLS) || !next.tcpVLESS {
			return nil, errNodeDetour
		}
		seen := map[string]bool{key: true}
		for target := e.detour; target != ""; target = entries[target].detour {
			if seen[target] {
				return nil, errNodeDetour
			}
			seen[target] = true
		}
		resolved[key] = next.tag
	}
	return resolved, nil
}
