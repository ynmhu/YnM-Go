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
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
	"fmt"
)

// connectToMaster - Master kapcsolat létrehozása és fenntartása
func (sb *SlaveBot) connectToMaster() {
	reconnectDelay := 2 * time.Second
	maxReconnectDelay := 30 * time.Second

	for sb.running {
		sb.handlerMutex.Lock()
		shouldConnect := sb.masterConn == nil && !sb.handlerRunning
		sb.handlerMutex.Unlock()

		if !shouldConnect {
			time.Sleep(2 * time.Second)
			continue
		}

		// ✅ RÖVID időtartamú kapcsolódási kísérlet
		conn, err := net.DialTimeout("unix", sb.socketPath, 60*time.Second)
		if err != nil {
			log.Printf("[%s] 🔶 Master unavailable: %v (retry in %v)", sb.config.Name, err, reconnectDelay)
			sb.standalone = true

			time.Sleep(reconnectDelay)

			reconnectDelay *= 2
			if reconnectDelay > maxReconnectDelay {
				reconnectDelay = maxReconnectDelay
			}
			continue
		}

		// ✅ GYORS kapcsolat beállítás
		sb.handlerMutex.Lock()
		sb.masterConn = conn
		sb.handlerRunning = true
		sb.handlerMutex.Unlock()

		sb.standalone = false
		reconnectDelay = 2 * time.Second

		log.Printf("[%s] ✅ Connected to master!", sb.config.Name)

		// Regisztráció
		sb.sendRegistration()

		// ✅ KÜLÖN GOROUTINE - NE BLOKKOLJA A FŐ CIKLUST
		go sb.handleMasterConnection()

		time.Sleep(2 * time.Second)
	}
}

// handleMasterConnection - Master kapcsolat kezelése
// master.go - handleMasterConnection() EGYSZERŰSÍTETT

func (sb *SlaveBot) handleMasterConnection() {
	log.Printf("[%s] 🔄 Master connection handler started", sb.config.Name)

	defer func() {
		sb.handlerMutex.Lock()
		if sb.masterConn != nil {
			sb.masterConn.Close()
			sb.masterConn = nil
		}
		sb.handlerRunning = false
		sb.handlerMutex.Unlock()

		sb.standalone = true
		log.Printf("[%s] 🔌 Master connection handler ended", sb.config.Name)
	}()

	reader := bufio.NewReader(sb.masterConn)

	for sb.running {
		// ✅ 5 PERC TIMEOUT - Ha 5 percig nincs semmi, akkor baj van
		sb.masterConn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		line, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if !sb.running {
					break
				}
				// Ha 5 percig nincs semmi aktivitás, újracsatlakozunk
				log.Printf("[%s] ⚠️ No activity for 5 minutes - reconnecting", sb.config.Name)
				break
			} else if err == io.EOF {
				log.Printf("[%s] 🔌 Master connection closed (EOF)", sb.config.Name)
				break
			} else {
				log.Printf("[%s] ❌ Read error: %v", sb.config.Name, err)
				break
			}
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var msg MasterMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("[%s] ❌ JSON parse error: %v", sb.config.Name, err)
			continue
		}

		sb.processMasterMessage(msg)
	}
}

// master.go (slave package) - processMasterMessage JAVÍTOTT RÉSZ

