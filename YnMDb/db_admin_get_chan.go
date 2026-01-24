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
)
func (a *AdminDB) GetChannel(channel string, currentUser ...string) (*ChannelInfo, error) {
    var ci ChannelInfo
    var owner, ownerHostmask, currentTopic, topicSetBy, currentModes, modesSetBy sql.NullString
    var topicSetAt, modesSetAt sql.NullTime
    
    // Alap lekérdezés
    err := a.db.QueryRow(`
        SELECT 
            id, name, auto_op, auto_voice, auto_halfop, 
            owner, owner_hostmask,
            current_topic, topic_set_by, topic_set_at,
            current_modes, modes_set_by, modes_set_at,
            created_at
        FROM channels 
        WHERE name = ?
    `, channel).Scan(
        &ci.ID, &ci.Name, &ci.AutoOp, &ci.AutoVoice, &ci.AutoHalfOp,
        &owner, &ownerHostmask,
        &currentTopic, &topicSetBy, &topicSetAt,
        &currentModes, &modesSetBy, &modesSetAt,
        &ci.CreatedAt,
    )
    
    if err != nil {
        return nil, err
    }
    
    // NULL kezelések
    if owner.Valid {
        ci.Owner = &owner.String
    }
    if ownerHostmask.Valid {
        ci.OwnerHostmask = &ownerHostmask.String
    }
	if currentTopic.Valid {
		ci.CurrentTopic = &currentTopic.String
	}
	if topicSetBy.Valid {
		ci.TopicSetBy = &topicSetBy.String
	}
	if topicSetAt.Valid {
		ci.TopicSetAt = &topicSetAt.Time
	}
	if currentModes.Valid {
		ci.CurrentModes = &currentModes.String
	}
	if modesSetBy.Valid {
		ci.ModesSetBy = &modesSetBy.String
	}
	if modesSetAt.Valid {
		ci.ModesSetAt = &modesSetAt.Time
	}
	
    if len(currentUser) > 0 && currentUser[0] != "" {
        user := currentUser[0]
        
        // Alapértelmezett jog
        ci.MyPermission = "none"
        
        // Tulajdonos ellenőrzés
        if ci.Owner != nil && *ci.Owner == user {
            ci.MyPermission = "owner"
        } else {
            // Admin ellenőrzés (ha van ilyen tábla)
            var isAdmin bool
            a.db.QueryRow(`
                SELECT EXISTS(
                    SELECT 1 FROM channel_admins 
                    WHERE channel_id = ? AND nickname = ?
                )`, ci.ID, user).Scan(&isAdmin)
            
            if isAdmin {
                ci.MyPermission = "admin"
            } else {
                // Op ellenőrzés (ha van ilyen tábla)
                var isOp bool
                a.db.QueryRow(`
                    SELECT EXISTS(
                        SELECT 1 FROM channel_ops 
                        WHERE channel_id = ? AND nickname = ?
                    )`, ci.ID, user).Scan(&isOp)
                
                if isOp {
                    ci.MyPermission = "op"
                }
            }
        }
    }
    
    return &ci, nil
}