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
	"time"
	"sync"
	"golang.org/x/crypto/bcrypt"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)
type PrivFloodEntry struct {
	messages   []time.Time
	mutedUntil time.Time
}
var (
	privFloodMap = make(map[string]*PrivFloodEntry)
	floodMutex   sync.RWMutex
)
// HashPassword létrehoz egy bcrypt hash-t a jelszóból
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (p *YnmAdminPlugin) HandleSetPass(nick string, hostmask string, args []string, isPrivate bool) {
	fmt.Printf("[DEBUG HandleSetPass] nick=%s, args=%v, isPrivate=%v\n", nick, args, isPrivate)

	// 1. Ellenőrizzük, hogy privát üzenet-e
	if !isPrivate {
		p.Bot.SendMessage(nick, "A jelszó beállítása csak privát üzenetben lehetséges. Írj PRIVÁT üzenetben: setpass <uj_jelszo>")
		return
	}
	
	// 2. Hostmask kezelés és jogosultság ellenőrzés
	fullHostmask := hostmask 
	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
	hasPermission, userRole := p.checkMinimumRole(simplifiedHostmask, "user")

	if !hasPermission {
		// ⚠️ AUDIT LOG: Nincs jogosultság
		p.LogSecurityEvent(
			nick,
			simplifiedHostmask,
			"⛔",
			fmt.Sprintf("SETPASS command denied (role: %s, required: user)", userRole),
		)
		    // Console channel értesítés (hiba nélkül)
    if p.Bot != nil && p.Bot.GetConfig().ConsoleChannel != "" {
        errMsg := fmt.Sprintf("⛔ [PERMISSION] %s %s tried SETPASS without User (role: %s)", 
            nick, simplifiedHostmask, userRole)
        p.Bot.SendMessage(p.Bot.GetConfig().ConsoleChannel, errMsg)
    }
		p.Bot.SendMessage(nick, "❌ Nincs jogod a jelszó beállításához (User szükséges)")
		return
	}
	
	// 3. AUDIT LOG: Sikeres jogosultság-ellenőrzés
	p.LogSecurityEvent(
		nick,
		simplifiedHostmask,
		"✅",
		fmt.Sprintf("User authorized for SETPASS (role: %s)", userRole),
	)
	
	// 4. Ellenőrizzük, hogy van-e paraméter
	if len(args) == 0 {
		p.Bot.SendMessage(nick, "Használat: setpass <uj_jelszo>")
		return
	}

	// 5. Ellenőrizzük, hogy van-e már jelszó
	hasPass, err := p.Db.UserHasPassword(nick)
	
	fmt.Printf("[DEBUG] UserHasPassword('%s') returned: hasPass=%v, err=%v\n", nick, hasPass, err)
	
	if err != nil {
		fmt.Printf("[ERROR] Failed to check password for %s: %v\n", nick, err)
		p.Bot.SendMessage(nick, "Hiba a jelszó ellenőrzése közben.")
		if p.Bot != nil && p.Bot.GetConfig().ConsoleChannel != "" {
            errMsg := fmt.Sprintf("🔴 [DB ERROR] Failed to write bot log: %v", err)
             p.Bot.SendMessage(p.Bot.GetConfig().ConsoleChannel, errMsg)
		}
		
		// AUDIT LOG: Adatbázis hiba
		p.LogSecurityEvent(
			nick,
			simplifiedHostmask,
			"SETPASS_ERROR",
			fmt.Sprintf("Database error while checking password: %v", err),
		)
		return
	}
	
	// 6. Ha már van jelszó, ne engedjük beállítani újra
	if hasPass {
		p.Bot.SendMessage(nick, "❌ Jelszó már létezik!")
		p.Bot.SendMessage(nick, "Használd a 'chgpass' parancsot a módosításhoz.")
		p.Bot.SendMessage(nick, "Vagy a 'forgetpass' parancsot az elfelejtett jelszóhoz.")
		fmt.Printf("[INFO] %s tried to set password but already has one\n", nick)
		
		// AUDIT LOG: Már létező jelszó
		p.LogSecurityEvent(
			nick,
			simplifiedHostmask,
			"SETPASS_DUPLICATE",
			"Attempted to set password when one already exists",
		)
		return
	}

	// 7. Új jelszó beállítása
	newPass := args[0]
	hash, err := HashPassword(newPass)
	if err != nil {
		fmt.Printf("[ERROR] Failed to hash password for %s: %v\n", nick, err)
		p.Bot.SendMessage(nick, "Hiba a jelszó hash-elése közben.")
		
		// AUDIT LOG: Hash hiba
		p.LogSecurityEvent(
			nick,
			simplifiedHostmask,
			"SETPASS_ERROR",
			fmt.Sprintf("Password hashing error: %v", err),
		)
		return
	}

	// 8. Jelszó mentése az adatbázisba
	err = p.Db.SetUserPassword(nick, hash)
	if err != nil {
		fmt.Printf("[ERROR] Failed to save password for %s: %v\n", nick, err)
		p.Bot.SendMessage(nick, "Hiba a jelszó mentése közben.")
		
		// AUDIT LOG: Mentési hiba
		p.LogSecurityEvent(
			nick,
			simplifiedHostmask,
			"SETPASS_ERROR",
			fmt.Sprintf("Database save error: %v", err),
		)
		return
	}

	// 9. ✅ SIKERES beállítás
	p.Bot.SendMessage(nick, "✅ Jelszó sikeresen beállítva.")
	fmt.Printf("[INFO] Password set for user: %s\n", nick)
	
	// 10. AUDIT LOG: SIKERES jelszó beállítás
	p.LogSecurityEvent(
		nick,
		simplifiedHostmask,
		"PASSWORD_SET",
		fmt.Sprintf("Password successfully set (role: %s)", userRole),
	)
}


