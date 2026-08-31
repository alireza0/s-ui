package database

import (
	"encoding/json"
	"log"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

// migratedKeyHTTPClientFix marks that stored remote rule-sets have been
// checked for HTTP client options sing-box cannot parse.
const migratedKeyHTTPClientFix = "migratedHTTPClientFix"

// repairRuleSetHTTPClients removes the two http_client settings that stop a
// config from loading:
//
//   - disable_empty_direct_check, which is an internal flag with no JSON field
//     of its own. sing-box rejects the whole config over the unknown key.
//   - a detour to `direct`, which sing-box refuses as pointless because it
//     dials exactly what it would have dialled anyway.
//
// Both come from an earlier rewrite of the deprecated download_detour option,
// which set the internal flag rather than exposing it.
func repairRuleSetHTTPClients() error {
	var flag model.Setting
	err := db.Where("key = ?", migratedKeyHTTPClientFix).First(&flag).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		changed, err := repairRuleSetHTTPClientsIn(tx)
		if err != nil {
			return err
		}
		if changed > 0 {
			log.Printf("http clients: repaired %d rule-set http_client option(s)", changed)
		}
		return tx.Create(&model.Setting{Key: migratedKeyHTTPClientFix, Value: "true"}).Error
	})
}

func repairRuleSetHTTPClientsIn(tx *gorm.DB) (int, error) {
	var setting model.Setting
	err := tx.Where("key = ?", "config").First(&setting).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var root map[string]json.RawMessage
	if err = json.Unmarshal([]byte(setting.Value), &root); err != nil {
		log.Printf("http clients: skipping, cannot parse config: %v", err)
		return 0, nil
	}
	rawRoute, ok := root["route"]
	if !ok || len(rawRoute) == 0 {
		return 0, nil
	}
	var route map[string]any
	if err = json.Unmarshal(rawRoute, &route); err != nil {
		return 0, nil
	}

	changed := repairRuleSetList(route)
	if changed == 0 {
		return 0, nil
	}

	encodedRoute, err := json.Marshal(route)
	if err != nil {
		return 0, err
	}
	root["route"] = encodedRoute
	encodedRoot, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return 0, err
	}
	setting.Value = string(encodedRoot)
	if err = tx.Save(&setting).Error; err != nil {
		return 0, err
	}
	return changed, nil
}

func repairRuleSetList(route map[string]any) int {
	ruleSets, ok := route["rule_set"].([]any)
	if !ok {
		return 0
	}
	changed := 0
	for _, entry := range ruleSets {
		ruleSet, isObject := entry.(map[string]any)
		if !isObject {
			continue
		}
		httpClient, isObject := ruleSet["http_client"].(map[string]any)
		if !isObject {
			continue
		}
		fixed := false
		if _, ok := httpClient["disable_empty_direct_check"]; ok {
			delete(httpClient, "disable_empty_direct_check")
			fixed = true
		}
		if detour, isString := httpClient["detour"].(string); isString && detour == "direct" {
			delete(httpClient, "detour")
			fixed = true
		}
		if !fixed {
			continue
		}
		// An http_client with nothing left in it means the defaults, which is
		// what leaving it out says more clearly.
		if len(httpClient) == 0 {
			delete(ruleSet, "http_client")
		}
		changed++
	}
	return changed
}
