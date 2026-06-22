package service

import (
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

type onlines struct {
	Inbound  []string `json:"inbound,omitempty"`
	User     []string `json:"user,omitempty"`
	Outbound []string `json:"outbound,omitempty"`
}

var onlineResources = &onlines{}

type StatsService struct {
}

func (s *StatsService) SaveStats(enableTraffic bool) error {
	if corePtr == nil || !corePtr.IsRunning() {
		return nil
	}
	box := corePtr.GetInstance()
	if box == nil {
		return nil
	}
	st := box.StatsTracker()
	if st == nil {
		return nil
	}
	stats := st.GetStats()

	// Reset onlines
	onlineResources.Inbound = nil
	onlineResources.Outbound = nil
	onlineResources.User = nil

	if len(*stats) == 0 {
		return nil
	}

	var err error
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	for _, stat := range *stats {
		if stat.Resource == "user" {
			if stat.Direction {
				err = tx.Model(model.Client{}).Where("name = ?", stat.Tag).
					UpdateColumn("up", gorm.Expr("up + ?", stat.Traffic)).Error
			} else {
				err = tx.Model(model.Client{}).Where("name = ?", stat.Tag).
					UpdateColumn("down", gorm.Expr("down + ?", stat.Traffic)).Error
			}
			if err != nil {
				return err
			}
		}
		if stat.Direction {
			switch stat.Resource {
			case "inbound":
				onlineResources.Inbound = append(onlineResources.Inbound, stat.Tag)
			case "outbound":
				onlineResources.Outbound = append(onlineResources.Outbound, stat.Tag)
			case "user":
				onlineResources.User = append(onlineResources.User, stat.Tag)
			}
		}
	}

	if !enableTraffic {
		return nil
	}
	err = tx.Create(&stats).Error
	return err
}

func (s *StatsService) GetStats(resource string, tag string, limit int) (any, error) {
	var err error
	var result []model.Stats

	currentTime := time.Now().Unix()
	timeDiff := currentTime - (int64(limit) * 3600)

	db := database.GetDB()
	resources := []string{resource}
	if resource == "endpoint" {
		resources = []string{"inbound", "outbound"}
	}
	err = db.Model(model.Stats{}).Where("resource in ? AND tag = ? AND date_time > ?", resources, tag, timeDiff).Order("date_time ASC").Scan(&result).Error
	if err != nil {
		return nil, err
	}

	return s.downsampleStats(result, timeDiff, currentTime, 360), nil
}

func (s *StatsService) downsampleStats(stats []model.Stats, startTime, endTime int64, numBuckets int) any {
	result := make(map[int64][]int64)
	bucketSpan := (endTime - startTime) / int64(numBuckets)
	if bucketSpan == 0 {
		bucketSpan = 1
	}

	for _, r := range stats {
		bucket := (r.DateTime - startTime) / bucketSpan
		if bucket < 0 {
			bucket = 0
		}
		if bucket >= int64(numBuckets) {
			bucket = int64(numBuckets) - 1
		}
		if _, ok := result[bucket]; !ok {
			result[bucket] = []int64{0, 0}
		}
		if r.Direction {
			result[bucket][0] += r.Traffic
		} else {
			result[bucket][1] += r.Traffic
		}
	}

	return map[string]any{"stats": result, "startTime": startTime, "bucketSpan": bucketSpan}
}

func (s *StatsService) GetOnlines() (onlines, error) {
	return *onlineResources, nil
}
func (s *StatsService) DelOldStats(days int) error {
	oldTime := time.Now().AddDate(0, 0, -(days)).Unix()
	db := database.GetDB()
	return db.Where("date_time < ?", oldTime).Delete(model.Stats{}).Error
}
