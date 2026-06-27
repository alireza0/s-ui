package cronjob

import (
	"time"

	"github.com/alireza0/s-ui/logger"

	"github.com/robfig/cron/v3"
)

// cronParser accepts standard 5-field cron, optional leading seconds (6-field)
// and descriptors (@daily, @weekly, @every 10s, ...). Used both for the cron
// engine and for parsing the user-provided globalReset spec.
var cronParser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

type CronJob struct {
	cron *cron.Cron
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

func (c *CronJob) Start(loc *time.Location, trafficAge int, statsBucketSeconds int64, globalReset string) error {
	c.cron = cron.New(cron.WithLocation(loc), cron.WithParser(cronParser))
	c.cron.Start()

	go func() {
		// Start stats job
		c.cron.AddJob("@every 10s", NewStatsJob(trafficAge > 0, statsBucketSeconds))
		// Start expiry job
		c.cron.AddJob("@every 1m", NewDepleteJob())
		// Periodic global traffic reset, only when a valid cron spec is configured
		if globalReset != "" && globalReset != "off" {
			schedule, err := cronParser.Parse(globalReset)
			if err != nil {
				logger.Warning("invalid globalReset cron spec <", globalReset, ">: ", err)
			} else {
				c.cron.AddJob(globalReset, NewResetTrafficJob(schedule))
			}
		}
		// Start deleting old stats
		if trafficAge > 0 {
			c.cron.AddJob("@daily", NewDelStatsJob(trafficAge))
		}
		// Start core if it is not running
		c.cron.AddJob("@every 5s", NewCheckCoreJob())
		// database WAL checkpoint
		c.cron.AddJob("@every 10m", NewWALCheckpointJob())
	}()

	return nil
}

func (c *CronJob) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}
