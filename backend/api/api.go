package api

import (
	"fmt"
	"s-ui/logger"
	"s-ui/service"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.PanelService
	service.StatsService
	service.ServerService
}

func NewAPIHandler(g *gin.RouterGroup) {
	a := &APIHandler{}
	a.initRouter(g)
}

func (a *APIHandler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasSuffix(path, "login") && !strings.HasSuffix(path, "logout") {
			checkLogin(c)
		}
	})

	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIHandler) postHandler(c *gin.Context) {
	var err error
	action := c.Param("postAction")
	remoteIP := getRemoteIp(c)

	switch action {
	case "login":
		loginUser, err := a.UserService.Login(c.Request.FormValue("user"), c.Request.FormValue("pass"), remoteIP)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}

		sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
		if err != nil {
			logger.Infof("Unable to get session's max age from DB")
		}

		if sessionMaxAge > 0 {
			err = SetMaxAge(c, sessionMaxAge*60)
			if err != nil {
				logger.Infof("Unable to set session's max age")
			}
		}

		err = SetLoginUser(c, loginUser)
		if err == nil {
			logger.Info("user ", loginUser, " login success")
		} else {
			logger.Warning("login failed: ", err)
		}

		jsonMsg(c, "", nil)
	case "changePass":
		id := c.Request.FormValue("id")
		oldPass := c.Request.FormValue("oldPass")
		newUsername := c.Request.FormValue("newUsername")
		newPass := c.Request.FormValue("newPass")
		err = a.UserService.ChangePass(id, oldPass, newUsername, newPass)
		if err == nil {
			logger.Info("change user credentials success")
			jsonMsg(c, "save", nil)
		} else {
			logger.Warning("change user credentials failed:", err)
			jsonMsg(c, "", err)
		}
	case "save":
		loginUser := GetLoginUser(c)
		data := map[string]string{}
		err = c.ShouldBind(&data)
		if err == nil {
			err = a.ConfigService.SaveChanges(data, loginUser)
		}
		jsonMsg(c, "save", err)
	case "restartApp":
		err = a.PanelService.RestartPanel(3)
		jsonMsg(c, "restartApp", err)
	default:
		jsonMsg(c, "API call", nil)
	}
}

func (a *APIHandler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "logout":
		loginUser := GetLoginUser(c)
		if loginUser != "" {
			logger.Infof("user %s logout", loginUser)
		}
		ClearSession(c)
		jsonMsg(c, "", nil)
	case "load":
		data, err := a.loadData(c)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, data, nil)
	case "users":
		users, err := a.UserService.GetUsers()
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, *users, nil)
	case "setting":
		data, err := a.SettingService.GetAllSetting()
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, data, err)
	case "stats":
		resource := c.Query("resource")
		tag := c.Query("tag")
		limit, err := strconv.Atoi(c.Query("limit"))
		if err != nil {
			limit = 100
		}
		data, err := a.StatsService.GetStats(resource, tag, limit)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, data, err)
	case "status":
		request := c.Query("r")
		result := a.ServerService.GetStatus(request)
		jsonObj(c, result, nil)
	case "onlines":
		onlines, err := a.StatsService.GetOnlines()
		jsonObj(c, onlines, err)
	default:
		jsonMsg(c, "API call", nil)
	}
}

func (a *APIHandler) loadData(c *gin.Context) (string, error) {
	var data string
	lu := c.Query("lu")
	isUpdated, err := a.ConfigService.CheckChnages(lu)
	if err != nil {
		return "", err
	}
	onlines, err := a.StatsService.GetOnlines()
	if err != nil {
		return "", err
	}
	if isUpdated {
		config, err := a.ConfigService.GetConfig()
		if err != nil {
			return "", err
		}
		clients, err := a.ClientService.GetAll()
		if err != nil {
			return "", err
		}
		subURI, err := a.SettingService.GetFinalSubURI(strings.Split(c.Request.Host, ":")[0])
		if err != nil {
			return "", err
		}
		data = fmt.Sprintf(`{"config": %s,"clients": %s,"subURI": "%s", "onlines": %s}`, string(*config), clients, subURI, onlines)
	} else {
		data = fmt.Sprintf(`{"onlines": %s}`, onlines)
	}

	return data, nil
}
