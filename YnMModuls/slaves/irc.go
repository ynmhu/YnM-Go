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

package main

import (
	"fmt"
	"log"
	"strings"
	"time"
	"encoding/json"

	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

// setupIRCHandlers - IRC eseménykezelők beállítása
func (sb *SlaveBot) setupIRCHandlers() {
    sb.ircClient.OnMessage = func(msg YnMIrC.Message) {
        
        // ✅ TOPIC UPDATER kezelés (ha engedélyezve van)
        if sb.topicUpdater != nil {
            sb.topicUpdater.HandleMessage(msg)
        }

        // ✅ KICK esemény kezelése
        if msg.Command == "KICK" {
            sb.handleKick(msg)
            return
        }

        // ✅ PRIVMSG kezelés
        if msg.Command != "PRIVMSG" {
            return
        }

        if msg.Channel == "" || msg.Text == "" {
            return
        }

        log.Printf("[%s] 💬 Processing PRIVMSG: %s -> %s",
            sb.config.Name, msg.Nick, msg.Text)

        sb.handlePrivmsg(msg)
    }

    // Kapcsolódás után csatornákhoz csatlakozás
    go func() {
        time.Sleep(3 * time.Second)
        sb.joinChannels()
    }()
}


// handleKick - KICK esemény kezelése
func (sb *SlaveBot) handleKick(msg YnMIrC.Message) {
	// KICK formátum: :nick!user@host KICK #channel kicked_nick :reason
	if len(msg.Params) < 2 {
		return
	}

	kickedNick := msg.Params[1]
	channel := msg.Params[0]

	// Ellenőrizzük, hogy minket rúgtak-e ki
	if kickedNick == sb.config.Nickname {
		reason := msg.Text
		if reason == "" {
			reason = "no reason"
		}

		log.Printf("[%s] 🦵 KICKED from %s (reason: %s) - rejoining in 3 seconds",
			sb.config.Name, channel, reason)

		// Várunk 3 másodpercet, majd visszacsatlakozunk
		go func(ch string) {
			time.Sleep(3 * time.Second)

			if sb.ircClient != nil && sb.running {
				log.Printf("[%s] 🔄 Auto-rejoining %s", sb.config.Name, ch)
				sb.ircClient.Join(ch)
				log.Printf("[%s] ✅ Rejoined %s", sb.config.Name, ch)
			}
		}(channel)
	}
}


// handlePrivmsg - PRIVMSG kezelése
func (sb *SlaveBot) handlePrivmsg(msg YnMIrC.Message) {
    // ✅ EGYSZER logoljunk
    log.Printf("[%s] 💬 Message from %s in %s: %s", 
        sb.config.Name, msg.Nick, msg.Channel, msg.Text)
    
    // ✅ ELŐRE ellenőrizzük a bot üzeneteket
    isKnownBot := false
    knownBotPrefixes := []string{"YnM-", "BT-", "Q-"}
    
    for _, prefix := range knownBotPrefixes {
        if strings.HasPrefix(msg.Nick, prefix) {
            isKnownBot = true
            break
        }
    }
    
    if strings.Contains(msg.Sender, "irc.") {
        isKnownBot = true
    }
    
    if isKnownBot {
        log.Printf("[%s] 🔇 Bot message ignored: %s", sb.config.Name, msg.Nick)
        return
    }

    // ✅ PRIVÁT ÜZENET DETEKTÁLÁSA
    isPrivate := !strings.HasPrefix(msg.Channel, "#")
    target := msg.Channel
    
    user := msg.Nick
    message := msg.Text
    hostmask := msg.Sender
    if hostmask == "" {
        hostmask = fmt.Sprintf("%s!%s@unknown", user, strings.ToLower(user))
    }

    // ✅ SESSION FRISSÍTÉS - minden üzenetnél
    sb.UpdateLastSeen(msg.Sender)
    
    // ✅ HA PRIVÁT ÜZENET A SLAVE BOTNAK, TOVÁBBÍTJUK A MASTERNEK
    if isPrivate {
        if target == sb.config.Nickname {
            log.Printf("[%s] 🔷 Private message forwarding to master", sb.config.Name)
            sb.forwardToMaster(target, user, hostmask, message)
        } else {
            log.Printf("[%s] 🔇 Ignoring private message to other target: %s", sb.config.Name, target)
        }
        return
    }
    
    // ✅ LOGIN PARANCS KEZELÉSE - minden csatornán
    if strings.HasPrefix(message, "!login") || strings.HasPrefix(message, "login") {
        log.Printf("[%s] 🔐 Login command detected: %s", sb.config.Name, msg.Nick)
        sb.forwardToMaster(msg.Channel, msg.Nick, msg.Sender, message)
        return
    }
	
	
	    // ✅ HELP PARANCS - lekérés a master-től
		if strings.HasPrefix(strings.ToLower(message), "!help") {
			log.Printf("[%s] 📖 Help command from %s", sb.config.Name, msg.Nick)
			
			// Egyszerűen küldjük el a masternek, ő fogja ellenőrizni a jogosultságot
			sb.requestHelpFromMaster(msg.Channel, msg.Text, msg.Nick, msg.Sender)
			return
		}
    
    // ✅ LOGOUT PARANCS KEZELÉSE
    if strings.HasPrefix(message, "!logout") {
        session := sb.GetSession(msg.Sender)
        if session != nil && session.LoggedIn {
            sb.DeleteSession(msg.Sender)
            sb.ircClient.SendMessage(msg.Channel, 
                fmt.Sprintf("%s: 🔐 Kijelentkeztél!", msg.Nick))
        } else {
            sb.ircClient.SendMessage(msg.Channel, 
                fmt.Sprintf("%s: 🔐 Nem voltál bejelentkezve!", msg.Nick))
        }
        return
    }
    
    // ✅ SESSION ELLENŐRZÉS PROTECTED PARANCSOKNÁL
    if sb.isProtectedCommand(message) && !sb.IsUserLoggedIn(msg.Sender) {
        // Session lekérése a master-től, ha nincs lokálisan
        if sb.GetSession(msg.Sender) == nil {
            sb.RequestSessionFromMaster(msg.Sender, msg.Channel)
        }
        
        //sb.ircClient.SendMessage(msg.Channel, 
        //fmt.Sprintf("%s: 🔐 Ehhez a parancshoz be kell jelentkezned! Használd: !login <jelszó>", msg.Nick))
        return
    }

    // ✅ SESSION INFO PARANCS
    if strings.HasPrefix(message, "!session") || strings.HasPrefix(message, "!whoami") {
        session := sb.GetSession(msg.Sender)
        var response string
        
        if session != nil && session.LoggedIn {
            response = fmt.Sprintf("🔐 Bejelentkezve: %s (since: %s)", 
                session.Username, session.LoginTime.Format("2006-01-02 15:04:05"))
        } else {
            response = "🔐 Nincs aktív session. Használd: !login <jelszó>"
        }
        
        // Privát vagy csatorna válasz
        if msg.Channel == sb.config.Nickname || !strings.HasPrefix(msg.Channel, "#") {
            sb.ircClient.SendMessage(msg.Nick, response)
        } else {
            sb.ircClient.SendMessage(msg.Channel, fmt.Sprintf("%s: %s", msg.Nick, response))
        }
        return
    }

    // ✅ CSAK CSATORNA ÜZENETEKET TOVÁBBÍTUNK
    if sb.standalone || sb.masterConn == nil {
        log.Printf("[%s] 🔶 Standalone mode - handling locally", sb.config.Name)
        sb.handleStandaloneCommand(target, user, message)
    } else {
        log.Printf("[%s] 🔷 Master mode - forwarding to master", sb.config.Name)
        sb.forwardToMaster(target, user, hostmask, message)
    }
}



// handleStandalonePrivate - Standalone privát üzenetek kezelése
func (sb *SlaveBot) handleStandalonePrivate(user, message string) {
    if !strings.HasPrefix(message, "!") {
        return
    }

    parts := strings.Fields(message)
    if len(parts) == 0 {
        return
    }

    cmd := strings.TrimPrefix(parts[0], "!")

    switch cmd {
    case "status":
        statusMsg := fmt.Sprintf("🤖 Online | 🔶 Standalone Mode | Master: 🔴 Offline")
        sb.ircClient.SendMessage(user, statusMsg)

    default:
        unknownMsg := fmt.Sprintf("Master offline - try !status ")
        sb.ircClient.SendMessage(user, unknownMsg)
    }
}
// joinChannels - Csatornákhoz csatlakozás
func (sb *SlaveBot) joinChannels() {
	if sb.ircClient == nil {
		return
	}

	log.Printf("[%s] 🔌 Joining channels...", sb.config.Name)
	for _, channel := range sb.config.Channels {
		sb.ircClient.Join(channel)
		log.Printf("[%s] ✅ Joined channel: %s", sb.config.Name, channel)
		time.Sleep(500 * time.Millisecond) // ✅ FLOOD VÉDELEM
	}
}

// ircConnectionManager - IRC kapcsolat monitorozása és újracsatlakozás
func (sb *SlaveBot) ircConnectionManager() {
	reconnectDelay := 10 * time.Second
	maxReconnectDelay := 300 * time.Second

	for sb.running {
		isConnected := sb.checkRealIRCConnection()

		if !isConnected {
			log.Printf("[%s] 🔌 IRC CONNECTION LOST - attempting reconnect", sb.config.Name)
			sb.reconnecting = true

			time.Sleep(reconnectDelay)

			log.Printf("[%s] 🔌 Reconnecting to IRC...", sb.config.Name)

			// ✅ ÚJ IRC KLIENS LÉTREHOZÁSA
			ircConfig := &YnMConfig.Config{
				Server:                sb.config.Server,
				Port:                  fmt.Sprintf("%d", sb.config.Port),
				UseTLS:                sb.config.SSL,
				NickName:              sb.config.Nickname,
				UserName:              sb.config.Username,
				RealName:              sb.config.Realname,
				Channels:              sb.config.Channels,
				ReconnectOnDisconnect: 5,
				UseSASL:               false,
				Version:               "YnM-Go Slave Bot 1.0",
			}

			// Régi kliens leállítása
			if sb.ircClient != nil {
				sb.ircClient.Disconnect()
			}

			// Új kliens
			sb.ircClient = YnMIrC.NewClient(ircConfig)
			sb.ircClient = YnMIrC.NewClient(ircConfig)
			sb.ircClient.SetChannelModeHandler(&YnMIrC.EmptyChannelModeHandler{})
			sb.setupIRCHandlers()

			if err := sb.ircClient.Connect(); err != nil {
				log.Printf("[%s] ❌ IRC reconnect failed: %v", sb.config.Name, err)
				reconnectDelay *= 2
				if reconnectDelay > maxReconnectDelay {
					reconnectDelay = maxReconnectDelay
				}
			} else {
				log.Printf("[%s] ✅ IRC reconnected successfully", sb.config.Name)
				reconnectDelay = 10 * time.Second

				time.Sleep(3 * time.Second)
				sb.joinChannels()
			}
			sb.reconnecting = false
		}

		time.Sleep(15 * time.Second)
	}
}

// checkRealIRCConnection - IRC kapcsolat ellenőrzése
func (sb *SlaveBot) checkRealIRCConnection() bool {
	if sb.ircClient == nil {
		log.Printf("[%s] 🔍 IRC check: client is nil", sb.config.Name)
		return false
	}

	return true
}

// checkIRCConnection - Egyszerű IRC kapcsolat ellenőrzés
func (sb *SlaveBot) checkIRCConnection() bool {
	return sb.ircClient != nil
}

// handleStandaloneCommand - Standalone parancsok kezelése
func (sb *SlaveBot) handleStandaloneCommand(channel, user, message string) {
	if !strings.HasPrefix(message, "!") {
		return
	}

	parts := strings.Fields(message)
	if len(parts) == 0 {
		return
	}

	cmd := strings.TrimPrefix(parts[0], "!")

	switch cmd {
	case "status":
		statusMsg := fmt.Sprintf("%s: 🤖 Online | 🔶 Standalone Mode | Master: 🔴 Offline", user)
		sb.ircClient.SendMessage(channel, statusMsg)

	case "uptime":
		// ✅ SLAVE UPTIME - FORWARD TO MASTER
		if sb.masterConn != nil && !sb.standalone {
			// Master van - továbbítás
			sb.forwardToMaster(channel, user, "", "!uptime")
		} else {
			// Standalone mód - egyszerű válasz
			sb.ircClient.SendMessage(channel, 
				fmt.Sprintf("%s: 🤖 Slave Bot | Master: 🔴 Offline | Uptime handled by master", user))
		}
	default:
		unknownMsg := fmt.Sprintf("%s: Master offline - try !status ", user)
		sb.ircClient.SendMessage(channel, unknownMsg)
	}
}
 // handleLoginCommand - Login parancs kezelése
func (sb *SlaveBot) handleLoginCommand(channel, nick, hostmask, message string) {
    parts := strings.Fields(message)
    if len(parts) < 2 {
        sb.ircClient.SendMessage(channel, "Használat: !login <jelszó>")
        return
    }
    
    password := parts[1]
    sb.SendLoginAttempt(hostmask, channel, nick, password)
    sb.ircClient.SendMessage(channel, fmt.Sprintf("%s: 🔐 Bejelentkezés folyamatban...", nick))
}

// handleLogoutCommand - Logout parancs kezelése
func (sb *SlaveBot) handleLogoutCommand(channel, nick, hostmask string) {
    sb.DeleteSession(hostmask)
    sb.sendSessionRequest("logout", hostmask, channel, nick, nil)
    sb.ircClient.SendMessage(channel, fmt.Sprintf("%s: 🔐 Kijelentkeztél!", nick))
}

// isProtectedCommand - Védett parancsok ellenőrzése
func (sb *SlaveBot) isProtectedCommand(message string) bool {
    protectedCommands := []string{"!op", "!voice", "!kick", "!ban", "!mode"}
    
    for _, cmd := range protectedCommands {
        if strings.HasPrefix(message, cmd) {
            return true
        }
    }
    return false
}
// requestHelpFromMaster - Help lekérése a master-től
func (sb *SlaveBot) requestHelpFromMaster(channel, text, user, hostmask string) {
    if sb.masterConn == nil {
        sb.ircClient.SendMessage(channel, "❌ Master nem elérhető")
        return
    }
    
    helpMsg := MasterMessage{
        Type:    "help_request",
        BotName: sb.config.Name,
        Data: map[string]interface{}{
            "channel":   channel,
            "text":      text,
            "user":      user,
            "hostmask":  hostmask,
            "for_slave": true,
        },
    }
    data, _ := json.Marshal(helpMsg)
    sb.masterConn.Write(append(data, '\n'))
    log.Printf("[%s] 📤 Help request sent to master", sb.config.Name)
}