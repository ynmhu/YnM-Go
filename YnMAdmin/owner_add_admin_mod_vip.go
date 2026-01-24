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

	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)
func (p *YnmAdminPlugin) isOwnerOrBotNick(nick string) bool {
	return strings.EqualFold(nick, "YnM-Go") || 
	       strings.EqualFold(nick, p.Bot.GetNick())
}

func (p *YnmAdminPlugin) isOwnerOrBotByHostmask(hostmask string) bool {
    // Extra biztonság: nick alapú ellenőrzés hostmaskból is
    parts := strings.Split(hostmask, "!")
    if len(parts) > 0 {
        return p.isOwnerOrBotNick(parts[0])
    }
    return false
}
func (p *YnmAdminPlugin) checkMinimumRole(fullHostmask string, minimumRole string) (bool, string) {
	
	hasOwner := p.Db.HasAnyowner() 
	if !hasOwner {
		return true, "YnM-Go"
	}
	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
	info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)

	if err != nil || info == nil {
		return false, "user"  
	}
	
	userRole := strings.ToLower(info.Role)
	minRole := strings.ToLower(minimumRole)
	
	userLevel := RoleHierarchy[userRole]
	minLevel := RoleHierarchy[minRole]
	
	return userLevel >= minLevel, userRole
}

