package api

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/nodes"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/util/common"
	"github.com/gin-gonic/gin"
)

// Both HTTP APIs register these routes after installing their own authentication.
func (a *ApiService) registerNodeRoutes(g *gin.RouterGroup, actor func(*gin.Context) string) {
	withActor := func(handler func(*gin.Context, string)) gin.HandlerFunc {
		return func(c *gin.Context) {
			handler(c, actor(c))
		}
	}
	g.GET("/nodes", a.GetNodes)
	g.GET("/node-template-history", a.GetNodeTemplateHistory)
	g.GET("/node-resources", a.GetNodeResources)
	g.GET("/node-template-version", a.GetNodeTemplateVersion)
	g.GET("/node-workspace", a.GetNodeWorkspace)
	g.GET("/node-links", a.GetNodeLinks)

	g.POST("/node-provision", withActor(a.ProvisionNode))
	g.POST("/node-resource", withActor(a.SaveNodeResources))
	g.POST("/node-dns-zones", a.DiscoverNodeDNSZones)
	g.POST("/node-resource-retry", withActor(a.RetryNodeResources))
	g.POST("/node-dns-cleanup", withActor(a.CleanupNodeDNS))
}

// The shared save endpoint retains the same request and response contract.
func saveNodeObject(c *gin.Context, obj, action string, data json.RawMessage, actor string) bool {
	if !nodes.IsObject(obj) {
		return false
	}
	var err error
	if obj == "nodes" && action == "del" {
		err = nodes.DeleteNode(c.Request.Context(), data, actor)
	} else {
		err = nodes.Save(obj, action, data, actor)
	}
	if err != nil {
		jsonMsg(c, "save Nodes", err)
		return true
	}
	bundle, err := nodes.Bundle()
	service.LastUpdate = time.Now().Unix()
	jsonObj(c, bundle, err)
	return true
}

func (a *ApiService) GetNodes(c *gin.Context) { bundle, err := nodes.Bundle(); jsonObj(c, bundle, err) }

// Confirmed node links are a read-only projection, never cached as Client.Links.
func (a *ApiService) GetNodeLinks(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil || id == 0 {
		jsonObj(c, nil, common.NewError("invalid client ID"))
		return
	}
	clients, err := a.ClientService.Get(strconv.FormatUint(id, 10))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	if len(*clients) != 1 {
		jsonObj(c, nil, common.NewError("client not found"))
		return
	}
	client := (*clients)[0]
	inbounds, err := nodes.ClientInbounds(database.GetDB(), client)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	remark := ""
	if a.SettingService.SubRemarkPrefix(database.GetDB()) {
		remark = client.Remark
	}
	type Link struct {
		Remark string `json:"remark"`
		URI    string `json:"uri"`
	}
	links := []Link{}
	for _, inbound := range inbounds {
		for _, uri := range util.LinkGenerator(client.Config, inbound, "", remark) {
			links = append(links, Link{inbound.Tag, uri})
		}
	}
	jsonObj(c, links, nil)
}
