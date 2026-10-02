package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

type TokenInMemory struct {
	Token    string
	Expiry   int64
	Username string
}

type APIv2Handler struct {
	ApiService
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
	g.Use(a.checkToken)

	system := g.Group("/system")
	{
		system.POST("/restart-app", a.RestartApp)
		system.POST("/restart-sb", a.RestartSb)
		system.POST("/maintenance", a.SetMaintenance)
		system.GET("/status", a.GetStatus)
		system.GET("/logs", a.GetLogs)
	}

	users := g.Group("/users")
	{
		users.GET("", a.GetUsers)
		users.POST("/reset-traffic", a.ResetTraffic)
		users.POST("/close-sessions", a.CloseSessions)
	}

	tokens := g.Group("/tokens")
	{
		tokens.GET("", a.GetTokens)
		tokens.POST("/add", a.AddToken)
		tokens.POST("/delete", a.DeleteToken)
	}

	config := g.Group("/config")
	{
		config.POST("/save", a.Save)
		config.GET("/load", a.LoadData)
		config.GET("/changes", a.CheckChanges)
		config.GET("/singbox", a.GetSingboxConfig)
	}

	stats := g.Group("/stats")
	{
		stats.GET("", a.GetStats)
		stats.GET("/onlines", a.GetOnlines)
		stats.GET("/sessions", a.GetSessions)
	}

	tools := g.Group("/tools")
	{
		tools.POST("/link-convert", a.LinkConvert)
		tools.POST("/sub-convert", a.SubConvert)
		tools.GET("/keypairs", a.GetKeypairs)
		tools.GET("/check-outbound", a.GetCheckOutbound)
		tools.POST("/cert-ping", a.GetCertPing)
	}

	db := g.Group("/database")
	{
		db.GET("/export", a.GetDb)
		db.POST("/import", a.ImportDb)
		db.GET("/info", a.GetDatabaseInfo)
	}

	g.GET("/inbounds", a.GetInbounds)
	g.GET("/outbounds", a.GetOutbounds)
	g.GET("/endpoints", a.GetEndpoints)
	g.GET("/services", a.GetServices)
	g.GET("/tls", a.GetTls)
	g.GET("/clients", a.GetClients)
	g.GET("/settings", a.GetSettings)

	legacy := g.Group("", a.deprecationMiddleware)
	{
		legacy.POST("/save", a.Save)
		legacy.POST("/restartApp", a.RestartApp)
		legacy.POST("/restartSb", a.RestartSb)
		legacy.POST("/maintenance", a.SetMaintenance)
		legacy.POST("/resetTraffic", a.ResetTraffic)
		legacy.POST("/linkConvert", a.LinkConvert)
		legacy.POST("/subConvert", a.SubConvert)
		legacy.POST("/importdb", a.ImportDb)
		legacy.POST("/closeSessions", a.CloseSessions)
		legacy.POST("/getCertPing", a.GetCertPing)

		legacy.GET("/load", a.LoadData)
		legacy.GET("/status", a.GetStatus)
		legacy.GET("/onlines", a.GetOnlines)
		legacy.GET("/sessions", a.GetSessions)
		legacy.GET("/logs", a.GetLogs)
		legacy.GET("/changes", a.CheckChanges)
		legacy.GET("/keypairs", a.GetKeypairs)
		legacy.GET("/getdb", a.GetDb)
		legacy.GET("/checkOutbound", a.GetCheckOutbound)
	}
}

func (a *APIv2Handler) deprecationMiddleware(c *gin.Context) {
	c.Header("Deprecation", "true")
	c.Header("X-API-Warn", "This RPC-style endpoint is deprecated. Consider using structured REST routes.")
	c.Next()
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
		if t.Expiry > 0 && t.Expiry < now {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(t.Token), []byte(token)) == 1 {
			return t.Username
		}
	}
	return ""
}

func (a *APIv2Handler) checkToken(c *gin.Context) {
	username := a.findUsername(c)
	if username != "" {
		c.Set("auth_username", username)
		c.Next()
		return
	}
	c.AbortWithStatusJSON(http.StatusUnauthorized, Msg{
		Success: false,
		Msg:     "invalid or expired token",
	})
}