func (p *YnmAdminPlugin) handleAddAdminCommand(fullHostmask string, userArgs []string, issuingChannel string) string {
    
    hasPermission, _ := p.checkMinimumRole(fullHostmask, "owner")
    if !hasPermission {
        return "" 
    }

    simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
    issuerNick := strings.Split(fullHostmask, "!")[0]
    info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
    
    if err != nil || info == nil || info.Role != "owner" {
        return "Csak globális adminnál kell owner jog"
    }
    
    if len(userArgs) < 1 {
        return "Használat: !ynm add admin [#csatorna] nick1 [nick2 ...]\n  - Csatornával: lokális admin jog\n  - Csatorna nélkül: GLOBÁLIS admin jog (csak owner!)"
    }
    
    // ✅ Csatorna vagy globális jog?
    var channelName string
    var targetNicks []string
    isGlobal := false

    if strings.HasPrefix(userArgs[0], "#") {
        channelName = strings.ToLower(userArgs[0])
        if !p.Db.ChannelExists(channelName) {
            return "A megadott csatorna nem létezik: " + channelName
        }
        if len(userArgs) < 2 {
            return "Használat: !ynm add admin #csatorna nick1 [nick2 ...]"
        }
        targetNicks = userArgs[1:]
    } else {
        isGlobal = true
        targetNicks = userArgs
    }

    if isGlobal {
        validTargets := []string{}
        for _, targetNick := range targetNicks {
            // ✅ OWNER ÉS BOT VÉDELME NICK ALAPJÁN
            if p.isOwnerOrBotNick(targetNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", targetNick))
                continue
            }
            
            // ✅ SAJÁT MAGAD VÉDELME
            if strings.EqualFold(targetNick, issuerNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját magadat nem módosíthatod: %s", targetNick))
                continue
            }
            
            validTargets = append(validTargets, targetNick)
        }
        
        if len(validTargets) == 0 {
            return "❌ Nincs érvényes felhasználó a módosításhoz"
        }
        
        // ✅ Csak a validTargets-eken dolgozunk
        for _, targetNick := range validTargets {
            go func(nick string) {
                respChan := p.Bot.GetWhoisChannel(nick)
                p.Bot.RequestWhois(nick)
                
                var whois *YnMIrC.WhoisData
                select {
                case w := <-respChan:
                    whois = w
                case <-time.After(10 * time.Second):
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout (globális admin): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                if whois == nil {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat (globális admin): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                // ✅ 1. NICK ALAPÚ VÉDELEM (WHOIS adatokból)
                if p.isOwnerOrBotNick(whois.Nick) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                targetHostmask := whois.Hostmask
                if targetHostmask == "" {
                    targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
                }
                targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)
				p.Bot.SendMessage(issuingChannel, "DEBUG whois.Hostmask: "+whois.Hostmask)
p.Bot.SendMessage(issuingChannel, "DEBUG simplified hostmask: "+targetHostmask)

                // ✅ 2. HOSTMASK ALAPÚ VÉDELEM
                if p.isOwnerOrBotByHostmask(targetHostmask) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s (hostmask: %s) nem módosítható!", whois.Nick, targetHostmask))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)

                // ✅ 3. MEGLÉVŐ USER OWNER ELLENŐRZÉSE
                if existingUser != nil && strings.ToLower(existingUser.Role) == "owner" {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Owner joga nem módosítható: %s", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                if existingUser == nil {
                    // Új felhasználó admin joggal
                    _ = p.Db.AddUserWithRole(nick, targetHostmask, "admin", p.Cfg, issuerNick, simplifiedHostmask)
                } else {
                    // Meglévő felhasználó admin jogra frissítése
                    _ = p.Db.UpdateUserGlobalRole(nick, targetHostmask, "admin", issuerNick)
                }
                
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Globális admin jog hozzáadva: %s", nick))
                p.Bot.CleanupWhoisChannel(nick)
            }(targetNick)
        }
        
        return fmt.Sprintf("🌐 Globális admin jog hozzáadása folyamatban %d felhasználóhoz...", len(validTargets))
        
    } else {
        // ========== LOKÁLIS ADMIN JOG HOZZÁADÁSA ==========
        var issuerRole string
        issuerRole, _ = p.Db.GetUserRoleInChannel(issuerNick, simplifiedHostmask, channelName)
        if issuerRole == "" {
            info, _ := p.Db.GetUserInfoByHost(simplifiedHostmask)
            if info != nil {
                issuerRole = info.Role
            } else {
                issuerRole = "user"
            }
        }

        issuerRoleLower := strings.ToLower(issuerRole)
        if issuerRoleLower != "owner" && issuerRoleLower != "admin" {
            return fmt.Sprintf("❌ Csak owner vagy admin adhat admin jogot (te: %s)", issuerRole)
        }
        
        validTargets := []string{}
        for _, targetNick := range targetNicks {
            // ✅ OWNER ÉS BOT VÉDELME NICK ALAPJÁN
            if p.isOwnerOrBotNick(targetNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", targetNick))
                continue
            }
            
            // ✅ SAJÁT MAGAD VÉDELME
            if strings.EqualFold(targetNick, issuerNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját magadat nem módosíthatod: %s", targetNick))
                continue
            }
            
            validTargets = append(validTargets, targetNick)
        }
        
        if len(validTargets) == 0 {
            return "❌ Nincs érvényes felhasználó a módosításhoz"
        }
        
        // ✅ Csak a validTargets-eken dolgozunk
        for _, targetNick := range validTargets {
            go func(nick string) {
                respChan := p.Bot.GetWhoisChannel(nick)
                p.Bot.RequestWhois(nick)
                
                var whois *YnMIrC.WhoisData
                select {
                case w := <-respChan:
                    whois = w
                case <-time.After(10 * time.Second):
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout (lokális admin): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                if whois == nil {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat (lokális admin): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                // ✅ 1. NICK ALAPÚ VÉDELEM (WHOIS adatokból)
                if p.isOwnerOrBotNick(whois.Nick) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                targetHostmask := whois.Hostmask
                if targetHostmask == "" {
                    targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
                }
                targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

                // ✅ 2. HOSTMASK ALAPÚ VÉDELEM
                if p.isOwnerOrBotByHostmask(targetHostmask) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s (hostmask: %s) nem módosítható!", whois.Nick, targetHostmask))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
                
                // ✅ 3. MEGLÉVŐ USER OWNER ELLENŐRZÉSE
                if existingUser != nil && strings.ToLower(existingUser.Role) == "owner" {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Owner joga nem módosítható: %s", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                // Jogellenőrzés - csak alacsonyabb rangúnak adhatunk jogot
                targetChannelRole, _ := p.Db.GetUserRoleInChannel(nick, targetHostmask, channelName)
                targetChannelRoleLower := strings.ToLower(targetChannelRole)
                
                if issuerRoleLower == "admin" && (targetChannelRoleLower == "admin" || targetChannelRoleLower == "owner") {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Admin nem adhat jogot magasabb vagy azonos szintű felhasználónak: %s (%s)", nick, targetChannelRole))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                if existingUser == nil {
                    _ = p.Db.AddUserWithRole(nick, targetHostmask, "user", p.Cfg, issuerNick, simplifiedHostmask)
                }

                exists, _ := p.Db.IsUserInChannel(nick, channelName)
                if exists {
                    _ = p.Db.UpdateUserRoleInChannel(nick, targetHostmask, channelName, "admin")
                } else {
                    _ = p.Db.AddUserToChannel(nick, targetHostmask, channelName, "admin", 1, 0, 0, issuerNick, simplifiedHostmask)
                }

                p.Bot.SendRaw(fmt.Sprintf("MODE %s +o %s", channelName, nick))
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Lokális admin jog hozzáadva: %s -> %s", nick, channelName))
                p.Bot.CleanupWhoisChannel(nick)
            }(targetNick)
        }

        return fmt.Sprintf("📍 Lokális admin jog hozzáadása folyamatban %d felhasználóhoz (%s)...", len(validTargets), channelName)
    }
}

func (p *YnmAdminPlugin) handleAddModCommand(fullHostmask string, userArgs []string, issuingChannel string) string {

    simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
    issuerNick := strings.Split(fullHostmask, "!")[0]

    
    if len(userArgs) < 1 {
        return "Használat: !ynm add mod [#csatorna] nick1 [nick2 ...]\n  - Csatornával: lokális mod jog\n  - Csatorna nélkül: GLOBÁLIS mod jog (csak owner!)"
    }

    var channelName string
    var targetNicks []string
    isGlobal := false

    if strings.HasPrefix(userArgs[0], "#") {
        channelName = strings.ToLower(userArgs[0])
        if !p.Db.ChannelExists(channelName) {
            return "A megadott csatorna nem létezik: " + channelName
        }
        if len(userArgs) < 2 {
            return "Használat: !ynm add mod #csatorna nick1 [nick2 ...]"
        }
        targetNicks = userArgs[1:]
    } else {
        isGlobal = true
        targetNicks = userArgs

        info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
        if err != nil || info == nil || info.Role != "owner" {
            return "❌ Csak owner adhat GLOBÁLIS mod jogot! (Használd: !ynm add mod #csatorna nick)"
        }
    }

    if isGlobal {
        // ========== GLOBÁLIS MOD JOG ==========
        validTargets := []string{}
        for _, targetNick := range targetNicks {
            // ✅ OWNER ÉS BOT VÉDELME NICK ALAPJÁN
            if p.isOwnerOrBotNick(targetNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", targetNick))
                continue
            }
            
            // ✅ SAJÁT MAGAD VÉDELME
            if strings.EqualFold(targetNick, issuerNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját magadat nem módosíthatod: %s", targetNick))
                continue
            }
            
            validTargets = append(validTargets, targetNick)
        }
        
        if len(validTargets) == 0 {
            return "❌ Nincs érvényes felhasználó a módosításhoz"
        }
        
        for _, targetNick := range validTargets {
            go func(nick string) {
                respChan := p.Bot.GetWhoisChannel(nick)
                p.Bot.RequestWhois(nick)
                
                var whois *YnMIrC.WhoisData
                select {
                case w := <-respChan:
                    whois = w
                case <-time.After(10 * time.Second):
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout (globális mod): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                if whois == nil {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat (globális mod): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                // ✅ 1. NICK ALAPÚ VÉDELEM (WHOIS adatokból)
                if p.isOwnerOrBotNick(whois.Nick) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                targetHostmask := whois.Hostmask
                if targetHostmask == "" {
                    targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
                }
                targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

                // ✅ 2. HOSTMASK ALAPÚ VÉDELEM
                if p.isOwnerOrBotByHostmask(targetHostmask) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s (hostmask: %s) nem módosítható!", whois.Nick, targetHostmask))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
                
                // ✅ 3. MEGLÉVŐ USER OWNER ELLENŐRZÉSE
                if existingUser != nil && strings.ToLower(existingUser.Role) == "owner" {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Owner joga nem módosítható: %s", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                if existingUser == nil {
                    _ = p.Db.AddUserWithRole(nick, targetHostmask, "mod", p.Cfg, issuerNick, simplifiedHostmask)
                } else {
                    _ = p.Db.UpdateUserGlobalRole(nick, targetHostmask, "mod", issuerNick)
                }
                
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Globális mod jog hozzáadva: %s", nick))
                p.Bot.CleanupWhoisChannel(nick)
            }(targetNick)
        }
        
        return fmt.Sprintf("🌐 Globális mod jog hozzáadása folyamatban %d felhasználóhoz...", len(validTargets))
        
    } else {
        // ========== LOKÁLIS MOD JOG ==========
        var issuerRole string
        issuerRole, _ = p.Db.GetUserRoleInChannel(issuerNick, simplifiedHostmask, channelName)
        if issuerRole == "" {
            info, _ := p.Db.GetUserInfoByHost(simplifiedHostmask)
            if info != nil {
                issuerRole = info.Role
            } else {
                issuerRole = "user"
            }
        }

        issuerRoleLower := strings.ToLower(issuerRole)
        if issuerRoleLower != "owner" && issuerRoleLower != "admin" {
            return fmt.Sprintf("❌ Csak owner vagy admin adhat mod jogot (te: %s)", issuerRole)
        }

        validTargets := []string{}
        for _, targetNick := range targetNicks {
            // ✅ OWNER ÉS BOT VÉDELME NICK ALAPJÁN
            if p.isOwnerOrBotNick(targetNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", targetNick))
                continue
            }
            
            // ✅ SAJÁT MAGAD VÉDELME
            if strings.EqualFold(targetNick, issuerNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját magadat nem módosíthatod: %s", targetNick))
                continue
            }
            
            validTargets = append(validTargets, targetNick)
        }
        
        if len(validTargets) == 0 {
            return "❌ Nincs érvényes felhasználó a módosításhoz"
        }
        
        for _, targetNick := range validTargets {
            go func(nick string) {
                respChan := p.Bot.GetWhoisChannel(nick)
                p.Bot.RequestWhois(nick)
                
                var whois *YnMIrC.WhoisData
                select {
                case w := <-respChan:
                    whois = w
                case <-time.After(10 * time.Second):
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout (lokális mod): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                if whois == nil {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat (lokális mod): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                // ✅ 1. NICK ALAPÚ VÉDELEM (WHOIS adatokból)
                if p.isOwnerOrBotNick(whois.Nick) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                targetHostmask := whois.Hostmask
                if targetHostmask == "" {
                    targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
                }
                targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

                // ✅ 2. HOSTMASK ALAPÚ VÉDELEM
                if p.isOwnerOrBotByHostmask(targetHostmask) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s (hostmask: %s) nem módosítható!", whois.Nick, targetHostmask))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
                
                // ✅ 3. MEGLÉVŐ USER OWNER ELLENŐRZÉSE
                if existingUser != nil && strings.ToLower(existingUser.Role) == "owner" {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Owner joga nem módosítható: %s", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                targetChannelRole, _ := p.Db.GetUserRoleInChannel(nick, targetHostmask, channelName)
                targetChannelRoleLower := strings.ToLower(targetChannelRole)
                
                if issuerRoleLower == "admin" && (targetChannelRoleLower == "admin" || targetChannelRoleLower == "owner") {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Admin nem adhat jogot magasabb szintű felhasználónak: %s (%s)", nick, targetChannelRole))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                if existingUser == nil {
                    _ = p.Db.AddUserWithRole(nick, targetHostmask, "user", p.Cfg, issuerNick, simplifiedHostmask)
                }

                exists, _ := p.Db.IsUserInChannel(nick, channelName)
                if exists {
                    _ = p.Db.UpdateUserRoleInChannel(nick, targetHostmask, channelName, "mod")
                } else {
                    _ = p.Db.AddUserToChannel(nick, targetHostmask, channelName, "mod", 0, 0, 1, issuerNick, simplifiedHostmask)
                }

                p.Bot.SendRaw(fmt.Sprintf("MODE %s +h %s", channelName, nick))
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Lokális mod jog hozzáadva: %s -> %s", nick, channelName))
                p.Bot.CleanupWhoisChannel(nick)
            }(targetNick)
        }

        return fmt.Sprintf("📍 Lokális mod jog hozzáadása folyamatban %d felhasználóhoz (%s)...", len(validTargets), channelName)
    }
}
func (p *YnmAdminPlugin) handleAddVipCommand(fullHostmask string, userArgs []string, issuingChannel string) string {

    simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
    issuerNick := strings.Split(fullHostmask, "!")[0]

    
    if len(userArgs) < 1 {
        return "Használat: !ynm add vip [#csatorna] nick1 [nick2 ...]\n  - Csatornával: lokális VIP jog\n  - Csatorna nélkül: GLOBÁLIS VIP jog (csak owner!)"
    }

    var channelName string
    var targetNicks []string
    isGlobal := false

    if strings.HasPrefix(userArgs[0], "#") {
        channelName = strings.ToLower(userArgs[0])
        if !p.Db.ChannelExists(channelName) {
            return "A megadott csatorna nem létezik: " + channelName
        }
        if len(userArgs) < 2 {
            return "Használat: !ynm add vip #csatorna nick1 [nick2 ...]"
        }
        targetNicks = userArgs[1:]
    } else {
        isGlobal = true
        targetNicks = userArgs

        info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
        if err != nil || info == nil || info.Role != "owner" {
            return "❌ Csak owner adhat GLOBÁLIS VIP jogot! (Használd: !ynm add vip #csatorna nick)"
        }
    }

    if isGlobal {
        // ========== GLOBÁLIS VIP JOG ==========
        validTargets := []string{}
        for _, targetNick := range targetNicks {
            // ✅ OWNER ÉS BOT VÉDELME NICK ALAPJÁN
            if p.isOwnerOrBotNick(targetNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", targetNick))
                continue
            }
            
            // ✅ SAJÁT MAGAD VÉDELME
            if strings.EqualFold(targetNick, issuerNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját magadat nem módosíthatod: %s", targetNick))
                continue
            }
            
            validTargets = append(validTargets, targetNick)
        }
        
        if len(validTargets) == 0 {
            return "❌ Nincs érvényes felhasználó a módosításhoz"
        }
        
        for _, targetNick := range validTargets {
            go func(nick string) {
                respChan := p.Bot.GetWhoisChannel(nick)
                p.Bot.RequestWhois(nick)
                
                var whois *YnMIrC.WhoisData
                select {
                case w := <-respChan:
                    whois = w
                case <-time.After(10 * time.Second):
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout (globális VIP): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                if whois == nil {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat (globális VIP): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                // ✅ 1. NICK ALAPÚ VÉDELEM (WHOIS adatokból)
                if p.isOwnerOrBotNick(whois.Nick) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                targetHostmask := whois.Hostmask
                if targetHostmask == "" {
                    targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
                }
                targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

                // ✅ 2. HOSTMASK ALAPÚ VÉDELEM
                if p.isOwnerOrBotByHostmask(targetHostmask) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s (hostmask: %s) nem módosítható!", whois.Nick, targetHostmask))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
                
                // ✅ 3. MEGLÉVŐ USER OWNER ELLENŐRZÉSE
                if existingUser != nil && strings.ToLower(existingUser.Role) == "owner" {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Owner joga nem módosítható: %s", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                if existingUser == nil {
                    _ = p.Db.AddUserWithRole(nick, targetHostmask, "vip", p.Cfg, issuerNick, simplifiedHostmask) 
                } else {
                    _ = p.Db.UpdateUserGlobalRole(nick, targetHostmask, "vip", issuerNick)
                }
                
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Globális VIP jog hozzáadva: %s", nick))
                p.Bot.CleanupWhoisChannel(nick)
            }(targetNick)
        }
        
        return fmt.Sprintf("🌐 Globális VIP jog hozzáadása folyamatban %d felhasználóhoz...", len(validTargets))
        
    } else {
        // ========== LOKÁLIS VIP JOG ==========
        var issuerRole string
        issuerRole, _ = p.Db.GetUserRoleInChannel(issuerNick, simplifiedHostmask, channelName)
        if issuerRole == "" {
            info, _ := p.Db.GetUserInfoByHost(simplifiedHostmask)
            if info != nil {
                issuerRole = info.Role
            } else {
                issuerRole = "user"
            }
        }

        issuerRoleLower := strings.ToLower(issuerRole)
        if issuerRoleLower != "owner" && issuerRoleLower != "admin" && issuerRoleLower != "mod" {
            return fmt.Sprintf("❌ Csak owner, admin vagy mod adhat VIP jogot (te: %s)", issuerRole)
        }

        validTargets := []string{}
        for _, targetNick := range targetNicks {
            // ✅ OWNER ÉS BOT VÉDELME NICK ALAPJÁN
            if p.isOwnerOrBotNick(targetNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", targetNick))
                continue
            }
            
            // ✅ SAJÁT MAGAD VÉDELME
            if strings.EqualFold(targetNick, issuerNick) {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját magadat nem módosíthatod: %s", targetNick))
                continue
            }
            
            validTargets = append(validTargets, targetNick)
        }
        
        if len(validTargets) == 0 {
            return "❌ Nincs érvényes felhasználó a módosításhoz"
        }
        
        for _, targetNick := range validTargets {
            go func(nick string) {
                respChan := p.Bot.GetWhoisChannel(nick)
                p.Bot.RequestWhois(nick)
                
                var whois *YnMIrC.WhoisData
                select {
                case w := <-respChan:
                    whois = w
                case <-time.After(10 * time.Second):
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout (lokális VIP): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                if whois == nil {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat (lokális VIP): %s", nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                // ✅ 1. NICK ALAPÚ VÉDELEM (WHOIS adatokból)
                if p.isOwnerOrBotNick(whois.Nick) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                targetHostmask := whois.Hostmask
                if targetHostmask == "" {
                    targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
                }
                targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

                // ✅ 2. HOSTMASK ALAPÚ VÉDELEM
                if p.isOwnerOrBotByHostmask(targetHostmask) {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s (hostmask: %s) nem módosítható!", whois.Nick, targetHostmask))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
                
                // ✅ 3. MEGLÉVŐ USER OWNER ELLENŐRZÉSE
                if existingUser != nil && strings.ToLower(existingUser.Role) == "owner" {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Owner joga nem módosítható: %s", whois.Nick))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                targetChannelRole, _ := p.Db.GetUserRoleInChannel(nick, targetHostmask, channelName)
                targetChannelRoleLower := strings.ToLower(targetChannelRole)
                
                if issuerRoleLower == "mod" && (targetChannelRoleLower == "mod" || targetChannelRoleLower == "admin" || targetChannelRoleLower == "owner") {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Mod nem adhat jogot magasabb szintű felhasználónak: %s (%s)", nick, targetChannelRole))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }
                
                if issuerRoleLower == "admin" && (targetChannelRoleLower == "admin" || targetChannelRoleLower == "owner") {
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Admin nem adhat jogot magasabb szintű felhasználónak: %s (%s)", nick, targetChannelRole))
                    p.Bot.CleanupWhoisChannel(nick)
                    return
                }

                if existingUser == nil {
                    _ = p.Db.AddUserWithRole(nick, targetHostmask, "user", p.Cfg, issuerNick, simplifiedHostmask)
                }

                exists, _ := p.Db.IsUserInChannel(nick, channelName)
                if exists {
                    // 1. Kérdezzük le, ki adta hozzá
                    addedBy, addedByHost, err := p.Db.GetAddedInfoForUserInChannel(nick, channelName)
                    if err != nil {
                        p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A felhasználó nincs a csatornában: %s", nick))
                        p.Bot.CleanupWhoisChannel(nick)
                        return
                    }
                    
                    // 2. Kérdezzük le a hozzáadó role-ját
                    var addedByRole string
                    addedByRole, _ = p.Db.GetUserRoleInChannel(addedBy, addedByHost, channelName)
                    if addedByRole == "" {
                        info, _ := p.Db.GetUserInfoByHost(addedByHost)
                        if info != nil {
                            addedByRole = info.Role
                        } else {
                            addedByRole = "user"
                        }
                    }
                    addedByRoleLower := strings.ToLower(addedByRole) 

                    modifiedBy, modifiedByHost, err := p.Db.GetModifiedInfoForUserInChannel(nick, channelName)
                    if err != nil {
                        modifiedBy = addedBy
                        _ = modifiedByHost 
                    }
                    
                    // 4. Ellenőrizzük, hogy jogosult-e visszaadni (ÚJ LOGIKA)
                    canRestore := false
                    
                    // 4.1 Owner mindig visszaadhatja
                    if issuerRoleLower == "owner" {
                        canRestore = true
                    } else if modifiedBy == "" || strings.EqualFold(modifiedBy, "YnM-Go") {
                        if strings.EqualFold(issuerNick, addedBy) {
                            canRestore = true
                        }
                    } else if strings.EqualFold(issuerNick, modifiedBy) {
                        canRestore = true
                    } else if issuerRoleLower == "admin" && 
                           (addedByRoleLower == "mod" || addedByRoleLower == "vip" || addedByRoleLower == "user") {
                        // De csak akkor, ha modified_by sem volt
                        if modifiedBy == "" || strings.EqualFold(modifiedBy, "YnM-Go") {
                            canRestore = true
                        }
                    }
                    
                    if !canRestore {
                        if modifiedBy == "" || strings.EqualFold(modifiedBy, "YnM-Go") {
                            p.Bot.SendMessage(issuingChannel, 
                                fmt.Sprintf("❌ Nem adhatod vissza a VIP jogot: %s (hozzáadta: %s, utoljára módosította: nem volt módosítás)", 
                                nick, addedBy))
                        } else {
                            p.Bot.SendMessage(issuingChannel, 
                                fmt.Sprintf("❌ Nem adhatod vissza a VIP jogot: %s (hozzáadta: %s, utoljára módosította: %s)", 
                                nick, addedBy, modifiedBy))
                        }
                        p.Bot.CleanupWhoisChannel(nick)
                        return
                    }
                    
                    // 5. Ha jogosult, visszaadjuk a VIP jogot
                    if !strings.EqualFold(issuerNick, addedBy) {
                        // Ha más adja vissza, átírjuk az added_by-t is
                        _, err = p.Db.Exec(`
                            UPDATE channel_users 
                            SET role = 'vip', 
                                added_by = ?,
                                added_by_host = ?,
                                modified_by = ?,
                                modified_by_host = ?,
                                changed_at = CURRENT_TIMESTAMP,
                                auto_voice = 1
                            WHERE hostmask = ? AND channel = ?
                        `, issuerNick, simplifiedHostmask, issuerNick, simplifiedHostmask, 
                           targetHostmask, channelName)
                        
                        if err != nil {
                            p.Bot.SendMessage(issuingChannel, 
                                fmt.Sprintf("❌ Hiba a VIP jog visszaadásakor: %s", err))
                            p.Bot.CleanupWhoisChannel(nick)
                            return
                        }
                        
                        p.Bot.SendMessage(issuingChannel, 
                            fmt.Sprintf("✅ VIP jog visszaadva és átvették a jogosultságokat: %s @ %s (új felelős: %s, korábbi felelős: %s)", 
                            nick, channelName, issuerNick, addedBy))
                    } else {
                        _, err = p.Db.Exec(`
                            UPDATE channel_users 
                            SET role = 'vip', 
                                modified_by = ?,
                                modified_by_host = ?,
                                changed_at = CURRENT_TIMESTAMP,
                                auto_voice = 1
                            WHERE hostmask = ? AND channel = ?
                        `, issuerNick, simplifiedHostmask, targetHostmask, channelName)
                        
                        if err != nil {
                            p.Bot.SendMessage(issuingChannel, 
                                fmt.Sprintf("❌ Hiba a VIP jog visszaadásakor: %s", err))
                            p.Bot.CleanupWhoisChannel(nick)
                            return
                        }
                        
                        p.Bot.SendMessage(issuingChannel, 
                            fmt.Sprintf("✅ VIP jog visszaadva: %s @ %s (felelős: %s)", 
                            nick, channelName, issuerNick))
                    }

                    p.Bot.SendRaw(fmt.Sprintf("MODE %s +v %s", channelName, nick))
                    
                    p.Bot.SendMessage(issuingChannel, 
                        fmt.Sprintf("✅ VIP jog visszaadva: %s @ %s (hozzáadta: %s, most módosította: %s)", 
                        nick, channelName, addedBy, issuerNick))
                } else {
                    // Új hozzáadás
                    _ = p.Db.AddUserToChannel(nick, targetHostmask, channelName, "vip", 0, 1, 0, issuerNick, simplifiedHostmask)
                    p.Bot.SendRaw(fmt.Sprintf("MODE %s +v %s", channelName, nick))
                    p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Lokális VIP jog hozzáadva: %s -> %s", nick, channelName))
                }
                
                p.Bot.CleanupWhoisChannel(nick)
            }(targetNick)
        }

        return fmt.Sprintf("📍 Lokális VIP jog hozzáadása folyamatban %d felhasználóhoz (%s)...", len(validTargets), channelName)
    }
}
func (p *YnmAdminPlugin) handleRemoveAdminCommand(fullHostmask string, userArgs []string, issuingChannel string) string {

	hasPermission, _ := p.checkMinimumRole(fullHostmask, "owner")
	if !hasPermission {
		return ""
	}
	
	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
	issuerNick := strings.Split(fullHostmask, "!")[0]  
	info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
	if err != nil || info == nil || info.Role != "owner" {
		return "❌ Csak owner törölhet admin jogot!"
	}

	var channelName string
	var targetNicks []string
	isGlobal := false

	if strings.HasPrefix(userArgs[0], "#") {
		channelName = strings.ToLower(userArgs[0])
		if !p.Db.ChannelExists(channelName) {
			return "A megadott csatorna nem létezik: " + channelName
		}
		if len(userArgs) < 2 {
			return "Usage: !ynm del admin #csatorna nick1 [nick2 ...]"
		}
		targetNicks = userArgs[1:]
	} else {
		isGlobal = true
		targetNicks = userArgs
	}
	
	if isGlobal {
				validTargets := []string{}
		for _, targetNick := range targetNicks {
			// ✅ SAJÁT JOGA ÉS BOT NICK ELLENŐRZÉSE
			if strings.EqualFold(targetNick, issuerNick) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját jogaidat nem módosíthatod: %s", targetNick))
				continue
			}

			targetNickLower := strings.ToLower(targetNick)
			if targetNickLower == strings.ToLower(p.Bot.GetNick()) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A bot nickjét nem lehet módosítani: %s", targetNick))
				continue
			}
			
			validTargets = append(validTargets, targetNick)
		}
		
		if len(validTargets) == 0 {
			return "❌ Nincs érvényes felhasználó a törléshez"
		}
		for _, targetNick := range targetNicks {
			go func(nick string) {
				respChan := p.Bot.GetWhoisChannel(nick)
				p.Bot.RequestWhois(nick)
				
				var whois *YnMIrC.WhoisData
				select {
				case w := <-respChan:
					whois = w
				case <-time.After(10 * time.Second):
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}
				
				if whois == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				targetHostmask := whois.Hostmask
				if targetHostmask == "" {
					targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
				}
				targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)
				if p.isOwnerOrBotNick(whois.Nick) {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s-t nem lehet módosítani!", whois.Nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}
				if p.isOwnerOrBotByHostmask(targetHostmask) {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ %s (hostmask: %s) nem módosítható!", whois.Nick, targetHostmask))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}


				existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
				
				if existingUser == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Felhasználó nem található: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				if strings.ToLower(existingUser.Role) == "owner" {
																								   
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				_ = p.Db.UpdateUserGlobalRole(nick, targetHostmask, "user", issuerNick)
				
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Globális admin jog törölve: %s → most 'user'", nick))
				p.Bot.CleanupWhoisChannel(nick)
			}(targetNick)
		}
		
		return fmt.Sprintf("🌐 Globális admin jog törlése folyamatban %d felhasználótól...", len(targetNicks))
		
	} else {
							validTargets := []string{}
		for _, targetNick := range targetNicks {
			if strings.EqualFold(targetNick, issuerNick) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját jogaidat nem módosíthatod: %s", targetNick))
				continue
			}

			targetNickLower := strings.ToLower(targetNick)
			if targetNickLower == strings.ToLower(p.Bot.GetNick()) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A bot nickjét nem lehet módosítani: %s", targetNick))
				continue
			}
			
			validTargets = append(validTargets, targetNick)
		}
		
		if len(validTargets) == 0 {
			return "❌ Nincs érvényes felhasználó a törléshez"
		}			 
		for _, targetNick := range targetNicks {
			go func(nick string) {
				respChan := p.Bot.GetWhoisChannel(nick)
				p.Bot.RequestWhois(nick)
				
				var whois *YnMIrC.WhoisData
				select {
				case w := <-respChan:
					whois = w
				case <-time.After(10 * time.Second):
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}
				
				if whois == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				targetHostmask := whois.Hostmask
				if targetHostmask == "" {
					targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
				}
				targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

				_ = p.Db.UpdateUserRoleInChannel(nick, targetHostmask, channelName, "user")
				p.Bot.SendRaw(fmt.Sprintf("MODE %s -o %s", channelName, nick))
				
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Lokális admin jog törölve: %s @ %s", nick, channelName))
				p.Bot.CleanupWhoisChannel(nick)
			}(targetNick)
		}

		return fmt.Sprintf("📍 Lokális admin jog törlése folyamatban %d felhasználótól (%s)...", len(targetNicks), channelName)
	}
}

func (p *YnmAdminPlugin) handleRemoveModCommand(fullHostmask string, userArgs []string, issuingChannel string) string {


	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
	issuerNick := strings.Split(fullHostmask, "!")[0]  

	var channelName string
	var targetNicks []string
	isGlobal := false
	if strings.HasPrefix(userArgs[0], "#") {
		channelName = strings.ToLower(userArgs[0])
		if !p.Db.ChannelExists(channelName) {
			return "A megadott csatorna nem létezik: " + channelName
		}
		if len(userArgs) < 2 {
			return "Usage: !ynm del mod #csatorna nick1 [nick2 ...]"
		}
		targetNicks = userArgs[1:]
	} else {
		isGlobal = true
		targetNicks = userArgs

		// Owner vagy Admin jogosultság kell
		info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
		if err != nil || info == nil || (info.Role != "owner" && info.Role != "admin") {
			return "❌ Csak owner vagy admin törölhet GLOBÁLIS mod jogot!"
		}
	}
	if isGlobal {
				// ========== GLOBÁLIS mod JOG TÖRLÉSE ==========
		validTargets := []string{}
		for _, targetNick := range targetNicks {
			// ✅ SAJÁT JOGA ÉS BOT NICK ELLENŐRZÉSE
			if strings.EqualFold(targetNick, issuerNick) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját jogaidat nem módosíthatod: %s", targetNick))
				continue
			}

			targetNickLower := strings.ToLower(targetNick)
			if targetNickLower == strings.ToLower(p.Bot.GetNick()) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A bot nickjét nem lehet módosítani: %s", targetNick))
				continue
			}
			
			validTargets = append(validTargets, targetNick)
		}
		
		if len(validTargets) == 0 {
			return "❌ Nincs érvényes felhasználó a törléshez"
		}
		for _, targetNick := range targetNicks {
			go func(nick string) {
				respChan := p.Bot.GetWhoisChannel(nick)
				p.Bot.RequestWhois(nick)
				
				var whois *YnMIrC.WhoisData
				select {
				case w := <-respChan:
					whois = w
				case <-time.After(10 * time.Second):
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}
				
				if whois == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				targetHostmask := whois.Hostmask
				if targetHostmask == "" {
					targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
				}
				targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

				existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
				
				if existingUser == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Felhasználó nem található: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				if strings.ToLower(existingUser.Role) == "owner" {
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				_ = p.Db.UpdateUserGlobalRole(nick, targetHostmask, "user", issuerNick)
				
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Globális mod jog törölve: %s → most 'user'", nick))
				p.Bot.CleanupWhoisChannel(nick)
			}(targetNick)
		}		
		return fmt.Sprintf("🌐 Globális mod jog törlése folyamatban %d felhasználótól...", len(targetNicks))		
	} else {
		var issuerRole string
		issuerRole, _ = p.Db.GetUserRoleInChannel(issuerNick, simplifiedHostmask, channelName)
		if issuerRole == "" {
			info, _ := p.Db.GetUserInfoByHost(simplifiedHostmask)
			if info != nil {
				issuerRole = info.Role
			} else {
				issuerRole = "user"
			}
		}
		issuerRoleLower := strings.ToLower(issuerRole)
		if issuerRoleLower != "owner" && issuerRoleLower != "admin" {
			return fmt.Sprintf("❌ Csak owner vagy admin törölhet mod jogot (te: %s)", issuerRole)
		}
		validTargets := []string{}
		for _, targetNick := range targetNicks {
			if strings.EqualFold(targetNick, issuerNick) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját jogaidat nem módosíthatod: %s", targetNick))
				continue
			}

			targetNickLower := strings.ToLower(targetNick)
			if targetNickLower == strings.ToLower(p.Bot.GetNick()) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A bot nickjét nem lehet módosítani: %s", targetNick))
				continue
			}
			
			validTargets = append(validTargets, targetNick)
		}
		
		if len(validTargets) == 0 {
			return "❌ Nincs érvényes felhasználó a törléshez"
		}

		for _, targetNick := range targetNicks {
			go func(nick string) {
				respChan := p.Bot.GetWhoisChannel(nick)
				p.Bot.RequestWhois(nick)
				
				var whois *YnMIrC.WhoisData
				select {
				case w := <-respChan:
					whois = w
				case <-time.After(10 * time.Second):
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}				
				if whois == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}
				targetHostmask := whois.Hostmask
				if targetHostmask == "" {
					targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
				}
				targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

				// Get the added_by for the user in the channel
				addedBy, err := p.Db.GetAddedByForUserInChannel(nick, channelName)
				if err != nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A felhasználó nincs a csatornában: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				// If the issuer is an admin (and not owner) and the issuerNick does not match addedBy, then deny.
				if issuerRoleLower == "admin" && !strings.EqualFold(issuerNick, addedBy){
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Csak az admin törölheti a mod jogot, aki hozzáadta: %s (hozzáadta: %s)", nick, addedBy))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}


				_, err = p.Db.Exec(`UPDATE channel_users SET role = 'user', auto_halfop = 0 WHERE nick = ? AND channel = ?`, nick, channelName)
				p.Bot.SendRaw(fmt.Sprintf("MODE %s -h %s", channelName, nick))
				
				p.Bot.SendMessage(issuingChannel, 
				fmt.Sprintf("✅ Lokális mod jog törölve: %s @ %s (hozzáadta: %s)", 
				nick, channelName, addedBy))
				p.Bot.CleanupWhoisChannel(nick)
			}(targetNick)
		}

		return fmt.Sprintf("📍 Lokális mod jog törlése folyamatban %d felhasználótól (%s)...", len(targetNicks), channelName)
	}
}

func (p *YnmAdminPlugin) handleRemoveVipCommand(fullHostmask string, userArgs []string, issuingChannel string) string {

	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
	issuerNick := strings.Split(fullHostmask, "!")[0]  // ✅ MINDIG KELL!

	var channelName string
	var targetNicks []string
	isGlobal := false

	if strings.HasPrefix(userArgs[0], "#") {
		channelName = strings.ToLower(userArgs[0])
		if !p.Db.ChannelExists(channelName) {
			return "A megadott csatorna nem létezik: " + channelName
		}
		if len(userArgs) < 2 {
			return "Usage: !ynm del vip #csatorna nick1 [nick2 ...]"
		}
		targetNicks = userArgs[1:]
	} else {
		isGlobal = true
		targetNicks = userArgs
		info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
		if err != nil || info == nil || info.Role != "owner" {
			return "❌ Csak owner törölhet GLOBÁLIS VIP jogot! (Használd: !ynm del vip #csatorna nick)"
		}
	}

	if isGlobal {
		// ========== GLOBÁLIS VIP JOG TÖRLÉSE ==========
		validTargets := []string{}
		for _, targetNick := range targetNicks {
			if strings.EqualFold(targetNick, issuerNick) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját jogaidat nem módosíthatod: %s", targetNick))
				continue
			}

			targetNickLower := strings.ToLower(targetNick)
			if targetNickLower == strings.ToLower(p.Bot.GetNick()) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A bot nickjét nem lehet módosítani: %s", targetNick))
				continue
			}
			
			validTargets = append(validTargets, targetNick)
		}
		
		if len(validTargets) == 0 {
			return "❌ Nincs érvényes felhasználó a törléshez"
		}
		
		for _, targetNick := range validTargets {

			go func(nick string) {
				respChan := p.Bot.GetWhoisChannel(nick)
				p.Bot.RequestWhois(nick)
				
				var whois *YnMIrC.WhoisData
				select {
				case w := <-respChan:
					whois = w
				case <-time.After(10 * time.Second):
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout (globális VIP törlés): %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}
				
				if whois == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				targetHostmask := whois.Hostmask
				if targetHostmask == "" {
					targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
				}
				targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

				existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
				
				if existingUser == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Felhasználó nem található: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				if strings.ToLower(existingUser.Role) == "owner" {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⚠️ Owner joga nem törölhető: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				_ = p.Db.UpdateUserGlobalRole(nick, targetHostmask, "user", issuerNick)
				
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("✅ Globális VIP jog törölve: %s → most 'user'", nick))
				p.Bot.CleanupWhoisChannel(nick)
			}(targetNick)
		}
		
		return fmt.Sprintf("🌐 Globális VIP jog törlése folyamatban %d felhasználótól...", len(targetNicks))
		
	} else {
		// ========== LOKÁLIS VIP JOG TÖRLÉSE ==========
		var issuerRole string
		issuerRole, _ = p.Db.GetUserRoleInChannel(issuerNick, simplifiedHostmask, channelName)
		if issuerRole == "" {
			info, _ := p.Db.GetUserInfoByHost(simplifiedHostmask)
			if info != nil {
				issuerRole = info.Role
			} else {
				issuerRole = "user"
			}
		}

		issuerRoleLower := strings.ToLower(issuerRole)
		if issuerRoleLower != "owner" && issuerRoleLower != "admin" && issuerRoleLower != "mod" {
			return fmt.Sprintf("❌ Csak owner, admin vagy mod törölhet VIP jogot (te: %s)", issuerRole)
		}

		validTargets := []string{}
		for _, targetNick := range targetNicks {
			// ✅ SAJÁT JOGA ÉS BOT NICK ELLENŐRZÉSE
			if strings.EqualFold(targetNick, issuerNick) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Saját jogaidat nem módosíthatod: %s", targetNick))
				continue
			}

			targetNickLower := strings.ToLower(targetNick)
			if targetNickLower == strings.ToLower(p.Bot.GetNick()) {
				p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A bot nickjét nem lehet módosítani: %s", targetNick))
				continue
			}
			
			validTargets = append(validTargets, targetNick)
		}
		
		if len(validTargets) == 0 {
			return "❌ Nincs érvényes felhasználó a törléshez"
		}

		for _, targetNick := range validTargets {

			go func(nick string) {
				var err error
				
				respChan := p.Bot.GetWhoisChannel(nick)
				p.Bot.RequestWhois(nick)
				
				var whois *YnMIrC.WhoisData
				select {
				case w := <-respChan:
					whois = w
				case <-time.After(10 * time.Second):
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("⏱️ Timeout: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}
				
				if whois == nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ Nincs WHOIS adat: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				targetHostmask := whois.Hostmask
				if targetHostmask == "" {
					targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
				}
				targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

				// 1. Kérdezzük le, ki adta hozzá eredetileg
				originalAddedBy, originalAddedByHost, err := p.Db.GetAddedInfoForUserInChannel(nick, channelName)
				if err != nil {
					p.Bot.SendMessage(issuingChannel, fmt.Sprintf("❌ A felhasználó nincs a csatornában: %s", nick))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				// 2. Kérdezzük le az added_by role-ját
				var addedByRole string
				addedByRole, _ = p.Db.GetUserRoleInChannel(originalAddedBy, originalAddedByHost, channelName)
				if addedByRole == "" {
					info, _ := p.Db.GetUserInfoByHost(originalAddedByHost)
					if info != nil {
						addedByRole = info.Role
					} else {
						addedByRole = "user"
					}
				}
				addedByRoleLower := strings.ToLower(addedByRole)

				// 3. Ellenőrizzük, hogy jogosult-e törölni
				canRemove := false

				// 3.1 Ha ő adta hozzá
				if strings.EqualFold(issuerNick, originalAddedBy) {
					canRemove = true
				}else if RoleHierarchy[issuerRoleLower] > RoleHierarchy[addedByRoleLower] {
					canRemove = true
				}else if issuerRoleLower == "owner" {
					canRemove = true
				}

				if !canRemove {
					p.Bot.SendMessage(issuingChannel, 
						fmt.Sprintf("❌ Nem veheted el a VIP jogot: %s (hozzáadta: %s, role: %s, te: %s)", 
						nick, originalAddedBy, addedByRole, issuerRole))
					p.Bot.CleanupWhoisChannel(nick)
					return
				}

				// 4. Ha jogosult, akkor töröljük
				if !strings.EqualFold(issuerNick, originalAddedBy) {
					// Más törli -> átírjuk a modified_by-t, de az added_by marad!
					_, err = p.Db.Exec(`
						UPDATE channel_users 
						SET role = 'user', 
							modified_by = ?, 
							modified_by_host = ?,
							changed_at = CURRENT_TIMESTAMP,
							auto_voice = 0
						WHERE hostmask = ? AND channel = ?
					`, issuerNick, simplifiedHostmask, targetHostmask, channelName)
					
					p.Bot.SendMessage(issuingChannel, 
						fmt.Sprintf("✅ VIP jog elvéve: %s @ %s (módosította: %s, eredeti hozzáadó: %s)", 
						nick, channelName, issuerNick, originalAddedBy))
				} else {
					// Ugyanaz törli -> modified_by is ő lesz
					_, err = p.Db.Exec(`
						UPDATE channel_users 
						SET role = 'user', 
							modified_by = ?, 
							modified_by_host = ?,
							changed_at = CURRENT_TIMESTAMP,
							auto_voice = 0
						WHERE hostmask = ? AND channel = ?
					`, issuerNick, simplifiedHostmask, targetHostmask, channelName)
					
					p.Bot.SendMessage(issuingChannel, 
						fmt.Sprintf("✅ VIP jog elvéve: %s @ %s (törölte: %s)", 
						nick, channelName, issuerNick))
				}

				p.Bot.SendRaw(fmt.Sprintf("MODE %s -v %s", channelName, nick))
				p.Bot.CleanupWhoisChannel(nick)
			}(targetNick)
		}

		return fmt.Sprintf("📍 Lokális VIP jog törlése folyamatban %d felhasználótól (%s)...", len(targetNicks), channelName)
	}
}