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

)

func (p *YnmAdminPlugin) SetChannelTopic(issuerFullHostmask, channel, topicText string) string {
    effectiveUser, effectiveHost := p.GetEffectiveUser(issuerFullHostmask)
    channelLower := strings.ToLower(channel)
    
    // Ellenőrizzük a jogosultságot (legalább VIP kell)
    issuerRole, _ := p.Db.GetUserRoleInChannel(effectiveUser, effectiveHost, channelLower)
    issuerLevel := RoleHierarchy[strings.ToLower(issuerRole)]
    
    if issuerLevel < RoleHierarchy["vip"] {
        return "Nincs jogosultságod a topic módosításához!"
    }
    
    // Mentés az adatbázisba
    err := p.Db.SetChannelTopic(channel, topicText, effectiveUser)
    if err != nil {
        fmt.Printf("[ERROR] Failed to save topic to database: %v\n", err)
        return "Hiba történt a topic mentése közben!"
    }
    
    // IRC parancs küldése
    p.Bot.SendRaw(fmt.Sprintf("TOPIC %s :%s", channel, topicText))
    
    return ""
}