func (sb *SlaveBot) processMasterMessage(msg MasterMessage) {
    switch msg.Type {
    case "reply":
        log.Printf("[%s] ✅ REPLY received: Channel='%s' Reply='%s' User='%s'", 
            sb.config.Name, msg.Channel, msg.Reply, msg.User)
        
        if msg.Channel == "" {
            log.Printf("[%s] ❌ REPLY ERROR: Channel is empty!", sb.config.Name)
            return
        }
        
        if msg.Reply == "" {
            log.Printf("[%s] ❌ REPLY ERROR: Reply message is empty!", sb.config.Name)
            return
        }
        
        if sb.ircClient == nil {
            log.Printf("[%s] ❌ REPLY ERROR: IRC client is nil!", sb.config.Name)
            return
        }
        
        target := msg.Channel
        if target == sb.config.Nickname && msg.User != "" {
            target = msg.User
            log.Printf("[%s] 🔄 Privát válasz átirányítása: %s -> %s", sb.config.Name, msg.Channel, target)
        }
        
		log.Printf("[%s] 📤 Sending to IRC: %s -> %s", sb.config.Name, target, msg.Reply)

		// ✅ Biztonsági védelem: távolítsuk el az összes új sor karaktert
		cleanReply := strings.ReplaceAll(msg.Reply, "\n", "")
		cleanReply = strings.ReplaceAll(cleanReply, "\r", "")

		if cleanReply != msg.Reply {
			log.Printf("[%s] ⚠️ WARNING: Reply tartalmazott \\n karaktert! Eredeti: %q, Tisztított: %q", 
				sb.config.Name, msg.Reply, cleanReply)
		}

		rawMsg := fmt.Sprintf("PRIVMSG %s :%s", target, cleanReply)
		sb.ircClient.SendRaw(rawMsg)

		log.Printf("[%s] ✅ Message sent to IRC!", sb.config.Name)
        
    case "session":
        sb.processSessionMessage(msg)
        
    case "pong":
        log.Printf("[%s] 🏓 Pong received", sb.config.Name)
        
    case "shutdown":
        log.Printf("[%s] Master shutdown signal received", sb.config.Name)
        sb.Stop()
        
    case "command":
        // ✅ JAVÍTOTT: MODE PARANCS KEZELÉS
        log.Printf("[%s] 🔧 Command received: action=%s, user=%s, mode=%s", 
            sb.config.Name, msg.Action, msg.User, msg.Message)
        
        // Message mező tartalmazza a mode type-ot ("o", "h", "v")
        if msg.Action == "mode" {
            modeType := msg.Message
            if modeType == "" {
                // Fallback: próbáljuk az action-ből kitalálni
                switch msg.Action {
                case "o":
                    modeType = "o"
                case "h":
                    modeType = "h"
                case "v":
                    modeType = "v"
                }
            }
            
            if modeType != "" {
                sb.executeModeCommand(msg.Channel, msg.User, modeType)
            } else {
                log.Printf("[%s] ❌ Unknown mode type in command", sb.config.Name)
            }
        } else {
            // Régi formátum: action = op/halfop/voice
            sb.handleMasterCommand(msg)
        }
        

        
    default:
        log.Printf("[%s] 🔍 Unknown message type: %s", sb.config.Name, msg.Type)
    }
}

// sendToMaster - Biztonságos küldés a masternek
func (sb *SlaveBot) sendToMaster(msg MasterMessage) {
	sb.handlerMutex.Lock()
	defer sb.handlerMutex.Unlock()

	if sb.masterConn == nil {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[%s] ❌ Marshal error: %v", sb.config.Name, err)
		return
	}

	data = append(data, '\n')
	_, err = sb.masterConn.Write(data)
	if err != nil {
		log.Printf("[%s] ❌ Send to master error: %v", sb.config.Name, err)
	}
}

// sendRegistration - Regisztráció küldése a masternek
func (sb *SlaveBot) sendRegistration() {
	if sb.masterConn == nil {
		return
	}
	
	reg := MasterMessage{
		Type:    "register",
		BotName: sb.config.Name,
		Data: map[string]interface{}{
			"pid":        os.Getpid(),
			"started_at": sb.startTime.Unix(),
		},
	}
	
	data, _ := json.Marshal(reg)
	sb.masterConn.Write(append(data, '\n'))
	log.Printf("[%s] ✅ Registration sent (PID: %d, Uptime: %s)", 
		sb.config.Name, os.Getpid(), time.Since(sb.startTime).Round(time.Second))
}

// forwardToMaster - Üzenet továbbítása a masternek
// master.go - javítsd a forwardToMaster függvényt a login parancsokhoz

func (sb *SlaveBot) forwardToMaster(channel, user, hostmask, message string) {
    if sb.masterConn == nil {
        sb.standalone = true
        return
    }

    // ✅ SPECIÁLIS KEZELÉS LOGIN/LOGOUT PARANCSOKNAK
    var targetUser string
    
    if strings.HasPrefix(message, "!login") || strings.HasPrefix(message, "login") {
        // ❌ ROSSZ: targetUser = parts[1]  // Ez a "Markus" lesz a parancsból
        // ✅ JÓ: 
        targetUser = user  // Ez a "Csirke" - aki küldte az üzenetet
        
    } else if strings.HasPrefix(message, "!logout") || strings.HasPrefix(message, "logout") {
        targetUser = user
        log.Printf("[%s] 🔐 Logout command - target user: %s", sb.config.Name, targetUser)
    } else {
        targetUser = user
    }

    msg := MasterMessage{
        Type:     "message",
        BotName:  sb.config.Name,
        Channel:  channel,
        User:     targetUser,
        Hostmask: hostmask,
        Message:  message,
        Source:   "slave-" + sb.config.Name,
    }

    data, err := json.Marshal(msg)
    if err != nil {
        log.Printf("[%s] Marshal error: %v", sb.config.Name, err)
        return
    }

    _, err = sb.masterConn.Write(append(data, '\n'))
    if err != nil {
        log.Printf("[%s] Send error: %v", sb.config.Name, err)
        sb.masterConn.Close()
        sb.masterConn = nil
        sb.standalone = true
        log.Printf("[%s] 🔶 Master connection lost", sb.config.Name)
    }
}


