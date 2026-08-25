package service

import (
	"encoding/json"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"

	"gorm.io/gorm"
)

// AddTraffic atomically adds up/down deltas to a client matched by name.
// Used by external clients (e.g. a custom Android app) to report locally
// measured usage. Security model (tokenless by design):
//   - deltas must be >= 0 (counters can never be reduced or reset here)
//   - unknown names are rejected (RowsAffected == 0 -> ErrRecordNotFound)
//   - endpoint exposes nothing to read and cannot enable/disable clients
type trafficDelta struct {
	Name string `json:"name"`
	Up   int64  `json:"up"`
	Down int64  `json:"down"`
}

func (s *ClientService) AddTraffic(data json.RawMessage) error {
	var d trafficDelta
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	if d.Name == "" {
		return gorm.ErrRecordNotFound
	}
	if d.Up < 0 || d.Down < 0 {
		return gorm.ErrInvalidData
	}
	if d.Up == 0 && d.Down == 0 {
		return nil
	}
	db := database.GetDB()
	res := db.Model(model.Client{}).
		Where("name = ?", d.Name).
		Updates(map[string]interface{}{
			"up":        gorm.Expr("up + ?", d.Up),
			"down":      gorm.Expr("down + ?", d.Down),
			"online_at": time.Now().Unix(),
		})
	if res.Error != nil {
		logger.Warning("addTraffic failed for", d.Name, ":", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
