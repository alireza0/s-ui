package service

import (
	"encoding/json"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
)

type TlsService struct {
	InboundService
	ServicesService
}

func (s *TlsService) GetAll() ([]model.Tls, error) {
	db := database.GetDB()
	tlsConfig := []model.Tls{}
	err := db.Model(model.Tls{}).Scan(&tlsConfig).Error
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

func (s *TlsService) Save(tx *gorm.DB, action string, data json.RawMessage, hostname string) error {
	var err error

	switch action {
	case "new", "edit":
		var tls model.Tls
		err = json.Unmarshal(data, &tls)
		if err != nil {
			return err
		}
		err = tx.Save(&tls).Error
		if err != nil {
			return err
		}
		if action == "edit" {
			var inbounds []model.Inbound
			err = tx.Model(model.Inbound{}).Preload("Tls").Where("tls_id = ?", tls.Id).Find(&inbounds).Error
			if err != nil {
				return err
			}
			if len(inbounds) > 0 {
				err = s.ClientService.UpdateLinksByInboundChange(tx, &inbounds, hostname, "")
				if err != nil {
					return err
				}
				var inboundIds []uint
				for _, inbound := range inbounds {
					inboundIds = append(inboundIds, inbound.Id)
				}
				err = s.InboundService.UpdateOutJsons(tx, inboundIds, hostname)
				if err != nil {
					return common.NewError("unable to update out_json of inbounds: ", err.Error())
				}
				err = s.InboundService.RestartInbounds(tx, inboundIds)
				if err != nil {
					return err
				}
			}
			var serviceIds []uint
			err = tx.Model(model.Service{}).Where("tls_id = ?", tls.Id).Scan(&serviceIds).Error
			if err != nil {
				return err
			}
			if len(serviceIds) > 0 {
				err = s.ServicesService.RestartServices(tx, serviceIds)
				if err != nil {
					return err
				}
			}
		}
	case "del":
		var id uint
		err = json.Unmarshal(data, &id)
		if err != nil {
			return err
		}
		var inboundCount int64
		err = tx.Model(model.Inbound{}).Where("tls_id = ?", id).Count(&inboundCount).Error
		if err != nil {
			return err
		}
		var serviceCount int64
		err = tx.Model(model.Service{}).Where("tls_id = ?", id).Count(&serviceCount).Error
		if err != nil {
			return err
		}
		if inboundCount > 0 || serviceCount > 0 {
			return common.NewError("tls in use")
		}
		err = tx.Where("id = ?", id).Delete(model.Tls{}).Error
		if err != nil {
			return err
		}
	}

	return nil
}
