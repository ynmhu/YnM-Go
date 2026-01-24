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
	"strings"
)

func (a *AdminDB) UpdateChannelSettings(name string, autoOp, autoVoice, autoHalfOp bool) error {
    _, err := a.db.Exec(`
        UPDATE channels 
        SET auto_op = ?, auto_voice = ?, auto_halfop = ?
        WHERE name = ?
    `, autoOp, autoVoice, autoHalfOp, strings.ToLower(name))
    return err
}


func (a *AdminDB) ChannelUserExists(nick, hostmask, channel string) (bool, error) {
    var count int
    err := a.db.QueryRow(`
        SELECT COUNT(*) FROM channel_users
        WHERE LOWER(nick) = LOWER(?) AND hostmask = ? AND LOWER(channel) = LOWER(?)`,
        nick, hostmask, channel).Scan(&count)
    if err != nil {
        return false, err
    }
    return count > 0, nil
}
func (a *AdminDB) AddUserToChannel(nick, hostmask, channel, role string, autoOp, autoVoice, autoHalfop int, addedBy, addedByHost string) error {
    _, err := a.db.Exec(`
        INSERT INTO channel_users 
        (nick, hostmask, channel, role, auto_op, auto_voice, auto_halfop, added_by, added_by_host, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(nick, channel) DO UPDATE SET
            hostmask = excluded.hostmask,
            role = excluded.role,
            auto_op = excluded.auto_op,
            auto_voice = excluded.auto_voice,
            auto_halfop = excluded.auto_halfop,
            added_by = excluded.added_by,
            added_by_host = excluded.added_by_host,
            created_at = CURRENT_TIMESTAMP
    `, nick, hostmask, channel, role, autoOp, autoVoice, autoHalfop, addedBy, addedByHost)
    return err
}