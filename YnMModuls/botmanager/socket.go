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

package botmanager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
	"path/filepath"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModule"

	
)

// startSocketServer elindítja a Unix socket szervert
func (bm *BotManagerPlugin) startSocketServer() {
	log.Printf("🔌 [1] startSocketServer() ELINDULT! Socket path: %s", bm.socketPath)
	defer log.Printf("🔌 [6] startSocketServer() BEFEJEZVE")

	socketDir := filepath.Dir(bm.socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		log.Printf("❌ [2a] Socket könyvtár létrehozási hiba: %v", err)
		return
	}

	if err := os.Remove(bm.socketPath); err != nil && !os.IsNotExist(err) {
		log.Printf("⚠️ [3a] Socket törlése sikertelen: %v", err)
	}

	listener, err := net.Listen("unix", bm.socketPath)
	if err != nil {
		log.Printf("❌❌❌ [4a] SZERVEZŐ HIBA: Socket szerver indítási hiba: %v", err)
		return
	}
	bm.listener = listener
	
	log.Printf("✅✅✅ [4b] Socket szerver sikeresen létrehozva: %s", bm.socketPath)

	if err := os.Chmod(bm.socketPath, 0666); err != nil {
		log.Printf("⚠️ [5a] Socket jogosultság beállítási hiba: %v", err)
	}

	go bm.acceptConnections()
}

func (bm *BotManagerPlugin) acceptConnections() {
	for {
		select {
		case <-bm.stopChan:
			log.Printf("⏹️ Socket szerver leállítva")
			return
		default:
			if unixListener, ok := bm.listener.(*net.UnixListener); ok {
				unixListener.SetDeadline(time.Now().Add(100 * time.Millisecond))
			}
			
			conn, err := bm.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if !strings.Contains(err.Error(), "use of closed network connection") {
					log.Printf("⚠️ Kapcsolat elfogadási hiba: %v", err)
				}
				continue
			}
			
			go bm.handleSlaveConnection(conn)
		}
	}
}

// handleSlaveConnection kezeli a slave bot kapcsolatokat
func (bm *BotManagerPlugin) handleSlaveConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("🔗 Új slave kapcsolat: %s", remoteAddr)

	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := strings.TrimSpace(scanner.Text())
		if message != "" {
			log.Printf("📨 Üzenet a slave-től (%s): %s", remoteAddr, message)
			bm.processSlaveMessage(conn, message)
		}
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	}

	if err := scanner.Err(); err != nil {
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "closed") {
			log.Printf("❌ Olvasási hiba (%s): %v", remoteAddr, err)
		}
	}

	log.Printf("🔌 Kapcsolat bontva: %s", remoteAddr)
}


func (bm *BotManagerPlugin) getSlaveUserHostmask(botName, channel, nick string) string {
	return fmt.Sprintf("%s!%s@%s", nick, strings.ToLower(nick), botName)
}


