package service

import (
	"s-ui/singbox"
)

type SingBoxService struct {
	singbox.V2rayAPI
	singbox.Controller
	StatsService
}

func (s *SingBoxService) GetStats() error {
	s.V2rayAPI.Init(ApiAddr)
	defer s.V2rayAPI.Close()
	stats, err := s.V2rayAPI.GetStats(true)
	if err != nil {
		return err
	}
	err = s.StatsService.SaveStats(stats)
	if err != nil {
		return err
	}

	return nil
}

func (s *SingBoxService) GetSysStats() (*map[string]interface{}, error) {
	err := s.V2rayAPI.Init(ApiAddr)
	if err != nil {
		return nil, err
	}
	defer s.V2rayAPI.Close()
	resp, err := s.V2rayAPI.GetSysStats()
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	result["NumGoroutine"] = resp.NumGoroutine
	result["Alloc"] = resp.Alloc
	result["Uptime"] = resp.Uptime

	return &result, nil
}
