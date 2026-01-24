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
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

func (p *YnmAdminPlugin) handleAddUserToChannelCommandFlexible(fullHostmask string, userArgs []string, issuingChannel string) string {
    // Hostmask egyszerűsítése
    simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
    info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
    if err != nil || info == nil {
        return "Nem található a felhasználó az adatbázisban"
    }

    if len(userArgs) < 3 {
        return "Használat: !ynm add user #csatorna nick role"
    }

    channelName := strings.ToLower(userArgs[0])
    targetNick := userArgs[1]
    role := strings.ToLower(userArgs[2])

	if !strings.HasPrefix(channelName, "#") || !p.Db.ChannelExists(channelName) {
		return fmt.Sprintf("Érvénytelen csatorna vagy nem létezik az adatbázisban: %s", channelName)
	}

    // Parancsot kiadó felhasználó nickje
    issuerNick := strings.Split(fullHostmask, "!")[0]

    // Ellenőrzés: parancsot kiadó felhasználó szerepe a csatornában
    roleInChannel, _ := p.Db.GetUserRoleInChannel(issuerNick, simplifiedHostmask, channelName)
    if roleInChannel != "owner" && roleInChannel != "admin" && roleInChannel != "mod" {
        return fmt.Sprintf("Csak az adott csatornában lévő owner/admin/mod adhat felhasználót: %s", channelName)
    }

    // Aszinkron WHOIS várás
    go func() {
        respChan := p.Bot.GetWhoisChannel(targetNick)
        p.Bot.RequestWhois(targetNick)
        whois := <-respChan
        if whois == nil {
            p.Bot.SendMessage(issuingChannel, fmt.Sprintf("Nem sikerült lekérni a hostmaskot: %s", targetNick))
            p.Bot.CleanupWhoisChannel(targetNick)
            return
        }

        targetHostmask := whois.Hostmask
        if targetHostmask == "" {
            targetHostmask = fmt.Sprintf("%s!%s@%s", whois.Nick, whois.Username, whois.Hostname)
        }
        targetHostmask = YnMModule.SimplifyHostmask(targetHostmask)

        // Ha nincs a users táblában, hozzáadjuk
        existingUser, _ := p.Db.GetUserInfoByHost(targetHostmask)
        if existingUser == nil {
            if err := p.Db.AddUserWithRole(targetNick, targetHostmask, role, p.Cfg, issuerNick, simplifiedHostmask); err != nil {
                p.Bot.SendMessage(issuingChannel, fmt.Sprintf("Hiba felhasználó hozzáadásakor: %v", err))
                p.Bot.CleanupWhoisChannel(targetNick)
                return
            }
        }

        var autoOp, autoVoice, autoHalfop int
        switch role {
        case "admin":
            autoOp = 1
        case "vip":
            autoVoice = 1
        case "mod":
            autoHalfop = 1
        }

        // Hozzáadás csatornához, issuerNick lesz az addedBy
        if err := p.Db.AddUserToChannel(targetNick, targetHostmask, channelName, role, autoOp, autoVoice, autoHalfop, issuerNick, simplifiedHostmask); err != nil {
            p.Bot.SendMessage(issuingChannel, fmt.Sprintf("Hiba felhasználó csatornához adásakor: %v", err))
            p.Bot.CleanupWhoisChannel(targetNick)
            return
        }

        // MODE parancs a megfelelő jogokhoz
        switch role {
        case "admin":
            p.Bot.SendRaw(fmt.Sprintf("MODE %s +o %s", channelName, targetNick))
        case "mod":
            p.Bot.SendRaw(fmt.Sprintf("MODE %s +h-o %s %s", channelName, targetNick, targetNick))
        case "vip":
            p.Bot.SendRaw(fmt.Sprintf("MODE %s +voh %s %s %s", channelName, targetNick, targetNick, targetNick))
        }

        // Visszajelzés a parancsot kiadó csatornába
        p.Bot.SendMessage(issuingChannel, fmt.Sprintf("Felhasználó hozzáadva: %s -> %s (%s)", targetNick, channelName, role))

        p.Bot.CleanupWhoisChannel(targetNick)
    }()

    return fmt.Sprintf("Hozzáadás folyamatban a %s hostjával...", targetNick)
}


