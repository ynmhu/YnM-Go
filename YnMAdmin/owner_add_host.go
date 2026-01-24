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
package owner

import (
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"

)

func (p *YnmAdminPlugin) HandleAddHost(nick string, args []string, isPrivate bool) {
	fmt.Printf("[DEBUG HandleAddHost] nick=%s, args=%v, isPrivate=%v\n", nick, args, isPrivate)

	if !isPrivate {
		p.Bot.SendMessage(nick, "Host hozzáadása csak privát üzenetben lehetséges. Írj PRIVÁT üzenetben: addhost <host>")
		return
	}

	if len(args) == 0 {
		p.Bot.SendMessage(nick, "Használat: addhost <host>")
		return
	}

	newHost := strings.TrimSpace(args[0])

	// Ellenőrizzük, hogy a host nem üres
	if newHost == "" {
		p.Bot.SendMessage(nick, "❌ A host nem lehet üres!")
		return
	}

	// ⚡ NORMALIZÁLJUK A HOSTOT ⚡
	normalizedHost := newHost
	if !strings.Contains(normalizedHost, "!") && !strings.Contains(normalizedHost, "@") {
		normalizedHost = "*!*@" + normalizedHost
	} else if !strings.HasPrefix(normalizedHost, "*!*@") && strings.Contains(normalizedHost, "@") {
		if !strings.Contains(normalizedHost, "!") {
			normalizedHost = "*!*@" + normalizedHost
		}
	}
	
	fmt.Printf("[DEBUG HandleAddHost] Formatted host: %s (normalized: %s)\n", newHost, normalizedHost)

	// Ellenőrizzük, hogy a felhasználó létezik-e
	exists, err := p.Db.UserExists(nick)
	if err != nil {
		fmt.Printf("[DEBUG] Error checking user existence: %v\n", err)
		p.Bot.SendMessage(nick, "Hiba a felhasználó ellenőrzése közben.")
		return
	}

	if !exists {
		fmt.Printf("[DEBUG] User does not exist: %s\n", nick)
		return
	}

	fmt.Printf("[DEBUG] User exists: %v\n", exists)

	// ⚡ ELŐSZÖR ELLENŐRZÉS: Már benne van-e? ⚡
	var currentHostmask string
	err = p.Db.QueryRow("SELECT hostmask FROM users WHERE LOWER(nick)=LOWER(?)", nick).Scan(&currentHostmask)
	if err == nil {
		hosts := strings.Split(currentHostmask, ",")
		alreadyExists := false
		
		for _, existingHost := range hosts {
			existingHost = strings.TrimSpace(existingHost)
			
			// ⚡ NORMALIZÁLJUK AZ ÖSSZEHASONLÍTÁSHOZ IS! ⚡
			normalizedExisting := existingHost
			if !strings.HasPrefix(existingHost, "*!*@") {
				if strings.Contains(existingHost, "@") {
					parts := strings.Split(existingHost, "@")
					if len(parts) == 2 {
						normalizedExisting = "*!*@" + parts[1]
					}
				} else {
					normalizedExisting = "*!*@" + existingHost
				}
			}
			
			if normalizedExisting == normalizedHost {
				alreadyExists = true
				break
			}
		}
		
		if alreadyExists {
			p.Bot.SendMessage(nick, fmt.Sprintf("ℹ️ Ez a hostmask már hozzá van adva: %s", normalizedHost))
			return
		}
	}

	// Host hozzáadása az adatbázishoz (normalizált hostot adjuk át)
	err = p.Db.AddUserHost(nick, normalizedHost)
	if err != nil {
		fmt.Printf("[DEBUG] Error adding host: %v\n", err)
		p.Bot.SendMessage(nick, "Hiba a host hozzáadása közben az adatbázisba.")
		return
	}

	fmt.Printf("[DEBUG] Host added successfully for user: %s, host: %s\n", nick, normalizedHost)
	p.Bot.SendMessage(nick, fmt.Sprintf("✅ Host sikeresen hozzáadva: %s", normalizedHost))
}


