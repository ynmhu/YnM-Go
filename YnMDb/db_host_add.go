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
	"strings"
	"fmt"
		"database/sql"
)

func (a *AdminDB) AddUserHost(nick, newHost string) error {
	normalizedNewHost := normalizeHostmask(newHost)
	var currentHostmask string
	err := a.db.QueryRow("SELECT hostmask FROM users WHERE nick=?", nick).Scan(&currentHostmask)
	if err != nil {
		return fmt.Errorf("felhasználó nem található: %s", nick)
	}
	hosts := strings.Split(currentHostmask, ",")
	for i, h := range hosts {
		hosts[i] = strings.TrimSpace(h)
		if hosts[i] == normalizedNewHost {
			return fmt.Errorf("hostmask már hozzá van adva: %s", newHost)
		}
	}

	hosts = append(hosts, normalizedNewHost)
	newHostmask := strings.Join(hosts, ",")
	_, err = a.db.Exec("UPDATE users SET hostmask=? WHERE nick=?", newHostmask, nick)
	if err != nil {
		return fmt.Errorf("adatbázis hiba: %v", err)
	}
	_, err = a.db.Exec("UPDATE channel_users SET hostmask=? WHERE LOWER(nick)=LOWER(?)", newHostmask, nick)
	if err != nil && err != sql.ErrNoRows {

		fmt.Printf("[INFO] channel_users update warning: %v\n", err)
	}

	var role string
	err = a.db.QueryRow("SELECT role FROM users WHERE LOWER(nick)=LOWER(?)", nick).Scan(&role)
	if err == nil && role == "owner" {
		_, err = a.db.Exec("UPDATE channels SET owner_hostmask=? WHERE LOWER(owner)=LOWER(?)", newHostmask, nick)
		if err != nil && err != sql.ErrNoRows {
			fmt.Printf("[INFO] channels owner_hostmask update warning: %v\n", err)
		}
	}
	
	return nil
}

func (a *AdminDB) DelUserHost(nick, delHost string) error {
    // 1. Normalizáljuk a törölni kívánt hostmask-ot
    normalizedDelHost := normalizeHostmask(delHost)
    
    // 2. Lekérdezzük a jelenlegi hostmask-ot
    var hostmask string
    err := a.db.QueryRow("SELECT hostmask FROM users WHERE LOWER(nick)=LOWER(?)", nick).Scan(&hostmask)
    if err != nil {
        return fmt.Errorf("felhasználó nem található: %s", nick)
    }
    
    // 3. Szétválasztjuk a hostokat
    hosts := strings.Split(hostmask, ",")
    for i, h := range hosts {
        hosts[i] = strings.TrimSpace(h)
    }
    
    // 4. Ellenőrizzük hány host van
    if len(hosts) == 1 {
        return fmt.Errorf("csak 1 hostmask van (%s). Először adj hozzá új hostmask-ot a !addhost paranccsal!", hosts[0])
    }
    
    // 5. Megkeressük a törölni kívánt hostot
    newHosts := []string{}
    found := false
    
    for _, h := range hosts {
        if h == normalizedDelHost {
            found = true
            continue // Kihagyjuk
        }
        newHosts = append(newHosts, h)
    }
    
    if !found {
        // Ha nem találta, írjuk ki mely hostok vannak
        availableHosts := make([]string, len(hosts))
        for i, h := range hosts {
            availableHosts[i] = fmt.Sprintf("'%s'", h)
        }
        
        return fmt.Errorf("a megadott hostmask (%s) nem található a listában. Elérhető hostmask-ok: %s",
            delHost, strings.Join(availableHosts, ", "))
    }
    
    // 6. Ellenőrizzük, hogy maradt-e legalább 1 host
    if len(newHosts) == 0 {
        return fmt.Errorf("nem törölheted az utolsó hostmask-ot")
    }
    
    newHostmask := strings.Join(newHosts, ",")
    
    // 7. Frissítjük a users táblát
    result, err := a.db.Exec("UPDATE users SET hostmask=? WHERE LOWER(nick)=LOWER(?)", newHostmask, nick)
    if err != nil {
        return fmt.Errorf("adatbázis hiba: %v", err)
    }
    
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        return fmt.Errorf("nem sikerült frissíteni a felhasználót")
    }
    
    // 8. Fontos: Frissítsük a channel_users táblát is!
    _, err = a.db.Exec(`UPDATE channel_users SET hostmask = ? WHERE LOWER(nick) = LOWER(?)`, newHostmask, nick)
    if err != nil && err != sql.ErrNoRows {
        // Nem dobunk hibát, csak logoljuk
        fmt.Printf("[INFO] channel_users update warning: %v\n", err)
    }
    
    // 9. Extra: Ha owner, frissítsük a channels táblát is
    var role string
    roleErr := a.db.QueryRow("SELECT role FROM users WHERE LOWER(nick)=LOWER(?)", nick).Scan(&role)
    if roleErr == nil && role == "owner" {
        _, err = a.db.Exec(`UPDATE channels SET owner_hostmask = ? WHERE LOWER(owner) = LOWER(?)`, newHostmask, nick)
        if err != nil && err != sql.ErrNoRows {
            fmt.Printf("[INFO] channels owner_hostmask update warning: %v\n", err)
        }
    } else if roleErr != nil && roleErr != sql.ErrNoRows {
        fmt.Printf("[INFO] Could not get role for user %s: %v\n", nick, roleErr)
    }
    
    return nil
}
