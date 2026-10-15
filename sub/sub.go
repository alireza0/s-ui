package sub

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/middleware"
	"github.com/alireza0/s-ui/network"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/http/httpguts"
)

type Server struct {
	httpServer *http.Server
	listener   net.Listener
	ctx        context.Context
	cancel     context.CancelFunc

	service.SettingService
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
	engine.Use(gzip.Gzip(gzip.DefaultCompression))

	customHeadersStr, err := s.SettingService.GetSubCustomHeaders()
	if err != nil {
		return nil, err
	}
	if customHeaders := parseCustomHeaders(customHeadersStr); len(customHeaders) > 0 {
		engine.Use(func(c *gin.Context) {
			for _, h := range customHeaders {
				c.Writer.Header().Set(h.key, h.value)
			}
			c.Next()
		})
	}

	subPath, err := s.SettingService.GetSubPath()
	if err != nil {
		return nil, err
	}

	subDomain, err := s.SettingService.GetSubDomain()
	if err != nil {
		return nil, err
	}

	if subDomain != "" {
		engine.Use(middleware.DomainValidator(subDomain))
	}

	g := engine.Group(subPath)
	NewSubHandler(g)

	return engine, nil
}

type customHeader struct {
	key   string
	value string
}

func parseCustomHeaders(raw string) []customHeader {
	var headers []customHeader
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "" || !httpguts.ValidHeaderFieldName(key) || !httpguts.ValidHeaderFieldValue(value) {
			continue
		}
		headers = append(headers, customHeader{key: key, value: value})
	}
	return headers
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

	certFile, err := s.SettingService.GetSubCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.SettingService.GetSubKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.SettingService.GetSubListen()
	if err != nil {
		return err
	}
	port, err := s.SettingService.GetSubPort()
	if err != nil {
		return err
	}

	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	// Both or neither, same as the panel: an OR here started TLS with half a
	// configuration and failed with an error that named neither setting.
	switch {
	case certFile != "" && keyFile != "":
		subDomain, err := s.SettingService.GetSubDomain()
		if err != nil {
			listener.Close()
			return err
		}
		c, err := network.NewTLSConfig(certFile, keyFile, subDomain)
		if err != nil {
			listener.Close()
			return err
		}
		listener = network.NewAutoHttpsListener(listener)
		listener = tls.NewListener(listener, c)
		logger.Info("Sub server run https on", listener.Addr())
	case certFile != "" || keyFile != "":
		listener.Close()
		missing, set := "subKeyFile", "subCertFile"
		if certFile == "" {
			missing, set = "subCertFile", "subKeyFile"
		}
		return common.NewError("TLS needs both a certificate and a key: ", set,
			" is set but ", missing, " is empty. Set both to serve HTTPS, or clear both to serve HTTP.")
	default:
		logger.Info("Sub server run http on", listener.Addr())
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler: engine,
		// This port faces every client of the panel, not just the operator.
		ReadHeaderTimeout: 20 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
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
