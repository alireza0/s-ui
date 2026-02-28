package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alireza0/s-ui/util/common"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/certificate"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/experimental/cachefile"
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

var _ adapter.SimpleLifecycle = (*Box)(nil)

type Box struct {
	createdAt       time.Time
	logFactory      log.Factory
	logger          log.ContextLogger
	network         *route.NetworkManager
	endpoint        *endpoint.Manager
	inbound         *inbound.Manager
	outbound        *outbound.Manager
	service         *boxService.Manager
	dnsTransport    *dns.TransportManager
	dnsRouter       *dns.Router
	connection      *route.ConnectionManager
	router          *route.Router
	internalService []adapter.LifecycleService
	statsTracker    *StatsTracker
	connTracker     *ConnTracker
	done            chan struct{}
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
	dnsTransportRegistry adapter.DNSTransportRegistry,
	serviceRegistry adapter.ServiceRegistry,
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
	if service.FromContext[adapter.DNSTransportRegistry](ctx) == nil {
		ctx = service.ContextWith[option.DNSTransportOptionsRegistry](ctx, dnsTransportRegistry)
		ctx = service.ContextWith[adapter.DNSTransportRegistry](ctx, dnsTransportRegistry)
	}
	if service.FromContext[adapter.ServiceRegistry](ctx) == nil {
		ctx = service.ContextWith[option.ServiceOptionsRegistry](ctx, serviceRegistry)
		ctx = service.ContextWith[adapter.ServiceRegistry](ctx, serviceRegistry)
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
	dnsTransportRegistry := service.FromContext[adapter.DNSTransportRegistry](ctx)
	serviceRegistry := service.FromContext[adapter.ServiceRegistry](ctx)

	if endpointRegistry == nil {
		return nil, common.NewError("missing endpoint registry in context")
	}
	if inboundRegistry == nil {
		return nil, common.NewError("missing inbound registry in context")
	}
	if outboundRegistry == nil {
		return nil, common.NewError("missing outbound registry in context")
	}
	if dnsTransportRegistry == nil {
		return nil, common.NewError("missing DNS transport registry in context")
	}
	if serviceRegistry == nil {
		return nil, common.NewError("missing service registry in context")
	}

	ctx = pause.WithDefaultManager(ctx)
	experimentalOptions := sbCommon.PtrValueOrDefault(options.Experimental)
	var needCacheFile bool
	var needClashAPI bool
	var needV2RayAPI bool
	if experimentalOptions.CacheFile != nil && experimentalOptions.CacheFile.Enabled {
		needCacheFile = true
	}
	if experimentalOptions.ClashAPI != nil {
		needClashAPI = true
	}
	if experimentalOptions.V2RayAPI != nil && experimentalOptions.V2RayAPI.Listen != "" {
		needV2RayAPI = true
	}
	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	var defaultLogWriter io.Writer
	if platformInterface != nil {
		defaultLogWriter = io.Discard
	}
	var logFactory log.Factory
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

	var internalServices []adapter.LifecycleService
	certificateOptions := sbCommon.PtrValueOrDefault(options.Certificate)
	if C.IsAndroid || certificateOptions.Store != "" && certificateOptions.Store != C.CertificateStoreSystem ||
		len(certificateOptions.Certificate) > 0 ||
		len(certificateOptions.CertificatePath) > 0 ||
		len(certificateOptions.CertificateDirectoryPath) > 0 {
		certificateStore, err := certificate.NewStore(ctx, logFactory.NewLogger("certificate"), certificateOptions)
		if err != nil {
			return nil, err
		}
		service.MustRegister[adapter.CertificateStore](ctx, certificateStore)
		internalServices = append(internalServices, certificateStore)
	}

	routeOptions := sbCommon.PtrValueOrDefault(options.Route)
	dnsOptions := sbCommon.PtrValueOrDefault(options.DNS)
	endpointManager := endpoint.NewManager(logFactory.NewLogger("endpoint"), endpointRegistry)
	inboundManager := inbound.NewManager(logFactory.NewLogger("inbound"), inboundRegistry, endpointManager)
	outboundManager := outbound.NewManager(logFactory.NewLogger("outbound"), outboundRegistry, endpointManager, routeOptions.Final)
	dnsTransportManager := dns.NewTransportManager(logFactory.NewLogger("dns/transport"), dnsTransportRegistry, outboundManager, dnsOptions.Final)
	serviceManager := boxService.NewManager(logFactory.NewLogger("service"), serviceRegistry)

	service.MustRegister[adapter.EndpointManager](ctx, endpointManager)
	service.MustRegister[adapter.InboundManager](ctx, inboundManager)
	service.MustRegister[adapter.OutboundManager](ctx, outboundManager)
	service.MustRegister[adapter.DNSTransportManager](ctx, dnsTransportManager)
	service.MustRegister[adapter.ServiceManager](ctx, serviceManager)

	dnsRouter := dns.NewRouter(ctx, logFactory, dnsOptions)
	service.MustRegister[adapter.DNSRouter](ctx, dnsRouter)

	networkManager, err := route.NewNetworkManager(ctx, logFactory.NewLogger("network"), routeOptions, dnsOptions)
	if err != nil {
		return nil, common.NewError("initialize network manager", err)
	}
	service.MustRegister[adapter.NetworkManager](ctx, networkManager)
	connectionManager := route.NewConnectionManager(logFactory.NewLogger("connection"))
	service.MustRegister[adapter.ConnectionManager](ctx, connectionManager)
	router := route.NewRouter(ctx, logFactory, routeOptions, dnsOptions)
	service.MustRegister[adapter.Router](ctx, router)
	err = router.Initialize(routeOptions.Rules, routeOptions.RuleSet)
	if err != nil {
		return nil, common.NewError("initialize router", err)
	}
	for i, transportOptions := range dnsOptions.Servers {
		var tag string
		if transportOptions.Tag != "" {
			tag = transportOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = dnsTransportManager.Create(
			ctx,
			logFactory.NewLogger(F.ToString("dns/", transportOptions.Type, "[", tag, "]")),
			tag,
			transportOptions.Type,
			transportOptions.Options,
		)
		if err != nil {
			return nil, common.NewError("initialize DNS server[", i, "]", err)
		}
	}
	err = dnsRouter.Initialize(dnsOptions.Rules)
	if err != nil {
		return nil, common.NewError("initialize dns router", err)
	}
	for i, endpointOptions := range options.Endpoints {
		var tag string
		if endpointOptions.Tag != "" {
			tag = endpointOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = endpointManager.Create(
			ctx,
			router,
			logFactory.NewLogger(F.ToString("endpoint/", endpointOptions.Type, "[", tag, "]")),
			tag,
			endpointOptions.Type,
			endpointOptions.Options,
		)
		if err != nil {
			return nil, common.NewError("initialize endpoint["+F.ToString(i)+"] "+tag, err)
		}
	}
	for i, inboundOptions := range options.Inbounds {
		var tag string
		if inboundOptions.Tag != "" {
			tag = inboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = inboundManager.Create(
			ctx,
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
			return nil, common.NewError("initialize outbound["+F.ToString(i)+"] "+tag, err)
		}
	}
	for i, serviceOptions := range options.Services {
		var tag string
		if serviceOptions.Tag != "" {
			tag = serviceOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		err = serviceManager.Create(
			ctx,
			logFactory.NewLogger(F.ToString("service/", serviceOptions.Type, "[", tag, "]")),
			tag,
			serviceOptions.Type,
			serviceOptions.Options,
		)
		if err != nil {
			return nil, common.NewError("initialize service["+F.ToString(i)+"]"+tag, err)
		}
	}
	outboundManager.Initialize(func() (adapter.Outbound, error) {
		return direct.NewOutbound(
			ctx,
			router,
			logFactory.NewLogger("outbound/direct"),
			"direct",
			option.DirectOutboundOptions{},
		)
	})
	dnsTransportManager.Initialize(func() (adapter.DNSTransport, error) {
		return local.NewTransport(
			ctx,
			logFactory.NewLogger("dns/local"),
			"local",
			option.LocalDNSServerOptions{},
		)
	})
	if platformInterface != nil {
		err = platformInterface.Initialize(networkManager)
		if err != nil {
			return nil, common.NewError("initialize platform interface", err)
		}
	}
	if statsTracker == nil {
		statsTracker = NewStatsTracker()
	}
	router.AppendTracker(statsTracker)
	if connTracker == nil {
		connTracker = NewConnTracker()
	}
	router.AppendTracker(connTracker)

	if needCacheFile {
		cacheFile := cachefile.New(ctx, sbCommon.PtrValueOrDefault(experimentalOptions.CacheFile))
		service.MustRegister[adapter.CacheFile](ctx, cacheFile)
		internalServices = append(internalServices, cacheFile)
	}
	if needClashAPI {
		clashAPIOptions := sbCommon.PtrValueOrDefault(experimentalOptions.ClashAPI)
		clashAPIOptions.ModeList = experimental.CalculateClashModeList(options.Options)
		clashServer, err := experimental.NewClashServer(ctx, logFactory.(log.ObservableFactory), clashAPIOptions)
		if err != nil {
			return nil, common.NewError(err, "create clash-server")
		}
		router.AppendTracker(clashServer)
		service.MustRegister[adapter.ClashServer](ctx, clashServer)
		internalServices = append(internalServices, clashServer)
	}
	if needV2RayAPI {
		v2rayServer, err := experimental.NewV2RayServer(logFactory.NewLogger("v2ray-api"), sbCommon.PtrValueOrDefault(experimentalOptions.V2RayAPI))
		if err != nil {
			return nil, common.NewError(err, "create v2ray-server")
		}
		if v2rayServer.StatsService() != nil {
			router.AppendTracker(v2rayServer.StatsService())
			internalServices = append(internalServices, v2rayServer)
			service.MustRegister[adapter.V2RayServer](ctx, v2rayServer)
		}
	}
	ntpOptions := sbCommon.PtrValueOrDefault(options.NTP)
	if ntpOptions.Enabled {
		ntpDialer, err := dialer.New(ctx, ntpOptions.DialerOptions, ntpOptions.ServerIsDomain())
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
		internalServices = append(internalServices, adapter.NewLifecycleService(timeService, "ntp service"))
	}
	return &Box{
		network:         networkManager,
		endpoint:        endpointManager,
		inbound:         inboundManager,
		outbound:        outboundManager,
		dnsTransport:    dnsTransportManager,
		service:         serviceManager,
		dnsRouter:       dnsRouter,
		connection:      connectionManager,
		router:          router,
		createdAt:       createdAt,
		logFactory:      logFactory,
		logger:          logFactory.Logger(),
		internalService: internalServices,
		statsTracker:    statsTracker,
		connTracker:     connTracker,
		done:            make(chan struct{}),
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
	err = adapter.StartNamed(s.logger, adapter.StartStateInitialize, s.internalService) // cache-file clash-api v2ray-api
	if err != nil {
		return err
	}
	err = adapter.Start(s.logger, adapter.StartStateInitialize, s.network, s.dnsTransport, s.dnsRouter, s.connection, s.router, s.outbound, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.Start(s.logger, adapter.StartStateStart, s.outbound, s.dnsTransport, s.dnsRouter, s.network, s.connection, s.router)
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
	err = adapter.StartNamed(s.logger, adapter.StartStateStart, s.internalService)
	if err != nil {
		return err
	}
	err = adapter.Start(s.logger, adapter.StartStateStart, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.Start(s.logger, adapter.StartStatePostStart, s.outbound, s.network, s.dnsTransport, s.dnsRouter, s.connection, s.router, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.StartNamed(s.logger, adapter.StartStatePostStart, s.internalService)
	if err != nil {
		return err
	}
	err = adapter.Start(s.logger, adapter.StartStateStarted, s.network, s.dnsTransport, s.dnsRouter, s.connection, s.router, s.outbound, s.inbound, s.endpoint, s.service)
	if err != nil {
		return err
	}
	err = adapter.StartNamed(s.logger, adapter.StartStateStarted, s.internalService)
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
		s.service, s.endpoint, s.inbound, s.outbound, s.router, s.connection, s.dnsRouter, s.dnsTransport, s.network,
	)
	for _, lifecycleService := range s.internalService {
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

func (s *Box) StatsTracker() *StatsTracker {
	return s.statsTracker
}

func (s *Box) ConnTracker() *ConnTracker {
	return s.connTracker
}
