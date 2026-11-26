package database

import (
	"os"
	"testing"

	"github.com/alireza0/s-ui/logger"
	"github.com/op/go-logging"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}
