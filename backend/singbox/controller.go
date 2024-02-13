package singbox

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"s-ui/config"
	"strings"
)

var serviceName = "sing-box"

type Controller struct {
}

func (s *Controller) GetBinaryName() string {
	return "sing-box"
}

func (s *Controller) GetBinaryPath() string {
	return config.GetBinFolderPath() + "/" + s.GetBinaryName()
}

func (s *Controller) GetConfigPath() string {
	return config.GetBinFolderPath() + "/config.json"
}

func (s *Controller) IsRunning() bool {
	cmd := exec.Command("pgrep", "sing-box")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If pgrep found the Controller, its output will not be empty
	return strings.TrimSpace(string(output)) != ""
}

func (s *Controller) signalSingbox(signal string) error {
	return os.WriteFile(config.GetBinFolderPath()+"/signal", []byte(signal), fs.ModePerm)
}

func (s *Controller) Restart() error {
	return s.signalSingbox("restart")
}

func (s *Controller) Stop() error {
	if !s.IsRunning() {
		return errors.New("Sing-Box is not running")
	}

	return s.signalSingbox("stop")
}
