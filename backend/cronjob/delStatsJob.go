package cronjob

import (
	"s-ui/logger"
	"s-ui/service"
)

type DelStatsJob struct {
	service.StatsService
	trafficAge int
}

func NewDelStatsJob(ta int) *DelStatsJob {
	return &DelStatsJob{
		trafficAge: ta,
	}
}

func (s *DelStatsJob) Run() {
	err := s.StatsService.DelOldStats(s.trafficAge)
	if err != nil {
		logger.Warning("Deleting old statistics failed: ", err)
		return
	}
}
