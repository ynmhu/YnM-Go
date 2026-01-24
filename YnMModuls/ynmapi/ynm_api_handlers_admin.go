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
    "encoding/json"
    "fmt"
    "net/http"
	"database/sql"
    "time"
	"strings"
    "crypto/md5"
    "encoding/hex"
)

func (p *YnMApiPlugin) handleStats(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    stats := map[string]interface{}{
        "total_requests": 123,       // lekérhető adatbázisból
        "unique_users":   45,        // lekérhető adatbázisból
        "success_rate":   98,        // számított érték
        "recent_users": []map[string]string{
            {"username": "Markus", "last_seen": "2025-09-07 14:00"},
        },
    }
    
    json.NewEncoder(w).Encode(stats)
}
func (p *YnMApiPlugin) handleAudit(w http.ResponseWriter, r *http.Request) {
    limitStr := r.URL.Query().Get("limit")
    limit := 50
    
    if limitStr != "" {
        fmt.Sscanf(limitStr, "%d", &limit)
    }
    
    logs, err := p.GetAuditLog(limit)
    if err != nil {
        http.Error(w, "Audit log error", http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "logs":  logs,
        "count": len(logs),
    })
}

// ✅ JAVÍTÁS: handleUsers - SQL query-ben ID mező hozzáadása

func (p *YnMApiPlugin) handleUsers(w http.ResponseWriter, r *http.Request) {
    // Ellenőrizd a HTTP metódust
    switch r.Method {
    case http.MethodGet:
        p.handleUsersList(w, r)  // GET: List users
    case http.MethodPost:
        p.handleUsersAdd(w, r)   // POST: Add user
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// =====================
// HANDLE USERS LIST (GET)
// =====================
func (p *YnMApiPlugin) handleUsersList(w http.ResponseWriter, r *http.Request) {
    type User struct {
        ID        int    `json:"id"`
        Username  string `json:"username"`
        Nick      string `json:"nick"`
        Email     string `json:"email"`
        Role      string `json:"role"`
        Hostmask  string `json:"hostmask"`
        Lang      string `json:"lang"`
        MyChar    string `json:"mychar"`
        Welcome   string `json:"welcome"`
        Website   string `json:"website"`
        DiscordID string `json:"discord_id"`
        TelegramID string `json:"telegram_id"`
        Facebook  string `json:"facebook"`
        AddedBy   string `json:"added_by"`
        CreatedAt string `json:"created_at"`
        LastSeen  string `json:"last_seen"`
        LastLogin string `json:"last_login"`
        Invites   int    `json:"invites"`
    }
    
    username := r.Header.Get("X-Username")
    if username == "" {
        username = "unknown"
    }
    
    effectiveRole := p.GetUserEffectiveRole(username)
    if effectiveRole != "admin" && effectiveRole != "owner" && effectiveRole != "mod" {
        http.Error(w, "Insufficient permissions", http.StatusForbidden)
        return
    }
    
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not ready", http.StatusInternalServerError)
		return
	}

    rows, err := db.Query(`
        SELECT 
            id, nick, email, role, hostmask, lang, mychar,
            welcome, website, discord_id, telegram_id, facebook,
            added_by, created_at, last_login, invites
        FROM users 
        ORDER BY last_login DESC 
        LIMIT 100
    `)
    if err != nil {
        fmt.Printf("[YnMApi] ❌ Database query error: %v\n", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var u User
        var lastLogin, email, website, discord, telegram, facebook, welcome, addedBy, hostmask, lang, mychar sql.NullString
        var createdAt sql.NullTime
        
        err := rows.Scan(
            &u.ID, &u.Nick, &email, &u.Role, &hostmask, &lang, &mychar,
            &welcome, &website, &discord, &telegram, &facebook,
            &addedBy, &createdAt, &lastLogin, &u.Invites,
        )
        if err != nil {
            fmt.Printf("[YnMApi] ❌ Row scan error: %v\n", err)
            continue
        }
        
        u.Username = u.Nick
        
        if lastLogin.Valid {
            u.LastLogin = lastLogin.String
            u.LastSeen = lastLogin.String
        } else {
            u.LastSeen = "Never"
            u.LastLogin = ""
        }
        
        if email.Valid { u.Email = email.String }
        if website.Valid { u.Website = website.String }
        if discord.Valid { u.DiscordID = discord.String }
        if telegram.Valid { u.TelegramID = telegram.String }
        if facebook.Valid { u.Facebook = facebook.String }
        if welcome.Valid { u.Welcome = welcome.String }
        if addedBy.Valid { u.AddedBy = addedBy.String }
        if hostmask.Valid { u.Hostmask = hostmask.String }
        if lang.Valid { u.Lang = lang.String } else { u.Lang = "en" }
        if mychar.Valid { u.MyChar = mychar.String } else { u.MyChar = "!" }
        if createdAt.Valid { u.CreatedAt = createdAt.Time.Format("2006-01-02 15:04:05") }
        
        users = append(users, u)
    }

    fmt.Printf("[YnMApi] ✅ Loaded %d users\n", len(users))

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "recent_users": users,
        "users": users,
        "count": len(users),
    })
}

