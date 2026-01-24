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
    "crypto/rand"
    "database/sql"
    "encoding/json"
    "fmt"
    "math/big"
    "net/http"
    "sort"
    "strconv"
    "strings"
    "time"
    
    "golang.org/x/crypto/bcrypt"
    "git.ynm.hu/markus/YnM-Go/YnMConfig"
)

// PasswordChangeRequest - jelszó módosítás kérés struktúra
type PasswordChangeRequest struct {
    Username      string `json:"username"`
    ExpiryMinutes int    `json:"expiry_minutes"`
    MaxUses       int    `json:"max_uses"`
}

// PasswordAddRequest - új jelszó hozzáadás kérés struktúra
type PasswordAddRequest struct {
    Username      string `json:"username"`
    Password      string `json:"password"`  
    ExpiryMinutes int    `json:"expiry_minutes"`
    MaxUses       int    `json:"max_uses"`
}

// PasswordDeleteRequest - jelszó törlés kérés struktúra
type PasswordDeleteRequest struct {
    Username string `json:"username"`
}

// PasswordInfo - jelszó információ struktúra (válaszhoz)
type PasswordInfo struct {
    Username    string    `json:"username"`
    Password    string    `json:"password,omitempty"` // csak generálásnál
    ExpiresAt   time.Time `json:"expires_at"`
    CreatedAt   time.Time `json:"created_at"`
    Uses        int       `json:"uses"`
    MaxUses     int       `json:"max_uses"`
    GeneratedBy string    `json:"generated_by"`
    Remaining   string    `json:"remaining"` 
}

func (p *YnMApiPlugin) generateSecurePassword(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
    password := make([]byte, length)
    
    for i := range password {
        num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
        password[i] = charset[num.Int64()]
    }
    
    return string(password)
}

func (p *YnMApiPlugin) generatePasswordForUser(username, requestedBy string, expiryMinutes int) string {
    p.mutex.Lock()
    defer p.mutex.Unlock()

    delete(p.passwords, username)

    password := p.generateSecurePassword(12)
    
    var expiresAt time.Time
    if expiryMinutes == 0 {
        expiresAt = time.Now().AddDate(100, 0, 0)
    } else {
        expiresAt = time.Now().Add(time.Duration(expiryMinutes) * time.Minute)
    }
    
    // MINDIG 10 HASZNÁLATI LIMIT
    maxUses := 10
    
    entry := &PasswordEntry{
        Username:    username,
        Password:    password,
        CreatedAt:   time.Now(),
        ExpiresAt:   expiresAt,
        UsedCount:   0,
        MaxUses:     maxUses,
        RequestedBy: requestedBy,
    }
    
    p.passwords[username] = entry
    if err := p.savePasswordToDB(username, password, requestedBy, expiresAt, maxUses); err != nil {
        fmt.Printf("[YnMApI] Failed to save to database: %v\n", err)
    } else {
        fmt.Printf("[YnMApI] Password saved to database for %s (expires in: %d minutes)\n", 
            username, expiryMinutes)
    }
    
    return password
}

func (p *YnMApiPlugin) generatePasswordForUserWithMaxUses(username, requestedBy string, expiryMinutes, maxUses int) string {
    p.mutex.Lock()
    defer p.mutex.Unlock()

    // Töröljük az előző jelszavakat
    delete(p.passwords, username)

    password := p.generateSecurePassword(12)
    
    var expiresAt time.Time
    if expiryMinutes == 0 {
        expiresAt = time.Now().AddDate(100, 0, 0)
    } else {
        expiresAt = time.Now().Add(time.Duration(expiryMinutes) * time.Minute)
    }
    
    entry := &PasswordEntry{
        Username:    username,
        Password:    password,
        CreatedAt:   time.Now(),
        ExpiresAt:   expiresAt,
        UsedCount:   0,
        MaxUses:     maxUses,
        RequestedBy: requestedBy,
    }
    
    p.passwords[username] = entry
    
    // Mentés adatbázisba
    if err := p.savePasswordToDB(username, password, requestedBy, expiresAt, maxUses); err != nil {
        fmt.Printf("[YnMApI] Failed to save to database: %v\n", err)
    } else {
        fmt.Printf("[YnMApI] Password saved to database for %s (expires in: %d minutes, max uses: %d)\n", 
            username, expiryMinutes, maxUses)
    }
    
    return password
}

