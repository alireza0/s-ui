package migration

import (
	"encoding/json"
	"fmt"

	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
)

// snellUserKeyLength matches what the panel generates for a new client. Snell
// takes an arbitrary key here; this is the same length as the inbound's psk.
const snellUserKeyLength = 32

// to1_6_3 gives every client a Snell key and clears the per-migration flags
// that 1.6.0 through 1.6.2 left in the settings table.
func to1_6_3(tx *gorm.DB) error {
	if err := addSnellClientConfig(tx); err != nil {
		return err
	}
	return dropMigrationFlags(tx)
}

// Between 1.6.0 and 1.6.2 each migration ran from InitDB on every start and
// recorded a `migratedX` flag of its own in the settings table, rather than
// running once from here against the version the same table already holds.
// Those migrations are now steps in this chain, so the flags decide nothing
// and are removed; a database that carries them has already had the work done,
// and every step above skips what it finds already converted.
func dropMigrationFlags(tx *gorm.DB) error {
	res := tx.Exec("DELETE FROM settings WHERE key LIKE ?", "migrated%")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		fmt.Printf("settings: removed %d per-migration flag(s), the version row tracks migrations again\n", res.RowsAffected)
	}
	return nil
}

// clientRow reads only the columns the migration touches, since the clients
// table has grown columns this database may not have yet.
type clientRow struct {
	Id     uint
	Name   string
	Config json.RawMessage
}

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
func addSnellClientConfig(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("clients") {
		return nil
	}
	changed, err := addSnellClientConfigIn(tx)
	if err != nil {
		return err
	}
	if changed > 0 {
		fmt.Printf("snell: added Snell credentials to %d client(s)\n", changed)
	}
	return nil
}

func addSnellClientConfigIn(tx *gorm.DB) (int, error) {
	var clients []clientRow
	if err := tx.Raw("SELECT id, name, config FROM clients").Scan(&clients).Error; err != nil {
		return 0, err
	}

	changed := 0
	for _, client := range clients {
		config := map[string]json.RawMessage{}
		if len(client.Config) > 0 {
			if err := json.Unmarshal(client.Config, &config); err != nil {
				// One unreadable config must not stop the rest. The client
				// keeps working on the protocols it already has.
				fmt.Printf("snell: skipping client %q, cannot parse config: %v\n", client.Name, err)
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
		if err := tx.Exec("UPDATE clients SET config = ? WHERE id = ?", encoded, client.Id).Error; err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}
