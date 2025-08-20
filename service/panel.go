package service

import (
	"os"
	"runtime"
	"s-ui/logger"
	"syscall"
	"time"
)

type PanelService struct {
}

func (s *PanelService) RestartPanel(delay time.Duration) error {
	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(delay)
		if runtime.GOOS == "windows" {
			err = p.Kill()
		} else {
			err = p.Signal(syscall.SIGHUP)
		}
		if err != nil {
			logger.Error("send signal SIGHUP failed:", err)
		}
	}()
	return nil
}
