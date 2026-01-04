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
	a := &SubHandler{
		SubService:   SubService{},
		JsonService:  JsonService{},
		ClashService: ClashService{},
	}
	a.initRouter(g)
}

func (s *SubHandler) initRouter(g *gin.RouterGroup) {
	g.GET("/:subid/:token", s.subs)
	g.GET("/:subid", s.subs)
}

func (s *SubHandler) subs(c *gin.Context) {
	var headers []string
	var result *string
	var err error
	subId := c.Param("subid")
	token := c.Param("token")
	format, isFormat := c.GetQuery("format")

	// Validate subscription token if provided
	if token != "" {
		clientService := service.ClientService{}
		client, err := clientService.GetClientBySubToken(token)
		if err != nil || client.Name != subId {
			logger.Error("Invalid subscription token:", err)
			c.String(400, "Error!")
			return
		}
	} else {
		// Check if token protection is enabled
		tokenProtection, err := s.SettingService.GetSubTokenProtection()
		if err != nil {
			logger.Error("Failed to get token protection setting:", err)
			c.String(500, "Internal server error")
			return
		}

		if tokenProtection {
			logger.Error("Subscription token required but not provided")
			c.String(400, "Error!")
			return
		}

		// For backward compatibility when token protection is disabled
		// we continue with normal processing
	}

	if isFormat {
		switch format {
		case "json":
			result, headers, err = s.JsonService.GetJson(subId, format)
		case "clash":
			result, headers, err = s.ClashService.GetClash(subId)
		default:
			result, headers, err = s.SubService.GetSubs(subId)
		}
		if err != nil || result == nil {
			logger.Error("Subscription error:", err)
			c.String(400, "Error!")
			return
		}
	} else {
		result, headers, err = s.SubService.GetSubs(subId)
		if err != nil || result == nil {
			logger.Error("Subscription error:", err)
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
