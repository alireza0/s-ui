package util

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

// maxExternalBody caps what an external subscription may return. The response
// used to be read with io.ReadAll straight into memory, so a URL an operator
// pasted once -- or a host that was later taken over -- could answer with an
// endless stream and take the panel down with it.
const maxExternalBody = 8 << 20 // 8 MiB

// externalClient verifies certificates. InsecureSkipVerify was set here, which
// is exactly the wrong trade for this call: the response is turned into client
// configurations, so anyone able to intercept it chooses the servers every
// client of the panel then connects to.
var externalClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	},
}

func GetExternalLink(url string) string {
	response, err := externalClient.Get(url)
	if err != nil {
		logger.Warning("sub: Error making HTTP request:", err)
		return ""
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalBody))
	if err != nil {
		logger.Warning("sub: Error reading response body:", err)
		return ""
	}
	if len(body) == maxExternalBody {
		logger.Warning("sub: external subscription exceeded ", maxExternalBody, " bytes, refusing: ", url)
		return ""
	}

	data := StrOrBase64Encoded(string(body))
	return data
}

func GetExternalSub(url string) ([]map[string]interface{}, error) {
	return getExternalSub(url, false)
}

func GetExternalSubForClash(url string) ([]map[string]interface{}, error) {
	return getExternalSub(url, true)
}

func getExternalSub(url string, forClash bool) ([]map[string]interface{}, error) {
	var err error
	var result []map[string]interface{}

	if len(url) == 0 {
		return nil, common.NewError("no url")
	}

	data := GetExternalLink(url)
	if len(data) == 0 {
		return nil, common.NewError("no result")
	}

	// if the data is a JSON object
	if strings.HasPrefix(data, "{") && strings.HasSuffix(data, "}") {
		var jsonData map[string]interface{}
		err = json.Unmarshal([]byte(data), &jsonData)
		if err != nil {
			logger.Warning("sub: Error unmarshalling JSON:", err)
			return nil, err
		}
		outbounds, ok := jsonData["outbounds"].([]any)
		if !ok {
			logger.Warning("sub: Error getting outbounds:", err)
			return nil, err
		}
		for _, outbound := range outbounds {
			outboundMap, ok := outbound.(map[string]interface{})
			if ok && len(outboundMap) > 0 {
				oType, _ := outboundMap["type"].(string)
				switch oType {
				case "urltest":
				case "direct":
				case "selector":
				case "block":
					continue
				default:
					result = append(result, outboundMap)
				}
			}
		}
		if len(result) == 0 {
			return nil, common.NewError("no result")
		}
		return result, nil
	} else {
		// if data is a text
		parseOutbound := GetOutbound
		if forClash {
			parseOutbound = GetOutboundForClash
		}
		links := strings.Split(data, "\n")
		for _, link := range links {
			link = strings.TrimSpace(link)
			if link == "" {
				continue
			}
			linkToJson, _, err := parseOutbound(link, 0)
			if err == nil && linkToJson != nil && len(*linkToJson) > 0 {
				result = append(result, *linkToJson)
			}
		}
	}
	if len(result) == 0 {
		return nil, common.NewError("no result")
	}
	return result, nil
}
