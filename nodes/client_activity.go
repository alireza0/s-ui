package nodes

import (
	"encoding/json"
	"time"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/nodeagent"
	"gorm.io/gorm"
)

// Activity is independent from traffic charts and never increments client quota.
func recordClientActivity(db *gorm.DB, node model.Node, sample *nodeagent.TrafficSample, observed time.Time) error {
	if sample == nil || len(sample.Activity) == 0 || len(sample.Users) > 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		found := tx.Where("id = ? AND address = ? AND control_epoch = ? AND management_generation = ?", node.Id, node.Address, node.ControlEpoch, node.ManagementGeneration).Find(&current)
		if found.Error != nil || found.RowsAffected == 0 {
			return found.Error
		}
		clients, err := accountingClients(tx, current, false)
		if err != nil {
			return err
		}
		aliases, byID := activityClientAliases(tx, current, clients)
		times := activityTimes(sample.Activity, aliases, byID, observed)
		for id, stamp := range times {
			if stamp <= byID[id].OnlineAt {
				continue
			}
			if err := tx.Model(&model.Client{}).Where("id = ? AND (online_at IS NULL OR online_at < ?)", id, stamp).Update("online_at", stamp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func activityClientAliases(tx *gorm.DB, current model.Node, clients []model.Client) (map[string]uint, map[uint]model.Client) {
	aliases := map[string]uint{}
	byID := map[uint]model.Client{}
	for _, client := range clients {
		byID[client.Id] = client
		addActivityAlias(aliases, effectiveUserName(tx, current, client), client.Id)
		for _, name := range activityUsernames(client.Config) {
			addActivityAlias(aliases, name, client.Id)
		}
	}
	return aliases, byID
}

func addActivityAlias(aliases map[string]uint, name string, id uint) {
	if name == "" {
		return
	}
	if old, ok := aliases[name]; ok && old != id {
		aliases[name] = 0
	} else {
		aliases[name] = id
	}
}

func activityTimes(activity map[string]int64, aliases map[string]uint, byID map[uint]model.Client, observed time.Time) map[uint]int64 {
	times := map[uint]int64{}
	for name, stamp := range activity {
		id := aliases[name]
		client, ok := byID[id]
		if !ok || stamp <= 0 || stamp < client.CreatedAt || stamp > observed.Add(5*time.Minute).Unix() {
			continue
		}
		times[id] = max(times[id], min(stamp, observed.Unix()))
	}
	return times
}

func activityUsernames(config json.RawMessage) []string {
	names := []string{}
	var credentials map[string]json.RawMessage
	if json.Unmarshal(config, &credentials) != nil {
		return nil
	}
	for kind, raw := range credentials {
		if !nodeagent.UsernameAuth(kind) {
			continue
		}
		var auth struct {
			Username string `json:"username"`
		}
		if json.Unmarshal(raw, &auth) == nil {
			names = append(names, auth.Username)
		}
	}
	return names
}