// activateStandaloneIfNeeded - Standalone mód aktiválása, ha szükséges
func (sb *SlaveBot) activateStandaloneIfNeeded() {
	time.Sleep(10 * time.Second)
	sb.standalone = (sb.masterConn == nil)
	if sb.standalone {
		log.Printf("[%s] 🔶 STANDALONE MODE ACTIVATED", sb.config.Name)
	}
}

func (sb *SlaveBot) processSessionMessage(msg MasterMessage) {
    log.Printf("[%s] 🔐 Session message received: %s", sb.config.Name, msg.Action)
    
    switch msg.Action {
    case "get_response":
        // Session válasz a master-től
        var hasSession bool
        var username string
        
        if msg.Data != nil {
            if dataMap, ok := msg.Data.(map[string]interface{}); ok {
                if sessionVal, exists := dataMap["has_session"]; exists {
                    hasSession, _ = sessionVal.(bool)
                }
                if userVal, exists := dataMap["username"]; exists {
                    username, _ = userVal.(string)
                }
            }
        }
        
        if hasSession {
            log.Printf("[%s] 🔐 Session active for %s: %s", sb.config.Name, msg.Hostmask, username)
            // Itt tárolhatod a session-t lokálisan
        } else {
            log.Printf("[%s] 🔐 No session for %s", sb.config.Name, msg.Hostmask)
        }
    case "login_response":
        log.Printf("[%s] 🔐 Login response received for %s", sb.config.Name, msg.User)
        
        var success bool
        var message string
        
        if msg.Data != nil {
            if dataMap, ok := msg.Data.(map[string]interface{}); ok {
                if successVal, exists := dataMap["success"]; exists {
                    success, _ = successVal.(bool)
                }
                if messageVal, exists := dataMap["message"]; exists {
                    message, _ = messageVal.(string)
                }
            }
        }
        
        if message == "" {
            if success {
                message = "Sikeres bejelentkezés!"
            } else {
                message = "Hibás jelszó!"
            }
        }
        
        log.Printf("[%s] 🔐 Final - success: %v, message: %s", sb.config.Name, success, message)
        
        // ✅ JAVÍTÁS: MEGFELELŐ CÉL KIVÁLASZTÁSA
        var target string
        
        // Ha privát üzenet (nem csatorna)
        if msg.Channel == sb.config.Nickname || !strings.HasPrefix(msg.Channel, "#") {
            // Privát üzenet - a választ a FELHASZNÁLÓNAK küldjük
            target = msg.User
            log.Printf("[%s] 🔐 Privát válasz küldése: %s", sb.config.Name, target)
        } else {
            // Csatorna üzenet - a választ a CSATORNÁNAK küldjük
            target = msg.Channel
            log.Printf("[%s] 🔐 Csatorna válasz küldése: %s", sb.config.Name, target)
        }
        
        if success {
            log.Printf("[%s] 🔐 ✅ Login successful for %s", sb.config.Name, msg.User)
            sb.ircClient.SendMessage(target, fmt.Sprintf("%s: ✅ %s", msg.User, message))
            
            // Session mentése lokálisan
            session := &UserSession{
                Username:  msg.User,
                Hostmask:  msg.Hostmask,
                LoggedIn:  true,
                LoginTime: time.Now(),
                LastSeen:  time.Now(),
                Data:      make(map[string]interface{}),
            }
            sb.SetSession(msg.Hostmask, session)
            
        } else {
            log.Printf("[%s] 🔐 ❌ Login failed for %s", sb.config.Name, msg.User)
            sb.ircClient.SendMessage(target, fmt.Sprintf("%s: ❌ %s", msg.User, message))
        }
        
		
		 case "logout_response":
        // Logout válasz a master-től
        log.Printf("[%s] 🔐 Logout response received for %s", sb.config.Name, msg.User)
        
        var success bool
        var message string
        
        if msg.Data != nil {
            if dataMap, ok := msg.Data.(map[string]interface{}); ok {
                if successVal, exists := dataMap["success"]; exists {
                    success, _ = successVal.(bool)
                }
                if messageVal, exists := dataMap["message"]; exists {
                    message, _ = messageVal.(string)
                }
            }
        }
        
        if message == "" {
            if success {
                message = "Sikeres kijelentkezés!"
            } else {
                message = "Nem volt aktív session!"
            }
        }
        
        // ✅ MINDIG a FELHASZNÁLÓNAK küldjük a választ (privát üzenet)
        target := msg.User
        
        if success {
            log.Printf("[%s] 🔐 ✅ Logout successful for %s", sb.config.Name, msg.User)
            sb.ircClient.SendMessage(target, fmt.Sprintf("✅ %s", message))
            
            // Session törlése lokálisan
            sb.DeleteSession(msg.Hostmask)
            
        } else {
            log.Printf("[%s] 🔐 ❌ Logout failed for %s", sb.config.Name, msg.User)
            sb.ircClient.SendMessage(target, fmt.Sprintf("❌ %s", message))
        }
    case "get_session_response":
        // Session válasz a master-től
        if msg.Data != nil {
            if sessionData, ok := msg.Data.(map[string]interface{}); ok {
                log.Printf("[%s] 🔐 Session response received for %s", sb.config.Name, msg.Hostmask)
                sb.handleSessionResponse(msg.Hostmask, sessionData)
            }
        } else {
            log.Printf("[%s] 🔐 Empty session response for %s", sb.config.Name, msg.Hostmask)
        }
        
    case "login_success":
        // Sikeres login értesítés
        log.Printf("[%s] 🔐 Login successful for %s", sb.config.Name, msg.User)
        sb.ircClient.SendMessage(msg.Channel, fmt.Sprintf("%s: ✅ Sikeres bejelentkezés!", msg.User))
        
    case "login_failed":
        // Sikertelen login értesítés
        log.Printf("[%s] 🔐 Login failed for %s", sb.config.Name, msg.User)
        sb.ircClient.SendMessage(msg.Channel, fmt.Sprintf("%s: ❌ Hibás jelszó!", msg.User))
        
    case "session_updated":
        // Session frissítés értesítés
        log.Printf("[%s] 🔐 Session updated for %s", sb.config.Name, msg.Hostmask)
        
    default:
        log.Printf("[%s] 🔐 Unknown session action: %s", sb.config.Name, msg.Action)
    }
}

