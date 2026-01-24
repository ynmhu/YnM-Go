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
	"log"
	 "strings"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

var RoleHierarchy = map[string]int{
    "owner": 5,
    "admin": 4,
    "mod":   3,
    "vip":   2,
    "user":  1,
    "":      0,
}


func canManageRole(issuerRole, targetRole, desiredRole string) bool {
	issuerLevel := RoleHierarchy[issuerRole]
	targetLevel := RoleHierarchy[targetRole]
	desiredLevel := RoleHierarchy[desiredRole]

	if issuerRole == "owner" {
		return true
	}
	if issuerRole == "admin" {
		if desiredRole == "admin" {
			return false
		}
		return targetLevel < issuerLevel || desiredLevel < targetLevel
	}
	if issuerRole == "mod" {
		if desiredRole != "vip" && desiredRole != "user" {
			return false
		}
		return targetLevel <= RoleHierarchy["vip"]
	}

	return false
}
func (p *YnmAdminPlugin) GetHostmaskByNick(nick string) (string, error) {
    return p.Db.GetHostmaskByNick(nick)
}

func (p *YnmAdminPlugin) GetUserLevel(nick, hostmask, channel string) int {
	globalRole, err := p.Db.GetUserGlobalRole(nick, hostmask)
	globalLevel := 0
	
	if err == nil && globalRole != "" {
		if level, exists := RoleHierarchy[strings.ToLower(globalRole)]; exists {
			globalLevel = level
			log.Printf("[DEBUG GetUserLevel] Found global level %d for role %s", globalLevel, globalRole)
		}
	}
	
	// ✅ MÁSODSZOR: Csatorna role lekérdezése
	channelRole, err := p.Db.GetUserRoleInChannel(nick, hostmask, channel)
	channelLevel := 0
	
	if err != nil {
		log.Printf("[DEBUG GetUserLevel] Error getting channel role: %v", err)
	}
	
	if channelRole != "" {
		if level, exists := RoleHierarchy[strings.ToLower(channelRole)]; exists {
			channelLevel = level
			log.Printf("[DEBUG GetUserLevel] Found channel level %d for role %s", channelLevel, channelRole)
		}
	}
	
	log.Printf("[DEBUG GetUserLevel] nick=%s, host=%s, channel=%s, globalRole=%s (level=%d), channelRole=%s (level=%d)", 
		nick, hostmask, channel, globalRole, globalLevel, channelRole, channelLevel)
	
	// ✅ A MAGASABBAT HASZNÁLJUK
	if globalLevel > channelLevel {
		log.Printf("[DEBUG GetUserLevel] Using global level %d", globalLevel)
		return globalLevel
	}
	
	if channelLevel > 0 {
		log.Printf("[DEBUG GetUserLevel] Using channel level %d", channelLevel)
		return channelLevel
	}
	
	// ✅ FALLBACK: Ha egyik sem adott eredményt
	log.Printf("[DEBUG GetUserLevel] No role found, checking with HasMinAdminLevelWithDB")
	
	if YnMModule.HasMinAdminLevelWithDB(p.Db, nick, hostmask, channel, 5) {
		log.Printf("[DEBUG GetUserLevel] Fallback: owner level")
		return 5
	} else if YnMModule.HasMinAdminLevelWithDB(p.Db, nick, hostmask, channel, 4) {
		log.Printf("[DEBUG GetUserLevel] Fallback: admin level")
		return 4
	} else if YnMModule.HasMinAdminLevelWithDB(p.Db, nick, hostmask, channel, 3) {
		log.Printf("[DEBUG GetUserLevel] Fallback: mod level")
		return 3
	} else if YnMModule.HasMinAdminLevelWithDB(p.Db, nick, hostmask, channel, 2) {
		log.Printf("[DEBUG GetUserLevel] Fallback: vip level")
		return 2
	} else if YnMModule.HasMinAdminLevelWithDB(p.Db, nick, hostmask, channel, 1) {
		log.Printf("[DEBUG GetUserLevel] Fallback: user level")
		return 1
	}
	
	log.Printf("[DEBUG GetUserLevel] No permissions found, returning 0")
	return 0
}

//Innen Uj 