func (p *YnMApiPlugin) validatePassword(username, password string) bool {
    // 1. Először nézzük a memóriát (ez a legújabb aktív jelszó)
    p.mutex.RLock()
    entry, exists := p.passwords[username]
    p.mutex.RUnlock()
    
    if exists {
        if time.Now().After(entry.ExpiresAt) || entry.UsedCount >= entry.MaxUses {
            p.mutex.Lock()
            delete(p.passwords, username)
            p.mutex.Unlock()
            // Menjünk tovább az adatbázisra
        } else if entry.Password == password {
            // ✅ Megvan a memóriában!
            entry.UsedCount++
            go p.incrementPasswordUse(username)
            
            if entry.UsedCount >= entry.MaxUses {
                p.mutex.Lock()
                delete(p.passwords, username)
                p.mutex.Unlock()
            }
            
            fmt.Printf("[YnMApI] ✅ Password validated from memory for %s (uses: %d/%d)\n",
                username, entry.UsedCount, entry.MaxUses)
            return true
        }
    }
    
    // 2. Ha nincs a memóriában vagy nem egyezik, nézzük az adatbázis összes jelszavát
    return p.validatePasswordFromDB(username, password)
}

func (p *YnMApiPlugin) validatePasswordFromDB(username, password string) bool {
	db, err := p.getDB()
	if err != nil || db == nil {
		return false
	}

    rows, err := db.Query(`
        SELECT 
            nick,
            pass, 
            password_expires, 
            COALESCE(password_uses, 0), 
            COALESCE(password_max_uses, 0)
        FROM users 
        WHERE nick = ? 
            AND pass IS NOT NULL
            AND (password_expires IS NULL OR password_expires > datetime('now'))
    `, username)
    
    if err != nil {
        if err != sql.ErrNoRows {
            fmt.Printf("[YnMApI] Database error for %s: %v\n", username, err)
        }
        return false
    }
    defer rows.Close()
    
    var found bool
    var dbPassword string
    var expiresAtStr sql.NullString
    var uses, maxUses int
    
    for rows.Next() {
        var nick string
        
        if err := rows.Scan(&nick, &dbPassword, &expiresAtStr, &uses, &maxUses); err != nil {
            fmt.Printf("[YnMApI] Scan error for %s: %v\n", username, err)
            continue
        }
        
		fmt.Printf("[YnMApI] DEBUG: Comparing password '%s' with hash '%s' (length: %d)\n", 
			password, dbPassword[:20], len(dbPassword))

		if p.checkPasswordHash(password, dbPassword) {
			fmt.Printf("[YnMApI] DEBUG: Hash match SUCCESS!\n")
			found = true
			break
		} else {
			fmt.Printf("[YnMApI] DEBUG: Hash match FAILED!\n")
		}

		// Kompatibilitás
		if dbPassword == password {
			fmt.Printf("[YnMApI] DEBUG: Plain text match SUCCESS!\n")
			found = true
			break
		}
    }
    
    if !found {
        fmt.Printf("[YnMApI] No matching password found for %s\n", username)
        return false
    }
    
    // Expiry kezelés
    var expiresAt time.Time
    if expiresAtStr.Valid && expiresAtStr.String != "" {
        var parseErr error
        expiresAt, parseErr = time.Parse(time.RFC3339, expiresAtStr.String)
        
        if parseErr != nil {
            expiresAt, parseErr = time.Parse("2006-01-02 15:04:05", expiresAtStr.String)
            if parseErr != nil {
                expiresAt, parseErr = time.Parse("2006-01-02T15:04:05Z", expiresAtStr.String)
                if parseErr != nil {
                    fmt.Printf("[YnMApI] Date parse error for %s (value: %s): %v\n", 
                        username, expiresAtStr.String, parseErr)
                    return false
                }
            }
        }
        
        if time.Now().After(expiresAt) {
            fmt.Printf("[YnMApI] Password expired for %s (was: %s)\n", 
                username, expiresAt.Format("2006-01-02 15:04:05"))
            return false
        }
    } else {
        expiresAt = time.Now().AddDate(100, 0, 0)
        fmt.Printf("[YnMApI] No expiry date for %s (IRC password)\n", username)
    }
    
    // ✅ JAVÍTÁS: max_uses ellenőrzés - 0 = unlimited!
    if maxUses > 0 && uses >= maxUses {
        fmt.Printf("[YnMApI] Max uses reached for %s (%d/%d)\n", username, uses, maxUses)
        return false
    }
    
    // Használat növelése - CSAK ha max_uses > 0
    if maxUses > 0 {
        go p.incrementPasswordUse(username)
    }
    
    // Memóriában frissítés
    p.mutex.Lock()
    p.passwords[username] = &PasswordEntry{
        Username:    username,
        Password:    password,
        CreatedAt:   time.Now(),
        ExpiresAt:   expiresAt,
        UsedCount:   uses + 1,
        MaxUses:     maxUses,
        RequestedBy: "YnM-Go",
    }
    p.mutex.Unlock()
    
    // Log
    if maxUses == 0 {
        fmt.Printf("[YnMApI] ✅ Password validated from DB for %s (IRC password, unlimited uses)\n", username)
    } else {
        fmt.Printf("[YnMApI] ✅ Password validated from DB for %s (uses: %d/%d)\n", 
            username, uses+1, maxUses)
    }
    
    return true
}




