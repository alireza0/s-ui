package cronjob

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	cron *cron.Cron
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

func (c *CronJob) Start(loc *time.Location) error {
	c.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	c.cron.Start()

	go func() {
		// Start stats job
		c.cron.AddJob("@every 10s", NewStatsJob())
		// Start expiry job
		c.cron.AddJob("@every 1m", NewDepleteJob())
		// Start deleting old stats
		c.cron.AddJob("@daily", NewDelStatsJob())
	}()

	return nil
}

func (c *CronJob) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}
