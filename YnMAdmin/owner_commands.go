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
	"log"
	_ "github.com/mattn/go-sqlite3"
//	"git.ynm.hu/markus/YnM-Go/YnMConfig"
//	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMLang"
//	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

func (p *YnmAdminPlugin) handleSetCommand(fullHostmask, field, value string) string {
    hostmask := YnMModule.SimplifyHostmask(fullHostmask)
    effectiveUser, _ := p.GetEffectiveUser(fullHostmask)
    _ = effectiveUser 
 
    info, err := p.Db.GetUserInfoByHost(fullHostmask)
    if err != nil || info == nil {
        return p.GetMessage(fullHostmask, YnMLang.MsgNotRegistered)
    }
    
    if !p.HasAccessWithSession(fullHostmask, "adduser") {
        return p.GetMessage(fullHostmask, YnMLang.MsgNoPermission)
    }

	switch field {
	case "char":
		if len(value) != 1 {
			return p.GetMessage(fullHostmask, YnMLang.MsgCharOneCharOnly)
		}

		err := p.Db.UpdateMyCharByHost(hostmask, value)
		if err != nil {
			return p.GetMessage(fullHostmask, YnMLang.MsgUpdateError, "char", err.Error())
		}
		return p.GetMessage(fullHostmask, YnMLang.MsgCharUpdated, value)

	case "welcome":
		err = p.Db.UpdateUserFieldByHost(hostmask, "welcome", value)
		if err != nil {
			return p.GetMessage(fullHostmask, YnMLang.MsgUpdateError, "welcome", err.Error())
		}
		return p.GetMessage(fullHostmask, YnMLang.MsgWelcomeUpdated, value)

	case "pass":
		err = p.Db.UpdateUserFieldByHost(hostmask, "pass", value)
		if err != nil {
			return p.GetMessage(fullHostmask, YnMLang.MsgUpdateError, "pass", err.Error())
		}
		return p.GetMessage(fullHostmask, YnMLang.MsgPassUpdated, value)

	case "lang":
		allowedLangs := map[string]bool{"En": true, "Ro": true, "Hu": true}
		if !allowedLangs[value] {
			return p.GetMessage(fullHostmask, YnMLang.MsgInvalidLanguage)
		}
		err = p.Db.UpdateUserFieldByHost(hostmask, "lang", value)
		if err != nil {
			return p.GetMessage(fullHostmask, YnMLang.MsgUpdateError, "lang", err.Error())
		}
		return p.GetMessage(fullHostmask, YnMLang.MsgLangUpdated, value)

	default:
		return p.GetMessage(fullHostmask, YnMLang.MsgUnknownField)
	}
}