func (p *YnMApiPlugin) savePasswordToDB(username, password, requestedBy string, expiresAt time.Time, maxUses int) error {
    if p.adminPlugin == nil || p.adminPlugin.Db == nil {
        return fmt.Errorf("database not available")
    }
    
    // Hash-elés, ha kell
    var passwordHash string
    if !strings.HasPrefix(password, "$2") && !strings.HasPrefix(password, "$5") && !strings.HasPrefix(password, "$6") {
        hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            return fmt.Errorf("error hashing password: %v", err)
        }
        passwordHash = string(hashed)
    } else {
        passwordHash = password
    }
    
    // Használjuk az AdminDB publikus metódusát
    return p.adminPlugin.Db.SetWebPassword(username, passwordHash, &expiresAt, maxUses, requestedBy)
}


func (p *YnMApiPlugin) incrementPasswordUse(username string) {
		db, err := p.getDB()
		if err != nil || db == nil {

			return
		}
    
    // JAVÍTOTT: Növeljük a legutolsó AKTÍV jelszó használatát
    _, err = db.Exec(`
        UPDATE users 
        SET password_uses = password_uses + 1
        WHERE nick = ? 
            AND pass IS NOT NULL
            AND (password_expires IS NULL OR password_expires > datetime('now'))
    `, username)
    
    if err != nil {
        fmt.Printf("[YnMApI] Failed to increment use for %s: %v\n", username, err)
    }
}

func (p *YnMApiPlugin) incrementPasswordUseForUser(username string) {
	db, err := p.getDB()
	if err != nil || db == nil {
		return
	}
    
    _, err = db.Exec(`
        UPDATE users 
        SET password_uses = password_uses + 1,
            password_last_used = datetime('now')
        WHERE nick = ? 
            AND pass IS NOT NULL
    `, username)
    
    if err != nil {
        fmt.Printf("[YnMApI] Failed to increment use for %s: %v\n", username, err)
    }
}

func (p *PasswordEntry) isUnlimited() bool {
    // 0 vagy 999999 mind unlimited
    return p.MaxUses == 0 || p.MaxUses >= 999999
}

