package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// AddTraffic handles POST <web_base_url>/api/addTraffic and
// <web_base_url>/apiv2/addTraffic, where <web_base_url> is the user-configured
// panel base path.
// Body (form field "data"): {"name":"<client>","up":<bytes>,"down":<bytes>}
//
// Deliberately tokenless: the endpoint is write-only and can ONLY add positive
// bytes to an EXISTING client. It cannot read data, disable clients, or reduce
// counters. If abuse appears, consider HMAC(secret, name); the secret would
// never leave the server.
func (a *ApiService) AddTraffic(c *gin.Context) {
	data := c.Request.FormValue("data")
	if err := a.ClientService.AddTraffic(json.RawMessage(data)); err != nil {
		jsonMsg(c, "addTraffic", err)
		return
	}
	jsonMsg(c, "addTraffic", nil)
}
