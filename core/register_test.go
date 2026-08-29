package core

import (
	"slices"
	"testing"

	C "github.com/sagernet/sing-box/constant"
)

// TestCertificateProviderRegistry guards that every certificate provider type
// s-ui can emit is registered. The types are registered behind build tags, but
// the stubs register too, so the set is the same in every build.
func TestCertificateProviderRegistry(t *testing.T) {
	types := CertificateProviderRegistry().OptionTypes()
	for _, expected := range []string{C.TypeACME, C.TypeTailscale, C.TypeCloudflareOriginCA} {
		if !slices.Contains(types, expected) {
			t.Errorf("certificate provider %q is not registered, got %v", expected, types)
		}
	}
}
