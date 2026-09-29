package cronjob

import (
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

type ResetTrafficJob struct {
	service.ClientService
	service.ConfigService
	service.SettingService
}

func NewResetTrafficJob() *ResetTrafficJob {
	return &ResetTrafficJob{}
}

// Run reads the spec on every tick, so a saved schedule applies without a
// panel restart. globalResetLast holds the next armed boundary; 0 means none.
func (s *ResetTrafficJob) Run() {
	spec, err := s.SettingService.GetGlobalReset()
	if err != nil {
		logger.Warning("ResetTrafficJob: get schedule failed: ", err)
		return
	}
	if spec == "" || spec == "off" {
		return
	}
	// Save rejects a bad spec, so this only trips on one written before that
	// check existed. Debug, not Warning: it would repeat every minute.
	schedule, err := service.CronParser.Parse(spec)
	if err != nil {
		logger.Debug("ResetTrafficJob: invalid cron spec <", spec, ">: ", err)
		return
	}

	loc, err := s.SettingService.GetTimeLocation()
	if err != nil {
		logger.Warning("ResetTrafficJob: get time location failed: ", err)
		return
	}
	now := time.Now().In(loc)

	next, err := s.SettingService.GetGlobalResetLast()
	if err != nil {
		logger.Warning("ResetTrafficJob: get last reset time failed: ", err)
		return
	}
	// A new or changed schedule: arm its first boundary rather than reset now.
	if next == 0 {
		next = schedule.Next(now).Unix()
		if err = s.SettingService.SetGlobalResetLast(next); err != nil {
			logger.Warning("ResetTrafficJob: set next reset time failed: ", err)
			return
		}
		logger.Info("ResetTrafficJob: next reset at ", time.Unix(next, 0).In(loc).Format(time.RFC3339))
		return
	}
	if next > now.Unix() {
		return
	}

	if err = s.ClientService.ResetAllClientsTraffic(); err != nil {
		logger.Warning("ResetTrafficJob: reset all clients failed: ", err)
		return
	}

	// Restart before the bookkeeping write: clients are re-enabled in the
	// database but the core still holds the old user list, and a failed write
	// used to return early and leave them disconnected. The watchdog does not
	// help -- it only starts a core that is not running.
	if err = s.ConfigService.RestartCore(); err != nil {
		logger.Error("ResetTrafficJob: unable to restart core: ", err)
	}

	// Advance to the next boundary. schedule.Next returns the nearest upcoming
	// occurrence, so if several periods were missed (e.g. downtime) it snaps
	// forward instead of resetting once per missed period.
	after := schedule.Next(now)
	if err = s.SettingService.SetGlobalResetLast(after.Unix()); err != nil {
		logger.Warning("ResetTrafficJob: set last reset time failed: ", err)
		return
	}
	logger.Info("ResetTrafficJob: traffic reset for all clients; next reset at ", after.Format(time.RFC3339))
}
