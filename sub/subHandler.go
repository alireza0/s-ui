package sub

import (
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"

	"github.com/gin-gonic/gin"
)

type SubHandler struct {
	service.SettingService
	SubService
	JsonService
	ClashService
}

func NewSubHandler(g *gin.RouterGroup) {
	a := &SubHandler{}
	a.initRouter(g)
}

func (s *SubHandler) initRouter(g *gin.RouterGroup) {
	g.GET("/:subid", s.subs)
}

func (s *SubHandler) subs(c *gin.Context) {
	subId := c.Param("subid")
	// Get client IP and HWID
	clientIP := c.ClientIP()
	hwid := c.GetHeader("X-HWID") // Or extract from User-Agent or other headers

	// Get protection settings
	ipProtectionEnabled, err := s.GetSubIPProtection()
	if err != nil {
		logger.Warning("Error getting IP protection setting: ", err)
		ipProtectionEnabled = false
	}
	hwidProtectionEnabled, err := s.GetSubHWIDProtection()
	if err != nil {
		logger.Warning("Error getting HWID protection setting: ", err)
		hwidProtectionEnabled = false
	}

	// If both protections are disabled, skip validation
	if !ipProtectionEnabled && !hwidProtectionEnabled {
		// Update client access info without validation
		clientService := service.ClientService{}
		err = clientService.UpdateClientAccessInfo(subId, clientIP, hwid)
		if err != nil {
			logger.Warning("Error updating client access info: ", err)
		}
		// Continue to serve the subscription
	} else {
		// Validate client access
		clientService := service.ClientService{}
		valid, err := clientService.ValidateClientAccess(subId, clientIP, hwid)
		if err != nil {
			logger.Error("Error validating client access: ", err)
			c.String(403, "Access validation error")
			return
		}
		if !valid {
			logger.Warning("Access denied for client: ", subId, " from IP: ", clientIP, " with HWID: ", hwid)
			c.String(403, "Access denied")
			return
		}

		// Update client access info
		err = clientService.UpdateClientAccessInfo(subId, clientIP, hwid)
		if err != nil {
			logger.Warning("Error updating client access info: ", err)
			// Continue anyway, don't block the request for this error
		}
	}

	var headers []string
	var result *string
	var err2 error
	format, isFormat := c.GetQuery("format")
	if isFormat {
		switch format {
		case "json":
			result, headers, err2 = s.JsonService.GetJson(subId, format)
		case "clash":
			result, headers, err2 = s.ClashService.GetClash(subId)
		}
		if err2 != nil || result == nil {
			logger.Error(err2)
			c.String(400, "Error!")
			return
		}
	} else {
		result, headers, err2 = s.SubService.GetSubs(subId)
		if err2 != nil || result == nil {
			logger.Error(err2)
			c.String(400, "Error!")
			return
		}
	}
	// Add headers
	c.Writer.Header().Set("Subscription-Userinfo", headers[0])
	c.Writer.Header().Set("Profile-Update-Interval", headers[1])
	c.Writer.Header().Set("Profile-Title", headers[2])

	c.String(200, *result)
}
