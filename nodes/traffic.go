package nodes

import (
	"strconv"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TrafficConfig struct {
	Enabled       bool
	BucketSeconds int64
}

type trafficConfigKey struct{}

func recordNodeTraffic(db *gorm.DB, node model.Node, sample *nodeagent.TrafficSample, at time.Time, bucketSeconds int64) error {
	if sample == nil || sample.Up < 0 || sample.Down < 0 {
		return nil
	}
	process, err := uuid.FromString(sample.Process)
	if err != nil || process == uuid.Nil {
		return nil
	}
	observed := at.UnixNano()
	period := max(bucketSeconds, 1)
	bucket := at.Unix() - at.Unix()%period
	return db.Transaction(func(tx *gorm.DB) error { return recordNodeTrafficSample(tx, node, sample, observed, bucket) })
}

func recordNodeTrafficSample(tx *gorm.DB, node model.Node, sample *nodeagent.TrafficSample, observed, bucket int64) error {
	var current model.Node
	query := tx.Where("id = ? AND address = ? AND control_epoch = ? AND management_generation = ?", node.Id, node.Address, node.ControlEpoch, node.ManagementGeneration)
	found := query.Find(&current)
	if found.Error != nil || found.RowsAffected == 0 || observed <= current.TrafficSampleAt {
		return found.Error
	}
	up, down, valid := nodeTrafficDelta(current, sample)
	if !valid {
		return nil
	}
	cursor := tx.Model(&model.Node{}).Where("id = ? AND traffic_sample_at = ?", current.Id, current.TrafficSampleAt).Updates(map[string]any{
		"traffic_process": sample.Process, "traffic_sample_at": observed, "traffic_up": sample.Up, "traffic_down": sample.Down,
	})
	if cursor.Error != nil || cursor.RowsAffected == 0 {
		return cursor.Error
	}
	tag := strconv.FormatUint(uint64(node.Id), 10)
	rows := make([]model.Stats, 0, 2)
	if up > 0 {
		rows = append(rows, model.Stats{DateTime: bucket, Resource: "node", Tag: tag, Direction: true, Traffic: up})
	}
	if down > 0 {
		rows = append(rows, model.Stats{DateTime: bucket, Resource: "node", Tag: tag, Traffic: down})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "resource"}, {Name: "tag"}, {Name: "date_time"}, {Name: "direction"}},
		DoUpdates: clause.Assignments(map[string]any{"traffic": gorm.Expr("stats.traffic + excluded.traffic")}),
	}).Create(&rows).Error
}

func nodeTrafficDelta(current model.Node, sample *nodeagent.TrafficSample) (int64, int64, bool) {
	up, down := sample.Up, sample.Down
	if current.TrafficProcess == sample.Process {
		if up < current.TrafficUp || down < current.TrafficDown {
			return 0, 0, false
		}
		up -= current.TrafficUp
		down -= current.TrafficDown
	}
	if current.TrafficProcess == sample.Process && up == 0 && down == 0 {
		return 0, 0, false
	}
	return up, down, true
}
