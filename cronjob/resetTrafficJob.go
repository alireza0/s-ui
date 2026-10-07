package cronjob

import (
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

type ResetTrafficJob struct {
	service.ClientService
	service.InboundService
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

	inboundIds, err := s.ClientService.ResetAllClientsTraffic()
	if err != nil {
		logger.Warning("ResetTrafficJob: reset all clients failed: ", err)
		return
	}

	// Before the bookkeeping write: clients are re-enabled in the database but
	// the core still holds the old user list. Updated in place, as the deplete
	// job does, rather than by restarting the core: a restart left every QUIC
	// client timing out on its dead session for ~30s (#1278).
	if len(inboundIds) > 0 {
		if err = s.InboundService.UpdateInboundsUsers(database.GetDB(), inboundIds); err != nil {
			logger.Error("ResetTrafficJob: unable to update inbound users: ", err)
		}
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
