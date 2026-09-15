package database

import (
	"encoding/json"
	"log"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
)

// migratedKeySnellClient marks that existing clients have been given their
// Snell credentials.
const migratedKeySnellClient = "migratedSnellClient"

// snellUserKeyLength matches what the panel generates for a new client. Snell
// takes an arbitrary key here; this is the same length as the inbound's psk.
const snellUserKeyLength = 32

// addSnellClientConfig gives every client a Snell key.
//
// Snell arrived as a protocol before the client system knew about it, so
// clients created until now carry credentials for every other protocol and
// nothing for Snell. Without a key of its own a client cannot be assigned to a
// Snell inbound at all, and asking every operator to open each client and
// press the shuffle button is not a migration.
//
// Only the `snell` key is added; nothing else in a client's config is touched,
// and a client that already has one is left exactly as it is.
func addSnellClientConfig() error {
	var flag model.Setting
	err := db.Where("key = ?", migratedKeySnellClient).First(&flag).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable("clients") {
			return tx.Create(&model.Setting{Key: migratedKeySnellClient, Value: "true"}).Error
		}
		changed, err := addSnellClientConfigIn(tx)
		if err != nil {
			return err
		}
		if changed > 0 {
			log.Printf("snell: added Snell credentials to %d client(s)", changed)
		}
		return tx.Create(&model.Setting{Key: migratedKeySnellClient, Value: "true"}).Error
	})
}

func addSnellClientConfigIn(tx *gorm.DB) (int, error) {
	var clients []model.Client
	if err := tx.Find(&clients).Error; err != nil {
		return 0, err
	}

	changed := 0
	for _, client := range clients {
		config := map[string]json.RawMessage{}
		if len(client.Config) > 0 {
			if err := json.Unmarshal(client.Config, &config); err != nil {
				// One unreadable config must not stop the rest. The client
				// keeps working on the protocols it already has.
				log.Printf("snell: skipping client %q, cannot parse config: %v", client.Name, err)
				continue
			}
		}
		if _, exists := config["snell"]; exists {
			continue
		}

		// The name is what the other protocols carry, and the panel keeps it
		// in step with the client name when it is renamed.
		entry, err := json.Marshal(map[string]string{
			"name":    client.Name,
			"userkey": common.Random(snellUserKeyLength),
		})
		if err != nil {
			return 0, err
		}
		config["snell"] = entry

		encoded, err := json.Marshal(config)
		if err != nil {
			return 0, err
		}
		// Written as raw bytes, not a string: the column holds a BLOB
		// everywhere else, and a TEXT value lands there as something GORM
		// cannot scan back into json.RawMessage.
		if err := tx.Model(model.Client{}).Where("id = ?", client.Id).
			Update("config", json.RawMessage(encoded)).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}