func (p *YnMApiPlugin) showExpiryOptions(nick, role string, cfg *YnMConfig.YnMApiConfig) string {
    messages := []string{
        "🔐 Web Password Generator - Available options:",
        "   Usage: !web [option]",
    }
    
    // Rendezzük a lejárati opciókat (csökkenő sorrend)
    var options []int
    for minutes := range cfg.YnM.Password.ExpiryOptions {
        options = append(options, minutes)
    }
    sort.Sort(sort.Reverse(sort.IntSlice(options)))
    
    // Adjuk hozzá az opciókat
    for _, minutes := range options {
        name := cfg.YnM.Password.ExpiryOptions[minutes]
        // Készítsünk egy rövid parancsot az első szó alapján
        firstWord := strings.ToLower(strings.Split(name, " ")[0])
        messages = append(messages, fmt.Sprintf("   !web %s - %s", firstWord, name))
    }
    
    // További gyors opciók
    messages = append(messages,
        "",
        "   Quick options:",
        "   !web 24h - 24 óra",
        "   !web week - 1 hét",
        "   !web month - 1 hónap",
        "   !web year - 1 év",
        "   !web never - Soha nem jár le",
        "   !web 30 - 30 perc",
        "   !web 180 - 3 óra",
        "",
        fmt.Sprintf("   Current role: %s", role),
        fmt.Sprintf("   🌐 Website: %s/login", cfg.YnM.WebsiteURL),
        "",
        "   💡 Examples:",
        "   !web 1h - 1 órás jelszó",
        "   !web 24h - 24 órás jelszó",
        "   !web month - 1 hónapos jelszó",
    )
    
    for _, message := range messages {
        p.client.SendMessage(nick, message)
    }
    
    return fmt.Sprintf("Password options sent to %s", nick)
}

func (p *YnMApiPlugin) formatExpiryText(minutes int) string {
    if minutes == 0 {
        return "soha nem jár le"
    }
    
    if minutes < 60 {
        return fmt.Sprintf("%d perc", minutes)
    }
    
    hours := minutes / 60
    if hours < 24 {
        return fmt.Sprintf("%d óra", hours)
    }
    
    days := hours / 24
    if days < 7 {
        return fmt.Sprintf("%d nap", days)
    }
    
    weeks := days / 7
    if weeks < 4 {
        return fmt.Sprintf("%d hét", weeks)
    }
    
    months := days / 30
    if months < 12 {
        return fmt.Sprintf("%d hónap", months)
    }
    
    years := days / 365
    return fmt.Sprintf("%d év", years)
}

// handlePasswordChange - HTTP endpoint: jelszó módosítása
func (p *YnMApiPlugin) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost && r.Method != http.MethodPut {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse request
    var req PasswordChangeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validáció
    if req.Username == "" {
        http.Error(w, "Username is required", http.StatusBadRequest)
        return
    }

    if req.ExpiryMinutes < 0 {
        http.Error(w, "Invalid expiry minutes", http.StatusBadRequest)
        return
    }

    if req.MaxUses < 0 {
        http.Error(w, "Invalid max uses", http.StatusBadRequest)
        return
    }

    // Default értékek
    if req.ExpiryMinutes == 0 {
        req.ExpiryMinutes = 60 // 1 óra default
    }
    if req.MaxUses == 0 {
        req.MaxUses = 999999 // unlimited
    }

    // User info a tokenből - BIZTONSÁGOS context kezelés
    requestedBy := "web"
    if ctx := r.Context(); ctx != nil {
        if val := ctx.Value("username"); val != nil {
            if username, ok := val.(string); ok && username != "" {
                requestedBy = username
            }
        }
    }

    // Új jelszó generálása
    password := p.generatePasswordForUserWithMaxUses(req.Username, requestedBy, req.ExpiryMinutes, req.MaxUses)

    p.logAudit(requestedBy, "🔄", "YnM-Go", fmt.Sprintf("Changed password for %s (expiry: %d min, max uses: %d)", 
        req.Username, req.ExpiryMinutes, req.MaxUses))

    // Válasz
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "message":  "Password changed successfully",
        "username": req.Username,
        "password": password,
        "expires_in_minutes": req.ExpiryMinutes,
        "max_uses": req.MaxUses,
    })
}

