package sub

import (
	"s-ui/logger"
	"s-ui/service"

	"github.com/gin-gonic/gin"
)

type SubHandler struct {
	service.SettingService
	SubService
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
	result, headers, err := s.SubService.GetSubs(subId)
	if err != nil || result == nil {
		logger.Error(err)
		c.String(400, "Error!")
	} else {

		// Add headers
		c.Writer.Header().Set("Subscription-Userinfo", headers[0])
		c.Writer.Header().Set("Profile-Update-Interval", headers[1])
		c.Writer.Header().Set("Profile-Title", headers[2])

		c.String(200, *result)
	}
}
