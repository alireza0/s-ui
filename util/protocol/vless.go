package protocol

import "encoding/json"

func TLSEnabled(tls interface{}) bool {
	tlsMap, ok := tls.(map[string]interface{})
	if !ok {
		return false
	}
	enabled, _ := tlsMap["enabled"].(bool)
	return enabled
}

func VlessVisionFlowAllowed(hasTLS bool, transport interface{}) bool {
	if !hasTLS {
		return false
	}
	transportMap, ok := transport.(map[string]interface{})
	if !ok {
		return true
	}
	transportType, _ := transportMap["type"].(string)
	return transportType == "" || transportType == "tcp"
}

func VlessVisionFlowAllowedFromOptions(hasTLS bool, options json.RawMessage) bool {
	if !hasTLS {
		return false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(options, &raw); err != nil {
		return true
	}
	return VlessVisionFlowAllowed(true, raw["transport"])
}
