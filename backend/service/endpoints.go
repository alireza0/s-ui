package service

import (
	"encoding/json"
	"s-ui/database"
	"s-ui/database/model"

	"gorm.io/gorm"
)

type EndpointService struct{}

func (o *EndpointService) GetAll() ([]*model.Endpoint, error) {
	db := database.GetDB()
	endpoints := []*model.Endpoint{}
	err := db.Model(model.Endpoint{}).Scan(&endpoints).Error
	if err != nil {
		return nil, err
	}
	return endpoints, nil
}

func (o *EndpointService) Get(id uint) (*model.Endpoint, error) {
	db := database.GetDB()
	endpoint := &model.Endpoint{}
	err := db.First(endpoint, id).Error
	if err != nil {
		return nil, err
	}
	return endpoint, nil
}

func (o *EndpointService) GetAllConfig(db *gorm.DB) ([]json.RawMessage, error) {
	var endpointsJson []json.RawMessage
	var endpoints []*model.Endpoint
	err := db.Model(model.Endpoint{}).Scan(&endpoints).Error
	if err != nil {
		return nil, err
	}
	for _, endpoint := range endpoints {
		endpointJson, err := endpoint.MarshalJSON()
		if err != nil {
			return nil, err
		}
		endpointsJson = append(endpointsJson, endpointJson)
	}
	return endpointsJson, nil
}