func (bm *BotManagerPlugin) processSlaveMessage(conn net.Conn, message string) {
    var msg struct {
        Type     string      `json:"type"`
        BotName  string      `json:"bot_name"`
        Channel  string      `json:"channel"`
        User     string      `json:"user"`
        Hostmask string      `json:"hostmask"`
        Message  string      `json:"message"`
        Source   string      `json:"source"`
        Data     interface{} `json:"data"`
        Action   string      `json:"action"`
    }

    if err := json.Unmarshal([]byte(message), &msg); err != nil {
        log.Printf("❌ JSON parse hiba: %v", err)
        return
    }

    switch msg.Type {
    case "register":
        bm.registerSlaveConnection(msg.BotName, conn)
		
// A master BotManager-ben, ahol a help_request-et kezeli:

	case "help_request":
		channel := ""
		text := ""
		user := ""
		hostmask := ""
		forSlave := false
    
    if dataMap, ok := msg.Data.(map[string]interface{}); ok {
        if ch, ok := dataMap["channel"].(string); ok {
            channel = ch
        }
        if txt, ok := dataMap["text"].(string); ok {
            text = txt
        }
        if u, ok := dataMap["user"].(string); ok {
            user = u
        }
        if h, ok := dataMap["hostmask"].(string); ok {
            hostmask = h
        }
        if fs, ok := dataMap["for_slave"].(bool); ok {
            forSlave = fs
        }
    }
    
    log.Printf("📖 Help request from slave: %s (channel: %s, user: %s, for_slave: %v)", 
        msg.BotName, channel, user, forSlave)
    
    var response string
    // Ellenőrizzük, hogy konkrét parancsot kértek-e
    parts := strings.Fields(text)
    hasSpecificCommand := len(parts) >= 2
    
    if forSlave && !hasSpecificCommand {
        // ✅ SLAVE általános help - a HelpPlugin fogja generálni, DE ellenőrizzük a jogot
        log.Printf("📖 Slave general help - delegating to HelpPlugin")
        
        if bm.pluginManager != nil {
            ircMsg := YnMIrC.Message{
                Nick:    user,
                Channel: channel,
                Text:    text,
                Command: "PRIVMSG",
                Sender:  hostmask,
            }
            
            // A HelpPlugin ellenőrzi a jogosultságot ÉS generálja a választ
            response = bm.pluginManager.HandleMessage(ircMsg)
            
            // Ha üres a válasz, akkor nincs joga
            if response == "" {
                log.Printf("❌ User %s has no permission (HelpPlugin returned empty)", user)
                return
            }
            
            log.Printf("✅ HelpPlugin generated response for slave")
            // ❌ NE ÍRJUK FELÜL! A HelpPlugin már generálta a helyes választ
        }
    } else {
        // MASTER válasz VAGY konkrét parancs help (plugin manager-en keresztül)
        log.Printf("📖 Generating help response via pluginManager (specific or master)")
        if bm.pluginManager != nil {
            ircMsg := YnMIrC.Message{
                Nick:    user,
                Channel: channel,
                Text:    text,
                Command: "PRIVMSG",
                Sender:  hostmask,
            }
            
            log.Printf("🔍 Calling pluginManager.HandleMessage with: Nick=%s, Channel=%s, Text=%s", user, channel, text)
            response = bm.pluginManager.HandleMessage(ircMsg)
            log.Printf("🔍 Response from pluginManager: %q (length: %d)", response, len(response))
        } else {
            log.Printf("❌ Plugin manager is nil!")
        }
    }
    
    if response != "" {
        log.Printf("📤 Sending response to slave...")
        bm.sendMultiLineResponse(msg.BotName, channel, "", response)
    } else {
        log.Printf("⚠️ Response is empty - user has no permission or command not found")
    }
    case "message":
        log.Printf("📩 [%s/%s] %s: %s", 
            msg.BotName, msg.Channel, msg.User, msg.Message)
        
        // ✅ SZERVER ÜZENETEK SZŰRÉSE
        if msg.User == "irc.ynm.hu" || strings.Contains(msg.Hostmask, "irc.ynm.hu") {
            return
        }
        
        // ✅ CSAK ISMERT BOTOK ÜZENETEIT SZŰRJÜK
        isKnownBot := false
        knownBotPrefixes := []string{"YnM-", "BT-", "Q-"}
        
        for _, prefix := range knownBotPrefixes {
            if strings.HasPrefix(msg.User, prefix) {
                isKnownBot = true
                break
            }
        }
        
        if isKnownBot {
            log.Printf("🔇 Bot üzenet ignorálva: %s", msg.User)
            return
        }
        
        // ✅ LOGOUT PARANCS KEZELÉSE (mindig megengedett)
        if strings.HasPrefix(msg.Message, "!logout") || strings.HasPrefix(msg.Message, "logout") {
            log.Printf("🔐 LOGOUT PARANCS ÉSZLELVE: %s -> %s", msg.User, msg.Message)
            bm.handleLogoutCommand(msg.BotName, msg.Channel, msg.User, msg.Hostmask)
            return
        }
        
        // ✅ LOGIN PARANCS KEZELÉSE (mindig megengedett)
        if strings.HasPrefix(msg.Message, "!login") || strings.HasPrefix(msg.Message, "login") {
            log.Printf("🔐 LOGIN PARANCS ÉSZLELVE: %s -> %s", msg.User, msg.Message)
            bm.handleLoginCommand(msg.BotName, msg.Channel, msg.User, msg.Hostmask, msg.Message)
            return
        }
        
        // ✅ PRIVÁT ÜZENET DETEKTÁLÁSA
        isPrivate := !strings.HasPrefix(msg.Channel, "#")
        
        // ✅ CSAK CSATORNA ÜZENETEKET TOVÁBBÍTUNK
        if isPrivate {
            log.Printf("🔇 [SOCKET] Privát üzenet - nincs továbbítás: %s", msg.Message)
            return
        }
        
		// ============================================================
		// ✅ AUTHENTICATION CHECK - ISMERT PARANCSOKHOZ
		// ============================================================
		knownCommands := []string{"debugsessions", "cycle", "uptime", "op", "o", "halfop", "h", "voice", "v", "bot"}

		if strings.HasPrefix(msg.Message, "!") {
			commandParts := strings.Fields(msg.Message)
			if len(commandParts) > 0 {
				commandName := strings.TrimPrefix(commandParts[0], "!")
				
				// Csak ismert parancsokhoz kérjünk autentikációt
				requiresAuth := false
				for _, knownCmd := range knownCommands {
					if commandName == knownCmd {
						requiresAuth = true
						break
					}
				}
				
				if requiresAuth {
					simplifiedHostmask := YnMModule.SimplifyHostmask(msg.Hostmask)
					hasAuth := false
					authMethod := ""
					
					if bm.adminPlugin != nil {
						// 1. Ellenőrizzük van-e session
						if session, exists := bm.adminPlugin.GetSessionByHost(simplifiedHostmask); exists {
							hasAuth = true
							authMethod = fmt.Sprintf("session (%s)", session.LoggedInAs)
							log.Printf("✅ Auth OK: %s has %s", msg.User, authMethod)
						} else {
							// 2. Ellenőrizzük van-e hostmask-based jogosultság
							if bm.adminPlugin.HasAccess(simplifiedHostmask, commandName) {
								hasAuth = true
								authMethod = "hostmask"
								log.Printf("✅ Auth OK: %s has %s access for %s", msg.User, authMethod, commandName)
							}
						}
					}
					
					if !hasAuth {
						//log.Printf("❌ No authentication: %s (hostmask: %s) for command: %s", msg.User, simplifiedHostmask, commandName)
						//bm.sendReplyToSlave(msg.BotName, msg.Channel, 
						//	"❌ Nincs jogosultságod! Használd: !login <felhasználónév> <jelszó>")
						return
					}
				}
			}
		}

        // ============================================================
        
        // ✅ DEBUG SESSIONS COMMAND
        if strings.HasPrefix(msg.Message, "!debugsessions") {
            log.Printf("🔍 DEBUG: Dumping all sessions")
            if bm.adminPlugin != nil {
                bm.adminPlugin.DebugDumpSessions()
                sessionInfo := bm.adminPlugin.GetAllSessionInfo()
                for _, line := range sessionInfo {
                    bm.sendReplyToSlave(msg.BotName, msg.Channel, line)
                }
            } else {
                bm.sendReplyToSlave(msg.BotName, msg.Channel, "❌ Admin plugin not available")
            }
            return
        }
        
        // ✅ PARANCSOK ÉSZLELÉSE ÉS TOVÁBBÍTÁSA
        if strings.HasPrefix(msg.Message, "!cycle") {
            bm.handleCycleCommand(msg.BotName, msg.Channel, msg.User, msg.Hostmask, msg.Message)
            return
        }
        
        // ✅ UPTIME PARANCS KEZELÉSE
        if strings.HasPrefix(msg.Message, "!uptime") {
            log.Printf("⏱️ Uptime command from slave %s", msg.BotName)
            bm.handleSlaveUptimeCommand(msg.BotName, msg.Channel, msg.User)
            return
        }
        
        // ✅ OP PARANCS
        if strings.HasPrefix(msg.Message, "!op") || strings.HasPrefix(msg.Message, "!o ") || msg.Message == "!o" {
            log.Printf("👑 OP command detected: %s", msg.Message)
            bm.handleModeCommand(msg.BotName, msg.Channel, msg.User, msg.Hostmask, msg.Message, "o")
            return
        }
        
        // ✅ HALFOP PARANCS
        if strings.HasPrefix(msg.Message, "!halfop") || strings.HasPrefix(msg.Message, "!h ") || msg.Message == "!h" {
            log.Printf("👥 HALFOP command detected: %s", msg.Message)
            bm.handleModeCommand(msg.BotName, msg.Channel, msg.User, msg.Hostmask, msg.Message, "h")
            return
        }
        
        // ✅ VOICE PARANCS
        if strings.HasPrefix(msg.Message, "!voice") || strings.HasPrefix(msg.Message, "!v ") || msg.Message == "!v" {
            log.Printf("🔊 VOICE command detected: %s", msg.Message)
            bm.handleModeCommand(msg.BotName, msg.Channel, msg.User, msg.Hostmask, msg.Message, "v")
            return
        }
        
        // ✅ TOVÁBBI CSATORNA ÜZENETEK FELDOLGOZÁSA
        if bm.pluginManager != nil {
            hostmask := msg.Hostmask
            if hostmask == "" {
                hostmask = bm.getSlaveUserHostmask(msg.BotName, msg.Channel, msg.User)
            }
            
            ircMsg := YnMIrC.Message{
                Nick:    msg.User,
                Channel: msg.Channel,
                Text:    msg.Message,
                Command: "PRIVMSG",
                Sender:  hostmask,
            }
            
            response := bm.pluginManager.HandleMessage(ircMsg)
            
            if response != "" {
                log.Printf("📤 Válasz elküldve slave-nek [%s/%s]: %s", msg.BotName, msg.Channel, response)
                
                bm.mutex.RLock()
                slave, exists := bm.slaves[msg.BotName]
                bm.mutex.RUnlock()
                
                if !exists || slave.Conn == nil {
                    log.Printf("⚠️ Nem lehet válaszolni: %s (nincs kapcsolat)", msg.BotName)
                    return
                }
                
// ✅ TÖBBSOROS VÁLASZ KEZELÉSE (~~~ szeparátorral)
if strings.Contains(response, "\n") || strings.Contains(response, "~~~") {
    var lines []string
    
    // Ha ~~~ szeparátort tartalmaz, azt használjuk
    if strings.Contains(response, "~~~") {
        lines = strings.Split(response, "~~~")
    } else {
        lines = strings.Split(response, "\n")
    }
                    for i, line := range lines {
                        line = strings.TrimSpace(line)
                        if line != "" {
                            replyMsg := map[string]string{
                                "type":    "reply", 
                                "channel": msg.Channel,
                                "reply":   line,
                                "user":    msg.User,
                            }
                            
                            data, err := json.Marshal(replyMsg)
                            if err != nil {
                                log.Printf("❌ Reply JSON marshal hiba: %v", err)
                                continue
                            }
                            
                            if i > 0 {
                                time.Sleep(100 * time.Millisecond)
                            }
                            
                            _, err = slave.Conn.Write(append(data, '\n'))
                            if err != nil {
                                log.Printf("❌ Reply küldési hiba (%s): %v", msg.BotName, err)
                                slave.Conn.Close()
                                slave.Conn = nil
                                break
                            }
                        }
                    }
                } else {
                    replyMsg := map[string]string{
                        "type":    "reply", 
                        "channel": msg.Channel,
                        "reply":   response,
                        "user":    msg.User,
                    }
                    
                    data, err := json.Marshal(replyMsg)
                    if err != nil {
                        log.Printf("❌ Reply JSON marshal hiba: %v", err)
                        return
                    }
                    
                    _, err = slave.Conn.Write(append(data, '\n'))
                    if err != nil {
                        log.Printf("❌ Reply küldési hiba (%s): %v", msg.BotName, err)
                        slave.Conn.Close()
                        slave.Conn = nil
                    }
                }
            }
        }
        
    case "session":
        sessionMsg := map[string]interface{}{
            "action":   msg.Action,
            "bot_name": msg.BotName,
            "hostmask": msg.Hostmask,
            "channel":  msg.Channel,
            "user":     msg.User,
            "data":     msg.Data,
        }
        bm.processSessionMessage(conn, sessionMsg)
        
    case "status":
        bm.handleStatusUpdate(msg.BotName, msg.Data)
        
    case "error":
        log.Printf("❌ Slave hiba (%s): %v", msg.BotName, msg.Data)
        
    default:
        log.Printf("⚠️ Ismeretlen üzenet típus: %s", msg.Type)
    }
}