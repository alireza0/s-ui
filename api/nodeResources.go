package api

import (
	"encoding/json"
	"errors"
	"github.com/alireza0/s-ui/nodes"
	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetNodeResources(c *gin.Context) {
	result, err := nodes.ResourcesBundle()
	jsonObj(c, result, err)
}
func (a *ApiService) DiscoverNodeDNSZones(c *gin.Context) {
	data := c.Request.FormValue("data")
	if len(data) > 16384 {
		jsonObj(c, nil, errors.New("node_resource_request_too_large"))
		return
	}
	zones, err := nodes.DiscoverDNSZones(c.Request.Context(), json.RawMessage(data))
	jsonObj(c, zones, err)
}
func (a *ApiService) SaveNodeResources(c *gin.Context, actor string) {
	body := c.Request.FormValue("data")
	if len(body) > 3<<20 {
		jsonObj(c, nil, errors.New("node_resource_request_too_large"))
		return
	}
	err := nodes.SaveNodeResource(c.Request.FormValue("resource"), c.Request.FormValue("action"), json.RawMessage(body), actor)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	bundle, err := nodes.ResourcesBundle()
	jsonObj(c, bundle, err)
}
func (a *ApiService) RetryNodeResources(c *gin.Context, actor string) {
	var request struct {
		NodeId uint `json:"node_id"`
	}
	if json.Unmarshal([]byte(c.Request.FormValue("data")), &request) != nil || request.NodeId == 0 {
		jsonObj(c, nil, errors.New("invalid_node_id"))
		return
	}
	jsonObj(c, nil, nodes.RetryNodeResources(request.NodeId, actor))
}
func (a *ApiService) CleanupNodeDNS(c *gin.Context, actor string) {
	var request struct {
		NodeId   uint `json:"node_id"`
		RecordId uint `json:"record_id"`
	}
	if json.Unmarshal([]byte(c.Request.FormValue("data")), &request) != nil || request.NodeId == 0 {
		jsonObj(c, nil, errors.New("invalid_node_id"))
		return
	}
	jsonObj(c, nil, nodes.CleanupNodeDNS(c.Request.Context(), request.NodeId, request.RecordId, actor))
}
