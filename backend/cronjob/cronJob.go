package cronjob

import (
	"s-ui/core"
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	cron *cron.Cron
	Core *core.Core
}

func NewCronJob(c *core.Core) *CronJob {
	return &CronJob{
		Core: c,
	}
}

func (c *CronJob) Start(loc *time.Location, trafficAge int) error {
	c.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	c.cron.Start()

	go func() {
		// Start stats job
		c.cron.AddJob("@every 10s", NewStatsJob())
		// Start expiry job
		c.cron.AddJob("@every 1m", NewDepleteJob())
		// Start deleting old stats
		c.cron.AddJob("@daily", NewDelStatsJob(trafficAge))
	}()

	return nil
}

func (c *CronJob) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}
