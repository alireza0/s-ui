package migration

import (
	"time"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

func to1_4(tx *gorm.DB) error {
	// Add traffic reset columns to clients table
	if !tx.Migrator().HasColumn(&model.Client{}, "created_at") {
		err := tx.Migrator().AddColumn(&model.Client{}, "created_at")
		if err != nil {
			return err
		}
		// Set created_at to current time for existing clients
		err = tx.Model(&model.Client{}).Where("created_at IS NULL OR created_at = 0").
			Update("created_at", time.Now().Unix()).Error
		if err != nil {
			return err
		}
	}

	if !tx.Migrator().HasColumn(&model.Client{}, "reset_mode") {
		err := tx.Migrator().AddColumn(&model.Client{}, "reset_mode")
		if err != nil {
			return err
		}
	}

	if !tx.Migrator().HasColumn(&model.Client{}, "reset_day_of_month") {
		err := tx.Migrator().AddColumn(&model.Client{}, "reset_day_of_month")
		if err != nil {
			return err
		}
	}

	if !tx.Migrator().HasColumn(&model.Client{}, "reset_period_days") {
		err := tx.Migrator().AddColumn(&model.Client{}, "reset_period_days")
		if err != nil {
			return err
		}
	}

	if !tx.Migrator().HasColumn(&model.Client{}, "last_reset_at") {
		err := tx.Migrator().AddColumn(&model.Client{}, "last_reset_at")
		if err != nil {
			return err
		}
	}

	// Create traffic_histories table
	if !tx.Migrator().HasTable(&model.TrafficHistory{}) {
		err := tx.Migrator().CreateTable(&model.TrafficHistory{})
		if err != nil {
			return err
		}
	}

	return nil
}
