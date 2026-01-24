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
package YnMCmd

import (
	"fmt"
	"strings"
	"log"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type HelpPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
}

type CommandInfo struct {
	Desc     string
	MinLevel int // 0=guest, 1=vip, 2=mod, 3=admin, 4=owner
}

type CommandCategory struct {
	Name     string
	Commands map[string]CommandInfo  // ✅ Változtatás: string -> CommandInfo
}

// SLAVE PARANCSOK (lokálisan kezelve a slave-ben)
var slaveCommands = map[string]CommandInfo{
    "!login":   {Desc: "Bejelentkezés: !login <jelszó>", MinLevel: 0},
    "!logout":  {Desc: "Kijelentkezés: !logout", MinLevel: 0},
    "!session": {Desc: "Session info: !session vagy !whoami", MinLevel: 0},
    "!help":    {Desc: "Súgó: !help vagy !help <parancs>", MinLevel: 1},
    "!uptime":  {Desc: "A bot és a szerver futási idejének (uptime) kiírása.", MinLevel: 1},
    "!o":       {Desc: "Op adása: !op [nick] (admin)", MinLevel: 3},
    "!h":       {Desc: "Halfop adása: !halfop [nick] (mod)", MinLevel: 2},
    "!v":       {Desc: "Voice adása: !voice [nick] (vip)", MinLevel: 1},
}



// MASTER PARANCSOK (továbbítva a master-nek)
var commandList = []CommandCategory{
	{
		Name: "Felhasználói parancsok",
		Commands: map[string]CommandInfo{
			"!kell":      {Desc: "Pl. !kell Mennydörgés 2012 (az évjárat megadása fontos).", MinLevel: 1},
			"!keres":     {Desc: "Pl. !keres Harry Potter (megmutatja, milyen néven található meg az YnM Median).", MinLevel: 1},
			"!horoszkop": {Desc: "Pl. !horoszkop skorpio (kiírja a napi horoszkópot).", MinLevel: 1},
			"!ido":       {Desc: "Pl. !ido Marosvásárhely (kiírja az aktuális időjárást).", MinLevel: 1},
			"!hirek":     {Desc: "Pl. !hirek chatgpt (kiírja a legfrissebb híreket a témában).", MinLevel: 1},
			"!seen":      {Desc: "Pl. !seen Markus (kiírja, mikor és mit írt utoljára).", MinLevel: 1},
			"!xp":        {Desc: "Pl. !xp vagy !xptop (kiírja a legtöbbet beszélő felhasználót).", MinLevel: 1},
			"!ora":       {Desc: "Pl. !ora 12h Csirkét kell venni (12 óra múlva emlékeztetni fog rá).", MinLevel: 1},
			"!nevnap":    {Desc: "Pl. !nevnap (kiírja a napi névnapot).", MinLevel: 1},
			"!vicc":      {Desc: "Pl. !vicc (vicceket fog kiírni).", MinLevel: 1},
			"!gpt":         {Desc: "Pl. !gpt romania lakossaga (AI kereső)", MinLevel: 1},
			"!wiki":      {Desc: "Pl. !wiki Marosvásárhely (Wikipédia összefoglaló az adott szóhoz).", MinLevel: 1},
			"!info":      {Desc: "Pl. !info Harry Potter (megírja, elérhető-e az adott film az YnM Median, és miről szól).", MinLevel: 1},
		},
	},
	{
		Name: "Moderátor parancsok",
		Commands: map[string]CommandInfo{
			"!h":     {Desc: "Halfop adása: !halfop [nick] (mod)", MinLevel: 2},

		},
	},
	{
		Name: "Admin parancsok",
		Commands: map[string]CommandInfo{
			"!uptime":        {Desc: "A bot és a szerver futási idejének (uptime) kiírása.", MinLevel: 3},
			"!restart":       {Desc: "Újraindítja a botot.", MinLevel: 4},
			"!del":           {Desc: "Törli a kérésedet.", MinLevel: 4},
			"!ok":            {Desc: "Pl. !ok pin (jóváhagyja a megadott PIN-hez tartozó kérést).", MinLevel: 4},
			"!keresek":       {Desc: "Kiírja, milyen kérések vannak jelenleg.", MinLevel: 4},
			"!shell":         {Desc: "Terminálparancsok végrehajtása (admin).", MinLevel: 4},
			"!pin":           {Desc: "Kiírja a megadott PIN-hez tartozó kérést.", MinLevel: 4},
			"!mp3":           {Desc: "Ellenőrzi, tartalmaz-e jogvédett zenét a https://legszebbnotak.ynm.hu oldalon.", MinLevel: 4},
			"!cycle": {Desc: "Csatorna újracsatlakozás (mod)", MinLevel: 3},
			"!debugsessions": {Desc: "Session lista (admin).", MinLevel: 4},
			"!o":            {Desc: "Op adása: !o [nick] (admin).", MinLevel: 3},
			"!h":        {Desc: "Halfop adása: !h [nick] (mod).", MinLevel: 2},
			"!v":         {Desc: "Voice adása: !v [nick] (vip).", MinLevel: 1},
		},
	},
}

func NewHelpPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin) *HelpPlugin {
	p := &HelpPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
	}
	
	return p
}

func (p *HelpPlugin) HandleMessage(msg YnMIrC.Message) string {
	// ✅ SZERVER ÜZENETEK KISZŰRÉSE - LEGELSŐ ELLENŐRZÉS
	if YnMModule.IsServerMessage(msg.Sender) || YnMModule.IsServerHostmask(msg.Sender) {
		return ""
	}
	
	// ✅ CSAK AKKOR LOGOLUNK, ha NEM szerver üzenet
	log.Printf("🔍 HelpPlugin.HandleMessage: Nick=%s, Text=%s, Sender=%s", msg.Nick, msg.Text, msg.Sender)
	
	var nick, hostmask string
	
	if msg.Sender != "" {
		// IRC user
		nick = strings.Split(msg.Sender, "!")[0]
		simplifiedHostmask := YnMModule.SimplifyHostmask(msg.Sender)
		
		if session, exists := p.adminPlugin.GetSessionByHost(simplifiedHostmask); exists {
			hostmask = session.LoggedInHost
		} else {
			// Ha nincs session, használjuk az eredeti hostmask-ot
			hostmask = simplifiedHostmask
		}
		hostmask = p.adminPlugin.GetEffectiveHostmask(simplifiedHostmask)
	} else if msg.Nick != "" {
		// Discord user
		userInfo, err := p.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
		if err != nil {
			log.Printf("❌ HelpPlugin: Discord user %s not found in DB", msg.Nick)
			return ""
		}
		nick = userInfo.Nick
		hostmask = userInfo.Hostmask
	} else {
		log.Printf("❌ HelpPlugin: No sender or nick provided")
		return ""
	}
	
	prefix := p.adminPlugin.GetPrefixForHost(hostmask)
	log.Printf("🔍 HelpPlugin: nick=%s, hostmask=%s, prefix=%s", nick, hostmask, prefix)
	
	text := strings.ToLower(msg.Text)
	helpCmd := prefix + "help"
	
	// ✅ PONTOSABB ELLENŐRZÉS: Csak akkor help parancs, ha:
	// 1. A szöveg pontosan "!help" (nincs semmi utána)
	// 2. Vagy "!help " (van szóköz utána, pl. "!help kell")
	// 3. Vagy tartalmazza " !help" vagy "]!help" formában
	isHelpCommand := false
		
	if text == helpCmd {
		// Pontosan "!help" és semmi más
		isHelpCommand = true
	}

	if !isHelpCommand {
		log.Printf("🔍 HelpPlugin: Not a help command")
		return ""
	}
	
	log.Printf("🔍 HelpPlugin: Help command detected!")
	
	// ✅ JOGOSULTSÁG ELLENŐRZÉS - csak VIP (level 1) és felette használhatja
	minLevel := 1
	if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
		log.Printf("❌ HelpPlugin: User %s (host: %s) does not have required level %d", nick, hostmask, minLevel)
		return ""
	}
	log.Printf("✅ HelpPlugin: User %s has sufficient permissions (min level: %d)", nick, minLevel)
	
	// Kivesszük a parancs részét (pl. "!help kell" -> "kell")
	parts := strings.Fields(msg.Text)
	var commandName string
	for i, part := range parts {
		if strings.ToLower(part) == helpCmd && i+1 < len(parts) {
			commandName = strings.ToLower(parts[i+1])
			log.Printf("🔍 HelpPlugin: Requested command help for: %s", commandName)
			break
		}
	}
	
	// Ha konkrét parancsot kér
	if commandName != "" {
		// Először a slave parancsoknál keresünk
		for cmd, info := range slaveCommands {
			if strings.TrimPrefix(cmd, "!") == strings.TrimPrefix(commandName, "!") {
				log.Printf("✅ HelpPlugin: Found in slave commands: %s", cmd)
				return fmt.Sprintf("%s - %s", cmd, info.Desc)
			}
		}
		
		// Aztán a master parancsoknál
		for _, category := range commandList {
			for cmd, info := range category.Commands {
				if strings.TrimPrefix(cmd, "!") == strings.TrimPrefix(commandName, "!") {
					log.Printf("✅ HelpPlugin: Found in master commands: %s", cmd)
					return fmt.Sprintf("%s - %s", cmd, info.Desc)
				}
			}
		}
		
		log.Printf("❌ HelpPlugin: Command not found: %s", commandName)
		return fmt.Sprintf("Ismeretlen parancs: %s (használd: !help)", commandName)
	}
	
	// ===== ÁLTALÁNOS !help =====
	log.Printf("📋 HelpPlugin: Displaying general help")
	
	// ✅ LEKÉRJÜK A FELHASZNÁLÓ SZINTJÉT
	userLevel := p.adminPlugin.GetUserLevel(nick, hostmask, msg.Channel)
	log.Printf("👤 User %s level: %d", nick, userLevel)
	
	// Ha p.bot != nil, akkor master környezetben vagyunk
	if p.bot != nil {

		// MASTER: Szintek szerint szűrjük a parancsokat
		var vipCmds, modCmds, adminCmds, ownerCmds []string

		// VIP parancsok (szint 1)
		if userLevel >= 1 {
			for _, category := range commandList {
				for cmd, info := range category.Commands {
					if info.MinLevel == 1 {
						vipCmds = append(vipCmds, strings.TrimPrefix(cmd, "!"))
					}
				}
			}
		}

		// MOD parancsok (szint 2)
		if userLevel >= 2 {
			for _, category := range commandList {
				for cmd, info := range category.Commands {
					if info.MinLevel == 2 {
						modCmds = append(modCmds, strings.TrimPrefix(cmd, "!"))
					}
				}
			}
		}

		// ADMIN parancsok (szint 3)
		if userLevel >= 3 {
			for _, category := range commandList {
				for cmd, info := range category.Commands {
					if info.MinLevel == 3 {
						adminCmds = append(adminCmds, strings.TrimPrefix(cmd, "!"))
					}
				}
			}
		}

		// owner parancsok (szint 4)
		if userLevel >= 4 {
			for _, category := range commandList {
				for cmd, info := range category.Commands {
					if info.MinLevel == 4 {
						ownerCmds = append(ownerCmds, strings.TrimPrefix(cmd, "!"))
					}
				}
			}
		}

		var lines []string

		if len(vipCmds) > 0 {
			lines = append(lines, "VIP 📖 Parancsok: !"+strings.Join(vipCmds, " !"))
		}
		if len(modCmds) > 0 {
			lines = append(lines, "MOD 📖 Parancsok: !"+strings.Join(modCmds, " !"))
		}
		if len(adminCmds) > 0 {
			lines = append(lines, "ADMIN 📖 Parancsok: !"+strings.Join(adminCmds, " !"))
		}
		if len(ownerCmds) > 0 {
			lines = append(lines, "owner 📖 Parancsok: !"+strings.Join(ownerCmds, " !"))
		}

		lines = append(lines, "Részletek: !help <parancs> (pl. !help kell)")

		return strings.Join(lines, "~~~")
	} else {
		// SLAVE: Szintek szerint szűrjük a slave parancsokat
		var availableCmds []string
		
		for cmd, info := range slaveCommands {
			if userLevel >= info.MinLevel {
				availableCmds = append(availableCmds, strings.TrimPrefix(cmd, "!"))
			}
		}
		
		if len(availableCmds) == 0 {
			return "Nincs elérhető parancsod."
		}
		
		line1 := "Elérhető parancsok: !" + strings.Join(availableCmds, " !")
		line2 := "Részletek: !help <parancs> (pl. !help login)"
		
		return line1 + "~~~" + line2
	}
}

func (p *HelpPlugin) OnTick() []YnMIrC.Message {
	return nil
}