// =====================
// HANDLE USERS ADD (POST) - ÚJ FUNKCIÓ!
// =====================
func (p *YnMApiPlugin) handleUsersAdd(w http.ResponseWriter, r *http.Request) {
    // 1. Ellenőrizd a permission-öket
    username := r.Header.Get("X-Username")
    if username == "" {
        sendJSON(w, http.StatusUnauthorized, map[string]interface{}{
            "success": false,
            "error":   "Authentication required",
        })
        return
    }
    
    callerRole := p.GetUserEffectiveRole(username)
    if !strings.Contains("mod,admin,owner", strings.ToLower(callerRole)) {
        sendJSON(w, http.StatusForbidden, map[string]interface{}{
            "success": false,
            "error":   "Insufficient permissions to add users",
        })
        return
    }
    
    // 2. Parse request data
    var reqData map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
        sendJSON(w, http.StatusBadRequest, map[string]interface{}{
            "success": false,
            "error":   "Invalid JSON: " + err.Error(),
        })
        return
    }
    
    // 3. Validáció - kötelező mezők
    nick, nickOk := reqData["nick"].(string)
    hostmask, hostmaskOk := reqData["hostmask"].(string)
    
    if !nickOk || nick == "" {
        sendJSON(w, http.StatusBadRequest, map[string]interface{}{
            "success": false,
            "error":   "Nick is required",
        })
        return
    }
    
    if !hostmaskOk || hostmask == "" {
        sendJSON(w, http.StatusBadRequest, map[string]interface{}{
            "success": false,
            "error":   "Hostmask is required",
        })
        return
    }
    
    // 4. Ellenőrizd, hogy létezik-e már a nick
		db, err := p.getDB()
		if err != nil || db == nil {
			sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   "Database unavailable",
			})
			return
		}
	var existingID int
	err = db.QueryRow("SELECT id FROM users WHERE nick = ?", nick).Scan(&existingID)
	if err == nil {
		sendJSON(w, http.StatusConflict, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("User '%s' already exists", nick),
		})
		return
	}
    
    // 5. Alapértékek beállítása
    role := "user"
    if r, ok := reqData["role"].(string); ok && r != "" {
        role = r
    }
    
    lang := "en"
    if l, ok := reqData["lang"].(string); ok && l != "" {
        lang = l
    }
    
    mychar := "!"
    if mc, ok := reqData["mychar"].(string); ok && mc != "" {
        mychar = mc
    }
    
    invites := 0
    if inv, ok := reqData["invites"].(float64); ok {
        invites = int(inv)
    }
    
    // 6. Jelszó hash (ha van)
    
    // 7. Insert into database
    query := `
        INSERT INTO users (
            nick, hostmask, role, email, lang, mychar, invites,
            welcome, website, discord_id, telegram_id, facebook,
            added_by, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
    `
    
    result, err := db.Exec(query,
        nick,
        hostmask,
        role,
        reqData["email"],
        lang,
        mychar,
        invites,
        reqData["welcome"],
        reqData["website"],
        reqData["discord_id"],
        reqData["telegram_id"],
        reqData["facebook"],
        username, // added_by
    )
    
    if err != nil {
        fmt.Printf("[YnMApi] ❌ Failed to add user %s: %v\n", nick, err)
        sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
            "success": false,
            "error":   "Database error: " + err.Error(),
        })
        return
    }
    
    // 8. Get new user ID
    userID, _ := result.LastInsertId()
    
    // 9. Audit log
    p.logAudit(username, "USER_ADDED", r.RemoteAddr, 
        fmt.Sprintf("Added user: %s (role: %s)", nick, role))
    
    fmt.Printf("[YnMApi] ✅ User '%s' added successfully with ID %d\n", nick, userID)
    
    // 10. Success response
    sendJSON(w, http.StatusOK, map[string]interface{}{
        "success":   true,
        "message":   fmt.Sprintf("User '%s' added successfully", nick),
        "user_id":   userID,
        "user_nick": nick,
    })
}

