package sub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sagernet/sing/common/json/badoption"
)

type subscriptionOutbound struct {
	Type           string              `json:"type"`
	Tag            string              `json:"tag"`
	ConnectTimeout *badoption.Duration `json:"connect_timeout,omitempty"`
}

func appendSubscriptionOutbounds(builtInAndNodes []map[string]interface{}, hasNodes bool, setting interface{}) ([]map[string]interface{}, error) {
	raw, err := json.Marshal(setting)
	if err != nil {
		return nil, err
	}
	var custom []subscriptionOutbound
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&custom); err != nil {
		return nil, fmt.Errorf("invalid custom_outbounds: %w", err)
	}
	if custom == nil {
		return nil, fmt.Errorf("custom_outbounds must be an array")
	}
	if len(custom) == 0 {
		return builtInAndNodes, nil
	}
	if !hasNodes {
		return nil, fmt.Errorf("cannot build custom sing-box outbounds for a subscription without nodes")
	}

	tags := make(map[string]bool, len(builtInAndNodes)+len(custom))
	for _, outbound := range builtInAndNodes {
		tag, _ := outbound["tag"].(string)
		if tags[tag] {
			return nil, fmt.Errorf("duplicate sing-box outbound tag %q", tag)
		}
		tags[tag] = true
	}
	for _, outbound := range custom {
		if strings.TrimSpace(outbound.Tag) == "" || outbound.Tag == subscriptionNodeTags || tags[outbound.Tag] {
			return nil, fmt.Errorf("invalid or duplicate custom sing-box outbound tag %q", outbound.Tag)
		}
		config := map[string]interface{}{"type": outbound.Type, "tag": outbound.Tag}
		switch outbound.Type {
		case "direct":
			if outbound.ConnectTimeout != nil {
				if *outbound.ConnectTimeout < 0 {
					return nil, fmt.Errorf("connect_timeout must be non-negative for direct outbound %q", outbound.Tag)
				}
				config["connect_timeout"] = outbound.ConnectTimeout
			}
		case "block":
			if outbound.ConnectTimeout != nil {
				return nil, fmt.Errorf("block outbound %q does not accept connect_timeout", outbound.Tag)
			}
		default:
			return nil, fmt.Errorf("unsupported custom sing-box outbound type %q", outbound.Type)
		}
		tags[outbound.Tag] = true
		builtInAndNodes = append(builtInAndNodes, config)
	}
	return builtInAndNodes, nil
}
