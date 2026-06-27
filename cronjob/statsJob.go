package cronjob

import (
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

type StatsJob struct {
	service.StatsService
	enableTraffic bool
	bucketSeconds int64
}

func NewStatsJob(saveTraffic bool, bucketSeconds int64) *StatsJob {
	return &StatsJob{
		enableTraffic: saveTraffic,
		bucketSeconds: bucketSeconds,
	}
}

func (s *StatsJob) Run() {
	err := s.StatsService.SaveStats(s.enableTraffic, s.bucketSeconds)
	if err != nil {
		logger.Warning("Get stats failed: ", err)
		return
	}
}
