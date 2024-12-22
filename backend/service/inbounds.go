package service

import (
	"s-ui/database"
	"s-ui/database/model"

	"gorm.io/gorm"
)

type InboundService struct{}

func (s *InboundService) GetAll() ([]model.Inbound, error) {
	db := database.GetDB()
	inbounds := []model.Inbound{}
	err := db.Model(model.Inbound{}).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) FromIds(ids []uint) ([]*model.Inbound, error) {
	db := database.GetDB()
	inbounds := []*model.Inbound{}
	err := db.Model(model.Inbound{}).Where("id in ?", ids).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) Save(db *gorm.DB, inbounds []*model.Inbound) error {
	return db.Save(inbounds).Error
}