func (p *YnmAdminPlugin) HandleDelHost(issuerHostmask, channel, args string) string {
	if args == "" {
		// Használjuk az issuerHostmask-ot közvetlenül
		effectiveHost := issuerHostmask
		
		// Ha van benne !, akkor kinyerjük csak a host részt
		if strings.Contains(effectiveHost, "!") && strings.Contains(effectiveHost, "@") {
			parts := strings.Split(effectiveHost, "@")
			if len(parts) > 1 {
				effectiveHost = "*!*@" + parts[1]
			}
		}
		
		// Ha nincs argumentum, mutassuk meg melyik hostjai vannak
		var hostmask string
		err := p.Db.QueryRow(`
			SELECT hostmask FROM users 
			WHERE hostmask LIKE ? 
			   OR hostmask LIKE ?
			   OR hostmask LIKE ?
			LIMIT 1`,
			"%"+effectiveHost+"%",
			effectiveHost+",%",
			"%,"+effectiveHost+"%",
		).Scan(&hostmask)
		
		if err != nil {
			return "❌ Nem található hostmask a felhasználónál"
		}
		
		hosts := strings.Split(hostmask, ",")
		
		// ✅ JAVÍTÁS: Használjunk ~~~ szeparátort \n helyett
		lines := []string{"📋 Az Ön hostmask-ai:"}
		
		for i, host := range hosts {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(host)))
		}
		
		lines = append(lines, "")
		lines = append(lines, "Használat: delhost <hostmask>")
		lines = append(lines, "Példák: delhost *!*@host.nev vagy delhost host.nev")
		
		if len(hosts) == 1 {
			lines = append(lines, "⚠️ Csak 1 hostmask van! Először adj hozzá újat a addhost paranccsal.")
		}
		
		return strings.Join(lines, " ")
	}
	
	// Törlés
	normalizedHost := args
	if !strings.Contains(normalizedHost, "!") && !strings.Contains(normalizedHost, "@") {
		normalizedHost = "*!*@" + normalizedHost
	} else if !strings.HasPrefix(normalizedHost, "*!*@") && strings.Contains(normalizedHost, "@") {
		if !strings.Contains(normalizedHost, "!") {
			normalizedHost = "*!*@" + normalizedHost
		}
	}
	
	fmt.Printf("[DEBUG HandleDelHost] Deleting host: %s -> %s\n", args, normalizedHost)
	
	// Megkeressük melyik usernek van ez a hostja
	var userNick string
	err := p.Db.QueryRow(`
		SELECT nick FROM users 
		WHERE hostmask LIKE ? 
		   OR hostmask LIKE ?
		   OR hostmask LIKE ?
		   OR hostmask LIKE ?
		LIMIT 1`,
		normalizedHost,
		normalizedHost+",%",
		"%,"+normalizedHost,
		"%,"+normalizedHost+",%",
	).Scan(&userNick)
	
	if err != nil {
		return fmt.Sprintf("❌ Ez a hostmask nem található egyetlen felhasználónál sem: %s", args)
	}
	
	fmt.Printf("[DEBUG HandleDelHost] Found owner: %s for host: %s\n", userNick, normalizedHost)
	
	// Töröljük a hostot
	err = p.Db.DelUserHost(userNick, normalizedHost)
	
	if err != nil {
		return fmt.Sprintf("❌ %s", err.Error())
	}
	
	return fmt.Sprintf("✅ Host sikeresen törölve: %s", args)
}

// Segédfüggvény host alapú törléshez
func (p *YnmAdminPlugin) deleteHostByHostmask(ownerNick, hostToDelete string) error {
	// Lekérdezzük a jelenlegi hostmask-ot
	var currentHostmask string
	err := p.Db.QueryRow("SELECT hostmask FROM users WHERE LOWER(nick)=LOWER(?)", ownerNick).Scan(&currentHostmask)
	if err != nil {
		return fmt.Errorf("felhasználó nem található")
	}
	
	// Szétválasztjuk a hostokat
	hosts := strings.Split(currentHostmask, ",")
	for i, h := range hosts {
		hosts[i] = strings.TrimSpace(h)
	}
	
	// Ellenőrizzük hány host van
	if len(hosts) == 1 {
		return fmt.Errorf("csak 1 hostmask van (%s). Először adj hozzá új hostmask-ot a addhost paranccsal!", hosts[0])
	}
	
	// Megkeressük a törölni kívánt hostot
	newHosts := []string{}
	found := false
	
	for _, h := range hosts {
		if h == hostToDelete {
			found = true
			continue // Kihagyjuk
		}
		newHosts = append(newHosts, h)
	}
	
	if !found {
		availableHosts := make([]string, len(hosts))
		for i, h := range hosts {
			availableHosts[i] = fmt.Sprintf("'%s'", h)
		}
		
		return fmt.Errorf("a megadott hostmask (%s) nem található a listában. Elérhető hostmask-ok: %s",
			hostToDelete, strings.Join(availableHosts, ", "))
	}
	
	// Ellenőrizzük, hogy maradt-e legalább 1 host
	if len(newHosts) == 0 {
		return fmt.Errorf("nem törölheted az utolsó hostmask-ot")
	}
	
	newHostmask := strings.Join(newHosts, ",")
	
	// Frissítjük a users táblát
	_, err = p.Db.Exec("UPDATE users SET hostmask=? WHERE LOWER(nick)=LOWER(?)", newHostmask, ownerNick)
	if err != nil {
		return fmt.Errorf("adatbázis hiba: %v", err)
	}
	
	// Frissítsük a channel_users táblát is
	_, err = p.Db.Exec("UPDATE channel_users SET hostmask=? WHERE LOWER(nick)=LOWER(?)", newHostmask, ownerNick)
	
	// Ha owner, frissítsük a channels táblát is
	var role string
	p.Db.QueryRow("SELECT role FROM users WHERE LOWER(nick)=LOWER(?)", ownerNick).Scan(&role)
	if role == "owner" {
		p.Db.Exec("UPDATE channels SET owner_hostmask=? WHERE LOWER(owner)=LOWER(?)", newHostmask, ownerNick)
	}
	
	return nil
}