func (a *APIv2Handler) getCallerUsername(c *gin.Context) string {
	if val, ok := c.Get("auth_username"); ok {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return a.findUsername(c)
}

func (a *APIv2Handler) ReloadTokens() {
	tokens, err := a.ApiService.LoadTokens()
	if err != nil {
		logger.Error("unable to load tokens: ", err)
		return
	}
	var newTokens []TokenInMemory
	if err := json.Unmarshal(tokens, &newTokens); err != nil {
		logger.Error("unable to load tokens: ", err)
		return
	}

	a.tokensMu.Lock()
	a.tokens = newTokens
	a.tokensMu.Unlock()
}

// --- Handlers with Swagger Annotations ---

// RestartApp godoc
// @Summary      Restart application panel
// @Tags         System
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/system/restart-app [post]
func (a *APIv2Handler) RestartApp(c *gin.Context) {
	a.ApiService.RestartApp(c)
}

// RestartSb godoc
// @Summary      Restart sing-box core
// @Tags         System
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/system/restart-sb [post]
func (a *APIv2Handler) RestartSb(c *gin.Context) {
	a.ApiService.RestartSb(c)
}

// SetMaintenance godoc
// @Summary      Set core maintenance mode
// @Tags         System
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        enable formData bool true "Enable or disable maintenance mode"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/system/maintenance [post]
func (a *APIv2Handler) SetMaintenance(c *gin.Context) {
	a.ApiService.SetMaintenance(c)
}

// GetStatus godoc
// @Summary      Get system and core status
// @Tags         System
// @Security     ApiKeyAuth
// @Produce      json
// @Param        r query string false "Subsystem filter (cpu,mem,dsk,dio,swp,net,sys,sbd,db)"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/system/status [get]
func (a *APIv2Handler) GetStatus(c *gin.Context) {
	a.ApiService.GetStatus(c)
}

// GetLogs godoc
// @Summary      Get server logs
// @Tags         System
// @Security     ApiKeyAuth
// @Produce      json
// @Param        c query int false "Number of lines" default(10)
// @Param        l query string false "Log level"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/system/logs [get]
func (a *APIv2Handler) GetLogs(c *gin.Context) {
	a.ApiService.GetLogs(c)
}

// GetUsers godoc
// @Summary      Get admin users
// @Tags         Users
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/users [get]
func (a *APIv2Handler) GetUsers(c *gin.Context) {
	a.ApiService.GetUsers(c)
}

// ResetTraffic godoc
// @Summary      Reset traffic for all clients
// @Tags         Users
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/users/reset-traffic [post]
func (a *APIv2Handler) ResetTraffic(c *gin.Context) {
	a.ApiService.ResetTraffic(c)
}

// CloseSessions godoc
// @Summary      Close active client sessions
// @Tags         Users
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        u formData string false "User name"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/users/close-sessions [post]
func (a *APIv2Handler) CloseSessions(c *gin.Context) {
	a.ApiService.CloseSessions(c)
}

// GetTokens godoc
// @Summary      Get API tokens for current user
// @Tags         Tokens
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tokens [get]
func (a *APIv2Handler) GetTokens(c *gin.Context) {
	username := a.getCallerUsername(c)
	tokens, err := a.UserService.GetUserTokens(username)
	jsonObj(c, tokens, err)
}

// AddToken godoc
// @Summary      Create a new API token
// @Tags         Tokens
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        expiry formData int true "Expiry in days (0 for permanent)"
// @Param        desc formData string false "Token description"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tokens/add [post]
func (a *APIv2Handler) AddToken(c *gin.Context) {
	username := a.getCallerUsername(c)
	expiryStr := c.Request.FormValue("expiry")
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		jsonMsg(c, "", common.NewError("invalid expiry value"))
		return
	}
	desc := c.Request.FormValue("desc")
	token, err := a.UserService.AddToken(username, expiry, desc)
	if err == nil {
		a.ReloadTokens()
	}
	jsonObj(c, token, err)
}