// ==================================================
// OnPrivMsg módosítása az új parancsokhoz
// ==================================================
func (p *YnmAdminPlugin) OnPrivMsg(nick, target, msg, hostmask string, isPrivate bool) {
    // FLOOD VÉDELEM - minimális változat
    if isPrivate {
        floodMutex.Lock()
        now := time.Now()
        
        entry, exists := privFloodMap[nick]
        if !exists {
            privFloodMap[nick] = &PrivFloodEntry{
                messages: []time.Time{now},
            }
        } else {
            // Mute ellenőrzés
            if !entry.mutedUntil.IsZero() && now.Before(entry.mutedUntil) {
                floodMutex.Unlock()
                return // STOP - flood
            }
            
            // Szűrés
            validMsgs := make([]time.Time, 0)
            for _, t := range entry.messages {
                if now.Sub(t) <= 3*time.Second {
                    validMsgs = append(validMsgs, t)
                }
            }
            entry.messages = validMsgs
            
            // Flood detektálás
            if len(entry.messages) >= 2 {
                entry.mutedUntil = now.Add(30 * time.Second)
                floodMutex.Unlock()
                return // STOP - flood
            }
            
            entry.messages = append(entry.messages, now)
        }
        floodMutex.Unlock()
    }

    parts := strings.Fields(msg)
    if len(parts) == 0 {
        return
    }

    cmd := strings.ToLower(parts[0])
    args := parts[1:]

    switch cmd {
    case "login":
        p.HandleLoginWithHostmask(hostmask, args, isPrivate)
    case "logout":
        p.HandleLogoutWithHostmask(hostmask, isPrivate)
	case "addhost":
	if len(args) > 0 {
		p.HandleAddHost(nick, args, isPrivate)  // új függvény hívása
	} else {
		p.Bot.SendMessage(nick, "Használat: addhost <hostmask>")
	}
	case "delhost":
    if len(args) > 0 {
        // Privát üzenet: nincs csatorna paraméter
        // Alakítsuk át a []string-et string-é
        hostmask := strings.Join(args, " ")  // ← ez a kulcsos változtatás!
        response := p.HandleDelHost(nick, "", hostmask)  // most már string
        p.Bot.SendMessage(strings.Split(nick, "!")[0], response)
    } else {
        // Üres argumentum esetén is hívjuk a HandleDelHost-ot
        response := p.HandleDelHost(nick, "", "")
        p.Bot.SendMessage(strings.Split(nick, "!")[0], response)
    }

    case "sessiondebug":
        p.HandleSessionDebug(nick, isPrivate)
    case "debugsession":
        p.DebugSessionState(nick)
    case "ynm":
        if len(args) == 0 {
            if response := p.handleYnmCommand(hostmask, "ynm"); response != "" {
                p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", nick, response))
            }
        } else {
            subcmd := strings.ToLower(args[0])
            subargs := args[1:]

            switch subcmd {
            case "info":
                if len(subargs) > 0 {
                    if response := p.handleInfoCommand(hostmask, subargs[0]); response != "" {
                        p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", nick, response))
                    }
                } else {
                    if response := p.handleInfoCommand(hostmask); response != "" {
                        p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", nick, response))
                    }
                }
            case "channels":
                if response := p.handleChannelsCommand(hostmask, ""); response != "" {
                    p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", nick, response))
                }
            case "add":
                if len(subargs) > 1 && subargs[0] == "chan" {
                    channel := subargs[1]
                    if response := p.handleAddRoomCommand(hostmask, channel, ""); response != "" {
                        p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", nick, response))
                    }
                } else {
                    p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :Használat: ynm add chan #channel", nick))
                }
            case "del":
                if len(subargs) > 1 && subargs[0] == "chan" {
                    channel := subargs[1]
                    if response := p.handleRemoveRoomCommand(hostmask, channel, ""); response != "" {
                        p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :%s", nick, response))
                    }
                } else {
                    p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :Használat: ynm del chan #channel", nick))
                }
            default:
                p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :Ismeretlen parancs: %s", nick, subcmd))
            }
        }
    case "setpass":
        p.HandleSetPass(nick, hostmask, args, isPrivate)
    case "chgpass":
        p.HandleChgPass(nick, hostmask, args, isPrivate)
    case "forgetpass":
        p.HandleForgetPass(nick, hostmask, isPrivate)
    case "setmail":
        p.HandleSetMail(nick, hostmask, args, isPrivate)
	case "auth":
		p.HandleAuth(nick, hostmask, isPrivate) 
    }
}


