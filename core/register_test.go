package core

import (
	"slices"
	"testing"

	C "github.com/sagernet/sing-box/constant"
)

// assertRegistered reports every expected type the registry is missing, so a
// protocol dropped from a registry fails loudly instead of only surfacing when
// someone writes a config for it.
func assertRegistered(t *testing.T, registry string, got []string, expected ...string) {
	t.Helper()
	for _, want := range expected {
		if !slices.Contains(got, want) {
			t.Errorf("%s registry is missing %q; got %v", registry, want, got)
		}
	}
}

func TestInboundRegistry(t *testing.T) {
	assertRegistered(t, "inbound", InboundRegistry().OptionTypes(),
		// carried over from 1.13
		C.TypeMixed, C.TypeSOCKS, C.TypeHTTP, C.TypeShadowsocks, C.TypeVMess,
		C.TypeTrojan, C.TypeVLESS, C.TypeAnyTLS, C.TypeHysteria, C.TypeHysteria2,
		C.TypeTUIC, C.TypeShadowTLS, C.TypeNaive, C.TypeTun, C.TypeDirect,
		// new in 1.14
		C.TypeSnell, C.TypeCloudflared,
	)
}

func TestOutboundRegistry(t *testing.T) {
	assertRegistered(t, "outbound", OutboundRegistry().OptionTypes(),
		C.TypeDirect, C.TypeBlock, C.TypeSelector, C.TypeURLTest,
		C.TypeSOCKS, C.TypeHTTP, C.TypeShadowsocks, C.TypeVMess, C.TypeTrojan,
		C.TypeVLESS, C.TypeAnyTLS, C.TypeSSH, C.TypeTor, C.TypeShadowTLS,
		C.TypeHysteria, C.TypeHysteria2, C.TypeTUIC,
		// new in 1.14
		C.TypeSnell, C.TypeBridge,
	)
}

func TestEndpointRegistry(t *testing.T) {
	assertRegistered(t, "endpoint", EndpointRegistry().OptionTypes(),
		C.TypeWireGuard, C.TypeTailscale,
		// new in 1.14
		C.TypeOpenConnect, C.TypeOpenVPNClient, C.TypeOpenVPNServer,
	)
}

func TestDNSTransportRegistry(t *testing.T) {
	assertRegistered(t, "dns transport", DNSTransportRegistry().OptionTypes(),
		C.DNSTypeLocal, C.DNSTypeHosts, C.DNSTypeTCP, C.DNSTypeUDP, C.DNSTypeTLS,
		C.DNSTypeHTTPS, C.DNSTypeQUIC, C.DNSTypeHTTP3, C.DNSTypeDHCP,
		C.DNSTypeFakeIP, C.DNSTypeTailscale,
	)
}

func TestServiceRegistry(t *testing.T) {
	assertRegistered(t, "service", ServiceRegistry().OptionTypes(),
		C.TypeResolved, C.TypeSSMAPI, C.TypeDERP, C.TypeCCM, C.TypeOCM,
		// new in 1.14
		C.TypeAPI, C.TypeOOMKiller, C.TypeUSBIPServer, C.TypeUSBIPClient,
	)
}

// TestCertificateProviderRegistry guards that every certificate provider type
// s-ui can emit is registered. The types are registered behind build tags, but
// the stubs register too, so the set is the same in every build.
func TestCertificateProviderRegistry(t *testing.T) {
	assertRegistered(t, "certificate provider", CertificateProviderRegistry().OptionTypes(),
		C.TypeACME, C.TypeTailscale, C.TypeCloudflareOriginCA,
	)
}
