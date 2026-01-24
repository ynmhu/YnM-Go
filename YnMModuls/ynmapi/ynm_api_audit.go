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

package ynmapi

import (
    "fmt"
    "time"

    _ "github.com/mattn/go-sqlite3"
)

func (p *YnMApiPlugin) logAudit(username, action, ipAddress, details string) {
    db, err := p.getDB()
    if err != nil || db == nil {
        fmt.Printf("[YnMApI] Audit log skipped: database not ready (user: %s, action: %s, err: %v)\n",
            username, action, err)
        return
    }

    _, err = db.Exec(`
        INSERT INTO web_logs (username, action, ip_address, timestamp, details)
        VALUES (?, ?, ?, ?, ?)
    `, username, action, ipAddress, time.Now(), details)

    if err != nil {
        fmt.Printf("[YnMApI] Audit log error: %v\n", err)
    }
}

func (p *YnMApiPlugin) GetAuditLog(limit int) ([]map[string]interface{}, error) {
    db, err := p.getDB()
    if err != nil || db == nil {
        return nil, fmt.Errorf("database not available: %w", err)
    }

    rows, err := db.Query(`
        SELECT username, action, ip_address, timestamp, details
        FROM web_logs
        ORDER BY timestamp DESC
        LIMIT ?
    `, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var logs []map[string]interface{}
    for rows.Next() {
        var username, action, ipAddress, details string
        var timestamp time.Time

        if err := rows.Scan(&username, &action, &ipAddress, &timestamp, &details); err != nil {
            continue
        }

        logs = append(logs, map[string]interface{}{
            "username":   username,
            "action":     action,
            "ip_address": ipAddress,
            "timestamp":  timestamp.Format("2006-01-02 15:04:05"),
            "details":    details,
        })
    }

    return logs, nil
}