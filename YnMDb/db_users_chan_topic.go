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

	"time"
	"strings"
)
func (a *AdminDB) SetChannelTopic(channel, topic, setBy string) error {
    channelLower := strings.ToLower(channel)
    now := time.Now().Format("2006-01-02 15:04:05")
    
    _, err := a.db.Exec(`
        UPDATE channels 
        SET current_topic = ?, topic_set_by = ?, topic_set_at = ?
        WHERE LOWER(name) = ?
    `, topic, setBy, now, channelLower)
    
    return err
}
// SaveTopic elmenti a channel topicját
func (a *AdminDB) SaveTopic(channel, topic, setBy string) error {
	_, err := a.db.Exec(`
		UPDATE channels SET 
			current_topic = ?,
			topic_set_by = ?,
			topic_set_at = CURRENT_TIMESTAMP
		WHERE name = ?
	`, topic, setBy, channel)
	return err
}