func (sb *SlaveBot) sendSessionRequest(action, hostmask, channel, user string, data interface{}) {
    sessionMsg := MasterMessage{
        Type:     "session",
        Action:   action,
        BotName:  sb.config.Name,
        Hostmask: hostmask,
        Channel:  channel,
        User:     user,
        Data:     data,
        Source:   "slave-" + sb.config.Name,
    }
    
    sb.sendToMaster(sessionMsg)
}
// handleSessionResponse - Session válasz feldolgozása
func (sb *SlaveBot) handleSessionResponse(hostmask string, sessionData map[string]interface{}) {
    // Session létrehozása a kapott adatokból
    session := &UserSession{
        Hostmask:  hostmask,
        LastSeen:  time.Now(),
        Data:      make(map[string]interface{}),
    }
    
    // Adatok kitöltése
    if username, ok := sessionData["username"].(string); ok {
        session.Username = username
    }
    if loggedIn, ok := sessionData["logged_in"].(bool); ok {
        session.LoggedIn = loggedIn
    }
    if loginTimeStr, ok := sessionData["login_time"].(string); ok {
        if loginTime, err := time.Parse(time.RFC3339, loginTimeStr); err == nil {
            session.LoginTime = loginTime
        }
    }
    if data, ok := sessionData["data"].(map[string]interface{}); ok {
        session.Data = data
    }
    
    // Session mentése lokálisan
    sb.SetSession(hostmask, session)
    log.Printf("[%s] 🔐 Session stored for %s (logged_in: %v)", sb.config.Name, hostmask, session.LoggedIn)
}

// RequestSessionFromMaster - Session lekérése a master-től
func (sb *SlaveBot) RequestSessionFromMaster(hostmask, channel string) {
    log.Printf("[%s] 🔐 Requesting session for %s from master", sb.config.Name, hostmask)
    sb.sendSessionRequest("get", hostmask, channel, "", nil)
}

// SendLoginAttempt - Bejelentkezési kísérlet küldése a masternek
func (sb *SlaveBot) SendLoginAttempt(hostmask, channel, user, password string) {
    loginData := map[string]interface{}{
        "password": password,
        "attempt_time": time.Now().Format(time.RFC3339),
    }
    
    log.Printf("[%s] 🔐 Sending login attempt for %s", sb.config.Name, user)
    sb.sendSessionRequest("login", hostmask, channel, user, loginData)
}