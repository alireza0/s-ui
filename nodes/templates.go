package nodes

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
)

func validateInbound(value *model.NodeInbound, extended bool) error {
	if strings.TrimSpace(value.Tag) == "" {
		return errors.New("remote inbound needs a tag")
	}
	if !nodeagent.SupportsInbound(value.Type, extended) {
		return errors.New("unsupported node protocol")
	}
	var options map[string]interface{}
	if json.Unmarshal(value.Options, &options) != nil || options == nil {
		return errors.New("invalid node inbound options")
	}
	port, ok := options["listen_port"].(float64)
	if !ok || port < 1 || port > 65535 || float64(int(port)) != port {
		return errors.New("invalid remote listen port")
	}
	for _, key := range []string{"users", "type", "tag", "id", "tls_id", "out_json", "addrs"} {
		delete(options, key)
	}
	value.Options, _ = json.Marshal(options)
	_, _, err := clientTLS(*value)
	return err
}