// handlePasswordAdd - HTTP endpoint: új jelszó hozzáadása
func (p *YnMApiPlugin) handlePasswordAdd(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse request
    var req PasswordAddRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validáció
    if req.Username == "" {
        http.Error(w, "Username is required", http.StatusBadRequest)
        return
    }

    // Default értékek
    if req.ExpiryMinutes == 0 {
        req.ExpiryMinutes = 60
    }
    if req.MaxUses == 0 {
        req.MaxUses = 999999
    }

    // User info a tokenből
    requestedBy := "YnM-Go"
    if ctx := r.Context(); ctx != nil {
        if val := ctx.Value("username"); val != nil {
            if username, ok := val.(string); ok && username != "" {
                requestedBy = username
            }
        }
    }

    var password string
    var err error
    
    // 👇 EZ A FONTOS RÉSZ: ha van megadott jelszó, azt használjuk
    if req.Password != "" {
        password = req.Password
        // Számítsuk ki a lejárati dátumot
        var expiresAt time.Time
        if req.ExpiryMinutes == 0 {
            expiresAt = time.Now().AddDate(100, 0, 0)
        } else {
            expiresAt = time.Now().Add(time.Duration(req.ExpiryMinutes) * time.Minute)
        }
        
        // Használjuk a már létező savePasswordToDB-t
        err = p.savePasswordToDB(req.Username, password, requestedBy, expiresAt, req.MaxUses)
        if err != nil {
            http.Error(w, "Failed to save password to database", http.StatusInternalServerError)
            return
        }
    } else {
        // Ha nincs megadva, akkor generálunk
        password = p.generatePasswordForUserWithMaxUses(req.Username, requestedBy, req.ExpiryMinutes, req.MaxUses)
    }

    p.logAudit(requestedBy, "➕", "web", fmt.Sprintf("Added password for %s (expiry: %d min, max uses: %d)", 
        req.Username, req.ExpiryMinutes, req.MaxUses))

    // Válasz
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "message":  "Password created successfully",
        "username": req.Username,
        "password": password,
        "expires_in_minutes": req.ExpiryMinutes,
        "max_uses": req.MaxUses,
    })
}

