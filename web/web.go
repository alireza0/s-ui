package web

import (
	"context"
	"crypto/tls"
	"embed"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/api"
	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/middleware"
	"github.com/alireza0/s-ui/network"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

//go:embed *
var content embed.FS

// trustedProxies are the peers whose X-Forwarded-* headers are believed: the
// loopback and private ranges a reverse proxy in front of the panel lives in.
var trustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
	"169.254.0.0/16", "fe80::/10",
}

type Server struct {
	httpServer     *http.Server
	listener       net.Listener
	ctx            context.Context
	cancel         context.CancelFunc
	settingService service.SettingService
}

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	// gin trusts every proxy by default, so any client could set
	// X-Forwarded-For and forge the address written to the login log. Only a
	// loopback or private peer is a plausible reverse proxy; a direct client
	// from the internet is now reported by its real address.
	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return nil, err
	}

	// Load the HTML template
	t := template.New("").Funcs(engine.FuncMap)
	template, err := t.ParseFS(content, "html/index.html")
	if err != nil {
		return nil, err
	}
	engine.SetHTMLTemplate(template)

	base_url, err := s.settingService.GetWebPath()
	if err != nil {
		return nil, err
	}

	webDomain, err := s.settingService.GetWebDomain()
	if err != nil {
		return nil, err
	}

	if webDomain != "" {
		engine.Use(middleware.DomainValidator(webDomain))
	}

	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, err
	}

	engine.Use(gzip.Gzip(gzip.DefaultCompression))
	assetsBasePath := base_url + "assets/"

	sessionMaxAge, err := s.settingService.GetSessionMaxAge()
	if err != nil {
		return nil, err
	}

	store := cookie.NewStore(secret)
	// The per-session options set at login only reach the cookie the login
	// response writes. Without the same lifetime on the store, every later
	// response rewrote the cookie as a session cookie, so "remember me" lasted
	// until the browser was closed whatever the operator configured.
	store.Options(api.BaseSessionOptions(sessionMaxAge))
	engine.Use(sessions.Sessions("s-ui", store))

	engine.Use(func(c *gin.Context) {
		uri := c.Request.RequestURI
		if strings.HasPrefix(uri, assetsBasePath) {
			c.Header("Cache-Control", "max-age=31536000")
		}
	})

	// Serve the assets folder
	assetsFS, err := fs.Sub(content, "html/assets")
	if err != nil {
		panic(err)
	}

	engine.StaticFS(assetsBasePath, http.FS(assetsFS))

	group_apiv2 := engine.Group(base_url + "apiv2")
	apiv2 := api.NewAPIv2Handler(group_apiv2)

	// Only the cookie-authenticated group. apiv2 authenticates with a Token
	// header, which a cross-site page cannot set without the panel answering a
	// CORS preflight it never answers.
	group_api := engine.Group(base_url+"api", middleware.SameOrigin())
	api.NewAPIHandler(group_api, apiv2)

	// Serve index.html as the entry point
	// Handle all other routes by serving index.html
	engine.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == strings.TrimSuffix(base_url, "/") {
			c.Redirect(http.StatusTemporaryRedirect, base_url)
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, base_url) {
			c.String(404, "")
			return
		}
		if c.Request.URL.Path != base_url+"login" && !api.IsLogin(c) {
			c.Redirect(http.StatusTemporaryRedirect, base_url+"login")
			return
		}
		if c.Request.URL.Path == base_url+"login" && api.IsLogin(c) {
			c.Redirect(http.StatusTemporaryRedirect, base_url)
			return
		}
		c.HTML(http.StatusOK, "index.html", gin.H{"BASE_URL": base_url})
	})

	return engine, nil
}

// exposedListen reports whether a listen address binds every interface
// on the machine, so the panel is reachable from any network the host
// sits on -- the default posture, and the one scanners find.
func exposedListen(listen string) bool {
	return listen == "" || listen == "0.0.0.0" || listen == "::"
}

func (s *Server) Start() (err error) {
	//This is an anonymous function, no function name
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	engine, err := s.initRouter()
	if err != nil {
		return err
	}

	certFile, err := s.settingService.GetCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetPort()
	if err != nil {
		return err
	}
	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	if exposedListen(listen) {
		// The panel is the keys to everything on the host, and the
		// default bind exposes it to every network the host sits on.
		// Nothing here changes that default -- deployments bind all
		// interfaces on purpose -- but it should never be silent.
		logger.Warning("web panel is listening on all interfaces (", listenAddr,
			"); anyone who can reach this port can attempt to log in. "+
				"Restrict it with the webListen setting (for example 127.0.0.1 with a tunnel or a firewall whitelist in front) if this host does not need it exposed.")
	}
	// Both or neither. This was an OR, so setting only one of the two put the
	// server into TLS mode with half a configuration and failed to start with
	// an error that named neither setting.
	switch {
	case certFile != "" && keyFile != "":
		webDomain, err := s.settingService.GetWebDomain()
		if err != nil {
			listener.Close()
			return err
		}
		c, err := network.NewTLSConfig(certFile, keyFile, webDomain)
		if err != nil {
			listener.Close()
			return err
		}
		listener = network.NewAutoHttpsListener(listener)
		listener = tls.NewListener(listener, c)
		logger.Info("web server run https on", listener.Addr())
	case certFile != "" || keyFile != "":
		listener.Close()
		missing, set := "webKeyFile", "webCertFile"
		if certFile == "" {
			missing, set = "webCertFile", "webKeyFile"
		}
		return common.NewError("TLS needs both a certificate and a key: ", set,
			" is set but ", missing, " is empty. Set both to serve HTTPS, or clear both to serve HTTP.")
	default:
		logger.Info("web server run http on", listener.Addr())
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler: engine,
		// Without a header deadline a connection that never finishes its
		// request headers holds a goroutine and a file descriptor for good.
		// The body and response limits are deliberately generous: a database
		// import uploads a file, and checkOutbound runs a 15s probe.
		ReadHeaderTimeout: 20 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		s.httpServer.Serve(listener)
	}()

	return nil
}

func (s *Server) Stop() error {
	var err error
	if s.httpServer != nil {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
		err = s.httpServer.Shutdown(shutdownCtx)
		cancelShutdown()
		if err != nil {
			s.cancel()
			if s.listener != nil {
				_ = s.listener.Close()
			}
			return err
		}
	} else if s.listener != nil {
		err = s.listener.Close()
		if err != nil {
			s.cancel()
			return err
		}
	}
	s.cancel()
	return nil
}

func (s *Server) GetCtx() context.Context {
	return s.ctx
}
