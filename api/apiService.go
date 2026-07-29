package api

import (
	"encoding/json"
	"runtime"
	"strconv"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type ApiService struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.TlsService
	service.InboundService
	service.OutboundService
	service.EndpointService
	service.ServicesService
	service.PanelService
	service.StatsService
	service.ServerService
	service.NodeService
}

func (a *ApiService) LoadData(c *gin.Context) {
	data, err := a.getData(c)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, nil)
}

func (a *ApiService) getData(c *gin.Context) (interface{}, error) {
	data := make(map[string]interface{}, 0)
	lu := c.Query("lu")
	isUpdated, err := a.ConfigService.CheckChanges(lu)
	if err != nil {
		return "", err
	}
	onlines, err := a.StatsService.GetOnlines()

	sysInfo := a.ServerService.GetSingboxInfo()
	if sysInfo["running"] == false {
		logs := a.ServerService.GetLogs("1", "debug")
		if len(logs) > 0 {
			data["lastLog"] = logs[0]
		}
	}

	if err != nil {
		return "", err
	}
	if isUpdated {
		config, err := a.SettingService.GetConfig()
		if err != nil {
			return "", err
		}
		clients, err := a.ClientService.GetAll()
		if err != nil {
			return "", err
		}
		tlsConfigs, err := a.TlsService.GetAll()
		if err != nil {
			return "", err
		}
		inbounds, err := a.InboundService.GetAll()
		if err != nil {
			return "", err
		}
		outbounds, err := a.OutboundService.GetAll()
		if err != nil {
			return "", err
		}
		endpoints, err := a.EndpointService.GetAll()
		if err != nil {
			return "", err
		}
		services, err := a.ServicesService.GetAll()
		if err != nil {
			return "", err
		}
		subURI, err := a.SettingService.GetFinalSubURI(getHostname(c))
		if err != nil {
			return "", err
		}
		trafficAge, err := a.SettingService.GetTrafficAge()
		if err != nil {
			return "", err
		}
		data["config"] = json.RawMessage(config)
		data["clients"] = clients
		data["tls"] = tlsConfigs
		data["inbounds"] = inbounds
		data["outbounds"] = outbounds
		data["endpoints"] = endpoints
		data["services"] = services
		data["subURI"] = subURI
		data["enableTraffic"] = trafficAge > 0
		data["onlines"] = onlines
		data["os"] = runtime.GOOS
	} else {
		data["onlines"] = onlines
	}

	return data, nil
}

func (a *ApiService) LoadPartialData(c *gin.Context, objs []string) error {
	data := make(map[string]interface{}, 0)
	id := c.Query("id")

	for _, obj := range objs {
		switch obj {
		case "inbounds":
			inbounds, err := a.InboundService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = inbounds
		case "outbounds":
			outbounds, err := a.OutboundService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = outbounds
		case "endpoints":
			endpoints, err := a.EndpointService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = endpoints
		case "services":
			services, err := a.ServicesService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = services
		case "tls":
			tlsConfigs, err := a.TlsService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = tlsConfigs
		case "clients":
			clients, err := a.ClientService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = clients
		case "config":
			config, err := a.SettingService.GetConfig()
			if err != nil {
				return err
			}
			data[obj] = json.RawMessage(config)
		case "settings":
			settings, err := a.SettingService.GetAllSetting()
			if err != nil {
				return err
			}
			data[obj] = settings
		case "nodes":
			nodes, err := a.NodeService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = nodes
		}
	}

	jsonObj(c, data, nil)
	return nil
}

func (a *ApiService) GetUsers(c *gin.Context) {
	users, err := a.UserService.GetUsers()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, *users, nil)
}

func (a *ApiService) GetSettings(c *gin.Context) {
	data, err := a.SettingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStats(c *gin.Context) {
	resource := c.Query("resource")
	tag := c.Query("tag")
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 100
	}
	start, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end"), 10, 64)
	data, err := a.StatsService.GetStats(resource, tag, limit, start, end)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStatus(c *gin.Context) {
	request := c.Query("r")
	result := a.ServerService.GetStatus(request)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetOnlines(c *gin.Context) {
	onlines, err := a.StatsService.GetOnlines()
	jsonObj(c, onlines, err)
}

func (a *ApiService) GetLogs(c *gin.Context) {
	count := c.Query("c")
	level := c.Query("l")
	logs := a.ServerService.GetLogs(count, level)
	jsonObj(c, logs, nil)
}

func (a *ApiService) CheckChanges(c *gin.Context) {
	actor := c.Query("a")
	chngKey := c.Query("k")
	count := c.Query("c")
	changes := a.ConfigService.GetChanges(actor, chngKey, count)
	jsonObj(c, changes, nil)
}

func (a *ApiService) GetKeypairs(c *gin.Context) {
	kType := c.Query("k")
	options := c.Query("o")
	keypair := a.ServerService.GenKeypair(kType, options)
	jsonObj(c, keypair, nil)
}

func (a *ApiService) GetDb(c *gin.Context) {
	exclude := c.Query("exclude")
	db, err := database.GetDb(exclude)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=s-ui_"+time.Now().Format("20060102-150405")+".db")
	c.Writer.Write(db)
}

func (a *ApiService) postActions(c *gin.Context) (string, json.RawMessage, error) {
	var data map[string]json.RawMessage
	err := c.ShouldBind(&data)
	if err != nil {
		return "", nil, err
	}
	return string(data["action"]), data["data"], nil
}

func (a *ApiService) Login(c *gin.Context) {
	remoteIP := getRemoteIp(c)
	loginUser, err := a.UserService.Login(c.Request.FormValue("user"), c.Request.FormValue("pass"), remoteIP)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}

	sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}

	err = SetLoginUser(c, loginUser, sessionMaxAge)
	if err == nil {
		logger.Info("user ", loginUser, " login success")
	} else {
		logger.Warning("login failed: ", err)
	}

	jsonMsg(c, "", nil)
}