// SetUserLoggedIn beállítja a felhasználó bejelentkezett státuszát
func (p *YnmAdminPlugin) SetUserLoggedIn(currentNick, targetNick string, loggedIn bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loggedInUsers == nil {
		p.loggedInUsers = make(map[string]string)
	}

	if loggedIn {
		p.loggedInUsers[currentNick] = targetNick
	} else {
		delete(p.loggedInUsers, currentNick)
	}
}

// GetLoggedInUser visszaadja, hogy egy felhasználó kit impersonál
func (p *YnmAdminPlugin) GetLoggedInUser(nick string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.loggedInUsers == nil {
		return "", false
	}

	targetNick, exists := p.loggedInUsers[nick]
	return targetNick, exists
}

// IsUserLoggedIn ellenőrzi, hogy a felhasználó be van-e jelentkezve
func (p *YnmAdminPlugin) IsUserLoggedIn(nick string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.loggedInUsers != nil && p.loggedInUsers[nick] != ""
}

func (p *YnmAdminPlugin) HandleAuth(nick, hostmask string, isPrivate bool) {
    // Get effective user based on HOSTMASK
    effectiveUser, _ := p.GetEffectiveUser(hostmask)
    role := p.GetUserRoleWithSession(hostmask)
    
    // Az eredeti nick a PARAMÉTERBŐL jön, NEM a hostmask-ból!
    originalNick := nick  // ← Ezt használd a paraméter nick helyett
    
    if effectiveUser != originalNick {
        p.Bot.SendMessage(nick, fmt.Sprintf("Be vagy jelentkezve mint: %s (%s)", effectiveUser, role))
        p.Bot.SendMessage(nick, fmt.Sprintf("Eredeti nick: %s", originalNick))
        
        if session, exists := p.GetSessionByHost(hostmask); exists {
            loginDuration := time.Since(session.LoginTime)
            p.Bot.SendMessage(nick, fmt.Sprintf("Bejelentkezve: %v perce", int(loginDuration.Minutes())))
        }
    } else {
        p.Bot.SendMessage(nick, fmt.Sprintf("Saját felhasználó: %s (%s)", effectiveUser, role))
    }
}


func (p *YnmAdminPlugin) HasRole(userRole, requiredRole string) bool {
	userLevel, userExists := RoleHierarchy[userRole]
	requiredLevel, requiredExists := RoleHierarchy[requiredRole]

	if !userExists || !requiredExists {
		return false
	}

	return userLevel >= requiredLevel
}

