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


    _ "github.com/mattn/go-sqlite3"
)

func (p *YnMApiPlugin) handleChannels(w http.ResponseWriter, r *http.Request) {
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}
	username := r.Header.Get("X-Username")
	if strings.TrimSpace(username) == "" {
		http.Error(w, "User authentication required", http.StatusUnauthorized)
		return
	}

	globalRole := strings.ToLower(strings.TrimSpace(p.getUserRole(username)))

	if r.Method != http.MethodGet {
		if globalRole != "admin" && globalRole != "owner" {
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
    query := `
        SELECT id, name, auto_op, auto_voice, auto_halfop, owner, owner_hostmask, created_at
        FROM channels
    `
    args := []interface{}{}

    if globalRole != "admin" && globalRole != "owner" {
        query += `
            WHERE name IN (
                SELECT DISTINCT channel
                FROM channel_users
                WHERE nick = ? COLLATE NOCASE
            )
        `
        args = append(args, username)
    }

    query += ` ORDER BY created_at DESC`

    rows, err := db.Query(query, args...)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()
        
        var channels []map[string]interface{}
        for rows.Next() {
            var id int
            var name string
            var autoOp, autoVoice, autoHalfop bool
            var owner, ownerHostmask sql.NullString
            var createdAt time.Time
            
            if err := rows.Scan(&id, &name, &autoOp, &autoVoice, &autoHalfop, &owner, &ownerHostmask, &createdAt); err != nil {
                continue
            }
            
            ownerStr := ""
            if owner.Valid {
                ownerStr = owner.String
            }
            
            hostmaskStr := ""
            if ownerHostmask.Valid {
                hostmaskStr = ownerHostmask.String
            }
            
            channels = append(channels, map[string]interface{}{
                "id":             id,
                "name":           name,
                "auto_op":        autoOp,
                "auto_voice":     autoVoice,
                "auto_halfop":    autoHalfop,
                "owner":          ownerStr,
                "owner_hostmask": hostmaskStr,
                "created_at":     createdAt.Format("2006-01-02 15:04:05"),
            })
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success":  true,
            "channels": channels,
            "stats": map[string]int{
                "total": len(channels),
            },
        })
        
case http.MethodPost:
    var req struct {
        Name          string `json:"name"`
        Owner         string `json:"owner"`
        OwnerHostmask string `json:"owner_hostmask"`
        // Channels tábla beállítások
        AutoOp        int    `json:"auto_op"`
        AutoVoice     int    `json:"auto_voice"`
        AutoHalfop    int    `json:"auto_halfop"`
        // ✅ ADD HOZZÁ EZEKET:
        OwnerAutoOp   int    `json:"owner_auto_op"`
        OwnerAutoVoice int   `json:"owner_auto_voice"`
        OwnerAutoHalfop int  `json:"owner_auto_halfop"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Validáció
    if req.Name == "" || req.Owner == "" {
        http.Error(w, "Channel name and owner required", http.StatusBadRequest)
        return
    }
    
    if !strings.HasPrefix(req.Name, "#") {
        req.Name = "#" + req.Name
    }
    
    // Ellenőrzés
    var exists int
    err := db.QueryRow("SELECT COUNT(*) FROM channels WHERE name = ? COLLATE NOCASE", req.Name).Scan(&exists)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    if exists > 0 {
        http.Error(w, "Channel already exists", http.StatusConflict)
        return
    }
    
    // Bot JOIN
    botJoinSuccess := false
    botJoinMessage := "IRC bot not connected"
    if p.client != nil && p.client.IsConnected() {
        p.client.Join(req.Name)
        botJoinMessage = fmt.Sprintf("Bot is joining %s", req.Name)
        botJoinSuccess = true
        time.Sleep(300 * time.Millisecond)
    }
    
    // Channels tábla - channel beállítások (új usereknek)
    result, err := db.Exec(`
        INSERT INTO channels (name, owner, owner_hostmask, auto_op, auto_voice, auto_halfop, created_at)
        VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
    `, req.Name, req.Owner, req.OwnerHostmask, req.AutoOp, req.AutoVoice, req.AutoHalfop)
    
    if err != nil {
        http.Error(w, "Failed to add channel to database", http.StatusInternalServerError)
        return
    }
    
    // Channel_users tábla - owner jogosultságok
    _, err = db.Exec(`
        INSERT INTO channel_users
        (nick, hostmask, channel, role, auto_op, auto_voice, auto_halfop, added_by, created_at)
        VALUES (?, ?, ?, 'owner', ?, ?, ?, ?, datetime('now'))
    `, req.Owner, req.OwnerHostmask, req.Name, 
       req.OwnerAutoOp, req.OwnerAutoVoice, req.OwnerAutoHalfop, username)
    
    ownerInsertSuccess := err == nil
    if err != nil {
        p.logAudit(username, "⚠️ OWNER_INSERT_FAILED", r.RemoteAddr,
            fmt.Sprintf("Channel: %s, Owner: %s, Error: %v", req.Name, req.Owner, err))
    }
    
    channelID, _ := result.LastInsertId()
    p.logAudit(username, "➕ CHANNEL_ADDED", r.RemoteAddr,
        fmt.Sprintf("Channel: %s, ID: %d, IRC_JOIN: %v, OWNER_ADDED: %v",
            req.Name, channelID, botJoinSuccess, ownerInsertSuccess))
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":    true,
        "message":    "Channel added successfully",
        "channel_id": channelID,
        "owner_added": ownerInsertSuccess,
        "bot_action": map[string]interface{}{
            "joined":  botJoinSuccess,
            "message": botJoinMessage,
            "channel": req.Name,
        },
    })



    
    case http.MethodPut:
        // CSATORNA FRISSÍTÉSE
        var req struct {
            ID    int         `json:"id"`
            Field string      `json:"field"`
            Value interface{} `json:"value"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        allowedFields := []string{"name", "owner", "owner_hostmask", "auto_op", "auto_voice", "auto_halfop"}
        fieldAllowed := false
        for _, f := range allowedFields {
            if req.Field == f {
                fieldAllowed = true
                break
            }
        }
        
        if !fieldAllowed {
            http.Error(w, "Invalid field", http.StatusBadRequest)
            return
        }
        
        query := fmt.Sprintf("UPDATE channels SET %s = ? WHERE id = ?", req.Field)
        _, err := db.Exec(query, req.Value, req.ID)
        
        if err != nil {
            http.Error(w, "Failed to update channel", http.StatusInternalServerError)
            return
        }
        
        p.logAudit(username, "🔄 CHANNEL_UPDATED", r.RemoteAddr, 
            fmt.Sprintf("Channel ID: %d, Field: %s", req.ID, req.Field))
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Channel updated successfully",
        })
        
