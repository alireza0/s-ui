package cronjob

import (
	"s-ui/logger"
	"s-ui/service"
)

type DepleteJob struct {
	service.ClientService
}

func NewDepleteJob() *DepleteJob {
	return new(DepleteJob)
}

func (s *DepleteJob) Run() {
	err := s.ClientService.DepleteClients()
	if err != nil {
		logger.Warning("Disable depleted users failed: ", err)
		return
	}
}
