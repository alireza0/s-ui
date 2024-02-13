package cronjob

import (
	"s-ui/logger"
	"s-ui/service"
)

type StatsJob struct {
	service.SingBoxService
}

func NewStatsJob() *StatsJob {
	return new(StatsJob)
}

func (s *StatsJob) Run() {
	err := s.SingBoxService.GetStats()
	if err != nil {
		logger.Warning("Get stats failed: ", err)
		return
	}
}
