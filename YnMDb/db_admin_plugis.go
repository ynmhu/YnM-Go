// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ==================================================
package YnMDb

import (
	"database/sql"
	"fmt"
)

func insertBasePlugins(db *sql.DB) error {
	basePlugins := []struct {
		name        string
		description string
	}{
		{"ora", "Óra/emlékeztető plugin"},
		{"media", "Media kezelő plugin"},
		{"admin", "Adminisztrációs plugin"},
		{"nameday", "Névnap plugin"},
	}

	for _, plugin := range basePlugins {
		_, err := db.Exec(`INSERT OR IGNORE INTO plugins (name, description) VALUES (?, ?)`,
			plugin.name, plugin.description)
		if err != nil {
			return fmt.Errorf("error inserting base plugin %s: %v", plugin.name, err)
		}
	}

	return nil
}