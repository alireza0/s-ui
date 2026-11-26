package util

// HTTPTransportHosts reads the scalar and list forms of a sing-box listable field.
func HTTPTransportHosts(value any) []string {
	switch hosts := value.(type) {
	case string:
		if hosts != "" {
			return []string{hosts}
		}
	case []string:
		return hosts
	case []any:
		result := make([]string, 0, len(hosts))
		for _, item := range hosts {
			if host, ok := item.(string); ok && host != "" {
				result = append(result, host)
			}
		}
		return result
	}
	return nil
}
