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
	"time"
	"sync"

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

func (p *YnmAdminPlugin) sendChannelsWithPrefixes(issuingChannel string) {
    channels, err := p.Db.GetAllChannels()
    if err != nil || len(channels) == 0 {
        p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :Nincs csatorna adat.", issuingChannel))
        return
    }

    out := make([]string, 0, len(channels))
    for _, ch := range channels {
        prefix := p.Bot.GetMyPrefix(ch)
        if prefix == "" {
            prefix = "-"
        }
        out = append(out, fmt.Sprintf("%s(%s)", ch, prefix))
    }

    p.Bot.SendRaw(fmt.Sprintf("PRIVMSG %s :A bot jelenlegi csatornái: %s", issuingChannel, strings.Join(out, ", ")))
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

	// ✅ JAVÍTÁS: Nincs szükség type assertion-re
	needsRefresh := false
	
	for _, ch := range channels {
		if !p.Bot.IsWhoCacheFresh(ch, 10*time.Second) {
			needsRefresh = true
			break
		}
	}
	
	if !needsRefresh {
		// Cache is fresh, instant response
		p.sendChannelsWithPrefixes(replyTo)
		return ""
	}

	// ✅ WHO refresh
	var wg sync.WaitGroup
	for i, ch := range channels {
		wg.Add(1)
		
		go func(channel string, delay int) {
			defer wg.Done()
			time.Sleep(time.Duration(delay) * 150 * time.Millisecond)
			p.Bot.WaitForWhoResponse(channel, 2*time.Second)
		}(ch, i)
	}
	
	done := make(chan bool)
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
	
	p.sendChannelsWithPrefixes(replyTo)
	return ""
}