package service

import (
	"encoding/json"
	"s-ui/database"
	"s-ui/database/model"

	"gorm.io/gorm"
)

type InDataService struct {
}

func (s *InDataService) GetAll() ([]model.InboundData, error) {
	db := database.GetDB()
	inData := []model.InboundData{}
	err := db.Model(model.InboundData{}).Scan(&inData).Error
	if err != nil {
		return nil, err
	}

	return inData, nil
}

func (s *InDataService) Save(tx *gorm.DB, changes []model.Changes) error {
	var err error
	for _, change := range changes {
		inData := model.InboundData{}
		err = json.Unmarshal(change.Obj, &inData)
		if err != nil {
			return err
		}
		switch change.Action {
		case "new":
			err = tx.Create(&inData).Error
		case "del":
			err = tx.Where("id = ?", change.Index).Delete(model.InboundData{}).Error
		default:
			err = tx.Save(inData).Error
		}
		if err != nil {
			return err
		}
	}
	return err
}
