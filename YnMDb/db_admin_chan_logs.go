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
	"strings"
	"time"
	_ "github.com/mattn/go-sqlite3"
)
// ==================================================
// Bot Log Methods
// ==================================================


// AddBotLog új bot esemény napló bejegyzést hoz létre
func (a *AdminDB) AddBotLog(username, action, hostmask, details, channel, command string) error {
    _, err := a.db.Exec(`
        INSERT INTO bot_logs (username, action, hostmask, details, channel, command, timestamp)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `, username, action, hostmask, details, 
       toNullString(channel), 
       toNullString(command), 
       time.Now())
    return err
}

// GetBotLogs lekéri a bot naplókat (opcionális szűrés)
func (a *AdminDB) GetBotLogs(limit int, username, action string) ([]BotLog, error) {
    query := `
        SELECT id, username, action, hostmask, details, channel, command, timestamp
        FROM bot_logs
    `
    
    args := []interface{}{}
    conditions := []string{}
    
    if username != "" {
        conditions = append(conditions, "username = ?")
        args = append(args, username)
    }
    if action != "" {
        conditions = append(conditions, "action = ?")
        args = append(args, action)
    }
    
    if len(conditions) > 0 {
        query += " WHERE " + strings.Join(conditions, " AND ")
    }
    
    query += ` ORDER BY timestamp DESC LIMIT ?`
    args = append(args, limit)

    rows, err := a.db.Query(query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var logs []BotLog
    for rows.Next() {
        var log BotLog
        var channel, command sql.NullString
        
        if err := rows.Scan(&log.ID, &log.Username, &log.Action, &log.Hostmask, 
                           &log.Details, &channel, &command, &log.Timestamp); err != nil {
            return nil, err
        }
        
        if channel.Valid {
            log.Channel = &channel.String
        }
        if command.Valid {
            log.Command = &command.String
        }
        
        logs = append(logs, log)
    }
    return logs, nil
}

func toNullString(s string) interface{} {
    if s == "" {
        return nil
    }
    return s
}
