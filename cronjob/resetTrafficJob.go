package cronjob

import (
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

// ResetTrafficJob handles periodic traffic reset for clients
// It checks all clients with reset_mode enabled and resets their traffic
// based on their configured schedule (monthly or every N days)
type ResetTrafficJob struct {
	service.ClientService
	service.InboundService
}

func NewResetTrafficJob() *ResetTrafficJob {
	return new(ResetTrafficJob)
}

func (s *ResetTrafficJob) Run() {
	inboundIds, err := s.ClientService.ResetClientsTraffic()
	if err != nil {
		logger.Warning("Reset clients traffic failed: ", err)
		return
	}

	// Restart inbounds if any clients were re-enabled
	if len(inboundIds) > 0 {
		logger.Info("Re-enabled clients after traffic reset, restarting inbounds...")
		err := s.InboundService.RestartInbounds(database.GetDB(), inboundIds)
		if err != nil {
			logger.Error("Unable to restart inbounds after traffic reset: ", err)
		}
	}
}
