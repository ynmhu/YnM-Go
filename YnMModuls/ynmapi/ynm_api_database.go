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
    "strings"
	"log"
	"fmt"
	"time"
)

func (p *YnMApiPlugin) initializeDatabase() {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        if p.adminPlugin != nil && p.adminPlugin.Db != nil && p.adminPlugin.Db.SQL != nil {

            p.dbMutex.Lock()
            p.db = p.adminPlugin.Db.SQL
            p.dbReady = true
            p.dbMutex.Unlock()

            if err := p.ensureWebAuthTables(); err != nil {
                log.Printf("[YnMApI] Error creating tables: %v", err)
            }

            p.loadActivePasswordsFromDB()
            go p.statusUpdateLoop()

            fmt.Println("[YnMApI] Database ready and linked with AdminDB.")
            return
        }

        fmt.Printf("[YnMApI] Waiting for admin database (attempt %d/%d)...\n", i+1, maxRetries)
        time.Sleep(60 * time.Second)
    }
    fmt.Println("[YnMApI] WARNING: Database not available after retries.")
}

func (p *YnMApiPlugin) isDatabaseReady() bool {
    p.dbMutex.RLock()
    defer p.dbMutex.RUnlock()
    return p.dbReady && p.db != nil && p.adminPlugin != nil && p.adminPlugin.Db != nil
}

func (p *YnMApiPlugin) getDB() (*sql.DB, error) {
    // ha az admin db már elérhető, add vissza
    if p.adminPlugin != nil && p.adminPlugin.Db != nil && p.adminPlugin.Db.SQL != nil {
        return p.adminPlugin.Db.SQL, nil
    }

    p.dbMutex.RLock()
    db := p.db
    p.dbMutex.RUnlock()
    if db == nil {
        return nil, fmt.Errorf("admin db not ready")
    }
    return db, nil
}

func (p *YnMApiPlugin) ensureWebAuthTables() error {
    db, err := p.getDB()
    if err != nil {
        return err
    }

    queries := []string{
        `CREATE TABLE IF NOT EXISTS web_sessions (
            token TEXT PRIMARY KEY,
            username TEXT NOT NULL,
            created_at DATETIME NOT NULL,
            expires_at DATETIME NOT NULL,
            ip_address TEXT,
            user_agent TEXT
        )`,
        `CREATE TABLE IF NOT EXISTS web_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT NOT NULL,
            action TEXT NOT NULL,
            ip_address TEXT,
            timestamp DATETIME NOT NULL,
            details TEXT
        )`,
    }
    
    for _, query := range queries {
        if _, err := db.Exec(query); err != nil {
            return fmt.Errorf("failed to create table: %w", err)
        }
    }
    
    // ✅ Ellenőrizzük a users tábla password oszlopait
    p.ensureUserPasswordColumns(db)
   
    // Try to add last_login column (ignore if it already exists)
    _, err = db.Exec(`ALTER TABLE users ADD COLUMN last_login DATETIME`)
    if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
        fmt.Printf("[YnMApI] Warning adding last_login column: %v\n", err)
    }
    
    return nil
}

func (p *YnMApiPlugin) ensureUserPasswordColumns(db *sql.DB) {
    var hasPasswordMaxUses bool
    err := db.QueryRow(`
        SELECT COUNT(*) > 0
        FROM pragma_table_info('users')
        WHERE name = 'password_max_uses'
    `).Scan(&hasPasswordMaxUses)
    if err != nil {
        fmt.Printf("[YnMApI] Error checking table columns: %v\n", err)
        return
    }

    if !hasPasswordMaxUses {
        fmt.Println("[YnMApI] Adding missing password_max_uses column to users table...")
        _, err := db.Exec(`ALTER TABLE users ADD COLUMN password_max_uses INTEGER DEFAULT 10`)
        if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
            fmt.Printf("[YnMApI] Failed to add password_max_uses column: %v\n", err)
        } else {
            fmt.Println("[YnMApI] password_max_uses column added successfully")
        }
    }

    columnsToCheck := []string{"password_expires", "password_uses", "password_last_used", "password_created"}
    for _, column := range columnsToCheck {
        var hasColumn bool
        err := db.QueryRow(`
            SELECT COUNT(*) > 0
            FROM pragma_table_info('users')
            WHERE name = ?
        `, column).Scan(&hasColumn)

        if err == nil && !hasColumn {
            columnType := "DATETIME"
            if column == "password_uses" {
                columnType = "INTEGER DEFAULT 0"
            }

            fmt.Printf("[YnMApI] Adding missing %s column to users table...\n", column)
            query := fmt.Sprintf("ALTER TABLE users ADD COLUMN %s %s", column, columnType)
            _, err := db.Exec(query)
            if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
                fmt.Printf("[YnMApI] Failed to add %s column: %v\n", column, err)
            }
        }
    }
}

