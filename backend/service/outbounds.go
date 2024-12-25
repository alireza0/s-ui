package service

import (
	"encoding/json"
	"s-ui/database"
	"s-ui/database/model"

	"gorm.io/gorm"
)

type OutboundService struct{}

func (o *OutboundService) GetAll() ([]*model.Outbound, error) {
	db := database.GetDB()
	outbounds := []*model.Outbound{}
	err := db.Model(model.Outbound{}).Scan(&outbounds).Error
	if err != nil {
		return nil, err
	}
	return outbounds, nil
}

func (o *OutboundService) Get(id uint) (*model.Outbound, error) {
	db := database.GetDB()
	outbound := &model.Outbound{}
	err := db.First(outbound, id).Error
	if err != nil {
		return nil, err
	}
	return outbound, nil
}

func (o *OutboundService) GetAllConfig(db *gorm.DB) ([]json.RawMessage, error) {
	var outboundsJson []json.RawMessage
	var outbounds []*model.Outbound
	err := db.Model(model.Outbound{}).Scan(&outbounds).Error
	if err != nil {
		return nil, err
	}
	for _, outbound := range outbounds {
		outboundJson, err := outbound.MarshalJSON()
		if err != nil {
			return nil, err
		}
		outboundsJson = append(outboundsJson, outboundJson)
	}
	return outboundsJson, nil
}
