package owner

import (
	"fmt"
	"strings"
	"regexp"
	_ "github.com/mattn/go-sqlite3"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

// HandleModeCommand kezeli a !mode parancsot
func (p *YnmAdminPlugin) HandleModeCommand(nick, hostmask, channel, message string) {
	prefix := p.GetPrefixForHost(hostmask)
	if !strings.HasPrefix(message, prefix+"mode ") {
		return
	}

	modeArgs := strings.TrimPrefix(message, prefix+"mode ")
	modeArgs = strings.TrimSpace(modeArgs)
	
	// Ellenőrizzük, hogy üres-e a parancs
	if modeArgs == "" {
		p.Bot.SendMessage(channel, "Helytelen mode formátum. Használd: !mode +nt vagy !mode #channel +nt")
		return
	}

	modeRegex := regexp.MustCompile(`^(?:(#\S+)\s+)?([+-][a-zA-Z+-]+)(?:\s+(.*))?$`)
	matches := modeRegex.FindStringSubmatch(modeArgs)
	
	if len(matches) < 3 {
		p.Bot.SendMessage(channel, "Helytelen mode formátum. Használd: !mode +nt, !mode #channel +nt, vagy !mode +nts-mi")
		return
	}
	
	// Meghatározzuk a target channelt
	var targetChannel string
	if matches[1] != "" {
		// Ha meg van adva specifikus channel
		targetChannel = matches[1]
	} else {
		// Ha nincs megadva channel, akkor az aktuális channelt használjuk
		targetChannel = channel
	}
	
	modeString := matches[2] // pl: +nt, -s, +nts-mi
	var modeParams string
	if len(matches) > 3 && matches[3] != "" {
		modeParams = matches[3] // További paraméterek, ha vannak (pl. nick-ek +o, +v esetén)
	}
	
	// Ellenőrizzük, hogy a felhasználó jogosult-e a parancs használatára a target channelen
	if !p.hasPermission(nick, hostmask, targetChannel) {
		//p.Bot.SendMessage(channel, fmt.Sprintf("Nincs jogosultságod a channel mode-ok módosítására a(z) %s channelben.", targetChannel))
		return
	}
	
	// Alkalmazzuk a mode-ot a target channelen
	var cmd string
	if modeParams != "" {
		cmd = fmt.Sprintf("MODE %s %s %s", targetChannel, modeString, modeParams)
	} else {
		cmd = fmt.Sprintf("MODE %s %s", targetChannel, modeString)
	}
	
	if err := p.Bot.SendRaw(cmd); err != nil {
		p.Bot.SendMessage(channel, "Hiba történt a mode alkalmazása során.")
		return
	}
	
	// Mentsük el az adatbázisba a meglévő SaveChannelModes metódussal
	if err := p.Db.SaveChannelModes(targetChannel, modeString, modeParams, nick); err != nil {
		// Log the error, de ne küldjünk üzenetet róla
		// log.Printf("Hiba a channel mode-ok mentése során: %v", err)
	}
	
	// Visszajelzés küldése
	if targetChannel != channel {
		// Ha más channelre alkalmaztuk, akkor jelezzük ezt
		p.Bot.SendMessage(channel, fmt.Sprintf("Channel mode beállítva és elmentve a(z) %s channelben: %s", targetChannel, modeString))
	} else {
		p.Bot.SendMessage(channel, fmt.Sprintf("Channel mode beállítva és elmentve: %s", modeString))
	}
}

// HandleClearModeCommand törli a mentett channel mode-okat
func (p *YnmAdminPlugin) HandleClearModeCommand(nick, hostmask, channel, message string) {
	prefix := p.GetPrefixForHost(hostmask)
	if !strings.HasPrefix(message, prefix+"clearmode") {
		return
	}

	// Ellenőrizzük a jogosultságot
	if !p.hasPermission(nick, hostmask, channel) {
		return
	}

	// Töröljük az adatbázisból a mentett mode-okat
	if err := p.Db.ClearChannelModes(channel); err != nil {
		p.Bot.SendMessage(channel, "Hiba történt a mode-ok törlésekor.")
		//log.Printf("❌ Error clearing channel modes: %v", err)
		return
	}

	// Levesszük az összes mode-ot a csatornáról
	p.Bot.SendRaw(fmt.Sprintf("MODE %s -mnstiklp", channel))	
	p.Bot.SendMessage(channel, "Channel mode-ok törölve. A csatorna most mode-ok nélkül van.")
//	log.Printf("✅ Channel modes cleared for %s by %s", channel, nick)
}
// hasPermission ellenőrzi, hogy a felhasználónak van-e joga channel mode-ok módosítására
func (p *YnmAdminPlugin) hasPermission(nick, hostmask, channel string) bool {
	hostmaskSimple := YnMModule.SimplifyHostmask(hostmask)
	
	// 1. Globális owner jogosultság ellenőrzése
	globalRole, err := p.Db.GetUserGlobalRole(nick, hostmaskSimple)
	if err == nil && globalRole == "owner" {
		return true
	}
	
	// 2. Channel owner jogosultság ellenőrzése
	isChannelowner, err := p.Db.IsChannelowner(nick, channel)
	if err == nil && isChannelowner {
		return true
	}
	
	// 3. Channel-specifikus szerepkör ellenőrzése (meglévő függvény használata)
	userChannelRole, err := p.Db.GetUserChannelRole(hostmaskSimple, channel)
	if err == nil && userChannelRole != nil {
		// Admin vagy magasabb szint jogosult
		if RoleHierarchy[userChannelRole.Role] >= RoleHierarchy["admin"] {
			return true
		}
	}
	
	// 4. Ha globális admin, akkor minden channelben jogosult
	if err == nil && globalRole == "owner" {
		return true
	}
	
	// 5. Fallback: ellenőrizzük a régi op rendszert
	isOp, err := p.Db.IsUserOp(nick, hostmaskSimple, channel)
	if err == nil && isOp {
		return true
	}
	
	return false
}

func (p *YnmAdminPlugin) ApplyStoredChannelModes(channel string) {
	p.Db.ApplySavedChannelModes(p.Bot, channel)
}