// RemoveUserFromChannelCommandFlexible eltávolít egy felhasználót a csatornából
func (p *YnmAdminPlugin) handleRemoveUserFromChannelCommandFlexible(fullHostmask string, userArgs []string) string {
	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)
	info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
	if err != nil || info == nil || info.Role != "owner" {
		return "Csak owner tud felhasználót eltávolítani"
	}

	if len(userArgs) < 2 {
		return "Használat: !ynm del user #csatorna nick"
	}
	channelName := strings.ToLower(userArgs[0])
	targetNick := userArgs[1]
	
	if !strings.HasPrefix(channelName, "#") || !p.Db.ChannelExists(channelName) {
		return "Érvénytelen csatorna vagy nem létezik"
	}
	
	// Ellenőrizzük, hogy a felhasználó benne van-e a csatornában
	targetHostmask, err := p.Db.GetUserHostmaskInChannel(targetNick, channelName)
	if err != nil || targetHostmask == "" {
		return fmt.Sprintf("A felhasználó %s nincs benne a csatornában %s", targetNick, channelName)
	}
	
	// Felhasználó törlése a csatornából
	if err := p.Db.RemoveUserFromChannel(channelName, targetNick, targetHostmask); err != nil {
		return fmt.Sprintf("Hiba felhasználó törlésekor: %v", err)
	}
	
	return fmt.Sprintf("Felhasználó eltávolítva: %s -> %s", targetNick, channelName)
}

func (p *YnmAdminPlugin) handleRemoveUserEverywhereCommand(fullHostmask string, targetNick string) string {
    simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)

    // 1. Csak owner indíthatja
    info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
    if err != nil || info == nil || info.Role != "owner" {
        return "Csak owner tud felhasználót eltávolítani"
    }

    // 2. Lekérdezzük a célnick adatait
    targetInfo, err := p.Db.GetUserInfoByNick(targetNick)
    if err != nil || targetInfo == nil {
        return fmt.Sprintf("A felhasználó %s nem található az adatbázisban.", targetNick)
    }

    // 3. owner-t nem szabad törölni – itt állítsuk le teljesen
    if targetInfo.Role == "owner" {
        return "owner felhasználót nem lehet eltávolítani."
    }

    // **Csak ekkor megyünk tovább, azaz már biztosan törölhető**

    channels, err := p.Db.GetAllChannels()
    if err != nil {
        return fmt.Sprintf("Hiba csatornák lekérésekor: %v", err)
    }

    var removedChannels []string

    // 4. Minden csatornából törlés + módok levétele
    for _, ch := range channels {
        targetHostmask, _ := p.Db.GetUserHostmaskInChannel(targetNick, ch)
        if targetHostmask != "" {

            // Módok levétele csak akkor, ha törölhető – és itt már az
            p.Bot.SendRaw(fmt.Sprintf("MODE %s -ohv %s %s %s", ch, targetNick, targetNick, targetNick))

            if err := p.Db.RemoveUserFromChannel(ch, targetNick, targetHostmask); err == nil {
                removedChannels = append(removedChannels, ch)
            }
        }
    }

    // 5. Végül törlés a users táblából
    if err := p.Db.DeleteUser(targetNick); err != nil {
        return fmt.Sprintf("Hiba a felhasználó törlésekor a users táblából: %v", err)
    }

    // 6. Visszajelző üzenet
    msg := fmt.Sprintf("A felhasználó %s törölve lett a users táblából.", targetNick)
    if len(removedChannels) > 0 {
        msg += fmt.Sprintf(" Csatornákból eltávolítva: %v", removedChannels)
    }

    return msg
}
