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
package YnMDb

import (
	"database/sql"
	"strings"
	"fmt"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type UserRoleWithModes struct {
    Role       string
    AutoOp     bool
    AutoVoice  bool
    AutoHalfop bool
}
// normalizeHostmask helper függvény
func normalizeHostmask(hostmask string) string {
	if strings.Contains(hostmask, "@") {
		parts := strings.Split(hostmask, "@")
		if len(parts) == 2 {
			return "*!*@" + parts[1]
		}
	}
	if strings.HasPrefix(hostmask, "*!*@") {
		return hostmask
	}
	return "*!*@" + hostmask
}

func (a *AdminDB) GetUserRoleInChannel(nick, hostmask, channel string) (string, error) {
    
    // ✅ 1. CACHE ELLENŐRZÉS
    if cachedRole, found := a.getCachedRole(nick, channel); found {
        return cachedRole, nil
    }
    
    normalizedHost := normalizeHostmask(hostmask)
    
    // ========== SZERVER HOSTMASKEK KISZŰRÉSE ==========
    if YnMModule.IsServerHostmask(normalizedHost) {
        return "", nil
    }
    
    if YnMModule.ShouldDebugForHostmask(normalizedHost) {
        fmt.Printf("[DEBUG GetUserRoleInChannel] Called: nick=%s, host=%s, channel=%s\n", 
            nick, normalizedHost, channel)
    }
    
    var roles []string
    
    // ========== 2. GLOBÁLIS ROLE (users tábla) ==========
    queryGlobal := `
        SELECT role
        FROM users
        WHERE hostmask = ? 
           OR hostmask LIKE ? 
           OR hostmask LIKE ?
           OR hostmask LIKE ?
        LIMIT 1`
    
    pattern1 := normalizedHost + ",%"
    pattern2 := "%," + normalizedHost
    pattern3 := "%," + normalizedHost + ",%"
    
    var globalRole string
    err := a.db.QueryRow(queryGlobal, normalizedHost, 
        pattern1, pattern2, pattern3).Scan(&globalRole)
    
    if err == nil && globalRole != "" {
        roles = append(roles, strings.ToLower(globalRole))
        if YnMModule.ShouldDebugForHostmask(normalizedHost) {
            fmt.Printf("[DEBUG] Found global role: %s for hostmask: %s\n", globalRole, normalizedHost)
        }
    } else if err == sql.ErrNoRows {
        if YnMModule.ShouldDebugForHostmask(normalizedHost) {
            fmt.Printf("[DEBUG] No global role found for hostmask: %s\n", normalizedHost)
        }
    } else if err != nil {
        fmt.Printf("[DEBUG] Error getting global role: %v\n", err)
    }
    
    // ========== 3. LOKÁLIS ROLE (channel_users tábla) ==========
    var localRole string
    
    if channel != "" {
        queryChannel := `
            SELECT role
            FROM channel_users
            WHERE LOWER(channel) = LOWER(?)
              AND (hostmask = ? 
                   OR hostmask LIKE ? 
                   OR hostmask LIKE ? 
                   OR hostmask LIKE ?)
            LIMIT 1`
        
        err := a.db.QueryRow(queryChannel, channel, normalizedHost, 
            pattern1, pattern2, pattern3).Scan(&localRole)
        
        if err == nil && localRole != "" {
            roles = append(roles, strings.ToLower(localRole))
            if YnMModule.ShouldDebugForHostmask(normalizedHost) {
                fmt.Printf("[DEBUG] Found local role: %s in channel %s\n", localRole, channel)
            }
        } else if err == sql.ErrNoRows {
            if YnMModule.ShouldDebugForHostmask(normalizedHost) {
                fmt.Printf("[DEBUG] No local role found in channel %s\n", channel)
            }
        }
    } else {
        queryAllChannels := `
            SELECT role
            FROM channel_users
            WHERE hostmask = ? 
               OR hostmask LIKE ? 
               OR hostmask LIKE ?
               OR hostmask LIKE ?
            ORDER BY CASE role 
                WHEN 'owner' THEN 5
                WHEN 'admin' THEN 4
                WHEN 'mod' THEN 3
                WHEN 'vip' THEN 2
                ELSE 1
            END DESC
            LIMIT 1`
        
        err := a.db.QueryRow(queryAllChannels, normalizedHost, 
            pattern1, pattern2, pattern3).Scan(&localRole)
        
        if err == nil && localRole != "" {
            roles = append(roles, strings.ToLower(localRole))
            if YnMModule.ShouldDebugForHostmask(normalizedHost) {
                fmt.Printf("[DEBUG] Found local role from ANY channel: %s\n", localRole)
            }
        } else if err == sql.ErrNoRows {
            if YnMModule.ShouldDebugForHostmask(normalizedHost) {
                fmt.Printf("[DEBUG] No local role found in ANY channel\n")
            }
        }
    }
    
    // ========== 4. LEGMAGASABB ROLE KIVÁLASZTÁSA ==========
    if YnMModule.ShouldDebugForHostmask(normalizedHost) {
        fmt.Printf("[DEBUG] All found roles: %v\n", roles)
    }
    
    if len(roles) == 0 {
        if YnMModule.ShouldDebugForHostmask(normalizedHost) {
            fmt.Printf("[DEBUG] No roles found, returning empty\n")
        }
        // ✅ CACHE ÜRES EREDMÉNY IS (hogy ne kelljen mindig query-zni)
        a.setCachedRole(nick, channel, "")
        return "", nil
    }
    
    roleHierarchy := map[string]int{
        "owner": 5,
        "admin": 4,
        "mod":   3,
        "vip":   2,
        "user":  1,
        "":      0,
    }
    
    highestRole := ""
    highestLevel := -1
    
    for _, role := range roles {
        level, exists := roleHierarchy[strings.ToLower(role)]
        if !exists {
            continue
        }
        
        if level > highestLevel {
            highestLevel = level
            highestRole = role
        }
    }
    
    if highestRole == "" {
        a.setCachedRole(nick, channel, "")
        return "", nil
    }
    
    if YnMModule.ShouldDebugForHostmask(normalizedHost) {
        fmt.Printf("[DEBUG] Returning highest role: %s (level: %d)\n", highestRole, highestLevel)
    }
    
    // ✅ CACHE SIKERES EREDMÉNY
    a.setCachedRole(nick, channel, highestRole)
    
    return highestRole, nil
}
func (a *AdminDB) GetUserRoleWithModes(nick, hostmask, channel string) (*UserRoleWithModes, error) {
    // 1. Először role lekérdezése (a meglévő logikával)
    role, err := a.GetUserRoleInChannel(nick, hostmask, channel)
    if err != nil || role == "" {
        return nil, err
    }
    
    // 2. Auto mode-ok lekérdezése
    simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
    
    query := `SELECT auto_op, auto_voice, auto_halfop 
              FROM channel_users 
              WHERE hostmask = ? AND channel = ? 
              LIMIT 1`
    
    var autoOp, autoVoice, autoHalfop int
    err = a.db.QueryRow(query, simplifiedHostmask, channel).Scan(
        &autoOp, &autoVoice, &autoHalfop)
    
    // Ha nincs rekord, akkor false-ok
    if err != nil {
        autoOp, autoVoice, autoHalfop = 0, 0, 0
    }
    
    return &UserRoleWithModes{
        Role:       role,
        AutoOp:     autoOp == 1,
        AutoVoice:  autoVoice == 1,
        AutoHalfop: autoHalfop == 1,
    }, nil
}
func (a *AdminDB) GetUserPermissionInChannel(nick, channel string) (string, error) {
	query := `
		SELECT CASE WHEN owner = ? THEN 'owner' ELSE 'user' END as role
		FROM channels
		WHERE LOWER(name) = LOWER(?)
		LIMIT 1
	`
	var role string
	err := a.db.QueryRow(query, nick, strings.ToLower(channel)).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

// UpdateUserRole - JAVÍTOTT verzió
func (db *AdminDB) UpdateUserRole(nick, hostmask, role string) error {
	normalizedHost := normalizeHostmask(hostmask)
	
	// Először pontos egyezéssel próbáljuk
	result, err := db.db.Exec(`UPDATE users SET role = ? WHERE LOWER(nick) = LOWER(?) AND hostmask = ?`, role, nick, normalizedHost)
	
	if err != nil {
		return err
	}
	
	// Ha nem talált semmit, próbáljuk LIKE-al
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		_, err = db.db.Exec(`
			UPDATE users 
			SET role = ? 
			WHERE LOWER(nick) = LOWER(?) 
			  AND (hostmask LIKE ? OR hostmask LIKE ?)`,
			role, nick, normalizedHost+",%", "%,"+normalizedHost+"%")
	}
	
	return err
}

func (a *AdminDB) IsUserInChannel(nick, channel string) (bool, error) {
	channel = strings.ToLower(channel)
	nick = strings.ToLower(nick)
	var count int
	err := a.db.QueryRow(`
		SELECT COUNT(*) 
		FROM channel_users 
		WHERE LOWER(nick) = ? AND LOWER(channel) = ?`,
		nick, channel).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetUserVipAutoVoice - JAVÍTOTT verzió
func (db *AdminDB) SetUserVipAutoVoice(nick, hostmask, channel string) error {
	normalizedHost := normalizeHostmask(hostmask)
	
	// Először megpróbáljuk pontos egyezéssel
	result, err := db.db.Exec(`
		UPDATE channel_users
		SET auto_op = 0, auto_voice = 1, auto_halfop = 0
		WHERE nick = ? AND channel = ? AND hostmask = ?`,
		nick, channel, normalizedHost)
	
	if err != nil {
		return err
	}
	
	// Ha nem talált semmit, próbáljuk LIKE-al
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		_, err = db.db.Exec(`
			UPDATE channel_users
			SET auto_op = 0, auto_voice = 1, auto_halfop = 0
			WHERE nick = ? AND channel = ? AND (hostmask LIKE ? OR hostmask LIKE ?)`,
			nick, channel, normalizedHost+",%", "%,"+normalizedHost+"%")
	}
	
	return err
}

// GetUserHostmaskInChannel - JAVÍTOTT verzió
func (a *AdminDB) GetUserHostmaskInChannel(nick, channel string) (string, error) {
	var hostmask string
	err := a.db.QueryRow(`SELECT hostmask FROM channel_users WHERE LOWER(nick)=LOWER(?) AND LOWER(channel)=LOWER(?)`, nick, channel).Scan(&hostmask)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hostmask, err
}

// RemoveUserFromChannel - JAVÍTOTT verzió
func (a *AdminDB) RemoveUserFromChannel(channel, nick, hostmask string) error {
	// 1. Lekérdezzük, hogy a user owner-e
	var role string
	err := a.db.QueryRow(`SELECT role FROM users WHERE nick = ?`, nick).Scan(&role)
	if err != nil {
		return err
	}
	
	// 2. Ha owner, nem töröljük
	if role == "owner" {
		return fmt.Errorf("az 'owner' felhasználót nem lehet eltávolítani a csatornáról")
	}
	
	normalizedHost := normalizeHostmask(hostmask)
	
	// 3. Ha nem owner → törlés mehet (többszörös host támogatással)
	_, err = a.db.Exec(`
		DELETE FROM channel_users
		WHERE LOWER(channel)=LOWER(?) 
		  AND LOWER(nick)=LOWER(?) 
		  AND (hostmask = ? OR hostmask LIKE ? OR hostmask LIKE ?)
	`, channel, nick, normalizedHost, normalizedHost+",%", "%,"+normalizedHost+"%")
	
	return err
}

// DeleteUser - JAVÍTOTT verzió
func (a *AdminDB) DeleteUser(nick string) error {
	// Első lépés: lekérdezzük a user szerepét
	var role string
	err := a.db.QueryRow(`SELECT role FROM users WHERE nick = ?`, nick).Scan(&role)
	if err != nil {
		return err
	}
	
	// Ha owner, akkor nem töröljük
	if role == "owner" {
		return fmt.Errorf("az 'owner' felhasználót nem lehet törölni")
	}
	
	// Ha nem owner, töröljük
	query := `DELETE FROM users WHERE nick = ?`
	_, err = a.db.Exec(query, nick)
	return err
}

// GetAnyownerNick - JAVÍTOTT verzió
func (a *AdminDB) GetAnyownerNick() (string, error) {
	var nick string
	err := a.db.QueryRow(`SELECT nick FROM users WHERE role='owner' LIMIT 1`).Scan(&nick)
	if err != nil {
		return "", err
	}
	return nick, nil
}

func (a *AdminDB) UpdateUserGlobalRole(nick, hostmask, newRole, addedBy string) error {
    normalizedHost := normalizeHostmask(hostmask)
    
    fmt.Printf("[DEBUG UpdateUserGlobalRole] nick=%s, hostmask=%s, newRole=%s\n", 
        nick, normalizedHost, newRole)
    
    // ✅ 1. ELLENŐRZÉS: owner-t nem degradálunk!
    var currentRole string
    queryCurrent := `SELECT role FROM users WHERE hostmask = ? LIMIT 1`
    err := a.db.QueryRow(queryCurrent, normalizedHost).Scan(&currentRole)
    
    if err == nil && strings.ToLower(currentRole) == "owner" {
        return nil
    }
	// Először pontos egyezéssel próbáljuk
    result, err := a.db.Exec(`
        UPDATE users 
        SET role = ?, 
            added_by = ?
        WHERE LOWER(nick) = LOWER(?) AND hostmask = ?
    `, newRole, addedBy, nick, normalizedHost)
	
	if err != nil {
		return err
	}
	
	// Ha nem talált semmit, próbáljuk LIKE-al
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		result, err = a.db.Exec(`
			UPDATE users 
			SET role = ? 
			WHERE LOWER(nick) = LOWER(?) 
			  AND (hostmask LIKE ? OR hostmask LIKE ? OR hostmask LIKE ?)
		`, newRole, nick, normalizedHost+",%", "%,"+normalizedHost, "%,"+normalizedHost+",%")
		
		if err != nil {
			return err
		}
		
		rowsAffected, _ = result.RowsAffected()
		if rowsAffected == 0 {
			return fmt.Errorf("nem található felhasználó: %s (%s)", nick, normalizedHost)
		}
	}
	
	fmt.Printf("[DEBUG UpdateUserGlobalRole] Updated %d row(s)\n", rowsAffected)
	return nil
}
