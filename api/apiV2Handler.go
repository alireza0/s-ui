package api

import (
	"crypto/subtle"
	"encoding/json"
	"sync"
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type TokenInMemory struct {
	Token    string
	Expiry   int64
	Username string
}

type APIv2Handler struct {
	ApiService
	// tokensMu guards tokens. Every request reads the slice while addToken and
	// deleteToken replace it from a gin handler on another connection.
	tokensMu sync.RWMutex
	tokens   []TokenInMemory
}

func NewAPIv2Handler(g *gin.RouterGroup) *APIv2Handler {
	a := &APIv2Handler{}
	a.ReloadTokens()
	a.initRouter(g)
	return a
}

func (a *APIv2Handler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		a.checkToken(c)
	})
	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIv2Handler) postHandler(c *gin.Context) {
	username := a.findUsername(c)
	action := c.Param("postAction")

	switch action {
	case "save":
		a.ApiService.Save(c, username)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "maintenance":
		a.ApiService.SetMaintenance(c)
	case "resetTraffic":
		a.ApiService.ResetTraffic(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "subConvert":
		a.ApiService.SubConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	case "closeSessions":
		a.ApiService.CloseSessions(c)
	case "getCertPing":
		a.ApiService.GetCertPing(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "load":
		a.ApiService.LoadData(c)
	case "inbounds", "outbounds", "endpoints", "services", "tls", "clients", "config":
		err := a.ApiService.LoadPartialData(c, []string{action})
		if err != nil {
			jsonMsg(c, action, err)
		}
		return
	case "users":
		a.ApiService.GetUsers(c)
	case "settings":
		a.ApiService.GetSettings(c)
	case "stats":
		a.ApiService.GetStats(c)
	case "status":
		a.ApiService.GetStatus(c)
	case "onlines":
		a.ApiService.GetOnlines(c)
	case "sessions":
		a.ApiService.GetSessions(c)
	case "logs":
		a.ApiService.GetLogs(c)
	case "changes":
		a.ApiService.CheckChanges(c)
	case "keypairs":
		a.ApiService.GetKeypairs(c)
	case "getdb":
		a.ApiService.GetDb(c)
	case "checkOutbound":
		a.ApiService.GetCheckOutbound(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) findUsername(c *gin.Context) string {
	token := c.Request.Header.Get("Token")
	if token == "" {
		return ""
	}

	a.tokensMu.RLock()
	defer a.tokensMu.RUnlock()

	now := time.Now().Unix()
	for _, t := range a.tokens {
		// Expired entries are skipped, not spliced out. Deleting from the
		// slice being ranged over shifted every later element back by one and
		// skipped it, so one expired token could hide the valid token stored
		// immediately after it -- and the write happened with no lock held.
		if t.Expiry > 0 && t.Expiry < now {
			continue
		}
		// Constant time, so the response time of a wrong token does not reveal
		// how many leading characters were right.
		if subtle.ConstantTimeCompare([]byte(t.Token), []byte(token)) == 1 {
			return t.Username
		}
	}
	return ""
}

func (a *APIv2Handler) checkToken(c *gin.Context) {
	username := a.findUsername(c)
	if username != "" {
		c.Next()
		return
	}
	jsonMsg(c, "", common.NewError("invalid token"))
	c.Abort()
}

func (a *APIv2Handler) ReloadTokens() {
	tokens, err := a.ApiService.LoadTokens()
	if err != nil {
		logger.Error("unable to load tokens: ", err)
		return
	}
	var newTokens []TokenInMemory
	if err := json.Unmarshal(tokens, &newTokens); err != nil {
		// Published only on success. The old code installed the half-filled
		// slice from a failed unmarshal, which revoked every working token.
		logger.Error("unable to load tokens: ", err)
		return
	}

	a.tokensMu.Lock()
	a.tokens = newTokens
	a.tokensMu.Unlock()
}