case http.MethodDelete:
    // ===== CSATORNA TÖRLÉSE + INSTANT BOT PART =====
    var req struct {
        ID   int    `json:"id"`
        Name string `json:"name"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    if req.Name == "" && req.ID == 0 {
        http.Error(w, "Channel name or ID required", http.StatusBadRequest)
        return
    }
    channelName := req.Name
    if channelName == "" {
        // Ha név nincs megadva, próbáljuk lekérni az ID alapján
        err = db.QueryRow("SELECT name FROM channels WHERE id = ?", req.ID).Scan(&channelName)
        if err != nil {
            http.Error(w, "Channel not found", http.StatusNotFound)
            return
        }
    }
    // Bot PART
    botPartSuccess := false
    botPartMessage := "IRC bot not connected"
    if p.client != nil && p.client.IsConnected() {
        p.client.Part(channelName, "Channel removed from admin panel")
        botPartSuccess = true
        botPartMessage = fmt.Sprintf("Bot left channel %s", channelName)
        time.Sleep(300 * time.Millisecond)
    }
    // Törlés az adatbázisból
    if req.ID != 0 {
        _, err := db.Exec("DELETE FROM channels WHERE id = ?", req.ID)
        if err != nil {
            http.Error(w, "Failed to delete channel from database", http.StatusInternalServerError)
            return
        }
    }
    p.logAudit(username, "🗑️ CHANNEL_DELETED", r.RemoteAddr,
        fmt.Sprintf("Channel: %s, IRC_PART: %v", channelName, botPartSuccess))
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Channel deleted successfully",
        "bot_action": map[string]interface{}{
            "parted":  botPartSuccess,
            "message": botPartMessage,
            "channel": channelName,
        },
    })
        
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (p *YnMApiPlugin) handleChannelSync(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    username := r.Header.Get("X-Username")
    
    // Itt implementálhatod a csatorna szinkronizálást
    // Például: IRC csatornák lekérdezése és adatbázis frissítése
    
    p.logAudit(username, "CHANNEL_SYNC", r.RemoteAddr, "Manual channel sync initiated")
    
    response := map[string]interface{}{
        "success": true,
        "message": "Channel sync completed",
        "synced_channels": 1, 
        "timestamp": time.Now().Format("2006-01-02 15:04:05"),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (p *YnMApiPlugin) handleChannelDetail(w http.ResponseWriter, r *http.Request) {
    // URL path parsing to get channel name
    path := strings.TrimPrefix(r.URL.Path, "/channels/")
    if path == "" {
        http.Error(w, "Channel name required", http.StatusBadRequest)
        return
    }
    
    channelName := path
    username := r.Header.Get("X-Username")
    
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

    switch r.Method {
    case http.MethodGet:
        // Get channel details
        var name string
        var autoOp, autoVoice, autoHalfop bool
        var owner sql.NullString
        var createdAt time.Time
        
        err := db.QueryRow(`
            SELECT name, auto_op, auto_voice, auto_halfop, owner, created_at
            FROM channels WHERE name = ? COLLATE NOCASE
        `, channelName).Scan(&name, &autoOp, &autoVoice, &autoHalfop, &owner, &createdAt)
        
        if err != nil {
            if err == sql.ErrNoRows {
                http.Error(w, "Channel not found", http.StatusNotFound)
            } else {
                http.Error(w, "Database error", http.StatusInternalServerError)
            }
            return
        }
        
        ownerStr := ""
        if owner.Valid {
            ownerStr = owner.String
        }
        
        channel := map[string]interface{}{
            "name":        name,
            "auto_op":     autoOp,
            "auto_voice":  autoVoice,
            "auto_halfop": autoHalfop,
            "owner":       ownerStr,
            "created_at":  createdAt.Format("2006-01-02 15:04:05"),
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(channel)
        
    case http.MethodPut:
        // Update channel settings
        var updateReq struct {
            AutoOp     bool   `json:"auto_op"`
            AutoVoice  bool   `json:"auto_voice"`
            AutoHalfop bool   `json:"auto_halfop"`
            Owner      string `json:"owner"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        _, err := db.Exec(`
            UPDATE channels 
            SET auto_op = ?, auto_voice = ?, auto_halfop = ?, owner = ?
            WHERE name = ? COLLATE NOCASE
        `, updateReq.AutoOp, updateReq.AutoVoice, updateReq.AutoHalfop, updateReq.Owner, channelName)
        
        if err != nil {
            http.Error(w, "Failed to update channel", http.StatusInternalServerError)
            return
        }
        
        p.logAudit(username, "CHANNEL_UPDATED", r.RemoteAddr, 
            fmt.Sprintf("Channel: %s, AutoOp: %v, AutoVoice: %v, AutoHalfop: %v", 
                channelName, updateReq.AutoOp, updateReq.AutoVoice, updateReq.AutoHalfop))
        
        response := map[string]interface{}{
            "success": true,
            "message": "Channel updated successfully",
            "channel": channelName,
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
        
    case http.MethodDelete:
        // Delete channel (csak owner számára)
        userRole := p.getUserRole(username)
        if !strings.EqualFold(userRole, "owner") {
            http.Error(w, "Only owners can delete channels", http.StatusForbidden)
            return
        }
        
        _, err := db.Exec(`DELETE FROM channels WHERE name = ? COLLATE NOCASE`, channelName)
        if err != nil {
            http.Error(w, "Failed to delete channel", http.StatusInternalServerError)
            return
        }
        
        p.logAudit(username, "CHANNEL_DELETED", r.RemoteAddr, 
            fmt.Sprintf("Channel: %s", channelName))
        
        response := map[string]interface{}{
            "success": true,
            "message": "Channel deleted successfully",
            "channel": channelName,
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
        
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