func (p *YnmAdminPlugin) HasAccess(hostmask, command string) bool {
	commandRoles := map[string]string{
		"setpass":      "user",
		"setmail":      "user",
		"login":        "user",
		"logout":       "user",
		"auth":         "user",
		"adduser":      "admin",
		"deluser":      "admin",
		"op":           "admin",
		"halfop":       "admin",
		"h":            "admin",
		"voice":        "admin",
		"v":            "admin",
		"o":            "admin",
		"shutdown":     "owner",
		"restart":      "owner",
		"uptime":       "user",  // Mindenki használhatja
		"cycle":        "admin",
		"debugsessions":"admin",
		"bot":          "user",
	}

	requiredRole, exists := commandRoles[command]
	if !exists {
		return false
	}

	// ✅ NORMALIZÁLJUK A HOSTMASK-OT
	normalizedHost := normalizeHostmask(hostmask)
	nick := strings.Split(hostmask, "!")[0]

	// ✅ 1. Ellenőrizzük, hogy be van-e jelentkezve (session)
	effectiveUser, _ := p.GetEffectiveUser(hostmask)
	if effectiveUser != nick {
		// Van session, használjuk a bejelentkezett user jogosultságát
		nick = effectiveUser
	}

	// ✅ 2. Lekérjük a user infót a hostmask alapján (többszörös host támogatással)
	userInfo, err := p.Db.GetUserInfoByHost(normalizedHost)
	if err == nil && userInfo != nil {
		// Van ilyen user az adatbázisban, használjuk a role-ját
		return p.HasRole(userInfo.Role, requiredRole)
	}

	// ✅ 3. Ha nincs user info, próbáljuk nick alapján (fallback)
	role := YnMModule.GetUserGlobalRoleWithDB(p.Db, nick, normalizedHost)
	return p.HasRole(role, requiredRole)
}

// normalizeHostmask helper (ha még nincs ebben a fájlban)
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

func (p *YnmAdminPlugin) HandleLogin(nick string, args []string, isPrivate bool) {
    fmt.Printf("[DEBUG HandleLogin] nick=%s, args=%v, isPrivate=%v\n", nick, args, isPrivate)

    if !isPrivate {
        p.Bot.SendMessage(nick, "A bejelentkezés csak privát üzenetben lehetséges.")
        return
    }

    if len(args) < 2 {
        p.Bot.SendMessage(nick, "Használat: login <nick> <jelszo>")
        p.Bot.SendMessage(nick, "Példa: login Markus tesztjelszo123")
        return
    }

    targetNick := args[0]
    password := args[1]

    // Get the full hostmask from the current message context
    hostmask := nick + "!*@*" // Ideiglenes, a valós hostmask-ot kell használni

    // Check if this host already has an active session
    if existingSession, exists := p.GetSessionByHost(hostmask); exists {
        p.Bot.SendMessage(nick, fmt.Sprintf("Már be vagy jelentkezve mint: %s", existingSession.LoggedInAs))
        p.Bot.SendMessage(nick, "Először jelentkezz ki: logout")
        return
    }

    // Verify password for TARGET nick
    valid, err := p.Db.VerifyPassword(targetNick, password)
    if err != nil {
        fmt.Printf("[DEBUG] Login error for %s: %v\n", targetNick, err)
        p.Bot.SendMessage(nick, "Hiba a bejelentkezés során.")
        return
    }

    if valid {
        // Create session based on HOSTMASK - JAVÍTOTT
        sessionID, sessionKey := p.CreateSession(hostmask, targetNick)
        _ = sessionID // használatlan változó
        
        p.Bot.SendMessage(nick, fmt.Sprintf("✅ Sikeres bejelentkezés %s felhasználóként!", targetNick))
        p.Bot.SendMessage(nick, fmt.Sprintf("Session Key: %s (24 óráig érvényes)", sessionKey))
        fmt.Printf("[DEBUG] Host %s logged in as %s with session %s\n", hostmask, targetNick, sessionID)
    } else {
        p.Bot.SendMessage(nick, "❌ Hibás jelszó!")
        fmt.Printf("[DEBUG] Failed login attempt for host %s as %s\n", hostmask, targetNick)
    }
}

func (p *YnmAdminPlugin) HandleLogout(nick string, isPrivate bool) {
    // Get hostmask for logout
    hostmask := nick + "!*@*" // Ideiglenes, a valós hostmask-ot kell használni
    
    // Delete session by hostmask
    if session, exists := p.GetSessionByHost(hostmask); exists {
        p.DeleteSessionByHost(hostmask)
        fmt.Printf("[DEBUG] Session terminated for host %s (was logged in as %s)\n", hostmask, session.LoggedInAs)
    }
    
    p.Bot.SendMessage(nick, "✅ Sikeres kijelentkezés!")
    fmt.Printf("[DEBUG] Host %s logged out\n", hostmask)
}