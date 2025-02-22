package cronjob

import (
	"s-ui/logger"
	"s-ui/service"
)

type StatsJob struct {
	service.StatsService
}

func NewStatsJob() *StatsJob {
	return &StatsJob{}
}

func (s *StatsJob) Run() {
	err := s.StatsService.SaveStats()
	if err != nil {
		logger.Warning("Get stats failed: ", err)
		return
	}
}
