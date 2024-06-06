package service

import (
	"encoding/json"
	"s-ui/database"
	"s-ui/database/model"

	"gorm.io/gorm"
)

type TlsService struct {
}

func (s *TlsService) GetAll() (string, error) {
	db := database.GetDB()
	tlsConfig := []model.Tls{}
	err := db.Model(model.Tls{}).Scan(&tlsConfig).Error
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(tlsConfig)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *TlsService) Save(tx *gorm.DB, changes []model.Changes) error {
	var err error
	for _, change := range changes {
		tlsConfig := model.Tls{}
		err = json.Unmarshal(change.Obj, &tlsConfig)
		if err != nil {
			return err
		}
		switch change.Action {
		case "new":
			err = tx.Create(&tlsConfig).Error
		case "del":
			err = tx.Where("id = ?", change.Index).Delete(model.Tls{}).Error
		default:
			err = tx.Save(tlsConfig).Error
		}
		if err != nil {
			return err
		}
	}
	return err
}
