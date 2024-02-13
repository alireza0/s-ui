package singbox

import (
	"context"

	"regexp"
	"s-ui/database/model"
	"s-ui/util/common"
	"time"

	statsService "github.com/v2fly/v2ray-core/v5/app/stats/command"
	"google.golang.org/grpc"
)

type V2rayAPI struct {
	StatsServiceClient *statsService.StatsServiceClient
	grpcClient         *grpc.ClientConn
	isConnected        bool
}

func (v *V2rayAPI) Init(ApiAddr string) (err error) {
	if len(ApiAddr) == 0 {
		return common.NewError("The api address is wrong: ", ApiAddr)
	}
	v.grpcClient, err = grpc.Dial(ApiAddr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	v.isConnected = true

	ssClient := statsService.NewStatsServiceClient(v.grpcClient)

	v.StatsServiceClient = &ssClient

	return
}

func (v *V2rayAPI) Close() {
	v.grpcClient.Close()
	v.StatsServiceClient = nil
	v.isConnected = false
}

func (v *V2rayAPI) GetStats(reset bool) ([]*model.Stats, error) {
	if v.grpcClient == nil {
		return nil, common.NewError("v2ray api is not initialized")
	}
	var trafficRegex = regexp.MustCompile("(inbound|outbound|user)>>>([^>]+)>>>traffic>>>(downlink|uplink)")

	client := *v.StatsServiceClient
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	request := &statsService.QueryStatsRequest{
		Reset_: reset,
	}
	resp, err := client.QueryStats(ctx, request)
	if err != nil {
		return nil, err
	}

	dt := time.Now().Unix()
	stats := make([]*model.Stats, 0)
	for _, stat := range resp.GetStat() {
		if stat.Value > 0 {
			matchs := trafficRegex.FindStringSubmatch(stat.Name)
			if len(matchs) > 3 {
				stat := model.Stats{
					DateTime:  dt,
					Resource:  matchs[1],
					Tag:       matchs[2],
					Direction: matchs[3] == "uplink",
					Traffic:   stat.Value,
				}
				stats = append(stats, &stat)
			}
		}
	}

	return stats, nil
}

func (v *V2rayAPI) GetSysStats() (*statsService.SysStatsResponse, error) {
	if v.grpcClient == nil {
		return nil, common.NewError("v2ray api is not initialized")
	}
	client := *v.StatsServiceClient
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	request := &statsService.SysStatsRequest{}
	resp, err := client.GetSysStats(ctx, request)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
