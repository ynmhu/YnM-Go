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
	"fmt"
	"strings"
	"database/sql"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

func normalizeChannelName(channel string) string {
    return strings.TrimSpace(channel)
}
func (a *AdminDB) EnsureConsoleChannel(name, owner, ownerHostmask string) error {
    _, err := a.GetChannel(name)
    if err == sql.ErrNoRows || err != nil {
        // Itt is 4 paraméter kell!
        err := a.AddChannel(name, owner, ownerHostmask, ownerHostmask)
        if err != nil {
            return fmt.Errorf("nem sikerült létrehozni a ConsoleChannel-t: %v", err)
        }
    }
    return nil
}


func (db *AdminDB) AssignownerIfMissing(channel string, nick string, hostmask string) error {
	channel = normalizeChannelName(channel) 
	var currentowner sql.NullString
	err := db.db.QueryRow(`SELECT owner FROM channels WHERE name = ?`, channel).Scan(&currentowner)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = db.db.Exec(`
				INSERT INTO channels (name, owner, owner_hostmask, created_at)
				VALUES (?, ?, ?, datetime('now'))
			`, channel, nick, hostmask)
			return err
		}
		return err
	}
	if currentowner.Valid && currentowner.String != "" {
		return nil
	}
	_, err = db.db.Exec(`UPDATE channels SET owner = ?, owner_hostmask = ? WHERE name = ?`, nick, hostmask, channel)
	return err
}


func (a *AdminDB) AddChannel(name, owner, hostmask, addedByHost string) error {
    // 1. Tranzakció indítása
    tx, err := a.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // Biztonság
    
    result, err := tx.Exec(`
        INSERT INTO channels 
        (name, owner, owner_hostmask, auto_op, auto_voice, auto_halfop, created_at) 
        VALUES (?, ?, ?, 0, 0, 0, datetime('now'))
    `, strings.ToLower(name), owner, hostmask)
    if err != nil {
        return err
    }
    
    _, err = result.LastInsertId()
    if err != nil {
        return err
    }

    _, err = tx.Exec(`
        INSERT INTO channel_users 
        (nick, hostmask, channel, role, auto_op, auto_voice, auto_halfop, added_by, added_by_host, created_at)
        VALUES (?, ?, ?, 'owner', 1, 0, 0, 'YnM-Go', ?, datetime('now'))
    `, owner, hostmask, strings.ToLower(name), addedByHost)
    if err != nil {
        return err
    }
    
    // 5. Tranzakció véglegesítése
    return tx.Commit()
}

func (a *AdminDB) RemoveChannel(channel string) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM channels WHERE LOWER(name) = LOWER(?)", channel)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.Exec("DELETE FROM channel_users WHERE LOWER(channel) = LOWER(?)", channel)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM channel_modes WHERE LOWER(channel) = LOWER(?)", channel)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (a *AdminDB) GetAllChannels() ([]string, error) {
	rows, err := a.db.Query("SELECT name FROM channels")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		channels = append(channels, name)
	}
	return channels, nil
}

func (a *AdminDB) GetUserGlobalRole(nick, hostmask string) (string, error) {
    var role string

    query := `
        SELECT role FROM users 
        WHERE LOWER(nick) = LOWER(?) OR hostmask = ? OR hostmask = ?
        LIMIT 1`
    
    simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
    err := a.db.QueryRow(query, nick, hostmask, simplifiedHostmask).Scan(&role)
    
    if err == sql.ErrNoRows {
        return "", nil
    }
    if err != nil {
        return "", err
    }
    return role, nil
}

func (a *AdminDB) IsChannelownerByHost(hostmask, channel string) (bool, error) {
	var count int
	err := a.db.QueryRow(`
		SELECT COUNT(*) FROM channel_users
		WHERE LOWER(name) = LOWER(?) AND owner_hostmask = ?
	`, strings.ToLower(channel), hostmask).Scan(&count)
	return count > 0, err
}

func (a *AdminDB) GetUserPermissionInChannelByHost(hostmask, channel string) (string, error) {
    channel = strings.ToLower(channel)
    hostmask = strings.ToLower(hostmask)

    // First check channel ownership
    var isowner bool
    err := a.db.QueryRow(`
        SELECT 1 FROM channels 
        WHERE LOWER(name) = ? AND LOWER(owner_hostmask) = ?`,
        channel, hostmask).Scan(&isowner)
    if err != nil && err != sql.ErrNoRows {
        return "", err
    }
    if isowner {
        return "owner", nil
    }

    var role string
    err = a.db.QueryRow(`
        SELECT ucr.role 
        FROM channel_users  ucr
        JOIN users u ON u.id = ucr.user_id
        JOIN channels c ON c.id = ucr.channel_id
        WHERE LOWER(u.hostmask) = ? AND LOWER(c.name) = ?`,
        hostmask, channel).Scan(&role)

    if err != nil {
        if err == sql.ErrNoRows {
            // Fallback to checking basic channel permissions
            err = a.db.QueryRow(`
                SELECT CASE WHEN LOWER(c.owner_hostmask) = LOWER(?) THEN 'owner' ELSE 'user' END as role
                FROM channels c
                WHERE LOWER(c.name) = ?
                LIMIT 1
            `, hostmask, channel).Scan(&role)
            
            if err != nil {
                if err == sql.ErrNoRows {
                    return "none", nil
                }
                return "", err
            }
            return role, nil
        }
        return "", err
    }
    return role, nil
}