// DeleteToken godoc
// @Summary      Delete an API token
// @Tags         Tokens
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        id formData string true "Token ID"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tokens/delete [post]
func (a *APIv2Handler) DeleteToken(c *gin.Context) {
	username := a.getCallerUsername(c)
	id := c.Request.FormValue("id")
	if id == "" {
		id = c.Query("id")
	}
	err := a.UserService.DeleteToken(username, id)
	if err == nil {
		a.ReloadTokens()
	}
	jsonMsg(c, "deleteToken", err)
}

// Save godoc
// @Summary      Save configuration object
// @Tags         Config
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        object formData string true "Target object (clients, inbounds, outbounds, settings, etc.)"
// @Param        action formData string true "Action (add, edit, del, etc.)"
// @Param        data formData string true "Payload JSON string"
// @Param        initUsers formData string false "Optional init users"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/config/save [post]
func (a *APIv2Handler) Save(c *gin.Context) {
	username := a.getCallerUsername(c)
	a.ApiService.Save(c, username)
}

// LoadData godoc
// @Summary      Load full panel configuration and state
// @Tags         Config
// @Security     ApiKeyAuth
// @Produce      json
// @Param        lu query string false "Last update timestamp"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/config/load [get]
func (a *APIv2Handler) LoadData(c *gin.Context) {
	a.ApiService.LoadData(c)
}

// CheckChanges godoc
// @Summary      Get audit logs/changes
// @Tags         Config
// @Security     ApiKeyAuth
// @Produce      json
// @Param        a query string false "Actor"
// @Param        k query string false "Key"
// @Param        c query string false "Count"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/config/changes [get]
func (a *APIv2Handler) CheckChanges(c *gin.Context) {
	a.ApiService.CheckChanges(c)
}

// GetSingboxConfig godoc
// @Summary      Download active sing-box JSON configuration
// @Tags         Config
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {file} file
// @Failure      401 {object} Msg
// @Router       /apiv2/config/singbox [get]
func (a *APIv2Handler) GetSingboxConfig(c *gin.Context) {
	a.ApiService.GetSingboxConfig(c)
}

// GetStats godoc
// @Summary      Get traffic statistics
// @Tags         Stats
// @Security     ApiKeyAuth
// @Produce      json
// @Param        resource query string false "Resource type"
// @Param        tag query string false "Resource tag"
// @Param        limit query int false "Result limit" default(100)
// @Param        start query int false "Start timestamp"
// @Param        end query int false "End timestamp"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/stats [get]
func (a *APIv2Handler) GetStats(c *gin.Context) {
	a.ApiService.GetStats(c)
}

// GetOnlines godoc
// @Summary      Get online clients list
// @Tags         Stats
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/stats/onlines [get]
func (a *APIv2Handler) GetOnlines(c *gin.Context) {
	a.ApiService.GetOnlines(c)
}

// GetSessions godoc
// @Summary      Get active user sessions
// @Tags         Stats
// @Security     ApiKeyAuth
// @Produce      json
// @Param        resource query string false "Resource type (default: user)"
// @Param        tag query string false "Tag"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/stats/sessions [get]
func (a *APIv2Handler) GetSessions(c *gin.Context) {
	a.ApiService.GetSessions(c)
}

// LinkConvert godoc
// @Summary      Convert proxy link to outbound configuration
// @Tags         Tools
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        link formData string true "Proxy link (vmess://, vless://, etc.)"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tools/link-convert [post]
func (a *APIv2Handler) LinkConvert(c *gin.Context) {
	a.ApiService.LinkConvert(c)
}

// SubConvert godoc
// @Summary      Convert external subscription URL
// @Tags         Tools
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        link formData string true "Subscription URL"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tools/sub-convert [post]
func (a *APIv2Handler) SubConvert(c *gin.Context) {
	a.ApiService.SubConvert(c)
}

// GetKeypairs godoc
// @Summary      Generate cryptographic keypairs
// @Tags         Tools
// @Security     ApiKeyAuth
// @Produce      json
// @Param        k query string true "Keypair type (reality, tls, ech, wireguard, openvpn)"
// @Param        o query string false "Options (e.g. serverName for TLS/ECH)"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tools/keypairs [get]
func (a *APIv2Handler) GetKeypairs(c *gin.Context) {
	a.ApiService.GetKeypairs(c)
}

