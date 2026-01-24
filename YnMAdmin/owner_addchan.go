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
	"log"
//	"time"

	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMLang"
)

// AddRoom csatorna hozzáadása csak ConsoleChannel-ból
func (p *YnmAdminPlugin) handleAddRoomCommand(fullHostmask string, newChannel string, issuingChannel string) string {
	nick := strings.Split(fullHostmask, "!")[0]
	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)

	info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
	if err != nil || info == nil {
		return "Nincs felhasználói info"
	}

	if info.Role != "owner" {
		return "Csak owner tud csatornát hozzáadni"
	}
	if issuingChannel != "" && strings.ToLower(issuingChannel) != strings.ToLower(p.Cfg.ConsoleChannel) {
		return fmt.Sprintf("Csak a ConsoleChannel (%s) használható csatorna hozzáadására", p.Cfg.ConsoleChannel)
	}
	if !strings.HasPrefix(newChannel, "#") {
		return "Érvénytelen csatorna név"
	}
	newChannel = strings.ToLower(newChannel)
	if p.Db.ChannelExists(newChannel) {
		return fmt.Sprintf("A csatorna már létezik: %s", newChannel)
	}

	if err := p.Db.AddChannel(newChannel, nick, simplifiedHostmask, simplifiedHostmask); err != nil {
		log.Printf("Hiba csatorna hozzáadásakor: %v", err)
		return "Hiba az adatbázisban"
	}

	p.Bot.Join(newChannel)
	return fmt.Sprintf("Csatorna hozzáadva: %s", newChannel)
}

func (p *YnmAdminPlugin) handleRemoveRoomCommand(fullHostmask, channelName, issuingChannel string) string {
	nick := strings.Split(fullHostmask, "!")[0]
	simplifiedHostmask := YnMModule.SimplifyHostmask(fullHostmask)

	info, err := p.Db.GetUserInfoByHost(simplifiedHostmask)
	if err != nil || info == nil {
		return p.GetMessage(nick, YnMLang.MsgNoUserInfo)
	}

	if info.Role != "owner" {
		return p.GetMessage(nick, YnMLang.MsgownerOnly)
	}

	// Ellenőrzés: csak ConsoleChannel lehet
	if issuingChannel != "" && strings.ToLower(issuingChannel) != strings.ToLower(p.Cfg.ConsoleChannel) {
		// Lekérjük a ConsoleChannel owner nickjét
		channelInfo, err := p.Db.GetChannel(p.Cfg.ConsoleChannel)
		var ownerNick string
		if err != nil || channelInfo == nil || channelInfo.Owner == nil {
			ownerNick = "ismeretlen"
		} else {
			ownerNick = *channelInfo.Owner
		}

		return fmt.Sprintf(
			"Csak a ConsoleChannel (%s) használható csatorna törlésére, ha tényleg törölni akarod a botot, keresd meg az ownert: %s",
			p.Cfg.ConsoleChannel, ownerNick,
		)
	}
	if strings.EqualFold(channelName, p.Cfg.ConsoleChannel) {
		return fmt.Sprintf("A(z) %s csatorna a bot ConsoleChannel-je, nem törölhető!", p.Cfg.ConsoleChannel)
	}

	err = p.Db.RemoveChannel(channelName)
	if err != nil {
		return p.GetMessage(nick, YnMLang.MsgRemoveRoomError) + ": " + err.Error()
	}

	p.Bot.Part(channelName, "")

	return p.GetMessage(nick, YnMLang.MsgRoomRemoved, channelName)
}

func (p *YnmAdminPlugin) onEndOfWho(channel string) {
    chKey := strings.ToLower(strings.TrimSpace(channel))

    p.channelsMu.Lock()
    if p.channelsPending == nil {
        p.channelsMu.Unlock()
        return
    }

    // ha nem várjuk, ignore
	if _, ok := p.channelsPending[chKey]; !ok {
		p.channelsMu.Unlock()
		return
	}

    delete(p.channelsPending, chKey)

    // ha még van pending, várunk tovább
    if len(p.channelsPending) > 0 {
        p.channelsMu.Unlock()
        return
    }

    replyTo := p.channelsReplyTo
    p.channelsPending = nil
    p.channelsReplyTo = ""
    p.channelsMu.Unlock()

    // minden WHO kész -> küldjük a választ
    p.sendChannelsWithPrefixes(replyTo)
}

func (p *YnmAdminPlugin) sendChannelsWithPrefixes(target string) {
    channels := p.Bot.GetChannels()
    if len(channels) == 0 {
        p.Bot.SendMessage(target, "A bot jelenleg nincs csatornában.")
        return
    }

    botNick := p.Bot.GetNick()
    var channelList []string

    for _, ch := range channels {
        prefix := p.getBotPrefixInChannel(ch, botNick)
        
        if prefix != "" {
            channelList = append(channelList, fmt.Sprintf("%s(%s)", ch, prefix))
        } else {
            channelList = append(channelList, fmt.Sprintf("%s(-)", ch))
        }
    }

    msg := fmt.Sprintf("A bot jelenlegi csatornái: %s", strings.Join(channelList, ", "))
    p.Bot.SendMessage(target, msg)
}
// HandleChannelsCommand kezeli az !channels parancsot (csak owner)
func (p *YnmAdminPlugin) handleChannelsCommand(sender, issuingChannel string) string {
    nick := strings.Split(sender, "!")[0]
    replyTo := strings.TrimSpace(issuingChannel)
    if replyTo == "" {
        replyTo = nick
    }

    hostmask := YnMModule.SimplifyHostmask(sender)
    info, err := p.Db.GetUserInfoByHost(hostmask)
    if err != nil || info == nil {
        return ""
    }
    if info.Role != "owner" {
        return ""
    }

    channels, err := p.Db.GetAllChannels()
    if err != nil {
        p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :Hiba a csatornák lekérdezésekor: %s", replyTo, err.Error()))
        return ""
    }

    if len(channels) == 0 {
        p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :A bot jelenleg nincs egyetlen csatornában sem.", replyTo))
        return ""
    }

    // ✅ AZONNALI VÁLASZ - WHO NÉLKÜL!
    // A prefix már a NAMES-ből jön, amikor JOIN-oltunk
    p.sendChannelsWithPrefixes(replyTo)
    return ""
}

// ===== 2. JAVÍTOTT sendChannelsWithPrefixes =====



// ===== 3. getBotPrefixInChannel HELPER =====

func (p *YnmAdminPlugin) getBotPrefixInChannel(channel, botNick string) string {
    // 1️⃣ Először nézzük a NAMES cache-ből (handleNamesReply)
    prefix := p.Bot.GetMyPrefix(channel)
    if prefix != "" {
        return prefix
    }

    // 2️⃣ Ha nincs cache, nézzük a Channel.Users-ből
    modes := p.Bot.GetUserModes(channel, botNick)
    if modes == "" {
        return ""
    }

    // 3️⃣ Konvertáld mode -> prefix
    if strings.Contains(modes, "q") { return "~" }  // owner
    if strings.Contains(modes, "a") { return "&" }  // admin
    if strings.Contains(modes, "o") { return "@" }  // op
    if strings.Contains(modes, "h") { return "%" }  // halfop
    if strings.Contains(modes, "v") { return "+" }  // voice

    return ""
}