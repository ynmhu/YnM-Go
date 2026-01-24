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
    "time"
	"strings"
	"database/sql"
    _ "github.com/mattn/go-sqlite3"
)

func (p *YnMApiPlugin) handleDatabasePasswords(w http.ResponseWriter, r *http.Request) {
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

    // ✅ Get current user from header (requireAuth már beállította)
    currentUser := r.Header.Get("X-Username")
    if currentUser == "" {
        http.Error(w, "User not identified", http.StatusInternalServerError)
        return
    }
    
    // ✅ Get user's effective role
    userEffectiveRole := p.GetUserEffectiveRole(currentUser)

    switch r.Method {
    case http.MethodGet:
        // ✅ Build SQL query based on user role
        baseQuery := `
            SELECT 
                u.id,
                u.nick,
                u.pass,
                u.password_expires,
                u.password_created,
                COALESCE(u.password_uses, 0) as password_uses,
                COALESCE(u.password_max_uses, 10) as password_max_uses,
                'YnM-Go' as generated_by,
                CASE 
                    WHEN u.password_expires IS NULL THEN NULL
                    WHEN strftime('%s', u.password_expires) < strftime('%s', 'now') THEN 1
                    ELSE 0
                END as expired,
                CASE
                    WHEN u.password_expires IS NULL THEN NULL
                    ELSE CAST((strftime('%s', u.password_expires) - strftime('%s', 'now')) AS INTEGER)
                END as time_left,
                CASE 
                    WHEN u.password_expires IS NULL THEN NULL
                    WHEN strftime('%s', u.password_expires) < strftime('%s', 'now') THEN 
                        CAST((strftime('%s', 'now') - strftime('%s', u.password_expires)) / 3600.0 AS INTEGER)
                    ELSE 0
                END as hours_since_expiry
            FROM users u
            WHERE u.pass IS NOT NULL    -- ✅ MINDEN jelszó (webes + IRC)
        `
        
        // ✅ WHERE clause és paraméterek role alapján
        var whereClauses []string
        var queryParams []interface{}
        
        // Ha nem owner, akkor csak saját jelszavát lássa
        if userEffectiveRole != "owner" {
            whereClauses = append(whereClauses, "u.nick = ?")
            queryParams = append(queryParams, currentUser)
        }
        
        // Ha van where clause, add hozzá
        if len(whereClauses) > 0 {
            baseQuery += " AND " + strings.Join(whereClauses, " AND ")
        }
        
        // ORDER BY
        baseQuery += " ORDER BY u.password_created DESC NULLS LAST"
        
        // ✅ Execute query with parameters
        rows, err := db.Query(baseQuery, queryParams...)
        
        if err != nil {
            http.Error(w, "Database error: " + err.Error(), http.StatusInternalServerError)
            return
        }
        defer rows.Close()
        
        var passwords []map[string]interface{}
        for rows.Next() {
            var id int
            var username, password, generatedBy string
            var expiresAt, createdAt sql.NullString
            var uses, maxUses int
            var expired, timeLeft, hoursSinceExpiry sql.NullInt64
            
            // ✅ ID beolvasása
            if err := rows.Scan(&id, &username, &password, &expiresAt, &createdAt, &uses, &maxUses, 
                      &generatedBy, &expired, &timeLeft, &hoursSinceExpiry); err != nil {
                fmt.Printf("[YnMApi] Scan error: %v\n", err)
                continue
            }
            
            expiredBool := false
            if expired.Valid {
                expiredBool = expired.Int64 == 1
            }
            
            timeLeftVal := 0
            if timeLeft.Valid {
                timeLeftVal = int(timeLeft.Int64)
            }
            
            hoursSinceExpiryVal := 0
            if hoursSinceExpiry.Valid {
                hoursSinceExpiryVal = int(hoursSinceExpiry.Int64)
            }
            
            passwords = append(passwords, map[string]interface{}{
                "id":                 id,
                "username":           username,
                "password":           password,
                "expires_at":         expiresAt.String,
                "created_at":         createdAt.String,
                "uses":               uses,
                "max_uses":           maxUses,
                "generated_by":       generatedBy,
                "expired":            expiredBool,
                "time_left":          timeLeftVal,
                "hours_since_expiry": hoursSinceExpiryVal,
                "max_uses_display": func() string {
                    if maxUses == 999999 || maxUses == 0 {
                        return "unlimited"
                    }
                    return fmt.Sprintf("%d", maxUses)
                }(),
            })
        }
        
        // Stats
        stats := map[string]interface{}{
            "total":      len(passwords),
            "active":     0,
            "expired":    0,
            "no_expiry":  0,
        }
        
        for _, pwd := range passwords {
            if expiresAtStr, ok := pwd["expires_at"].(string); !ok || expiresAtStr == "" {
                stats["no_expiry"] = stats["no_expiry"].(int) + 1
            } else if pwd["expired"].(bool) {
                stats["expired"] = stats["expired"].(int) + 1
            } else {
                stats["active"] = stats["active"].(int) + 1
            }
        }
        
        // ✅ Add user info to response
        response := map[string]interface{}{
            "success":   true,
            "passwords": passwords,
            "stats":     stats,
            "user_info": map[string]interface{}{
                "username":     currentUser,
                "effective_role": userEffectiveRole,
                "can_see_all":   userEffectiveRole == "owner",
                "total_in_system": p.getTotalPasswordsCount(), // opcionális
            },
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
        
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// ✅ Helper függvény az összes jelszó számához
func (p *YnMApiPlugin) getTotalPasswordsCount() int {
    db, err := p.getDB()
    if err != nil || db == nil {
        return 0
    }
    var count int
    if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE pass IS NOT NULL").Scan(&count); err != nil {
        return 0
    }
    return count
}

func (p *YnMApiPlugin) handleDatabaseStats(w http.ResponseWriter, r *http.Request) {
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}
    
    var stats struct {
        Active    int `json:"active"`
        Expired   int `json:"expired"`
        TotalUses int `json:"total_uses"`
        AvgUses   float64 `json:"avg_uses"`
    }
    
    row := db.QueryRow(`
        SELECT 
            COUNT(CASE WHEN strftime('%s', u.password_expires) >= strftime('%s', 'now') THEN 1 END) as active,
            COUNT(CASE WHEN strftime('%s', u.password_expires) < strftime('%s', 'now') THEN 1 END) as expired,
            COALESCE(SUM(u.password_uses), 0) as total_uses,
            COALESCE(ROUND(AVG(u.password_uses), 1), 0) as avg_uses
        FROM users u
        WHERE u.pass IS NOT NULL
    `)
    
    if err := row.Scan(&stats.Active, &stats.Expired, &stats.TotalUses, &stats.AvgUses); err != nil {
        http.Error(w, "Database error: " + err.Error(), http.StatusInternalServerError)
        return
    }
    
    response := map[string]interface{}{
        "success": true,
        "stats":   stats,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Handle password generation
func (p *YnMApiPlugin) handleDatabaseGenerate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        Username  string `json:"username"`
        ExpiresIn int    `json:"expires_in"`
        MaxUses   int    `json:"max_uses"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if req.Username == "" {
        http.Error(w, "Username required", http.StatusBadRequest)
        return
    }
    
    // Alapértelmezett értékek
    if req.ExpiresIn <= 0 {
        req.ExpiresIn = 30
    }
    
    // ✅ SIMPLIFIED: Elfogadjuk a maxUses-t, negatív esetén korrigáljuk
    if req.MaxUses < 0 {
        req.MaxUses = 10 // alapértelmezett
    }
    
    // Jelszó generálás
    password := p.generateSecurePassword(12)
    expiresAt := time.Now().Add(time.Duration(req.ExpiresIn) * time.Minute)
    
    // ✅ Unlimited (0) esetén 999999-re állítjuk
    maxUsesForDB := req.MaxUses
    if maxUsesForDB == 0 {
        maxUsesForDB = 999999 // unlimited
    }
    
    // Mentés memóriába
    p.mutex.Lock()
    p.passwords[req.Username] = &PasswordEntry{
        Username:    req.Username,
        Password:    password,
        CreatedAt:   time.Now(),
        ExpiresAt:   expiresAt,
        UsedCount:   0,
        MaxUses:     maxUsesForDB,
        RequestedBy: "web_admin",
    }
    p.mutex.Unlock()
    
    // Mentés adatbázisba - USERS táblába!
		db, err := p.getDB()
		if err == nil && db != nil {
			_, err = db.Exec(`
				UPDATE users SET 
					pass = ?,
					password_expires = ?,
					password_max_uses = ?,
					password_uses = 0,
					password_created = CURRENT_TIMESTAMP
				WHERE nick = ?
			`, password, expiresAt.Format("2006-01-02 15:04:05"),
				maxUsesForDB, req.Username)

			if err != nil {
				fmt.Printf("[YnMApI] Failed to save to database: %v\n", err)
				http.Error(w, "Failed to save password: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Database not available", http.StatusServiceUnavailable)
			return
		}
    
    response := map[string]interface{}{
        "success":     true,
        "username":    req.Username,
        "password":    password,
        "expires_in":  req.ExpiresIn,
        "max_uses":    req.MaxUses, // az eredeti (0 ha unlimited)
        "expires_at":  expiresAt.Format("2006-01-02 15:04:05"),
        "max_uses_display": func() string {
            if req.MaxUses == 0 {
                return "unlimited"
            }
            return fmt.Sprintf("%d", req.MaxUses)
        }(),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}