func (p *YnMApiPlugin) loadActivePasswordsFromDB() {
	db, err := p.getDB()
	if err != nil {
		return
	}
    
    rows, err := db.Query(`
        SELECT 
            nick,
            pass, 
            password_expires,
            COALESCE(password_uses, 0) as password_uses,
            COALESCE(password_max_uses, 10) as password_max_uses,
            password_created
        FROM users 
        WHERE pass IS NOT NULL 
          AND (password_expires IS NULL OR password_expires > datetime('now'))
            AND (
                password_uses < COALESCE(password_max_uses, 10) 
                OR password_uses IS NULL 
                OR password_max_uses = 999999  -- ⬅️ UNLIMITED
                OR password_max_uses = 0       -- ⬅️ UNLIMITED
            )
        ORDER BY nick, password_created DESC
    `)
    
    if err != nil {
        fmt.Printf("[YnMApI] Failed to load passwords from DB: %v\n", err)
        return
    }
    defer rows.Close()
    
    p.mutex.Lock()
    defer p.mutex.Unlock()
    
    // Töröljük a régi adatokat
    p.passwords = make(map[string]*PasswordEntry)
    
    loaded := 0
    
    for rows.Next() {
        var username, password string
        var expiresAtStr, createdAtStr sql.NullString
        var uses, maxUses int
        
        if err := rows.Scan(&username, &password, &expiresAtStr, &uses, &maxUses, &createdAtStr); err != nil {
            fmt.Printf("[YnMApI] Scan error: %v\n", err)
            continue
        }
        
        // Idő értelmezése
        var expiresAt, createdAt time.Time
        var parseErr error
        
        if expiresAtStr.Valid {
            expiresAt, parseErr = time.Parse(time.RFC3339, expiresAtStr.String)
            if parseErr != nil {
                expiresAt, parseErr = time.Parse("2006-01-02 15:04:05", expiresAtStr.String)
                if parseErr != nil {
                    expiresAt, parseErr = time.Parse("2006-01-02T15:04:05Z", expiresAtStr.String)
                }
            }
            if parseErr != nil {
                fmt.Printf("[YnMApI] ExpiresAt parse error for %s: %v\n", username, parseErr)
                continue
            }
        } else {
            // Nincs expire date, használjunk távoli jövőbeli dátumot
            expiresAt = time.Now().AddDate(100, 0, 0)
        }
        
        if createdAtStr.Valid {
            createdAt, parseErr = time.Parse(time.RFC3339, createdAtStr.String)
            if parseErr != nil {
                createdAt, parseErr = time.Parse("2006-01-02 15:04:05", createdAtStr.String)
                if parseErr != nil {
                    createdAt, parseErr = time.Parse("2006-01-02T15:04:05Z", createdAtStr.String)
                }
            }
            if parseErr != nil {
                createdAt = time.Now()
            }
        } else {
            createdAt = time.Now()
        }
        
        // Már létező felhasználó? Ha igen, csak a legújabbat tároljuk
        if existing, exists := p.passwords[username]; exists {
            // Ha ez a jelszó újabb, cseréljük
            if createdAt.After(existing.CreatedAt) {
                p.passwords[username] = &PasswordEntry{
                    Username:    username,
                    Password:    password,
                    CreatedAt:   createdAt,
                    ExpiresAt:   expiresAt,
                    UsedCount:   uses,
                    MaxUses:     maxUses,
                    RequestedBy: "YnM-Go", // Nincs generated_by a users táblában
                }
            }
        } else {
            // Új felhasználó
            p.passwords[username] = &PasswordEntry{
                Username:    username,
                Password:    password,
                CreatedAt:   createdAt,
                ExpiresAt:   expiresAt,
                UsedCount:   uses,
                MaxUses:     maxUses,
                RequestedBy: "YnM-Go",
            }
            loaded++
        }
    }
    
    fmt.Printf("[YnMApI] Loaded %d unique active passwords from database\n", loaded)
}