func (p *YnmAdminPlugin) handleInfoCommand(fullHostmask string, args ...string) string {
    // Hostmask alapú session kezelés
    effectiveUser, effectiveHost := p.GetEffectiveUser(fullHostmask)
    
    var infoNick string
    if len(args) > 0 {
        infoNick = args[0]
    } else {
        infoNick = effectiveUser // Use effective user for self-info
    }

    // Lekérdező info - TÉNYLEGES hostmask használata
    requesterInfo, err := p.Db.GetUserInfoByHost(effectiveHost)
    if err != nil || requesterInfo == nil {
        return "" // return p.GetMessage(fullHostmask, YnMLang.MsgNoInfo)
    }
	// Ha másik nicket adtak meg
	if len(args) > 0 && args[0] != "" {
		targetInfo, err := p.Db.GetUserInfoByNick(infoNick)
		if err != nil || targetInfo == nil {
			return p.GetMessage(fullHostmask, "Ez a felhasználó nem szerepel az adatbázisban")
		}

		if requesterInfo.Role != "vip" && requesterInfo.Role != "mod" && requesterInfo.Role != "admin" && requesterInfo.Role != "owner" {
			return p.GetMessage(fullHostmask, "Csak VIP vagy magasabb szint kérhet le más felhasználói adatot")
		}

		passDisplay := p.GetMessage(fullHostmask, YnMLang.MsgPasswordNotSet)
		if targetInfo.Pass != nil && *targetInfo.Pass != "" {
			passDisplay = p.GetMessage(fullHostmask, YnMLang.MsgPasswordSet)
		}

		channelsRoles, _ := p.Db.GetUserChannelsRoles(targetInfo.Hostmask)
		var channelsStr string
		for ch, role := range channelsRoles {
			channelsStr += fmt.Sprintf("%s: %s | ", ch, role)
		}
		if channelsStr == "" {
			channelsStr = "<none>"
		}

		email, err := p.Db.GetUserMail(targetInfo.Nick)
		if err == nil && email != "" {
			targetInfo.Email = &email  // & operátorral pointert adunk
		} else {
			targetInfo.Email = nil      // ha nincs e-mail, akkor nil
		}
		return p.GetMessage(fullHostmask, YnMLang.MsgInfoDisplay,
			targetInfo.Nick,
			targetInfo.Hostmask,
			targetInfo.AddedBy,
			targetInfo.Lang,
			nullToStr(targetInfo.MyChar),
			nullToStr(targetInfo.Welcome),
			passDisplay,
			targetInfo.Role,
			nullToStr(targetInfo.Email),
			targetInfo.CreatedAt.Format("2006-01-02 15:04:05"),
			channelsStr,
			p.Cfg.ConsoleChannel,
		)
	}

	// Saját info (using effective user)
	effectiveInfo, err := p.Db.GetUserInfoByNick(effectiveUser)
	if err != nil || effectiveInfo == nil {
		// Fallback to requester info if effective user not found
		effectiveInfo = requesterInfo
	}

	passDisplay := p.GetMessage(fullHostmask, YnMLang.MsgPasswordNotSet)
	if effectiveInfo.Pass != nil && *effectiveInfo.Pass != "" {
		passDisplay = p.GetMessage(fullHostmask, YnMLang.MsgPasswordSet)
	}

	// Saját csatorna-rangok
	channelsRoles, _ := p.Db.GetUserChannelsRoles(effectiveInfo.Hostmask)
	var channelsStr string
	for ch, role := range channelsRoles {
		channelsStr += fmt.Sprintf("%s: %s | ", ch, role)
	}
	if channelsStr == "" {
		channelsStr = "<none>"
	}

	email, err := p.Db.GetUserMail(effectiveInfo.Nick)
	if err == nil && email != "" {
		effectiveInfo.Email = &email
	} else {
		effectiveInfo.Email = nil
	}


	return p.GetMessage(fullHostmask, YnMLang.MsgInfoDisplay,
		effectiveInfo.Nick,
		effectiveInfo.Hostmask,
		effectiveInfo.AddedBy,
		effectiveInfo.Lang,
		nullToStr(effectiveInfo.MyChar),
		nullToStr(effectiveInfo.Welcome),
		passDisplay,
		effectiveInfo.Role,
		nullToStr(effectiveInfo.Email),
		effectiveInfo.CreatedAt.Format("2006-01-02 15:04:05"),
		channelsStr,
		p.Cfg.ConsoleChannel,
	)
}

func (p *YnmAdminPlugin) handleYnmCommand(fullHostmask string, message string) string {
	trimmedMessage := strings.TrimSpace(message)
	parts := strings.Fields(trimmedMessage)
	if len(parts) != 1 {
		return ""
	}
	if !strings.HasSuffix(parts[0], "ynm") {
		return ""
	}

	nick := strings.Split(fullHostmask, "!")[0]
	hostmask := YnMModule.SimplifyHostmask(fullHostmask)

	// Is user already owner?
	if p.Db.HasRoleByHost(hostmask, "owner") {
		return ""
	}

	// Is there already an owner?
	if p.Db.HasAnyowner() {
		return ""
	}

	// Register as owner (admin_users table)
	if err := p.Db.AddUserWithRole(nick, hostmask, "owner", p.Cfg, "YnM-Go", "YnM-Go"); err != nil {
		return p.GetMessage(nick, YnMLang.MsgRegisterError)
	}

	// Add owner to channels table if empty
	channel := strings.ToLower(p.Cfg.ConsoleChannel)
	if err := p.Db.AssignownerIfMissing(channel, nick, hostmask); err != nil {
		log.Printf("⚠️ Failed to set channel owner for %s: %v", channel, err)
	}
	
	// Feloldjuk a zárolást, mert már van owner
	key := p.Cfg.ConsoleKey
	p.Bot.SendRaw(fmt.Sprintf("MODE %s -k %s", channel, key))
	p.Bot.SendRaw(fmt.Sprintf("MODE %s +o %s", channel, nick))
	p.Bot.SendRaw(fmt.Sprintf("TOPIC %s :Welcome my New owner %s.", channel, nick))

	return p.GetMessage(nick, YnMLang.MsgNewowner, nick, time.Now().Format("2006-01-02 15:04:05"))
}