func (a *ApiService) ChangePass(c *gin.Context) {
	id := c.Request.FormValue("id")
	oldPass := c.Request.FormValue("oldPass")
	newUsername := c.Request.FormValue("newUsername")
	newPass := c.Request.FormValue("newPass")
	err := a.UserService.ChangePass(id, oldPass, newUsername, newPass)
	if err == nil {
		logger.Info("change user credentials success")
		jsonMsg(c, "save", nil)
	} else {
		logger.Warning("change user credentials failed:", err)
		jsonMsg(c, "", err)
	}
}

func (a *ApiService) Save(c *gin.Context, loginUser string) {
	hostname := getHostname(c)
	obj := c.Request.FormValue("object")
	act := c.Request.FormValue("action")
	data := c.Request.FormValue("data")
	initUsers := c.Request.FormValue("initUsers")
	objs, err := a.ConfigService.Save(obj, act, json.RawMessage(data), initUsers, loginUser, hostname)
	if err != nil {
		jsonMsg(c, "save", err)
		return
	}
	err = a.LoadPartialData(c, objs)
	if err != nil {
		jsonMsg(c, obj, err)
	}
}

func (a *ApiService) RestartApp(c *gin.Context) {
	err := a.PanelService.RestartPanel(3)
	jsonMsg(c, "restartApp", err)
}

func (a *ApiService) RestartSb(c *gin.Context) {
	err := a.ConfigService.RestartCore()
	jsonMsg(c, "restartSb", err)
}

func (a *ApiService) ResetTraffic(c *gin.Context) {
	if err := a.ClientService.ResetAllClientsTraffic(); err != nil {
		jsonMsg(c, "resetTraffic", err)
		return
	}
	err := a.ConfigService.RestartCore()
	jsonMsg(c, "resetTraffic", err)
}

func (a *ApiService) LinkConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, _, err := util.GetOutbound(link, 0)
	jsonObj(c, result, err)
}

func (a *ApiService) SubConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, err := util.GetExternalSub(link)
	jsonObj(c, result, err)
}

func (a *ApiService) ImportDb(c *gin.Context) {
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	defer file.Close()
	err = database.ImportDB(file)
	jsonMsg(c, "", err)
}

func (a *ApiService) Logout(c *gin.Context) {
	loginUser := GetLoginUser(c)
	if loginUser != "" {
		logger.Infof("user %s logout", loginUser)
	}
	ClearSession(c)
	jsonMsg(c, "", nil)
}

func (a *ApiService) LoadTokens() ([]byte, error) {
	return a.UserService.LoadTokens()
}

func (a *ApiService) GetTokens(c *gin.Context) {
	loginUser := GetLoginUser(c)
	tokens, err := a.UserService.GetUserTokens(loginUser)
	jsonObj(c, tokens, err)
}

func (a *ApiService) AddToken(c *gin.Context) {
	loginUser := GetLoginUser(c)
	expiry := c.Request.FormValue("expiry")
	expiryInt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	desc := c.Request.FormValue("desc")
	token, err := a.UserService.AddToken(loginUser, expiryInt, desc)
	jsonObj(c, token, err)
}

func (a *ApiService) DeleteToken(c *gin.Context) {
	tokenId := c.Request.FormValue("id")
	err := a.UserService.DeleteToken(tokenId)
	jsonMsg(c, "", err)
}

