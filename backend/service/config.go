package service

import (
	"encoding/json"
	"s-ui/config"
	"s-ui/core"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"s-ui/util/common"
	"strconv"
	"time"
)

var (
	LastUpdate int64
	IsSystemd  bool
	corePtr    *core.Core
)

type ConfigService struct {
	ClientService
	TlsService
	SettingService
	InboundService
	OutboundService
	EndpointService
}

type SingBoxConfig struct {
	Log          json.RawMessage   `json:"log"`
	Dns          json.RawMessage   `json:"dns"`
	Ntp          json.RawMessage   `json:"ntp"`
	Inbounds     []json.RawMessage `json:"inbounds"`
	Outbounds    []json.RawMessage `json:"outbounds"`
	Endpoints    []json.RawMessage `json:"endpoints"`
	Route        json.RawMessage   `json:"route"`
	Experimental json.RawMessage   `json:"experimental"`
}

func NewConfigService(core *core.Core) *ConfigService {
	corePtr = core
	return &ConfigService{}
}

func (s *ConfigService) InitConfig() error {
	IsSystemd = config.IsSystemd()
	return nil
}

func (s *ConfigService) GetConfig(data string) (*SingBoxConfig, error) {
	var err error
	if len(data) == 0 {
		data, err = s.SettingService.GetConfig()
		if err != nil {
			return nil, err
		}
	}
	singboxConfig := SingBoxConfig{}
	err = json.Unmarshal([]byte(data), &singboxConfig)
	if err != nil {
		return nil, err
	}

	singboxConfig.Inbounds, err = s.InboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Outbounds, err = s.OutboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Endpoints, err = s.EndpointService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	return &singboxConfig, nil
}

func (s *ConfigService) StartCore(defaultConfig string) error {
	if corePtr.IsRunning() {
		return nil
	}
	singboxConfig, err := s.GetConfig(defaultConfig)
	if err != nil {
		return err
	}
	rawConfig, err := json.MarshalIndent(singboxConfig, "", "  ")
	if err != nil {
		return err
	}
	err = corePtr.Start(rawConfig)
	if err != nil {
		logger.Error("start sing-box err:", err.Error())
		return err
	}
	logger.Info("sing-box started")
	return nil
}

func (s *ConfigService) RestartCore() error {
	err := s.StopCore()
	if err != nil {
		return err
	}
	return s.StartCore("")
}

func (s *ConfigService) restartCoreWithConfig(config json.RawMessage) error {
	err := s.StopCore()
	if err != nil {
		return err
	}
	return s.StartCore(string(config))
}

func (s *ConfigService) StopCore() error {
	err := corePtr.Stop()
	if err != nil {
		return err
	}
	logger.Info("sing-box stopped")
	return nil
}

func (s *ConfigService) Save(obj string, act string, data json.RawMessage, loginUser string, hostname string) ([]string, error) {
	var err error
	var inboundIds []uint
	var inboundId uint

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	switch obj {
	case "clients":
		inboundIds, err = s.ClientService.Save(tx, act, data, hostname)
	case "tls":
		inboundIds, err = s.TlsService.Save(tx, act, data)
	case "inbounds":
		inboundId, err = s.InboundService.Save(tx, act, data, hostname)
	case "outbounds":
		err = s.OutboundService.Save(tx, act, data)
	case "endpoints":
		err = s.EndpointService.Save(tx, act, data)
	case "config":
		err = s.SettingService.SaveConfig(tx, data)
		if err != nil {
			return nil, err
		}
		err = s.restartCoreWithConfig(data)
	default:
		return nil, common.NewError("unknown object: ", obj)
	}
	if err != nil {
		return nil, err
	}

	dt := time.Now().Unix()
	err = tx.Create(&model.Changes{
		DateTime: dt,
		Actor:    loginUser,
		Key:      obj,
		Action:   act,
		Obj:      data,
	}).Error
	if err != nil {
		return nil, err
	}
	// Commit changes so far
	tx.Commit()
	LastUpdate = time.Now().Unix()
	var objs []string = []string{obj}
	tx = db.Begin()

	// Update side changes

	// Update client links
	if obj == "tls" && len(inboundIds) > 0 {
		err = s.ClientService.UpdateLinksByInboundChange(tx, inboundIds, hostname)
		if err != nil {
			return nil, err
		}
		objs = append(objs, "clients")
	}
	if obj == "inbounds" && act != "add" {
		switch act {
		case "edit":
			err = s.ClientService.UpdateLinksByInboundChange(tx, []uint{inboundId}, hostname)
		case "del":
			var tag string
			err = json.Unmarshal(data, &tag)
			if err != nil {
				return nil, err
			}
			err = s.ClientService.UpdateClientsOnInboundDelete(tx, inboundId, tag)
		}
		if err != nil {
			return nil, err
		}
		objs = append(objs, "clients")
	}

	// Update out_json of inbounds when tls is changed
	if obj == "tls" && len(inboundIds) > 0 {
		err = s.InboundService.UpdateOutJsons(tx, inboundIds, hostname)
		if err != nil {
			return nil, common.NewError("unable to update out_json of inbounds: ", err.Error())
		}
		objs = append(objs, "inbounds")
	}

	if len(inboundIds) > 0 && corePtr.IsRunning() {
		err1 := s.InboundService.RestartInbounds(tx, inboundIds)
		if err1 != nil {
			logger.Error("unable to restart inbounds: ", err1)
		}
	}
	// Try to start core if it is not running
	if !corePtr.IsRunning() {
		s.StartCore("")
	}

	return objs, nil
}

func (s *ConfigService) CheckChanges(lu string) (bool, error) {
	if lu == "" {
		return true, nil
	}
	if LastUpdate == 0 {
		db := database.GetDB()
		var count int64
		err := db.Model(model.Changes{}).Where("date_time > " + lu).Count(&count).Error
		if err == nil {
			LastUpdate = time.Now().Unix()
		}
		return count > 0, err
	} else {
		intLu, err := strconv.ParseInt(lu, 10, 64)
		return LastUpdate > intLu, err
	}
}

func (s *ConfigService) GetChanges(actor string, chngKey string, count string) []model.Changes {
	c, _ := strconv.Atoi(count)
	whereString := "`id`>0"
	if len(actor) > 0 {
		whereString += " and `actor`='" + actor + "'"
	}
	if len(chngKey) > 0 {
		whereString += " and `key`='" + chngKey + "'"
	}
	db := database.GetDB()
	var chngs []model.Changes
	err := db.Model(model.Changes{}).Where(whereString).Order("`id` desc").Limit(c).Scan(&chngs).Error
	if err != nil {
		logger.Warning(err)
	}
	return chngs
}
