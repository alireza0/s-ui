package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/alireza0/s-ui/database/model"
)

var errNodeDNSCleanupRequired = errors.New("node_dns_cleanup_required")

func disableConfirmed(node model.Node) bool {
	if node.Enable || node.AppliedRevision == 0 || node.AppliedRevision != node.DesiredRevision {
		return false
	}
	var applied struct {
		Node     *Publication        `json:"node"`
		Inbounds []model.NodeInbound `json:"inbounds"`
	}
	return json.Unmarshal(node.AppliedSnapshot, &applied) == nil && applied.Node != nil && !applied.Node.Enable && len(applied.Inbounds) == 0
}

type nodeDeleteRequest struct {
	Id                   uint `json:"id"`
	DNSAlreadyRemoved    bool `json:"dns_already_removed"`
	ServerAlreadyStopped bool `json:"server_already_stopped,omitempty"`
}

func parseNodeDelete(data json.RawMessage) (nodeDeleteRequest, error) {
	var request nodeDeleteRequest
	data = bytes.TrimSpace(data)
	var err error
	if len(data) > 0 && data[0] == '{' {
		err = decodeJSON(data, &request, true)
	} else {
		err = json.Unmarshal(data, &request.Id)
	}
	if err != nil || request.Id == 0 {
		return request, errors.New("invalid_node_id")
	}
	return request, nil
}

// DeleteNode waits to contact the provider until all local deletion guards pass.
// A confirmed manual cleanup goes straight through the same database transaction.
func DeleteNode(ctx context.Context, data json.RawMessage, actor string) error {
	err := Save("nodes", "del", data, actor)
	if !errors.Is(err, errNodeDNSCleanupRequired) {
		return err
	}
	request, err := parseNodeDelete(data)
	if err != nil {
		return err
	}
	if err := CleanupNodeDNS(ctx, request.Id, 0, actor); err != nil {
		return err
	}
	return Save("nodes", "del", data, actor)
}
