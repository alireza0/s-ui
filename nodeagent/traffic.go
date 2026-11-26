package nodeagent

import (
	"encoding/json"
	"maps"
	"math"
	"time"

	"github.com/alireza0/s-ui/database/model"
)

// A cumulative sample can be read by multiple controllers without draining
// their data. The process ID distinguishes natural resets after a restart.
type TrafficSample struct {
	Process  string                 `json:"process"`
	Up       int64                  `json:"up"`
	Down     int64                  `json:"down"`
	Activity map[string]int64       `json:"activity,omitempty"`
	Users    map[string]UserTraffic `json:"users,omitempty"`
}

type UserTraffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
	At   int64 `json:"at"`
}

func (r *CoreRuntime) collectTraffic() {
	box := r.core.GetInstance()
	if box == nil || box.SessionTracker() == nil {
		return
	}
	r.accumulateTraffic(*box.SessionTracker().GetStats())
}

func (r *CoreRuntime) accumulateTraffic(rows []model.Stats) {
	for _, row := range rows {
		r.trafficDirty = true
		if row.Resource == "user" && row.Traffic > 0 {
			r.accumulateUserTraffic(row)
		}
		if row.Resource != "inbound" {
			continue
		}
		value := &r.traffic.Down
		if row.Direction {
			value = &r.traffic.Up
		}
		*value += min(row.Traffic, math.MaxInt64-*value)
	}
}

func (r *CoreRuntime) accumulateUserTraffic(row model.Stats) {
	if r.traffic.Activity == nil {
		r.traffic.Activity = map[string]int64{}
	}
	r.traffic.Activity[row.Tag] = max(r.traffic.Activity[row.Tag], row.DateTime)
	id := r.trafficAliases[row.Tag]
	if id == "" {
		return
	}
	if r.traffic.Users == nil {
		r.traffic.Users = map[string]UserTraffic{}
	}
	counter := r.traffic.Users[id]
	if row.Direction {
		counter.Up += min(row.Traffic, math.MaxInt64-counter.Up)
	} else {
		counter.Down += min(row.Traffic, math.MaxInt64-counter.Down)
	}
	counter.At = max(counter.At, row.DateTime)
	r.traffic.Users[id] = counter
}

func (r *CoreRuntime) Traffic() *TrafficSample {
	if !r.mu.TryLock() {
		return nil
	}
	defer r.mu.Unlock()
	r.collectTraffic()
	if r.trafficLoadError != nil || r.persistTraffic() != nil {
		return nil
	}
	sample := r.traffic
	sample.Activity = maps.Clone(r.traffic.Activity)
	sample.Users = maps.Clone(r.traffic.Users)
	return &sample
}

// Sample independently of control requests, including while the panel is offline.
func (r *CoreRuntime) SampleTraffic() {
	if !r.mu.TryLock() {
		return
	}
	defer r.mu.Unlock()
	r.collectTraffic()
	_ = r.persistTraffic()
}

func trafficAliases(access AccessSnapshot) map[string]string {
	aliases := map[string]string{}
	for _, grant := range access.Clients {
		if grant.TrafficID == "" {
			continue
		}
		aliases[grant.Name] = grant.TrafficID
		for kind, raw := range grant.Credentials {
			if !UsernameAuth(kind) {
				continue
			}
			var auth struct {
				Username string `json:"username"`
			}
			if json.Unmarshal(raw, &auth) == nil && auth.Username != "" {
				aliases[auth.Username] = grant.TrafficID
			}
		}
	}
	return aliases
}

func (r *CoreRuntime) setTrafficAliases(aliases []map[string]string) {
	if len(aliases) > 0 {
		r.trafficAliases = aliases[0]
	}
}

func (r *CoreRuntime) SetTrafficAccess(access AccessSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectTraffic()
	r.trafficAliases = trafficAliases(access)
}

func (r *CoreRuntime) TrafficAccountingError() string {
	if !r.mu.TryLock() {
		return ""
	}
	defer r.mu.Unlock()
	if r.trafficLoadError != nil || r.trafficWriteError != nil {
		return "traffic_accounting_failed"
	}
	return ""
}

func (r *CoreRuntime) pruneActivity() {
	if len(r.traffic.Activity) == 0 {
		return
	}
	var config struct {
		Inbounds []struct {
			Users []struct {
				Name     string `json:"name"`
				Username string `json:"username"`
			} `json:"users"`
		} `json:"inbounds"`
	}
	if json.Unmarshal(r.config, &config) != nil {
		return
	}
	allowed := map[string]bool{}
	for _, inbound := range config.Inbounds {
		for _, user := range inbound.Users {
			allowed[user.Name], allowed[user.Username] = true, true
		}
	}
	for user, at := range r.traffic.Activity {
		if !allowed[user] || at < time.Now().Add(-7*24*time.Hour).Unix() {
			delete(r.traffic.Activity, user)
		}
	}
}
