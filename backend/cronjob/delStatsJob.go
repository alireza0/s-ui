package cronjob

import (
	"s-ui/logger"
	"s-ui/service"
)

type DelStatsJob struct {
	service.StatsService
}

func NewDelStatsJob() *DelStatsJob {
	return &DelStatsJob{}
}

func (s *DelStatsJob) Run() {
	err := s.StatsService.DelOldStats(30)
	if err != nil {
		logger.Warning("Deleting old statistics failed: ", err)
		return
	}
}
