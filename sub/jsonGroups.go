package sub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sagernet/sing/common/json/badoption"
)

const subscriptionNodeTags = "$nodes"

type subscriptionGroup struct {
	Type                      string             `json:"type"`
	Tag                       string             `json:"tag"`
	Outbounds                 []string           `json:"outbounds"`
	Default                   string             `json:"default,omitempty"`
	InterruptExistConnections bool               `json:"interrupt_exist_connections,omitempty"`
	URL                       string             `json:"url,omitempty"`
	Interval                  badoption.Duration `json:"interval,omitempty"`
	Tolerance                 uint16             `json:"tolerance,omitempty"`
	IdleTimeout               badoption.Duration `json:"idle_timeout,omitempty"`
}

func mergeSubscriptionGroups(builtInAndNodes []map[string]interface{}, nodeTags []string, setting interface{}) ([]interface{}, error) {
	raw, err := json.Marshal(setting)
	if err != nil {
		return nil, err
	}
	var groups []subscriptionGroup
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&groups); err != nil {
		return nil, fmt.Errorf("invalid outbound_groups: %w", err)
	}
	if groups == nil {
		return nil, fmt.Errorf("outbound_groups must be an array")
	}
	if len(groups) == 0 {
		return nil, nil
	}
	if len(nodeTags) == 0 {
		return nil, fmt.Errorf("cannot build custom sing-box outbound groups for a subscription without nodes")
	}
	tags := make(map[string]bool, len(builtInAndNodes)+len(groups))
	merged := make([]interface{}, 0, len(builtInAndNodes)+len(groups))
	for _, outbound := range builtInAndNodes {
		tag, _ := outbound["tag"].(string)
		if tags[tag] {
			return nil, fmt.Errorf("duplicate sing-box outbound tag %q", tag)
		}
		tags[tag] = true
		merged = append(merged, outbound)
	}
	for _, group := range groups {
		switch group.Type {
		case "selector":
			if group.URL != "" || group.Interval != 0 || group.Tolerance != 0 || group.IdleTimeout != 0 {
				return nil, fmt.Errorf("sing-box selector %q contains urltest options", group.Tag)
			}
		case "urltest":
			if group.Default != "" {
				return nil, fmt.Errorf("sing-box urltest group %q cannot specify a default", group.Tag)
			}
		default:
			return nil, fmt.Errorf("unsupported sing-box outbound group type %q", group.Type)
		}
		if group.Tag == "" || group.Tag == subscriptionNodeTags || tags[group.Tag] {
			return nil, fmt.Errorf("invalid or duplicate sing-box outbound group tag %q", group.Tag)
		}
		tags[group.Tag] = true
	}
	dependencies := make(map[string][]string, len(groups))
	for _, group := range groups {
		members := make([]string, 0, len(group.Outbounds))
		for _, tag := range group.Outbounds {
			if tag == subscriptionNodeTags {
				members = append(members, nodeTags...)
			} else if tags[tag] {
				members = append(members, tag)
			} else {
				return nil, fmt.Errorf("sing-box outbound group %q references unknown outbound %q", group.Tag, tag)
			}
		}
		if len(members) == 0 {
			return nil, fmt.Errorf("sing-box outbound group %q has no members", group.Tag)
		}
		group.Outbounds = members[:0]
		seen := make(map[string]bool, len(members))
		for _, tag := range members {
			if !seen[tag] {
				seen[tag] = true
				group.Outbounds = append(group.Outbounds, tag)
			}
		}
		if group.Default == subscriptionNodeTags {
			group.Default = nodeTags[0]
		}
		if group.Default != "" && !slices.Contains(group.Outbounds, group.Default) {
			return nil, fmt.Errorf("default %q is not a member of sing-box outbound group %q", group.Default, group.Tag)
		}
		dependencies[group.Tag] = group.Outbounds
		merged = append(merged, group)
	}
	visiting := make(map[string]bool)
	var checkGroup func(string) error
	checkGroup = func(tag string) error {
		if visiting[tag] {
			return fmt.Errorf("circular dependency in sing-box outbound group %q", tag)
		}
		members, ok := dependencies[tag]
		if !ok {
			return nil
		}
		visiting[tag] = true
		for _, member := range members {
			if err := checkGroup(member); err != nil {
				return err
			}
		}
		delete(visiting, tag)
		delete(dependencies, tag)
		return nil
	}
	for _, group := range groups {
		if err := checkGroup(group.Tag); err != nil {
			return nil, err
		}
	}
	return merged, nil
}
