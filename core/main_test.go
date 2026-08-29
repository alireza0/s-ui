package core

import (
	"os"
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
