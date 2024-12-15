package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"s-ui/util/common"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/cachefile"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/route"
	sbCommon "github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.Service = (*Box)(nil)

type Box struct {
	createdAt   time.Time
	logFactory  log.Factory
	logger      log.ContextLogger
	network     *route.NetworkManager
	endpoint    *endpoint.Manager
	inbound     *inbound.Manager
	outbound    *outbound.Manager
	connection  *route.ConnectionManager
	router      *route.Router
	services    []adapter.LifecycleService
	connTracker *ConnTracker
	done        chan struct{}
}

type Options struct {
	option.Options
	Context context.Context
}

func Context(
	ctx context.Context,
	inboundRegistry adapter.InboundRegistry,
	outboundRegistry adapter.OutboundRegistry,
	endpointRegistry adapter.EndpointRegistry,
) context.Context {
	if service.FromContext[option.InboundOptionsRegistry](ctx) == nil ||
		service.FromContext[adapter.InboundRegistry](ctx) == nil {
		ctx = service.ContextWith[option.InboundOptionsRegistry](ctx, inboundRegistry)
		ctx = service.ContextWith[adapter.InboundRegistry](ctx, inboundRegistry)
	}
	if service.FromContext[option.OutboundOptionsRegistry](ctx) == nil ||
		service.FromContext[adapter.OutboundRegistry](ctx) == nil {
		ctx = service.ContextWith[option.OutboundOptionsRegistry](ctx, outboundRegistry)
		ctx = service.ContextWith[adapter.OutboundRegistry](ctx, outboundRegistry)
	}
	if service.FromContext[option.EndpointOptionsRegistry](ctx) == nil ||
		service.FromContext[adapter.EndpointRegistry](ctx) == nil {
		ctx = service.ContextWith[option.EndpointOptionsRegistry](ctx, endpointRegistry)
		ctx = service.ContextWith[adapter.EndpointRegistry](ctx, endpointRegistry)
	}
	return ctx
}

