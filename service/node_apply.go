package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"
)

// NodeApplyRequest is sent from master to node to upsert a managed inbound + users.
type NodeApplyRequest struct {
	Inbound json.RawMessage   `json:"inbound"`
	Users   []json.RawMessage `json:"users,omitempty"`
	Tag     string            `json:"tag"`
	Type    string            `json:"type"`
}

// NodeDeleteRequest is sent from master to node to remove a managed inbound.
type NodeDeleteRequest struct {
	Tag string `json:"tag"`
}

// NodeApplyUsersRequest replaces users on a managed inbound on the node.
type NodeApplyUsersRequest struct {
	Tag   string            `json:"tag"`
	Type  string            `json:"type"`
	Users []json.RawMessage `json:"users"`
}

// PushInboundToNode sends an inbound config + users to a remote node.
func (s *NodeService) PushInboundToNode(n *model.Node, req *NodeApplyRequest) error {
	target, err := s.APIv2URL(n, "nodeApplyInbound")
	if err != nil {
		return err
	}
	return s.doNodePOST(n, target, req)
}

// DeleteInboundOnNode removes a managed inbound from a remote node.
func (s *NodeService) DeleteInboundOnNode(n *model.Node, tag string) error {
	target, err := s.APIv2URL(n, "nodeDeleteInbound")
	if err != nil {
		return err
	}
	return s.doNodePOST(n, target, &NodeDeleteRequest{Tag: tag})
}

// PushUsersToNode replaces users on a managed inbound on a remote node.
func (s *NodeService) PushUsersToNode(n *model.Node, req *NodeApplyUsersRequest) error {
	target, err := s.APIv2URL(n, "nodeApplyUsers")
	if err != nil {
		return err
	}
	return s.doNodePOST(n, target, req)
}

// doNodePOST performs an authenticated POST with JSON body to a node endpoint.
func (s *NodeService) doNodePOST(n *model.Node, targetURL string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), nodeProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Token", n.ApiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client, err := s.httpClientFor(n)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	var env struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("invalid response from node: %s", string(body))
	}
	if !env.Success {
		return common.NewError("node rejected: " + env.Msg)
	}
	return nil
}

// IsRemoteInbound checks if an inbound is assigned to a remote node.
func IsRemoteInbound(inbound *model.Inbound) bool {
	return inbound.NodeId != nil && *inbound.NodeId > 0
}
