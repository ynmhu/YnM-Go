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

func (p *YnMApiPlugin) cleanupLoop() {
    cfg := p.GetConfig()
    ticker := time.NewTicker(time.Duration(cfg.YnM.Session.CleanupInterval) * time.Second)  // ✅ Config
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            p.cleanupExpiredPasswords()
            p.cleanupExpiredSessions()
        case <-p.quit:
            return
        }
    }
}

func (p *YnMApiPlugin) cleanupExpiredPasswords() {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    now := time.Now()
    cleaned := 0
    
    for username, entry := range p.passwords {
        // Lejárt VAGY (nem unlimited ÉS elérte a maxot)
        if now.After(entry.ExpiresAt) || 
           (!entry.isUnlimited() && entry.UsedCount >= entry.MaxUses) {
            delete(p.passwords, username)
            cleaned++
        }
    }
    
    if cleaned > 0 {
        fmt.Printf("[YnMApI] Cleaned %d expired/used passwords\n", cleaned)
    }
}

func (p *YnMApiPlugin) cleanupExpiredSessions() {
    db, err := p.getDB()
    if err != nil || db == nil {
        return
    }

    result, err := db.Exec(`DELETE FROM web_sessions WHERE expires_at < ?`, time.Now())
    if err != nil {
        fmt.Printf("[YnMApI] Session cleanup error: %v\n", err)
        return
    }

    if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
        fmt.Printf("[YnMApI] Cleaned %d expired sessions\n", rowsAffected)
    }
}