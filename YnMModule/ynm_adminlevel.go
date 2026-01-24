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
package YnMModule

import (
	"strings"
	"fmt"
	"time"
	"sync"

)
var (
    adminCheckCache = make(map[string]struct{
        result bool
        timestamp time.Time
    })
    cacheMutex sync.RWMutex
)

// Interfészek a circular import elkerülésére
type UserDB interface {
	GetUserRoleInChannel(nick, hostmask, channel string) (string, error)
	GetUserGlobalRole(nick, hostmask string) (string, error) 
}

type AdminPlugin interface {
	GetDB() UserDB
}

// Wrapper struct hogy ne kelljen minden alkalommal a DB-t átadni
type AuthHandler struct {
	db UserDB
}

func NewAuthHandler(db UserDB) *AuthHandler {
	return &AuthHandler{db: db}
}

func (a *AuthHandler) GetUserRole(nick, hostmask, channel string) string {
	if a.db != nil {
		role, err := a.db.GetUserRoleInChannel(nick, hostmask, channel)
		if err == nil {
			return role
		}
	}
	return ""
}

func (a *AuthHandler) GetUserAdminLevel(nick, hostmask, channel string) int {
	role := a.GetUserRole(nick, hostmask, channel)
	switch role {
	case "owner":
		return 4
	case "admin":
		return 3
	case "mod":
		return 2
	case "vip":
		return 1
	default:
		return 0
	}
}

func (a *AuthHandler) HasMinAdminLevel(nick, hostmask, channel string, minLevel int) bool {
	adminLevel := a.GetUserAdminLevel(nick, hostmask, channel)
	return adminLevel >= minLevel
}

// Standalone függvények is elérhetők, ha direkt DB-t adsz át
func GetUserRoleWithDB(db UserDB, nick, hostmask, channel string) string {
	if db != nil {
		role, err := db.GetUserRoleInChannel(nick, hostmask, channel)
		if err == nil {
			return role
		}
	}
	return ""
}

func GetUserAdminLevelWithDB(db UserDB, nick, hostmask, channel string) int {
	role := GetUserRoleWithDB(db, nick, hostmask, channel)
	switch role {
	case "owner":
		return 5
	case "admin":
		return 4
	case "mod":
		return 3
	case "vip":
		return 2
	default:
		return 1
	}
}

func GetUserGlobalRoleWithDB(db UserDB, nick, hostmask string) string {
	if db != nil {
		role, err := db.GetUserGlobalRole(nick, hostmask)
		if err == nil {
			return role
		}
	}
	return ""
}

// Új függvény ami ELŐSZÖR channel-t, AZTÁN globálisat néz
func GetUserRoleWithFallbackDB(db UserDB, nick, hostmask, channel string) string {
	if db == nil {
		return ""
	}

	// Kisbetűs nick a kereséshez
	lowerNick := strings.ToLower(nick)

	// 1. Először channel-specifikus jogot néz
	channelRole, err := db.GetUserRoleInChannel(lowerNick, hostmask, strings.ToLower(channel))
	if err == nil && channelRole != "" {
		return channelRole
	}

	// 2. Ha nincs channel-specifikus, akkor globálisat néz
	globalRole, err := db.GetUserGlobalRole(lowerNick, hostmask)
	if err == nil && globalRole != "" {
		return globalRole
	}

	return ""
}

// ✨ MÓDOSÍTOTT: Social ID támogatással - EXTRA PARAMÉTERREL ✨
func HasMinAdminLevelWithDB(db UserDB, nick, hostmask, channel string, minLevel int) bool {
    // Cache key
    cacheKey := nick + "|" + hostmask + "|" + channel + "|" + fmt.Sprintf("%d", minLevel)
    
    // Cache ellenőrzés
    cacheMutex.RLock()
    if cached, found := adminCheckCache[cacheKey]; found {
        if time.Since(cached.timestamp) < 30*time.Second { // 30 másodperc cache
            cacheMutex.RUnlock()
            return cached.result
        }
    }
    cacheMutex.RUnlock()
    
    role := GetUserRoleWithFallbackDB(db, nick, hostmask, channel)
    var adminLevel int

	switch role {
	case "owner":
		adminLevel = 4
	case "admin":
		adminLevel = 3
	case "mod":
		adminLevel = 2
	case "vip":
		adminLevel = 1
	default:
		adminLevel = 0
	}
    
    result := adminLevel >= minLevel
    
    // Cache-be mentés
    cacheMutex.Lock()
    adminCheckCache[cacheKey] = struct{
        result bool
        timestamp time.Time
    }{
        result: result,
        timestamp: time.Now(),
    }
    cacheMutex.Unlock()
    
    return result
}

// ✨ ÚJ: Külön függvény social ID ellenőrzéshez ✨
// Ezt csak azok a pluginok használják, amik tudják kezelni a YnMDb.UserInfo-t
func HasMinAdminLevelWithSocialDB(db UserDB, socialCheck func() bool, nick, hostmask, channel string, minLevel int) bool {
	// 1. Először a normál hostmask alapú ellenőrzés
	if HasMinAdminLevelWithDB(db, nick, hostmask, channel, minLevel) {
		return true
	}
	
	// 2. Ha az nem sikerül, megpróbáljuk social ID alapján
	return socialCheck != nil && socialCheck()
}