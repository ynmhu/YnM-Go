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
	"bytes"
    "io"
)
func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(data)
}
func nullStringToString(ns sql.NullString) string {
    if ns.Valid {
        return ns.String
    }
    return ""
}

func nullTimeToString(nt sql.NullTime) string {
    if nt.Valid {
        return nt.Time.Format("2006-01-02 15:04:05")
    }
    return ""
}


func (p *YnMApiPlugin) handleDashboard(w http.ResponseWriter, r *http.Request) {
    username := r.Header.Get("X-Username")
    
    // ✅ JAVÍTVA: Használjuk az EFFECTIVE ROLE-t, nem csak a globálist!
    userRole := p.GetUserEffectiveRole(username) // ← ÚJ függvény!
    
    profile, err := p.getUserProfile(username)
    if err != nil {
        http.Error(w, "Profile not found", http.StatusNotFound)
        return
    }
    
    // Alap statisztikák mindenkinek
    stats := map[string]interface{}{
        "active_passwords": p.GetActivePasswordsCount(),
        "last_login":       profile.LastLogin.Format("2006-01-02 15:04:05"),
        "role":             userRole, // ✅ Már effective role
        "effective_role":   userRole, // ✅ ÚJ: explicit
        "global_role":      p.getUserRole(username), // ✅ Összehasonlításhoz
        "database_status":  p.GetDatabaseStatus(),
        "server_time":      time.Now().Format("2006-01-02 15:04:05"),
    }
    
   // ✅ JAVÍTOTT: map elemek string index-szel
    channelRoles := p.getUserChannelRoles(username)
    if len(channelRoles) > 0 {
        stats["channel_roles"] = channelRoles
        
        // Ha van channel admin role, mutassuk
        for _, cr := range channelRoles {
            role := cr["role"].(string)      // ✅ JAVÍTVA: cr["role"]
            channel := cr["channel"].(string) // ✅ JAVÍTVA: cr["channel"]
            
            if role == "admin" || role == "owner" {
                stats["has_channel_admin"] = true
                stats["admin_channels"] = channel
                break
            }
        }
    }
    
    // Szerepkör-specifikus statisztikák
    var roleSpecificStats map[string]interface{}
    var features []string
    
    switch strings.ToLower(userRole) { // ✅ Már effective role alapján!
    case "vip":
        features = []string{
            "Profile management",
            "Password generation",
            "Basic statistics",
        }
        roleSpecificStats = map[string]interface{}{
            "vip_features": "Vip access",
        }
        
    case "mod":
        // Mod további statisztikák
        modStats, err := p.getModStats()
        if err == nil {
            stats["mod_stats"] = modStats
        }
        features = []string{
            "Profile management",
            "Password generation",
            "Basic statistics",
            "User management",
            "Channel management",
            "Audit logs access",
        }
        roleSpecificStats = map[string]interface{}{
            "mod_privileges": "Moderator access",
        }
        
    case "admin":
        // Admin statisztikák
        adminStats, err := p.getAdminStats()
        if err == nil {
            stats["admin_stats"] = adminStats
        }
        features = []string{
            "Profile management",
            "Password generation",
            "Advanced statistics",
            "Full user management",
            "Channel management",
            "Audit logs access",
            "System monitoring",
        }
        roleSpecificStats = map[string]interface{}{
            "admin_privileges": "Admin access",
        }
        
    case "owner":
        // owner statisztikák
        ownerStats, err := p.getownerStats()
        if err == nil {
            stats["owner_stats"] = ownerStats
        }
        features = []string{
            "Profile management",
            "Password generation",
            "Complete statistics",
            "Full user management",
            "Channel management",
            "Audit logs access",
            "System monitoring",
            "Database management",
            "Full system control",
        }
        roleSpecificStats = map[string]interface{}{
            "owner_privileges": "Full System",
        }
    }
    
    // Egyesítsd az alap és szerepkör-specifikus statisztikákat
    for k, v := range roleSpecificStats {
        stats[k] = v
    }
    
    dashboardData := map[string]interface{}{
        "user": profile,
        "stats": stats,
        "features": features,
        "user_role": userRole,
        "global_role": p.getUserRole(username), // ✅ Összehasonlításhoz
        "server_time": time.Now().Format("2006-01-02 15:04:05"),
        "welcome_message": p.getWelcomeMessage(userRole, username),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(dashboardData)
}
func (p *YnMApiPlugin) handleProfileAvatar(w http.ResponseWriter, r *http.Request) {
    username := r.Header.Get("X-Username")
    
    switch r.Method {
    case http.MethodPut:
        // Avatar frissítése
        var req struct {
            AvatarURL  string `json:"avatar_url"`
            AvatarType string `json:"avatar_type"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            fmt.Printf("[YnMApi] ❌ Avatar update - JSON decode error: %v\n", err)
            sendJSON(w, http.StatusBadRequest, map[string]interface{}{
                "success": false,
                "error":   "Invalid JSON",
            })
            return
        }
        
        fmt.Printf("[YnMApi] 🖼️ Avatar update for %s - URL: %s, Type: %s\n", 
            username, req.AvatarURL, req.AvatarType)
        
		db, err := p.getDB()
		if err != nil || db == nil {
			sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"success": false,
				"error":   "Database not available",
			})
			return
		}
        
        query := "UPDATE users SET avatar_url = ?, avatar_type = ? WHERE nick = ?"
        result, err := db.Exec(query, req.AvatarURL, req.AvatarType, username)
        
        if err != nil {
            fmt.Printf("[YnMApi] ❌ Avatar update failed: %v\n", err)
            sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
                "success": false,
                "error":   "Failed to update avatar",
            })
            return
        }
        
        rowsAffected, _ := result.RowsAffected()
        fmt.Printf("[YnMApi] ✅ Avatar updated, rows affected: %d\n", rowsAffected)
        
        p.logAudit(username, "AVATAR_UPDATED", r.RemoteAddr, "Avatar updated")
        
        sendJSON(w, http.StatusOK, map[string]interface{}{
            "success": true,
            "message": "Avatar updated successfully",
        })
        
    case http.MethodGet:
        // Avatar lekérése
		db, err := p.getDB()
		if err != nil || db == nil {
			sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"success": false,
				"error":   "Database not available",
			})
			return
		}
        
        var avatarURL, avatarType string
        query := "SELECT avatar_url, avatar_type FROM users WHERE nick = ?"
        err = db.QueryRow(query, username).Scan(&avatarURL, &avatarType)
        
        if err != nil {
            sendJSON(w, http.StatusNotFound, map[string]interface{}{
                "success": false,
                "error":   "User not found",
            })
            return
        }
        
        sendJSON(w, http.StatusOK, map[string]interface{}{
            "success": true,
            "avatar_url": avatarURL,
            "avatar_type": avatarType,
        })
        
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (p *YnMApiPlugin) handleProfile(w http.ResponseWriter, r *http.Request) {
    username := r.Header.Get("X-Username")
    
    fmt.Printf("[YnMApi] 🔍 Profile request: Method=%s, Path=%s, User=%s\n", 
        r.Method, r.URL.Path, username)
    fmt.Printf("[YnMApi] 🔍 Query string: %s\n", r.URL.RawQuery)
    fmt.Printf("[YnMApi] 🔍 Full URL: %s\n", r.URL.String())
    
    switch r.Method {
    case http.MethodGet:
        // ✅ Külön kezeljük, ha van query paraméter (form submit)
        if r.URL.RawQuery != "" {
            fmt.Printf("[YnMApi] ⚠️ GET request with query params (form submit detected)\n")
            
            // Parse query params
            query := r.URL.Query()
            fmt.Printf("[YnMApi] 📊 Query params: %v\n", query)
            
            // Konvertáljuk PUT requesté és kezeljük mint PUT
            r.Method = http.MethodPut
            
            // Hozzunk létre egy JSON body-t a query paraméterekből
            reqData := map[string]string{
                "email":       query.Get("email"),
                "lang":        query.Get("lang"),
                "mychar":      query.Get("mychar"),
                "welcome":     query.Get("welcome"),
                "website":     query.Get("website"),
                "discord_id":  query.Get("discord_id"),
                "telegram_id": query.Get("telegram_id"),
                "facebook":    query.Get("facebook"),
                "avatar_type": query.Get("avatar_type"),
                "avatar_url":  query.Get("avatar_url"),
            }
            
            fmt.Printf("[YnMApi] 📥 Converted to PUT data: %v\n", reqData)
            
            // Konvertáljuk JSON-né
            jsonData, err := json.Marshal(reqData)
            if err != nil {
                fmt.Printf("[YnMApi] ❌ JSON marshal error: %v\n", err)
                sendJSON(w, http.StatusBadRequest, map[string]interface{}{
                    "success": false,
                    "error":   "Invalid data format",
                })
                return
            }
            
            // Cseréljük ki a request body-t
            r.Body = io.NopCloser(bytes.NewBuffer(jsonData))
            r.ContentLength = int64(len(jsonData))
            r.Header.Set("Content-Type", "application/json")
            
            // Most kezeljük mint PUT request
            // (Folytatás a PUT résznél)
        } else {
            // Normál GET profil lekérés
            fmt.Printf("[YnMApi] 📥 Normal GET profile for user: %s\n", username)
            
            user, err := p.getFullUserData(username)
            if err != nil {
                fmt.Printf("[YnMApi] ❌ Error getting user data: %v\n", err)
                sendJSON(w, http.StatusNotFound, map[string]interface{}{
                    "success": false,
                    "error":   "User not found",
                })
                return
            }
            
            fmt.Printf("[YnMApi] ✅ Profile data retrieved for %s\n", username)
            
            sendJSON(w, http.StatusOK, map[string]interface{}{
                "success": true,
                "user":    user,
            })
            return
        }
        
        // Folytatás a PUT logikával...
        fallthrough
        
    case http.MethodPut:
        fmt.Printf("[YnMApi] 📥 PUT profile for user: %s\n", username)
        
        var req UpdateProfileRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            fmt.Printf("[YnMApi] ❌ JSON decode error: %v\n", err)
            sendJSON(w, http.StatusBadRequest, map[string]interface{}{
                "success": false,
                "error":   "Invalid JSON: " + err.Error(),
            })
            return
        }
        
        fmt.Printf("[YnMApi] 📥 Profile update fields:\n")
        fmt.Printf("  Email: '%s'\n", req.Email)
        fmt.Printf("  Lang: '%s'\n", req.Lang)
        fmt.Printf("  MyChar: '%s'\n", req.MyChar)
        fmt.Printf("  Website: '%s'\n", req.Website)
        fmt.Printf("  AvatarType: '%s'\n", req.AvatarType)
        fmt.Printf("  AvatarURL: '%s'\n", req.AvatarURL)
        
        // Ellenőrizzük, hogy van-e valami amit tényleg frissíteni kell
        hasChanges := false
        update := UserProfileUpdate{}
        
        // Csak az üresen nem hagyott mezőket frissítjük
        if req.Email != "" { update.Email = &req.Email; hasChanges = true }
        if req.Lang != "" { update.Lang = &req.Lang; hasChanges = true }
        if req.MyChar != "" { update.MyChar = &req.MyChar; hasChanges = true }
        if req.Welcome != "" { update.Welcome = &req.Welcome; hasChanges = true }
        if req.Website != "" { update.Website = &req.Website; hasChanges = true }
        if req.DiscordID != "" { update.DiscordID = &req.DiscordID; hasChanges = true }
        if req.TelegramID != "" { update.TelegramID = &req.TelegramID; hasChanges = true }
        if req.Facebook != "" { update.Facebook = &req.Facebook; hasChanges = true }
        if req.AvatarType != "" { update.AvatarType = &req.AvatarType; hasChanges = true }
        if req.AvatarURL != "" { update.AvatarURL = &req.AvatarURL; hasChanges = true }
        
        if !hasChanges {
            fmt.Printf("[YnMApi] ⚠️ No actual changes to update for %s\n", username)
            sendJSON(w, http.StatusOK, map[string]interface{}{
                "success": true,
                "message": "No changes detected",
            })
            return
        }
        
        err := p.updateUserProfile(username, update)
        if err != nil {
            fmt.Printf("[YnMApi] ❌ Update failed: %v\n", err)
            sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
                "success": false,
                "error":   "Failed to update profile: " + err.Error(),
            })
            return
        }
        
        fmt.Printf("[YnMApi] ✅ Profile updated successfully for %s\n", username)
        
        p.logAudit(username, "PROFILE_UPDATED", r.RemoteAddr, "Profile updated")
        
        sendJSON(w, http.StatusOK, map[string]interface{}{
            "success": true,
            "message": "Profile updated successfully",
        })
        
    default:
        fmt.Printf("[YnMApi] ❌ Invalid method: %s\n", r.Method)
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}
// Új helper function - JAVÍTOTT verzió
func (p *YnMApiPlugin) getFullUserData(username string) (map[string]interface{}, error) {
	db, err := p.getDB()
	if err != nil || db == nil {
		return nil, fmt.Errorf("database not available: %w", err)
	}
    // ✅ AVATAR MEZŐK HOZZÁADVA!
    query := `
        SELECT 
            id, nick, hostmask, role, email, lang, mychar, welcome,
            website, discord_id, telegram_id, facebook, 
            avatar_url, avatar_type, 
            added_by, created_at, last_login
        FROM users 
        WHERE nick = ?
    `
    
    var (
        id         int
        nick       string
        hostmask   sql.NullString
        role       string
        email      sql.NullString
        lang       sql.NullString
        mychar     sql.NullString
        welcome    sql.NullString
        website    sql.NullString
        discordID  sql.NullString
        telegramID sql.NullString
        facebook   sql.NullString
        avatarURL  sql.NullString  // ✅ ÚJ
        avatarType sql.NullString  // ✅ ÚJ
        addedBy    sql.NullString
        createdAt  sql.NullTime
        lastLogin  sql.NullTime
    )
    
    // ✅ SCAN is frissítve!
    err = db.QueryRow(query, username).Scan(
        &id, &nick, &hostmask, &role, &email,
        &lang, &mychar, &welcome, &website, &discordID,
        &telegramID, &facebook, 
        &avatarURL, &avatarType,  // ✅ ÚJ
        &addedBy, &createdAt, &lastLogin,
    )
    
    if err != nil {
        fmt.Printf("[YnMApi] ❌ Error fetching user data: %v\n", err)
        return nil, err
    }
    
    // ✅ Debug log
    fmt.Printf("[YnMApi] 🖼️ Avatar data for %s - URL: %s, Type: %s\n", 
        username, 
        nullStringToString(avatarURL), 
        nullStringToString(avatarType))
    
    return map[string]interface{}{
        "id":          id,
        "nick":        nick,
        "hostmask":    nullStringToString(hostmask),
        "role":        role,
        "email":       nullStringToString(email),
        "lang":        nullStringToString(lang),
        "mychar":      nullStringToString(mychar),
        "welcome":     nullStringToString(welcome),
        "website":     nullStringToString(website),
        "discord_id":  nullStringToString(discordID),
        "telegram_id": nullStringToString(telegramID),
        "facebook":    nullStringToString(facebook),
        "avatar_url":  nullStringToString(avatarURL),   
        "avatar_type": nullStringToString(avatarType),  
        "added_by":    nullStringToString(addedBy),
        "created_at":  nullTimeToString(createdAt),
        "last_login":  nullTimeToString(lastLogin),
    }, nil
}



func (p *YnMApiPlugin) handlePermissions(w http.ResponseWriter, r *http.Request) {
    username := r.Header.Get("X-Username")
    userRole := p.getUserRole(username)
    
    permissions := map[string]bool{
        "dashboard": true, // Mindig látható
        "profile":   true, // Mindig látható
        "logout":    true, // Mindig látható
    }
    
    // Szerepkör alapján további engedélyek
    switch strings.ToLower(userRole) {
    case "vip":
	    permissions["audit"] = true
        permissions["generate_password"] = true
        
    case "mod":
        permissions["generate_password"] = true
        permissions["users"] = true
        permissions["channels"] = true
        permissions["audit"] = true
        
    case "admin":
        permissions["generate_password"] = true
        permissions["users"] = true
        permissions["channels"] = true
        permissions["audit"] = true
        
    case "owner":
        permissions["generate_password"] = true
        permissions["users"] = true
        permissions["channels"] = true
        permissions["audit"] = true
        permissions["database"] = true
        permissions["system"] = true
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "role":        userRole,
        "permissions": permissions,
    })
}

func (p *YnMApiPlugin) getWelcomeMessage(role, username string) string {
    switch strings.ToLower(role) {
    case "vip":
        return "Welcome " + username + "! You have access to basic features."
    case "mod":
        return "Welcome " + username + "! You have extended moderation privileges."
    case "admin":
        return "Welcome " + username + "! Full system access granted."
    case "owner":
        return "Welcome " + username + "! Complete system control available."
    default:
        return "Welcome to YnM Admin Panel"
    }
}

func (p *YnMApiPlugin) getModStats() (map[string]interface{}, error) {
	db, err := p.getDB()
	if err != nil || db == nil {
		return nil, fmt.Errorf("database not available: %w", err)
	}
		
    var userCount, channelCount int
    err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
    if err != nil {
        userCount = 0
    }
    
    err = db.QueryRow("SELECT COUNT(*) FROM channels").Scan(&channelCount)
    if err != nil {
        channelCount = 0
    }
    
    return map[string]interface{}{
        "total_users":    userCount,
        "total_channels": channelCount,
        "mod_since":      time.Now().AddDate(0, -1, 0).Format("2006-01-02"), // példa
    }, nil
}

func (p *YnMApiPlugin) getAdminStats() (map[string]interface{}, error) {
	db, err := p.getDB()
	if err != nil || db == nil {
		return nil, fmt.Errorf("database not available: %w", err)
	}
    var activeSessions int
    err = db.QueryRow("SELECT COUNT(*) FROM web_sessions WHERE expires_at > ?", time.Now()).Scan(&activeSessions)
    if err != nil {
        activeSessions = 0
    }
    
    return map[string]interface{}{
        "active_sessions": activeSessions,
        "system_uptime":   "24 days", // példa
        "memory_usage":    "45%",     // példa
    }, nil
}

func (p *YnMApiPlugin) getownerStats() (map[string]interface{}, error) {
	db, err := p.getDB()
	if err != nil || db == nil {
		return nil, fmt.Errorf("database not available: %w", err)
	}
    
    var dbSize string
    err = db.QueryRow("SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&dbSize)
    if err != nil {
        dbSize = "N/A"
    }
    
    return map[string]interface{}{
        "database_size":    dbSize,
        "total_audit_logs": 1250, // példa
        "system_health":    "Excellent",
    }, nil
}