// GetCheckOutbound godoc
// @Summary      Test outbound connectivity
// @Tags         Tools
// @Security     ApiKeyAuth
// @Produce      json
// @Param        tag query string false "Outbound tag"
// @Param        link query string false "Test link"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tools/check-outbound [get]
func (a *APIv2Handler) GetCheckOutbound(c *gin.Context) {
	a.ApiService.GetCheckOutbound(c)
}

// GetCertPing godoc
// @Summary      Test TLS certificate handshake
// @Tags         Tools
// @Security     ApiKeyAuth
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        domain formData string true "Target domain"
// @Param        port formData string true "Target port"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tools/cert-ping [post]
func (a *APIv2Handler) GetCertPing(c *gin.Context) {
	a.ApiService.GetCertPing(c)
}

// GetDb godoc
// @Summary      Export database backup
// @Tags         Database
// @Security     ApiKeyAuth
// @Produce      application/octet-stream
// @Param        exclude query string false "Tables to exclude"
// @Success      200 {file} file
// @Failure      401 {object} Msg
// @Router       /apiv2/database/export [get]
func (a *APIv2Handler) GetDb(c *gin.Context) {
	a.ApiService.GetDb(c)
}

// ImportDb godoc
// @Summary      Import database backup
// @Tags         Database
// @Security     ApiKeyAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        db formData file true "Database file (.db)"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/database/import [post]
func (a *APIv2Handler) ImportDb(c *gin.Context) {
	a.ApiService.ImportDb(c)
}

// GetDatabaseInfo godoc
// @Summary      Get database summary counts and total traffic
// @Tags         Database
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/database/info [get]
func (a *APIv2Handler) GetDatabaseInfo(c *gin.Context) {
	info := a.ServerService.GetDatabaseInfo()
	jsonObj(c, info, nil)
}

// GetInbounds godoc
// @Summary      Get inbounds list
// @Tags         Resources
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id query string false "Filter by inbound ID"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/inbounds [get]
func (a *APIv2Handler) GetInbounds(c *gin.Context) {
	if err := a.ApiService.LoadPartialData(c, []string{"inbounds"}); err != nil {
		jsonMsg(c, "inbounds", err)
	}
}

// GetOutbounds godoc
// @Summary      Get outbounds list
// @Tags         Resources
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/outbounds [get]
func (a *APIv2Handler) GetOutbounds(c *gin.Context) {
	if err := a.ApiService.LoadPartialData(c, []string{"outbounds"}); err != nil {
		jsonMsg(c, "outbounds", err)
	}
}

// GetEndpoints godoc
// @Summary      Get endpoints list
// @Tags         Resources
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/endpoints [get]
func (a *APIv2Handler) GetEndpoints(c *gin.Context) {
	if err := a.ApiService.LoadPartialData(c, []string{"endpoints"}); err != nil {
		jsonMsg(c, "endpoints", err)
	}
}

// GetServices godoc
// @Summary      Get services list
// @Tags         Resources
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/services [get]
func (a *APIv2Handler) GetServices(c *gin.Context) {
	if err := a.ApiService.LoadPartialData(c, []string{"services"}); err != nil {
		jsonMsg(c, "services", err)
	}
}

// GetTls godoc
// @Summary      Get TLS configurations
// @Tags         Resources
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/tls [get]
func (a *APIv2Handler) GetTls(c *gin.Context) {
	if err := a.ApiService.LoadPartialData(c, []string{"tls"}); err != nil {
		jsonMsg(c, "tls", err)
	}
}

// GetClients godoc
// @Summary      Get clients list
// @Tags         Resources
// @Security     ApiKeyAuth
// @Produce      json
// @Param        id query string false "Filter by client ID (single or comma-separated list: 1,2,3)"
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/clients [get]
func (a *APIv2Handler) GetClients(c *gin.Context) {
	if err := a.ApiService.LoadPartialData(c, []string{"clients"}); err != nil {
		jsonMsg(c, "clients", err)
	}
}

// GetSettings godoc
// @Summary      Get application settings
// @Tags         Resources
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} Msg
// @Failure      401 {object} Msg
// @Router       /apiv2/settings [get]
func (a *APIv2Handler) GetSettings(c *gin.Context) {
	a.ApiService.GetSettings(c)
}