func NewBox(options Options) (*Box, error) {
	var err error
	createdAt := time.Now()
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = service.ContextWithDefaultRegistry(ctx)

	endpointRegistry := service.FromContext[adapter.EndpointRegistry](ctx)
	inboundRegistry := service.FromContext[adapter.InboundRegistry](ctx)
	outboundRegistry := service.FromContext[adapter.OutboundRegistry](ctx)

	if endpointRegistry == nil {
		return nil, common.NewError("missing endpoint registry in context")
	}
	if inboundRegistry == nil {
		return nil, common.NewError("missing inbound registry in context")
	}
	if outboundRegistry == nil {
		return nil, common.NewError("missing outbound registry in context")
	}

	ctx = pause.WithDefaultManager(ctx)
	experimentalOptions := sbCommon.PtrValueOrDefault(options.Experimental)
	var needCacheFile bool
	if experimentalOptions.CacheFile != nil && experimentalOptions.CacheFile.Enabled {
		needCacheFile = true
	}
	platformInterface := service.FromContext[platform.Interface](ctx)
	var defaultLogWriter io.Writer
	if platformInterface != nil {
		defaultLogWriter = io.Discard
	}
	var logFactory log.Factory
	if factory == nil {
		logFactory, err = NewFactory(log.Options{
			Context:       ctx,
			Options:       sbCommon.PtrValueOrDefault(options.Log),
			DefaultWriter: defaultLogWriter,
			BaseTime:      createdAt,
		})
		if err != nil {
			return nil, common.NewError("create log factory", err)
		}
		factory = logFactory
	} else {
		logFactory = factory
	}

	routeOptions := sbCommon.PtrValueOrDefault(options.Route)
	endpointManager := endpoint.NewManager(logFactory.NewLogger("endpoint"), endpointRegistry)
	inboundManager := inbound.NewManager(logFactory.NewLogger("inbound"), inboundRegistry, endpointManager)
	outboundManager := outbound.NewManager(logFactory.NewLogger("outbound"), outboundRegistry, endpointManager, routeOptions.Final)
	service.MustRegister[adapter.EndpointManager](ctx, endpointManager)
	service.MustRegister[adapter.InboundManager](ctx, inboundManager)
	service.MustRegister[adapter.OutboundManager](ctx, outboundManager)

	networkManager, err := route.NewNetworkManager(ctx, logFactory.NewLogger("network"), routeOptions)
	if err != nil {
		return nil, common.NewError("initialize network manager", err)
	}
	service.MustRegister[adapter.NetworkManager](ctx, networkManager)
	connectionManager := route.NewConnectionManager(logFactory.NewLogger("connection"))
	service.MustRegister[adapter.ConnectionManager](ctx, connectionManager)
	router, err := route.NewRouter(ctx, logFactory, routeOptions, sbCommon.PtrValueOrDefault(options.DNS))
	if err != nil {
		return nil, common.NewError("initialize router", err)
	}
	for i, endpointOptions := range options.Endpoints {
		var tag string
		if endpointOptions.Tag != "" {
			tag = endpointOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = endpointManager.Create(ctx,
			router,
			logFactory.NewLogger(F.ToString("endpoint/", endpointOptions.Type, "[", tag, "]")),
			tag,
			endpointOptions.Type,
			endpointOptions.Options,
		)
		if err != nil {
			return nil, common.NewError("initialize endpoint["+string(i)+"] "+tag, err)
		}
	}
	for i, inboundOptions := range options.Inbounds {
		var tag string
		if inboundOptions.Tag != "" {
			tag = inboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = inboundManager.Create(ctx,
			router,
			logFactory.NewLogger(F.ToString("inbound/", inboundOptions.Type, "[", tag, "]")),
			tag,
			inboundOptions.Type,
			inboundOptions.Options,
		)
		if err != nil {
			return nil, common.NewError("initialize inbound[", i, "] ", tag, err)
		}
	}
	for i, outboundOptions := range options.Outbounds {
		var tag string
		if outboundOptions.Tag != "" {
			tag = outboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		outboundCtx := ctx
		if tag != "" {
			// TODO: remove this
			outboundCtx = adapter.WithContext(outboundCtx, &adapter.InboundContext{
				Outbound: tag,
			})
		}
		err = outboundManager.Create(
			outboundCtx,
			router,
			logFactory.NewLogger(F.ToString("outbound/", outboundOptions.Type, "[", tag, "]")),
			tag,
			outboundOptions.Type,
			outboundOptions.Options,
		)
		if err != nil {
			return nil, common.NewError("initialize outbound["+string(i)+"] "+tag, err)
		}
	}
	outboundManager.Initialize(sbCommon.Must1(
		direct.NewOutbound(
			ctx,
			router,
			logFactory.NewLogger("outbound/direct"),
			"direct",
			option.DirectOutboundOptions{},
		),
	))
	if platformInterface != nil {
		err = platformInterface.Initialize(networkManager)
		if err != nil {
			return nil, common.NewError("initialize platform interface", err)
		}
	}
	if connTracker == nil {
		connTracker = NewConnTracker()
	}
	router.SetTracker(connTracker)

	var services []adapter.LifecycleService

	if needCacheFile {
		cacheFile := cachefile.New(ctx, sbCommon.PtrValueOrDefault(experimentalOptions.CacheFile))
		service.MustRegister[adapter.CacheFile](ctx, cacheFile)
		services = append(services, cacheFile)
	}
	ntpOptions := sbCommon.PtrValueOrDefault(options.NTP)
	if ntpOptions.Enabled {
		ntpDialer, err := dialer.New(ctx, ntpOptions.DialerOptions)
		if err != nil {
			return nil, common.NewError(err, "create NTP service")
		}
		timeService := ntp.NewService(ntp.Options{
			Context:       ctx,
			Dialer:        ntpDialer,
			Logger:        logFactory.NewLogger("ntp"),
			Server:        ntpOptions.ServerOptions.Build(),
			Interval:      time.Duration(ntpOptions.Interval),
			WriteToSystem: ntpOptions.WriteToSystem,
		})
		service.MustRegister[ntp.TimeService](ctx, timeService)
		services = append(services, adapter.NewLifecycleService(timeService, "ntp service"))
	}
	return &Box{
		network:     networkManager,
		endpoint:    endpointManager,
		inbound:     inboundManager,
		outbound:    outboundManager,
		connection:  connectionManager,
		router:      router,
		createdAt:   createdAt,
		logFactory:  logFactory,
		logger:      logFactory.Logger(),
		services:    services,
		connTracker: connTracker,
		done:        make(chan struct{}),
	}, nil
}

func (s *Box) PreStart() error {
	err := s.preStart()
	if err != nil {
		// TODO: remove catch error
		defer func() {
			v := recover()
			if v != nil {
				s.logger.Error(err.Error())
				s.logger.Error("panic on early close: " + fmt.Sprint(v))
			}
		}()
		s.Close()
		return err
	}
	s.logger.Info("sing-box pre-started (", F.Seconds(time.Since(s.createdAt).Seconds()), "s)")
	return nil
}

func (s *Box) Start() error {
	err := s.start()
	if err != nil {
		// TODO: remove catch error
		defer func() {
			v := recover()
			if v != nil {
				s.logger.Debug(err.Error())
				s.logger.Error("panic on early start: " + fmt.Sprint(v))
			}
		}()
		s.Close()
		return err
	}
	s.logger.Info("sing-box started (", F.Seconds(time.Since(s.createdAt).Seconds()), "s)")
	return nil
}

func (s *Box) preStart() error {
	monitor := taskmonitor.New(s.logger, C.StartTimeout)
	monitor.Start("start logger")
	err := s.logFactory.Start()
	monitor.Finish()
	if err != nil {
		return common.NewError(err, "start logger")
	}
	err = adapter.StartNamed(adapter.StartStateInitialize, s.services) // cache-file
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateInitialize, s.network, s.connection, s.router, s.outbound, s.inbound, s.endpoint)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateStart, s.outbound, s.network, s.connection, s.router)
	if err != nil {
		return err
	}
	return nil
}

func (s *Box) start() error {
	err := s.preStart()
	if err != nil {
		return err
	}
	err = adapter.StartNamed(adapter.StartStateStart, s.services)
	if err != nil {
		return err
	}
	err = s.inbound.Start(adapter.StartStateStart)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateStart, s.endpoint)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStatePostStart, s.outbound, s.network, s.connection, s.router, s.inbound, s.endpoint)
	if err != nil {
		return err
	}
	err = adapter.StartNamed(adapter.StartStatePostStart, s.services)
	if err != nil {
		return err
	}
	err = adapter.Start(adapter.StartStateStarted, s.network, s.connection, s.router, s.outbound, s.inbound, s.endpoint)
	if err != nil {
		return err
	}
	err = adapter.StartNamed(adapter.StartStateStarted, s.services)
	if err != nil {
		return err
	}
	return nil
}

func (s *Box) Close() error {
	select {
	case <-s.done:
		return os.ErrClosed
	default:
		close(s.done)
	}
	err := sbCommon.Close(
		s.inbound, s.outbound, s.router, s.connection, s.network,
	)
	for _, lifecycleService := range s.services {
		err1 := lifecycleService.Close()
		if err1 != nil {
			s.logger.Debug(lifecycleService.Name(), " close error: ", err1)
		}
	}
	err1 := s.logFactory.Close()
	if err1 != nil {
		s.logger.Debug("logger close error: ", err1)
	}
	return err
}

func (s *Box) Uptime() uint32 {
	return uint32(time.Now().Sub(s.createdAt).Seconds())
}

func (s *Box) Network() adapter.NetworkManager {
	return s.network
}

func (s *Box) Router() adapter.Router {
	return s.router
}

func (s *Box) Inbound() adapter.InboundManager {
	return s.inbound
}

func (s *Box) Outbound() adapter.OutboundManager {
	return s.outbound
}

func (s *Box) Endpoint() adapter.EndpointManager {
	return s.endpoint
}

func (s *Box) ConnTracker() *ConnTracker {
	return s.connTracker
}
