package core

import (
	"os"
	"strings"
	"testing"

	"github.com/alireza0/s-ui/logger"

	"github.com/op/go-logging"
)

// TestMain initialises the s-ui logger before any test runs. Building the
// registries logs through it (the naive stub reports that naive is absent), and
// an uninitialised logger panics.
func TestMain(m *testing.M) {
	logger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

// skipIfFeatureMissing skips the test when sing-box reports that the feature
// under test was compiled out. That is how it signals a missing build tag, so
// the defaults tests can cover every type in a release build without failing in
// a plain `go test ./...`. Only that exact signal is skipped: any other error
// still fails, and so does a reworded one.
func skipIfFeatureMissing(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "is not included in this build") {
		t.Skip(err.Error())
	}
}

// skipUnstartedEndpointInShort skips a case that builds an endpoint and closes
// it without starting it. sing-box adds an endpoint's device Close to the start
// scope, so such an endpoint leaks its gvisor stack, whose goroutines keep
// waking for the rest of the package. Under -race that slows every later test
// by minutes. Building is not concurrent, so -race finds nothing here, and the
// run without -short still covers it.
func skipUnstartedEndpointInShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("an unstarted endpoint leaks its gvisor stack; covered without -short")
	}
}
