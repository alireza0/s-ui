//go:build with_acme

package core

// acmeBuilt reports whether this build includes ACME, so tests over configs
// that declare an ACME certificate provider can skip when it is absent.
const acmeBuilt = true
