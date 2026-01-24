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

func (p *YnMApiPlugin) handleChannelUsers(w http.ResponseWriter, r *http.Request) {
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

    username := r.Header.Get("X-Username")

    switch r.Method {
    case http.MethodGet:
        // Channel Users lista
        rows, err := db.Query(`
            SELECT 
                id,
                nick,
                hostmask,
                channel,
                role,
                auto_op,
                auto_voice,
                auto_halfop,
                created_at,
                added_by
            FROM channel_users
            ORDER BY created_at DESC
        `)
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()
        
        var users []map[string]interface{}
        for rows.Next() {
            var id int
            var nick, hostmask, channel, role string
            var autoOp, autoVoice, autoHalfop bool
            var createdAt time.Time
            var addedBy sql.NullString
            
            if err := rows.Scan(&id, &nick, &hostmask, &channel, &role, 
                &autoOp, &autoVoice, &autoHalfop, &createdAt, &addedBy); err != nil {
                continue
            }
            
            addedByStr := ""
            if addedBy.Valid {
                addedByStr = addedBy.String
            }
            
            users = append(users, map[string]interface{}{
                "id":            id,
                "nick":          nick,
                "hostmask":      hostmask,
                "channel":       channel,
                "role":          role,
                "auto_op":       autoOp,
                "auto_voice":    autoVoice,
                "auto_halfop":   autoHalfop,
                "created_at":    createdAt.Format("2006-01-02 15:04:05"),
                "added_by":      addedByStr,
            })
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "channel_users": users,
            "stats": map[string]interface{}{
                "total": len(users),
            },
        })
        
    case http.MethodPost:
        // Channel User hozzáadása
        var req struct {
            Nick       string `json:"nick"`
            Hostmask   string `json:"hostmask"`
            Channel    string `json:"channel"`
            Role       string `json:"role"`
            AutoOp     bool   `json:"auto_op"`
            AutoVoice  bool   `json:"auto_voice"`
            AutoHalfop bool   `json:"auto_halfop"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        if req.Nick == "" || req.Channel == "" {
            http.Error(w, "Nick and Channel are required", http.StatusBadRequest)
            return
        }
        
        // Ellenőrizzük, hogy létezik-e már
        var exists int
        err := db.QueryRow(`
            SELECT COUNT(*) FROM channel_users 
            WHERE nick = ? AND channel = ? COLLATE NOCASE
        `, req.Nick, req.Channel).Scan(&exists)
        
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        
        if exists > 0 {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "error": "User already added to this channel",
            })
            return
        }
        
        // INSERT
        addedBy := username
        if addedBy == "" {
            addedBy = "system"
        }
        
        result, err := db.Exec(`
            INSERT INTO channel_users 
            (nick, hostmask, channel, role, auto_op, auto_voice, auto_halfop, created_at, added_by) 
            VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), ?)
        `, req.Nick, req.Hostmask, req.Channel, req.Role,req.AutoOp, req.AutoVoice, req.AutoHalfop, addedBy)
        
        if err != nil {
            http.Error(w, "Failed to add user to channel", http.StatusInternalServerError)
            return
        }
        
        userID, _ := result.LastInsertId()
        
        p.logAudit(username, "➕ CHANNEL_USER_ADDED", r.RemoteAddr,
            fmt.Sprintf("User: %s to %s (ID: %d)", req.Nick, req.Channel, userID))
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "User added to channel successfully",
            "id":      userID,
            "bot_action": map[string]string{
                "action":  "channel_user_added",
                "message": fmt.Sprintf("Added %s to %s", req.Nick, req.Channel),
            },
        })
        
    case http.MethodDelete:
        // Channel User törlése
        var req struct {
            ID      int    `json:"id"`
            Nick    string `json:"nick"`
            Channel string `json:"channel"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        if req.ID == 0 {
            http.Error(w, "ID is required", http.StatusBadRequest)
            return
        }
        
        result, err := db.Exec("DELETE FROM channel_users WHERE id = ?", req.ID)
        if err != nil {
            http.Error(w, "Failed to delete channel user", http.StatusInternalServerError)
            return
        }
        
        rowsAffected, _ := result.RowsAffected()
        if rowsAffected == 0 {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusNotFound)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "error": "Channel user not found",
            })
            return
        }
        
        p.logAudit(username, "🗑️ CHANNEL_USER_DELETED", r.RemoteAddr,
            fmt.Sprintf("User: %s from %s (ID: %d)", req.Nick, req.Channel, req.ID))
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Channel user removed successfully",
            "bot_action": map[string]interface{}{
                "action":  "channel_user_removed",
                "message": fmt.Sprintf("Removed %s from %s", req.Nick, req.Channel),
                "id":      req.ID,
            },
        })
        
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (p *YnMApiPlugin) handleChannelUsersUpdate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

    var req struct {
        ID      int         `json:"id"`
        Nick    string      `json:"nick"`
        Channel string      `json:"channel"`
        Field   string      `json:"field"`
        Value   interface{} `json:"value"`
		CurrentUser string `json:"current_user"`
        CurrentRole string `json:"current_role"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if req.ID == 0 || req.Field == "" {
        http.Error(w, "ID and Field are required", http.StatusBadRequest)
        return
    }
    // ========== ✅ ÚJ: PERMISSION CHECK (a te CheckChannelUserPermission függvényeddel) ==========
    // Get current user and role from request (PHP will send them)
    currentNick := req.CurrentUser
    currentRole := req.CurrentRole
    
    if currentNick == "" || currentRole == "" {
        // Fallback: try to get from headers
        currentNick = r.Header.Get("X-Username")
        currentRole = r.Header.Get("X-User-Role")
        
        if currentNick == "" || currentRole == "" {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "error":   "User authentication required",
                "message": "Missing current_user or current_role in request",
            })
            return
        }
    }
    
    // Use YOUR CheckChannelUserPermission function
    canEdit, reason := p.CheckChannelUserPermission(currentNick, currentRole, req.ID, "edit")
    if !canEdit {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusForbidden)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error":   "Permission denied",
            "message": reason,
            "details": map[string]interface{}{
                "user":       currentNick,
                "user_role":  currentRole,
                "target_id":  req.ID,
                "action":     "edit",
            },
        })
        return
    }
    // ========== PERMISSION CHECK VÉGE ==========
    // Engedélyezett mezők
    allowedFields := []string{"nick", "hostmask", "channel", "role", "auto_op", "auto_voice", "auto_halfop"}
    fieldAllowed := false
    for _, f := range allowedFields {
        if req.Field == f {
            fieldAllowed = true
            break
        }
    }
    
    if !fieldAllowed {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error": fmt.Sprintf("Invalid field: %s", req.Field),
        })
        return
    }
    
    // Boolean mezők kezelése
    var value interface{}
    var boolValue bool = false
    
    if req.Field == "auto_op" || req.Field == "auto_voice" || req.Field == "auto_halfop" {
        // Próbáljuk bool-ként értelmezni
        switch v := req.Value.(type) {
        case bool:
            boolValue = v
            value = v
        case string:
            boolValue = strings.ToLower(v) == "true" || v == "1"
            value = boolValue
        case float64:
            boolValue = v == 1
            value = boolValue
        default:
            boolValue = false
            value = false
        }
    } else {
        // Szöveges mezők
        value = fmt.Sprintf("%v", req.Value)
    }
    
    // UPDATE végrehajtása
    query := fmt.Sprintf("UPDATE channel_users SET `%s` = ? WHERE id = ?", req.Field)
    result, err := db.Exec(query, value, req.ID)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error": fmt.Sprintf("Database error: %v", err),
        })
        return
    }
    
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusNotFound)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error": "Channel user not found",
        })
        return
    }
    
    // ========== IRC MODE PARANCS KÜLDÉSE ==========
    // ✅ FIX: Lekérjük a nick-et és channel-t az adatbázisból
    var dbNick, dbChannel string
    err = db.QueryRow("SELECT nick, channel FROM channel_users WHERE id = ?", req.ID).
        Scan(&dbNick, &dbChannel)

    if err == nil && p.client != nil && p.client.IsConnected() {
        // Auto mode mezők kezelése
        if req.Field == "auto_voice" {
            if boolValue {
                // Auto voice bekapcsolva -> +v küldése
                p.client.SendRaw(fmt.Sprintf("MODE %s +v %s", dbChannel, dbNick))
                fmt.Printf("[YnMApi] ✅ Auto voice enabled, sent: MODE %s +v %s\n", 
                    dbChannel, dbNick)
            } else {
                // Auto voice kikapcsolva -> -v küldése
                p.client.SendRaw(fmt.Sprintf("MODE %s -v %s", dbChannel, dbNick))
                fmt.Printf("[YnMApi] ❌ Auto voice disabled, sent: MODE %s -v %s\n", 
                    dbChannel, dbNick)
            }
        } else if req.Field == "auto_op" {
            if boolValue {
                // Auto op bekapcsolva -> +o küldése
                p.client.SendRaw(fmt.Sprintf("MODE %s +o %s", dbChannel, dbNick))
                fmt.Printf("[YnMApi] ✅ Auto op enabled, sent: MODE %s +o %s\n", 
                    dbChannel, dbNick)
            } else {
                // Auto op kikapcsolva -> -o küldése
                p.client.SendRaw(fmt.Sprintf("MODE %s -o %s", dbChannel, dbNick))
                fmt.Printf("[YnMApi] ❌ Auto op disabled, sent: MODE %s -o %s\n", 
                    dbChannel, dbNick)
            }
        } else if req.Field == "auto_halfop" {
            if boolValue {
                // Auto halfop bekapcsolva -> +h küldése
                p.client.SendRaw(fmt.Sprintf("MODE %s +h %s", dbChannel, dbNick))
                fmt.Printf("[YnMApi] ✅ Auto halfop enabled, sent: MODE %s +h %s\n", 
                    dbChannel, dbNick)
            } else {
                // Auto halfop kikapcsolva -> -h küldése
                p.client.SendRaw(fmt.Sprintf("MODE %s -h %s", dbChannel, dbNick))
                fmt.Printf("[YnMApi] ❌ Auto halfop disabled, sent: MODE %s -h %s\n", 
                    dbChannel, dbNick)
            }
        }
    } else if err != nil {
        fmt.Printf("[YnMApi] ⚠️ Could not fetch nick/channel for ID %d: %v\n", req.ID, err)
    } else if p.client == nil || !p.client.IsConnected() {
        fmt.Printf("[YnMApi] ⚠️ IRC client not connected, skipping MODE command\n")
    }
    // ========== IRC MODE PARANCS VÉGE ==========
    
    username := r.Header.Get("X-Username")
    if username == "" {
        username = "system"
    }
    
    p.logAudit(username, "🔄 CHANNEL_USER_UPDATED", r.RemoteAddr,
        fmt.Sprintf("ID: %d, Field: %s = %v", req.ID, req.Field, value))
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Channel user updated successfully",
        "data": map[string]interface{}{
            "id":            req.ID,
            "field":         req.Field,
            "value":         value,
            "affected_rows": rowsAffected,
        },
        "bot_action": map[string]interface{}{
            "action":  "channel_user_updated",
            "message": fmt.Sprintf("Updated %s for %s in %s", req.Field, dbNick, dbChannel),
            "irc_mode_sent": p.client != nil,
        },
    })
}

func getBotActionMessage(success bool, action string) string {
    if success {
        switch action {
        case "join":
            return "Bot successfully joined the channel"
        case "part":
            return "Bot successfully left the channel"
        default:
            return "Bot action completed"
        }
    }
    return "Bot action failed - IRC not connected"
}

// handleChannelUsersAdd - Channel user hozzáadása
func (p *YnMApiPlugin) handleChannelUsersAdd(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    db, err := p.getDB()
    if err != nil || db == nil {
        http.Error(w, "Database not ready", http.StatusServiceUnavailable)
        return
    }

    var req ChannelUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.Nick == "" || req.Channel == "" {
        http.Error(w, "Missing required fields: nick and channel", http.StatusBadRequest)
        return
    }

    // létezik-e már?
    var existingID int
    err = db.QueryRow(
        "SELECT id FROM channel_users WHERE nick = ? AND channel = ?",
        req.Nick, req.Channel,
    ).Scan(&existingID)
    if err == nil {
        http.Error(w, "User already exists in this channel", http.StatusConflict)
        return
    }

    if req.Role == "" {
        req.Role = "vip"
    }

    autoOp := false
    if req.AutoOp != nil {
        autoOp = *req.AutoOp
    }
    autoVoice := false
    if req.AutoVoice != nil {
        autoVoice = *req.AutoVoice
    }
    autoHalfop := false
    if req.AutoHalfop != nil {
        autoHalfop = *req.AutoHalfop
    }

    addedBy := req.AddedBy
	if strings.TrimSpace(addedBy) == "" {
		addedBy = strings.TrimSpace(r.Header.Get("X-Username"))
	}
	if strings.TrimSpace(addedBy) == "" {
		http.Error(w, "User authentication required", http.StatusUnauthorized)
		return
	}
	// ===== PERMISSION: ki milyen local role-t adhat =====
    requestedRole := strings.ToLower(strings.TrimSpace(req.Role))
	req.Role = requestedRole
    // globális role (rendszer jog)
    globalRole := strings.ToLower(strings.TrimSpace(p.getUserRole(addedBy)))

    // local role (az adott csatornában milyen)
    channelName := strings.TrimSpace(req.Channel)
    currentChannelRole := strings.ToLower(strings.TrimSpace(p.GetUserChannelRole(addedBy, channelName)))
	if requestedRole == "admin" || requestedRole == "owner" {
    if globalRole != "owner" && globalRole != "admin" {
        http.Error(w, "Only global admin/owner can assign admin/owner role", http.StatusForbidden)
        return
		}
	}
    // Globál owner/admin: mindent adhat
    if globalRole != "owner" && globalRole != "admin" {
        switch currentChannelRole {
        case "owner", "admin":
            // local admin -> adhat vip-et és mod-ot
            if requestedRole != "vip" && requestedRole != "mod" {
                http.Error(w, "Local admin can only add vip or mod", http.StatusForbidden)
                return
            }

        case "mod":
            // local mod -> csak vip-et adhat
            if requestedRole != "vip" {
                http.Error(w, "Local mod can only add vip", http.StatusForbidden)
                return
            }

        default:
            // vip/user/unknown -> nem adhat hozzá senkit
            http.Error(w, "Insufficient permissions to add channel user", http.StatusForbidden)
            return
        }
    }
    // ===== PERMISSION END =====
    stmt, err := db.Prepare(`
        INSERT INTO channel_users
        (nick, hostmask, channel, role, auto_op, auto_voice, auto_halfop, created_at, added_by)
        VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), ?)
    `)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    defer stmt.Close()

    result, err := stmt.Exec(
        req.Nick,
        req.Hostmask,
        req.Channel,
        req.Role,
        autoOp,
        autoVoice,
        autoHalfop,
        addedBy,
    )
    if err != nil {
        http.Error(w, "Failed to add user to channel", http.StatusInternalServerError)
        return
    }

    id, _ := result.LastInsertId()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "User added to channel successfully",
        "id":      id,
        "data": map[string]interface{}{
            "nick":        req.Nick,
            "channel":     req.Channel,
            "role":        req.Role,
            "auto_op":     autoOp,
            "auto_voice":  autoVoice,
            "auto_halfop": autoHalfop,
            "added_by":    addedBy,
        },
    })
}

// handleChannelUsersDelete - Channel user törlése
func (p *YnMApiPlugin) handleChannelUsersDelete(w http.ResponseWriter, r *http.Request) {
    if !p.isDatabaseReady() {
        http.Error(w, "Database not ready", http.StatusServiceUnavailable)
        return
    }

    if r.Method != http.MethodPost && r.Method != http.MethodDelete {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req ChannelUserDeleteRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.ID == 0 {
        http.Error(w, "Missing ID", http.StatusBadRequest)
        return
    }

    // ========== ✅ JAVÍTVA: User lekérés a JÓ mezőkből ==========
    // ========== ✅ JAVÍTVA: User lekérés ==========
    // Próbáljuk a különböző mezőneveket
    currentNick := ""
    if req.CurrentUser != "" {
        currentNick = req.CurrentUser
    } else if req.User != "" {
        currentNick = req.User
    } else if req.DeletedBy != "" {
        currentNick = req.DeletedBy
    } else {
        currentNick = r.Header.Get("X-Username")
    }
    
    currentRole := ""
    if req.CurrentUserRole != "" {
        currentRole = req.CurrentUserRole
    } else if req.Role != "" {
        currentRole = req.Role
    } else if req.CurrentRole != "" {
        currentRole = req.CurrentRole
    } else {
        currentRole = r.Header.Get("X-User-Role")
    }
    
    if currentNick == "" {
        currentNick = "unknown"
    }
    if currentRole == "" {
        currentRole = "user"
    }
    // ========== USER LEKÉRÉS VÉGE ==========

    // Permission check (a te CheckChannelUserPermission függvényeddel)
    canDelete, reason := p.CheckChannelUserPermission(currentNick, currentRole, req.ID, "delete")
    if !canDelete {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusForbidden)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error":   "Permission denied",
            "message": reason,
        })
        return
    }

    // ========== PERMISSION CHECK VÉGE ==========

    // Ellenőrizzük, hogy létezik-e
    var nick, channel, addedBy string
    err := p.db.QueryRow("SELECT nick, channel, added_by FROM channel_users WHERE id = ?", req.ID).
        Scan(&nick, &channel, &addedBy)
    if err != nil {
        if err == sql.ErrNoRows {
            http.Error(w, "Channel user not found", http.StatusNotFound)
        } else {
            http.Error(w, "Database error", http.StatusInternalServerError)
        }
        return
    }

    // ✅ TÉNYLEGES DELETE az adatbázisból
    stmt, err := p.db.Prepare("DELETE FROM channel_users WHERE id = ?")
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    defer stmt.Close()

    result, err := stmt.Exec(req.ID)
    if err != nil {
        http.Error(w, "Failed to delete user from channel", http.StatusInternalServerError)
        return
    }

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        http.Error(w, "No user deleted", http.StatusNotFound)
        return
    }

    // Audit log
    p.logAudit(currentNick, "🗑️", "", 
        fmt.Sprintf("Deleted %s from %s (ID: %d, Added by: %s)", 
            nick, channel, req.ID, addedBy))

    // Sikeres válasz
    response := map[string]interface{}{
        "success": true,
        "message": "User removed from channel successfully",
        "deleted": map[string]interface{}{
            "id":        req.ID,
            "nick":      nick,
            "channel":   channel,
            "added_by":  addedBy,
            "deleted_by": currentNick,
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}