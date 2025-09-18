package cronjob

import (
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
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
	logger.Debug("Stats older than ", s.trafficAge, " days were deleted")
}