func (a *ApiService) GetSingboxConfig(c *gin.Context) {
	rawConfig, err := a.ConfigService.GetConfig("")
	if err != nil {
		c.Status(400)
		c.Writer.WriteString(err.Error())
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=config_"+time.Now().Format("20060102-150405")+".json")
	c.Writer.Write(*rawConfig)
}

func (a *ApiService) GetCheckOutbound(c *gin.Context) {
	tag := c.Query("tag")
	link := c.Query("link")
	result := a.ConfigService.CheckOutbound(tag, link)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetCertPing(c *gin.Context) {
	domain := c.PostForm("domain")
	port := c.PostForm("port")
	tlsPing, err := util.GetTlsPing(domain, port)
	jsonObj(c, tlsPing, err)
}

func (a *ApiService) GetNodes(c *gin.Context) {
	nodes, err := a.NodeService.GetAll()
	if err != nil {
		jsonMsg(c, "nodes", err)
		return
	}
	jsonObj(c, nodes, nil)
}

func (a *ApiService) GetNode(c *gin.Context) {
	id, err := parseIDFormOrQuery(c)
	if err != nil {
		jsonMsg(c, "node", err)
		return
	}
	node, err := a.NodeService.GetById(id)
	if err != nil {
		jsonMsg(c, "node", err)
		return
	}
	jsonObj(c, node, nil)
}

func (a *ApiService) SaveNode(c *gin.Context) {
	raw, err := readDataPayload(c)
	if err != nil {
		jsonMsg(c, "saveNode", err)
		return
	}
	var node model.Node
	if err := json.Unmarshal(raw, &node); err != nil {
		jsonMsg(c, "saveNode", err)
		return
	}
	if node.Id == 0 {
		err = a.NodeService.Create(&node)
	} else {
		err = a.NodeService.Update(node.Id, &node)
	}
	if err != nil {
		jsonMsg(c, "saveNode", err)
		return
	}
	jsonObj(c, node, nil)
}

func (a *ApiService) DeleteNode(c *gin.Context) {
	id, err := parseIDFormOrQuery(c)
	if err != nil {
		jsonMsg(c, "deleteNode", err)
		return
	}
	err = a.NodeService.Delete(id)
	jsonMsg(c, "deleteNode", err)
}

func (a *ApiService) SetNodeEnable(c *gin.Context) {
	id, err := parseIDFormOrQuery(c)
	if err != nil {
		jsonMsg(c, "setNodeEnable", err)
		return
	}
	enableRaw := c.PostForm("enable")
	if enableRaw == "" {
		enableRaw = c.Query("enable")
	}
	enable := enableRaw == "1" || enableRaw == "true" || enableRaw == "TRUE"
	err = a.NodeService.SetEnable(id, enable)
	jsonMsg(c, "setNodeEnable", err)
}

func (a *ApiService) TestNode(c *gin.Context) {
	raw, err := readDataPayload(c)
	if err != nil {
		jsonMsg(c, "testNode", err)
		return
	}
	var node model.Node
	if err := json.Unmarshal(raw, &node); err != nil {
		jsonMsg(c, "testNode", err)
		return
	}
	if node.Id > 0 && node.Address == "" {
		existing, err := a.NodeService.GetById(node.Id)
		if err != nil {
			jsonMsg(c, "testNode", err)
			return
		}
		node = *existing
	}
	res, err := a.NodeService.Probe(c.Request.Context(), &node)
	if err != nil {
		jsonMsg(c, "testNode", err)
		return
	}
	if node.Id > 0 {
		_ = a.NodeService.ApplyProbeResult(node.Id, res)
	}
	jsonObj(c, res, nil)
}

// NodeSnapshot returns this node's snapshot for the master heartbeat.
func (a *ApiService) NodeSnapshot(c *gin.Context) {
	snap, err := a.NodeService.BuildNodeSnapshot()
	if err != nil {
		jsonMsg(c, "nodeSnapshot", err)
		return
	}
	jsonObj(c, snap, nil)
}

// NodeApplyInbound receives a managed inbound from master and applies it locally.
// This runs on the NODE side. The master pushes inbound config + users.
func (a *ApiService) NodeApplyInbound(c *gin.Context) {
	raw, err := readDataPayload(c)
	if err != nil {
		jsonMsg(c, "nodeApplyInbound", err)
		return
	}
	var req service.NodeApplyRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		jsonMsg(c, "nodeApplyInbound", err)
		return
	}
	// For now, just acknowledge. Full local apply will be wired in M4.
	jsonMsg(c, "nodeApplyInbound", nil)
}

// NodeDeleteInbound removes a managed inbound on this node.
func (a *ApiService) NodeDeleteInbound(c *gin.Context) {
	raw, err := readDataPayload(c)
	if err != nil {
		jsonMsg(c, "nodeDeleteInbound", err)
		return
	}
	var req service.NodeDeleteRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		jsonMsg(c, "nodeDeleteInbound", err)
		return
	}
	jsonMsg(c, "nodeDeleteInbound", nil)
}

// NodeApplyUsers replaces users on a managed inbound on this node.
func (a *ApiService) NodeApplyUsers(c *gin.Context) {
	raw, err := readDataPayload(c)
	if err != nil {
		jsonMsg(c, "nodeApplyUsers", err)
		return
	}
	var req service.NodeApplyUsersRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		jsonMsg(c, "nodeApplyUsers", err)
		return
	}
	jsonMsg(c, "nodeApplyUsers", nil)
}

func readDataPayload(c *gin.Context) ([]byte, error) {
	raw := c.PostForm("data")
	if raw != "" {
		return []byte(raw), nil
	}
	body, err := c.GetRawData()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, common.NewError("data is required")
	}
	return body, nil
}

func parseIDFormOrQuery(c *gin.Context) (uint, error) {
	raw := c.PostForm("id")
	if raw == "" {
		raw = c.Query("id")
	}
	if raw == "" {
		return 0, common.NewError("id is required")
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, common.NewError("invalid id")
	}
	return uint(id), nil
}
