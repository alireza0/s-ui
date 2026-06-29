package service

import (
	"encoding/json"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util"
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
		setCertFingerprint(&tls)
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

func setCertFingerprint(t *model.Tls) {
	var server map[string]interface{}
	if len(t.Server) > 0 {
		_ = json.Unmarshal(t.Server, &server)
	}

	pin := ""
	isReality := false
	if r, ok := server["reality"].(map[string]interface{}); ok {
		isReality, _ = r["enabled"].(bool)
	}
	if !isReality {
		if certPEM := util.CertPEMFromTLS(server); util.CertIsSelfSigned(certPEM) {
			pin = util.CertPublicKeySha256(certPEM)
		}
	}

	var client map[string]interface{}
	if len(t.Client) > 0 {
		if err := json.Unmarshal(t.Client, &client); err != nil {
			return
		}
	}
	if client == nil {
		client = map[string]interface{}{}
	}

	if pin != "" {
		client["certificate_public_key_sha256"] = []string{pin}
		delete(client, "certificate")
		delete(client, "certificate_path")
	} else {
		delete(client, "certificate_public_key_sha256")
	}

	if newClient, err := json.MarshalIndent(client, "", "  "); err == nil {
		t.Client = newClient
	}
}
