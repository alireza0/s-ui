package cronjob

import (
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

type CheckCoreJob struct {
	service.ConfigService
	service.ClientService
}

func NewCheckCoreJob() *CheckCoreJob {
	return &CheckCoreJob{}
}

func (s *CheckCoreJob) Run() {
	s.ConfigService.StartCore("")
	err := s.ClientService.CheckAllClients()
	if err != nil {
		logger.Warning("Check clients failed:", err)
	}
}