// handlePasswordList - HTTP endpoint: aktív jelszavak listázása
func (p *YnMApiPlugin) handlePasswordList(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

    // ✅ Get current user from context or header
    currentUser := "" 
    if r.Header.Get("X-Username") != "" {
        currentUser = r.Header.Get("X-Username")
    } else if ctx := r.Context(); ctx != nil {
        if val := ctx.Value("username"); val != nil {
            if username, ok := val.(string); ok {
                currentUser = username
            }
        }
    }
    
    if currentUser == "" {
        http.Error(w, "User not identified", http.StatusUnauthorized)
        return
    }
    
    // ✅ Get user's effective role
    userEffectiveRole := p.GetUserEffectiveRole(currentUser)

    // Query paraméterek
    username := r.URL.Query().Get("username")
    limitStr := r.URL.Query().Get("limit")
    
    limit := 100 // default limit
    if limitStr != "" {
        if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
            limit = l
        }
    }

    // ✅ SQL query - Szűrés role alapján
    query := `
        SELECT 
            nick as username,
            password_expires as expires_at,
            COALESCE(password_uses, 0) as uses,
            COALESCE(password_max_uses, 10) as max_uses,
            COALESCE(password_created, CURRENT_TIMESTAMP) as created_at,
            'YnM-Go' as generated_by
        FROM users 
        WHERE pass IS NOT NULL
            AND (password_expires IS NULL OR password_expires > datetime('now'))
            AND (
                COALESCE(password_uses, 0) < COALESCE(password_max_uses, 10)
                OR password_max_uses = 999999
                OR password_max_uses = 0
            )
    `
    
    args := []interface{}{}
    
    // ✅ ROLE-BASED FILTERING
    // Ha van username paraméter, csak akkor használjuk ha owner
    if username != "" {
        // Ha nem owner és más user-t kérdez le, tiltjuk
        if userEffectiveRole != "owner" && username != currentUser {
            http.Error(w, "You can only view your own password", http.StatusForbidden)
            return
        }
        query += " AND nick = ?"
        args = append(args, username)
    } else {
        // Ha nincs username paraméter, akkor role alapján szűrünk
        if userEffectiveRole != "owner" {
            // Nem owner: csak sajátját láthatja
            query += " AND nick = ?"
            args = append(args, currentUser)
        }
        // Owner: mindent láthat (nincs extra where clause)
    }
    
    query += " ORDER BY password_created DESC LIMIT ?"
    args = append(args, limit)

    rows, err := db.Query(query, args...)
    if err != nil {
        fmt.Printf("[YnMApI] Query error: %v\n", err)
        http.Error(w, "Database query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var passwords []PasswordInfo
    for rows.Next() {
        var info PasswordInfo
        var expiresAtStr sql.NullString
        var createdAtStr sql.NullString
        
        if err := rows.Scan(
            &info.Username,
            &expiresAtStr,
            &info.Uses,
            &info.MaxUses,
            &createdAtStr,
            &info.GeneratedBy,
        ); err != nil {
            fmt.Printf("[YnMApI] Scan error: %v\n", err)
            continue
        }

        // ... a többi feldolgozás változatlan ...

        passwords = append(passwords, info)
    }
    
    // ✅ Audit log
    p.logAudit(currentUser, "📋", "YnM-Go", 
        fmt.Sprintf("Listed passwords (count: %d, role: %s)", 
            len(passwords), userEffectiveRole))

    // ✅ Válasz user infóval
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":   true,
        "count":     len(passwords),
        "passwords": passwords,
        "user_info": map[string]interface{}{
            "username": currentUser,
            "role":     userEffectiveRole,
            "can_see_all": userEffectiveRole == "owner",
        },
    })
}
// handlePasswordDelete - HTTP endpoint: jelszó törlése
func (p *YnMApiPlugin) handlePasswordDelete(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodDelete && r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req PasswordDeleteRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if req.Username == "" {
        http.Error(w, "Username is required", http.StatusBadRequest)
        return
    }
    
    // ✅ VÉDELEM: Owner jelszavát SOHA nem törölhetjük
    if strings.ToLower(req.Username) == "owner" {
        http.Error(w, "🛡️ Owner password is protected and cannot be deleted!", http.StatusForbidden)
        return
    }
    
    // User info
    requestedBy := "web"
    if ctx := r.Context(); ctx != nil {
        if val := ctx.Value("username"); val != nil {
            if username, ok := val.(string); ok && username != "" {
                requestedBy = username
                
                // ✅ VÉDELEM: Saját jelszavát nem törölheti
                if strings.EqualFold(username, req.Username) {
                    http.Error(w, "❌ You cannot delete your own password!", http.StatusForbidden)
                    return
                }
            }
        }
    }
    
    // Töröljük a memóriából
    p.mutex.Lock()
    delete(p.passwords, req.Username)
    p.mutex.Unlock()
    
    // Töröljük az adatbázisból
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}
    
    result, err := db.Exec(`
        UPDATE users 
        SET pass = NULL,
            password_expires = NULL,
            password_uses = 0
        WHERE nick = ?
    `, req.Username)
    
    if err != nil {
        http.Error(w, "Failed to delete password", http.StatusInternalServerError)
        return
    }
    
    rowsAffected, _ := result.RowsAffected()
    
    p.logAudit(requestedBy, "🗑️", "web", fmt.Sprintf("Deleted %d password(s) for %s", rowsAffected, req.Username))
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":       true,
        "message":       "Password(s) deleted successfully",
        "username":      req.Username,
        "rows_affected": rowsAffected,
    })
}
// formatDuration - időtartam formázása emberi olvasható formába
func (p *YnMApiPlugin) formatDuration(d time.Duration) string {
    if d < 0 {
        return "expired"
    }
    
    if d < time.Minute {
        return fmt.Sprintf("%d másodperc", int(d.Seconds()))
    }
    
    if d < time.Hour {
        return fmt.Sprintf("%d perc", int(d.Minutes()))
    }
    
    if d < 24*time.Hour {
        hours := int(d.Hours())
        minutes := int(d.Minutes()) % 60
        if minutes > 0 {
            return fmt.Sprintf("%d óra %d perc", hours, minutes)
        }
        return fmt.Sprintf("%d óra", hours)
    }
    
    days := int(d.Hours()) / 24
    hours := int(d.Hours()) % 24
    if hours > 0 {
        return fmt.Sprintf("%d nap %d óra", days, hours)
    }
    return fmt.Sprintf("%d nap", days)
}