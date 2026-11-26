package api

import (
	"encoding/json"
	"errors"

	"github.com/alireza0/s-ui/nodes"
	"github.com/gin-gonic/gin"
)

func (a *ApiService) ProvisionNode(c *gin.Context, actor string) {
	data := c.Request.FormValue("data")
	if len(data) > 64<<10 {
		jsonObj(c, nil, errors.New("invalid_install_request"))
		return
	}
	var request nodes.ProvisionRequest
	if json.Unmarshal([]byte(data), &request) != nil {
		jsonObj(c, nil, errors.New("invalid_install_request"))
		return
	}
	job, err := nodes.StartProvision(request, actor)
	jsonObj(c, job, err)
}