func (p *YnMApiPlugin) handleUsersUpdate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPut {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
	
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not ready", http.StatusInternalServerError)
		return
	}
    // Parse user ID from URL path
    pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(pathParts) < 2 {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }
    userID := pathParts[len(pathParts)-1]
    
    // Parse request body
    var updateData map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Build UPDATE query dynamically
    var setClauses []string
    var args []interface{}
    argCount := 1
    
    // Email
    if email, ok := updateData["email"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("email = $%d", argCount))
        args = append(args, email)
        argCount++
    }
    
    // Role
    if role, ok := updateData["role"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("role = $%d", argCount))
        args = append(args, role)
        argCount++
    }
    
    // Hostmask
    if hostmask, ok := updateData["hostmask"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("hostmask = $%d", argCount))
        args = append(args, hostmask)
        argCount++
    }
    
    // Lang
    if lang, ok := updateData["lang"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("lang = $%d", argCount))
        args = append(args, lang)
        argCount++
    }
    
    // MyChar
    if mychar, ok := updateData["mychar"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("mychar = $%d", argCount))
        args = append(args, mychar)
        argCount++
    }
    
    // Welcome
    if welcome, ok := updateData["welcome"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("welcome = $%d", argCount))
        args = append(args, welcome)
        argCount++
    }
    
    // Website
    if website, ok := updateData["website"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("website = $%d", argCount))
        args = append(args, website)
        argCount++
    }
    
    // Discord ID
    if discordID, ok := updateData["discord_id"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("discord_id = $%d", argCount))
        args = append(args, discordID)
        argCount++
    }
    
    // Telegram ID
    if telegramID, ok := updateData["telegram_id"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("telegram_id = $%d", argCount))
        args = append(args, telegramID)
        argCount++
    }
    
    // Facebook
    if facebook, ok := updateData["facebook"].(string); ok {
        setClauses = append(setClauses, fmt.Sprintf("facebook = $%d", argCount))
        args = append(args, facebook)
        argCount++
    }
    
    // Invites
    if invites, ok := updateData["invites"].(float64); ok {
        setClauses = append(setClauses, fmt.Sprintf("invites = $%d", argCount))
        args = append(args, int(invites))
        argCount++
    }
    
    // Password (hash it)
    if password, ok := updateData["password"].(string); ok {
        hashedPass := hashPassword(password)
        setClauses = append(setClauses, fmt.Sprintf("pass = $%d", argCount))
        args = append(args, hashedPass)
        argCount++
    }
    
    if len(setClauses) == 0 {
        http.Error(w, "No fields to update", http.StatusBadRequest)
        return
    }
    
    // Add user ID as last parameter
    args = append(args, userID)
    
    // Execute UPDATE
    query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", 
        strings.Join(setClauses, ", "), argCount)
    
    result, err := db.Exec(query, args...)
    if err != nil {
        fmt.Printf("[YnMApi] ❌ Update error: %v\n", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }
    
    fmt.Printf("[YnMApi] ✅ User %s updated successfully\n", userID)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "User updated successfully",
    })
}
// ✅ USER DELETE HANDLER - /users/:nick endpoint

func (p *YnMApiPlugin) handleUsersDelete(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodDelete {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not ready", http.StatusServiceUnavailable)
		return
	}
    
    // Parse user nick from URL path
    pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(pathParts) < 2 {
        http.Error(w, "Invalid user identifier", http.StatusBadRequest)
        return
    }
    userNick := pathParts[len(pathParts)-1]
    
    fmt.Printf("[YnMApi] 🗑️  Attempting to delete user: %s\n", userNick)
    
    // Check if user exists first
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE nick = ?)", userNick).Scan(&exists)
	if err != nil {
		fmt.Printf("[YnMApi] ❌ Database error: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
		
    if !exists {
        fmt.Printf("[YnMApi] ❌ User not found: %s\n", userNick)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error":   "User not found",
        })
        return
    }
    
    // Execute DELETE
    result, err := db.Exec("DELETE FROM users WHERE nick = ?", userNick)
    if err != nil {
        fmt.Printf("[YnMApi] ❌ Delete error: %v\n", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        fmt.Printf("[YnMApi] ❌ No rows deleted for user: %s\n", userNick)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error":   "Failed to delete user",
        })
        return
    }
    
    fmt.Printf("[YnMApi] ✅ User deleted successfully: %s\n", userNick)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": fmt.Sprintf("User '%s' deleted successfully", userNick),
    })
}
// Helper function to hash passwords (bcrypt)
func hashPassword(password string) string {
    // Simple MD5 hash (use bcrypt in production!)
    hash := md5.Sum([]byte(password))
    return hex.EncodeToString(hash[:])
}
func (p *YnMApiPlugin) handleDatabase(w http.ResponseWriter, r *http.Request) {
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}
    switch r.Method {
    case http.MethodGet:
        // Get all users from existing database
        rows, err := db.Query(`
            SELECT nick, hostmask, role, added_by, email, invites, created_at
            FROM users ORDER BY created_at DESC
        `)
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()
        
        var users []map[string]interface{}
        for rows.Next() {
            var nick, hostmask, role, addedBy string
            var email sql.NullString
            var invites int
            var createdAt time.Time
            
            if err := rows.Scan(&nick, &hostmask, &role, &addedBy, &email, &invites, &createdAt); err != nil {
                continue
            }
            
            emailStr := ""
            if email.Valid {
                emailStr = email.String
            }
            
            users = append(users, map[string]interface{}{
                "nick":       nick,
                "hostmask":   hostmask,
                "role":       role,
                "added_by":   addedBy,
                "email":      emailStr,
                "invites":    invites,
                "created_at": createdAt.Format("2006-01-02 15:04:05"),
            })
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "users": users,
            "count": len(users),
        })
        
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}
