package sub

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util"
)

type Link struct {
	Type   string `json:"type"`
	Remark string `json:"remark"`
	Uri    string `json:"uri"`
}

type LinkService struct {
}

func (s *LinkService) GetLinks(linkJson *json.RawMessage, types string, clientInfo string) []string {
	links := []Link{}
	var result []string
	err := json.Unmarshal(*linkJson, &links)
	if err != nil {
		return nil
	}
	for _, link := range links {
		switch link.Type {
		case "external":
			result = append(result, link.Uri)
		case "sub":
			subLinks := util.GetExternalLink(link.Uri)
			result = append(result, strings.Split(subLinks, "\n")...)
		case "local":
			if types == "all" {
				result = append(result, s.addClientInfo(link.Uri, clientInfo))
			}
		}
	}
	return result
}

func (s *LinkService) GetExternalOutbounds(linkJson *json.RawMessage) ([]map[string]interface{}, []string) {
	return s.getExternalOutbounds(linkJson, false)
}

func (s *LinkService) GetExternalOutboundsForClash(linkJson *json.RawMessage) ([]map[string]interface{}, []string) {
	return s.getExternalOutbounds(linkJson, true)
}

func (s *LinkService) getExternalOutbounds(linkJson *json.RawMessage, forClash bool) ([]map[string]interface{}, []string) {
	links := []Link{}
	err := json.Unmarshal(*linkJson, &links)
	if err != nil {
		return nil, nil
	}

	var outbounds []map[string]interface{}
	var tags []string
	parseOutbound := util.GetOutbound
	parseSub := util.GetExternalSub
	if forClash {
		parseOutbound = util.GetOutboundForClash
		parseSub = util.GetExternalSubForClash
	}

	for _, link := range links {
		switch link.Type {
		case "external":
			outbound, tag, err := parseOutbound(link.Uri, 0)
			if err == nil && outbound != nil && len(tag) > 0 {
				outbounds = append(outbounds, *outbound)
				tags = append(tags, tag)
			}
		case "sub":
			subOutbounds, err := parseSub(link.Uri)
			if err != nil {
				logger.Warning("sub: Error getting external sub:", err)
				continue
			}
			for _, outbound := range subOutbounds {
				if tag, _ := outbound["tag"].(string); len(tag) > 0 {
					outbounds = append(outbounds, outbound)
					tags = append(tags, tag)
				}
			}
		}
	}

	// Make tags unique; sing-box and clash reject duplicate tags/names.
	seen := make(map[string]int)
	for i, tag := range tags {
		if n := seen[tag]; n > 0 {
			newTag := fmt.Sprintf("%s-%d", tag, n)
			seen[tag] = n + 1
			tags[i] = newTag
			outbounds[i]["tag"] = newTag
		} else {
			seen[tag] = 1
		}
	}

	return outbounds, tags
}

func (s *LinkService) addClientInfo(uri string, clientInfo string) string {
	if len(clientInfo) == 0 {
		return uri
	}
	protocol := strings.Split(uri, "://")
	if len(protocol) < 2 {
		return uri
	}
	switch protocol[0] {
	case "vmess":
		var vmessJson map[string]interface{}
		config, err := util.B64StrToByte(protocol[1])
		if err != nil {
			logger.Warning("sub: Error decoding vmess content:", err)
			return uri
		}
		err = json.Unmarshal(config, &vmessJson)
		if err != nil {
			logger.Warning("sub: Error decoding vmess content:", err)
			return uri
		}
		// A vmess link with no "ps", or one whose "ps" is not a string, used
		// to panic here and take down the whole subscription response.
		ps, _ := vmessJson["ps"].(string)
		vmessJson["ps"] = ps + clientInfo
		result, err := json.MarshalIndent(vmessJson, "", "  ")
		if err != nil {
			logger.Warning("sub: Error decoding vmess + clientInfo content:", err)
			return uri
		}
		return "vmess://" + util.ByteToB64Str(result)
	default:
		return uri + clientInfo
	}
}
