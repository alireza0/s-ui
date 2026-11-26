package api

import (
	"errors"
	"strconv"

	"github.com/alireza0/s-ui/nodes"
	"github.com/gin-gonic/gin"
)

func (a *ApiService) GetNodeTemplateHistory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, errors.New("invalid_template_id"))
		return
	}
	versions, err := nodes.TemplateHistory(uint(id))
	jsonObj(c, versions, err)
}

func (a *ApiService) GetNodeTemplateVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, errors.New("invalid_template_id"))
		return
	}
	var version uint64
	if raw := c.Query("version"); raw != "" {
		version, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			jsonObj(c, nil, errors.New("invalid_template_version"))
			return
		}
	}
	view, err := nodes.ReadTemplateVersion(uint(id), version)
	jsonObj(c, view, err)
}

func (a *ApiService) GetNodeWorkspace(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 32)
	if err != nil || id == 0 {
		jsonObj(c, nil, errors.New("invalid_node_id"))
		return
	}
	view, err := nodes.ReadWorkspace(uint(id))
	jsonObj(c, view, err)
}
