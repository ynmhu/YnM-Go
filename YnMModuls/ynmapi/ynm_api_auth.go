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

    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
)

func (p *YnMApiPlugin) handleAuth(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Only POST requests", http.StatusMethodNotAllowed)
        return
    }
    
    var req AuthRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    clientIP := r.RemoteAddr
    if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
        clientIP = forwarded
    }
    
    success := p.validatePassword(req.Username, req.Password)
    response := AuthResponse{Success: success}
    
    if success {
        role := p.getUserRole(req.Username)
        userID := p.getUserID(req.Username) // ✅ ÚJ - user ID lekérése
        token := p.generateSessionToken(req.Username)
        
        response.Message = "Login successful"
        response.Token = token
        response.ExpiresIn = 3600
        response.Role = role
        response.Username = req.Username
        response.UserID = userID // ✅ ÚJ
        
        p.updateLastLogin(req.Username)
        p.logAudit(req.Username, "✅", clientIP, fmt.Sprintf("Role: %s", role))
    } else {
        response.Message = "Invalid username or password"
        p.logAudit(req.Username, "❌", clientIP, "Invalid credentials")
    }
    
    w.Header().Set("Content-Type", "application/json")
    if !success {
        w.WriteHeader(http.StatusUnauthorized)
    }
    json.NewEncoder(w).Encode(response)
}

// ✅ ÚJ függvény - user ID lekérése
func (p *YnMApiPlugin) getUserID(username string) int {
    db, err := p.getDB()
    if err != nil || db == nil {
        return 0
    }

    var id int
    err = db.QueryRow(`
        SELECT id FROM users WHERE nick = ? COLLATE NOCASE
    `, username).Scan(&id)
    if err != nil {
        return 0
    }
    return id
}

func (p *YnMApiPlugin) generateSessionToken(username string) string {
    db, err := p.getDB()
    if err != nil || db == nil {
        fmt.Printf("[YnMApI] Cannot generate session token: database not ready (%v)\n", err)
        return ""
    }

    cfg := p.GetConfig()
    token := generateSecretKey()
    expiresAt := time.Now().Add(time.Duration(cfg.YnM.Session.Lifetime) * time.Second)

    _, err = db.Exec(`
        INSERT OR REPLACE INTO web_sessions (token, username, created_at, expires_at)
        VALUES (?, ?, ?, ?)
    `, token, username, time.Now(), expiresAt)
    if err != nil {
        fmt.Printf("[YnMApI] Session storage error: %v\n", err)
        return ""
    }

    return token
}

func (p *YnMApiPlugin) validateSessionToken(token string) string {
    db, err := p.getDB()
    if err != nil || db == nil {
        return ""
    }

    var username string
    err = db.QueryRow(`
        SELECT username FROM web_sessions
        WHERE token = ? AND expires_at > ?
    `, token, time.Now()).Scan(&username)
    if err != nil {
        return ""
    }

    return username
}

func (p *YnMApiPlugin) updateLastLogin(username string) {
    db, err := p.getDB()
    if err != nil || db == nil {
        return
    }

    _, err = db.Exec(`
        UPDATE users SET last_login = ? WHERE nick = ? COLLATE NOCASE
    `, time.Now(), username)
    if err != nil {
        fmt.Printf("[YnMApI] Failed to update last login: %v\n", err)
    }
}

func (p *YnMApiPlugin) getUserRole(username string) string {
    db, err := p.getDB()
    if err != nil || db == nil {
        return "user"
    }

    var role string
    err = db.QueryRow(`
        SELECT role FROM users WHERE nick = ? COLLATE NOCASE
    `, username).Scan(&role)
    if err != nil {
        return "user"
    }
    return role
}

func (p *YnMApiPlugin) getUserProfile(username string) (*UserProfile, error) {
    db, err := p.getDB()
    if err != nil || db == nil {
        return nil, fmt.Errorf("database not available: %w", err)
    }

    var profile UserProfile
    var email, lang sql.NullString
    var lastLogin sql.NullTime

    err = db.QueryRow(`
        SELECT nick, COALESCE(email, ''), role, COALESCE(lang, 'en'), last_login
        FROM users WHERE nick = ? COLLATE NOCASE
    `, username).Scan(&profile.Username, &email, &profile.Role, &lang, &lastLogin)
    if err != nil {
        return nil, fmt.Errorf("user not found")
    }

    if email.Valid {
        profile.Email = email.String
    }
    if lang.Valid {
        profile.Language = lang.String
    } else {
        profile.Language = "en"
    }
    if lastLogin.Valid {
        profile.LastLogin = lastLogin.Time
    }

    return &profile, nil
}

func (p *YnMApiPlugin) updateUserProfile(username string, update UserProfileUpdate) error {
    db, err := p.getDB()
    if err != nil || db == nil {
        return fmt.Errorf("database not available: %w", err)
    }

    setParts := []string{}
    args := []interface{}{}

    if update.Email != nil {
        setParts = append(setParts, "email = ?")
        args = append(args, *update.Email)
    }
    if update.Lang != nil {
        setParts = append(setParts, "lang = ?")
        args = append(args, *update.Lang)
    }
    if update.MyChar != nil {
        setParts = append(setParts, "mychar = ?")
        args = append(args, *update.MyChar)
    }
    if update.Welcome != nil {
        setParts = append(setParts, "welcome = ?")
        args = append(args, *update.Welcome)
    }
    if update.Website != nil {
        setParts = append(setParts, "website = ?")
        args = append(args, *update.Website)
    }
    if update.DiscordID != nil {
        setParts = append(setParts, "discord_id = ?")
        args = append(args, *update.DiscordID)
    }
    if update.TelegramID != nil {
        setParts = append(setParts, "telegram_id = ?")
        args = append(args, *update.TelegramID)
    }
    if update.Facebook != nil {
        setParts = append(setParts, "facebook = ?")
        args = append(args, *update.Facebook)
    }

    if len(setParts) == 0 {
        fmt.Printf("[YnMApI] ⚠️ No fields to update\n")
        return nil
    }

    args = append(args, username)
    query := fmt.Sprintf("UPDATE users SET %s WHERE nick = ?", strings.Join(setParts, ", "))

    fmt.Printf("[YnMApI] 🔧 Executing: %s\n", query)
    fmt.Printf("[YnMApI] 📊 Args: %+v\n", args)

    result, err := db.Exec(query, args...)
    if err != nil {
        fmt.Printf("[YnMApI] ❌ Update error: %v\n", err)
        return err
    }

    rowsAffected, _ := result.RowsAffected()
    fmt.Printf("[YnMApI] ✅ Rows affected: %d\n", rowsAffected)

    return nil
}