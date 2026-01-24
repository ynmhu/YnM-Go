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
	"time"
)
// AddChannelBan hozzáad egy bant a channelhez
func (a *AdminDB) AddChannelBan(channel, mask, setBy, reason string, expiresAt *time.Time) error {
	_, err := a.db.Exec(`
		INSERT OR REPLACE INTO channel_bans 
		(channel, mask, set_by, reason, expires_at, active)
		VALUES (?, ?, ?, ?, ?, 1)
	`, channel, mask, setBy, reason, expiresAt)
	return err
}

// RemoveChannelBan eltávolít egy bant
func (a *AdminDB) RemoveChannelBan(channel, mask string) error {
	_, err := a.db.Exec(`
		UPDATE channel_bans SET active = 0
		WHERE channel = ? AND mask = ?
	`, channel, mask)
	return err
}

// GetActiveBans visszaadja az aktív banokat
func (a *AdminDB) GetActiveBans(channel string) ([]ChannelBan, error) {
	rows, err := a.db.Query(`
		SELECT id, channel, mask, set_by, reason, created_at, expires_at, active
		FROM channel_bans
		WHERE channel = ? 
			AND active = 1
			AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		ORDER BY created_at DESC
	`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bans []ChannelBan
	for rows.Next() {
		var cb ChannelBan
		var reason sql.NullString
		var expiresAt sql.NullTime
		
		if err := rows.Scan(&cb.ID, &cb.Channel, &cb.Mask, &cb.SetBy, &reason, &cb.CreatedAt, &expiresAt, &cb.Active); err != nil {
			return nil, err
		}
		
		if reason.Valid {
			cb.Reason = &reason.String
		}
		if expiresAt.Valid {
			cb.ExpiresAt = &expiresAt.Time
		}
		
		bans = append(bans, cb)
	}
	return bans, nil
}
