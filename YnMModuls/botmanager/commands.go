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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"time"
	 "sync"
	"path/filepath"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
		"git.ynm.hu/markus/YnM-Go/YnMModule"
)

var modeStates = make(map[string]map[string]map[string]bool)
var modeStatesMutex sync.RWMutex


func (bm *BotManagerPlugin) HandleMessage(msg YnMIrC.Message) string {
    text := msg.Text

    // ✅ SZERVER ÜZENETEK SZŰRÉSE
    if msg.Nick == "irc.ynm.hu" || strings.Contains(msg.Sender, "irc.ynm.hu") {
        return ""
    }

    // ✅ CSAK CSATORNA ÜZENETEKET KEZELJÜK A BOTMANAGERBEN!
 //   isPrivate := !strings.HasPrefix(msg.Channel, "#")
 //   if isPrivate {
  //      log.Printf("🔇 [BOTMANAGER] Privát üzenet ignorálva (socket handler dolga): %s -> %s", msg.Nick, text)
  //      return ""
  //  }


    // ✅ CSAK "!bot" PARANCSOKAT KEZELJÜK TOVÁBB
    if !strings.HasPrefix(text, "!bot") {
        return "" // ⬅️ Más parancsokat továbbengedjük
    }

    raw := strings.TrimPrefix(text, "!")
    parts := strings.Split(raw, " ")

    if len(parts) == 0 {
        return ""
    }

    cmd := parts[0]
    args := []string{}
    if len(parts) > 1 {
        args = parts[1:]
    }

    // Aszinkron kezelés
    go func() {
        bm.HandleCommand(msg.Channel, msg.Nick, cmd, args)
    }()

    return ""
}




func (bm *BotManagerPlugin) HandleCommand(channel, nick, cmd string, args []string) {
	if cmd != "bot" {
		return
	}

	if len(args) == 0 {
		bm.bot.SendMessage(channel, "Használat: !bot <list|status|start|stop|restart|kill> [név|all]")
		return
	}

	subCmd := args[0]

	switch subCmd {
	case "list":
		bm.cmdList(channel)
	case "status":
		bm.cmdStatus(channel)
	case "start":
		if len(args) < 2 {
			bm.bot.SendMessage(channel, "Használat: !bot start <név>")
			return
		}
		bm.cmdStart(channel, args[1])
	case "stop":
		if len(args) < 2 {
			bm.bot.SendMessage(channel, "Használat: !bot stop <név>")
			return
		}
		bm.cmdStop(channel, args[1])
	case "restart":
		if len(args) < 2 {
			bm.bot.SendMessage(channel, "Használat: !bot restart <név>")
			return
		}
		bm.cmdRestart(channel, args[1])
		
	// ✅ ÚJ: KILL PARANCS
	case "kill":
		if len(args) < 2 {
			bm.bot.SendMessage(channel, "Használat: !bot kill <név|all>")
			return
		}
		
		target := args[1]
		if target == "all" {
			bm.cmdKillAll(channel)
		} else {
			bm.cmdKill(channel, target)
		}
		
	default:
		bm.bot.SendMessage(channel, "Ismeretlen parancs. Használat: !bot <list|status|start|stop|restart|kill>")
	}
}

// ✅ ÚJ FÜGGVÉNY: Egy slave erőszakos leállítása
func (bm *BotManagerPlugin) cmdKill(channel, name string) {
	bm.mutex.RLock()
	slave, exists := bm.slaves[name]
	bm.mutex.RUnlock()
	
	if !exists {
		bm.bot.SendMessage(channel, fmt.Sprintf("❌ Slave bot '%s' nem található.", name))
		return
	}
	
	bm.bot.SendMessage(channel, fmt.Sprintf("💀 Slave bot '%s' erőszakos leállítása...", name))
	
	go func() {
		err := bm.forceKillSlave(name, slave)
		if err != nil {
			bm.bot.SendMessage(channel, fmt.Sprintf("❌ Kill hiba (%s): %v", name, err))
		} else {
			bm.bot.SendMessage(channel, fmt.Sprintf("✅ Slave bot '%s' sikeresen kilőve.", name))
		}
		bm.saveState()
	}()
}

// ✅ ÚJ FÜGGVÉNY: Minden slave erőszakos leállítása
func (bm *BotManagerPlugin) cmdKillAll(channel string) {

    bm.bot.SendMessage(channel, "💀 ÖSSZES SLAVE LEÁLLÍTÁSA (pkill -9 slave)...")

    go func() {
        cmd := exec.Command("pkill", "-9", "slave")

        if err := cmd.Run(); err != nil {
            bm.bot.SendMessage(channel, "❌ Hiba a slave kilövésénél: " + err.Error())
            return
        }

        bm.bot.SendMessage(channel, "✅ Minden slave kilőve (pkill -9 slave).")
    }()
}

// ✅ ÚJ FÜGGVÉNY: Erőszakos kill megvalósítás
func (bm *BotManagerPlugin) forceKillSlave(name string, slave *ManagedSlave) error {
	log.Printf("💀 Force killing slave: %s (PID: %d)", name, slave.PID)
	
	// 1. Socket kapcsolat bezárása
	bm.mutex.Lock()
	if slave.Conn != nil {
		slave.Conn.Close()
		slave.Conn = nil
		log.Printf("🔌 Socket closed: %s", name)
	}
	bm.mutex.Unlock()
	
	// 2. Process kill
	killed := false
	
	// 2a. Process object használata
	if slave.Process != nil {
		log.Printf("💀 Killing via Process object: %s (PID: %d)", name, slave.PID)
		if err := slave.Process.Kill(); err != nil {
			log.Printf("⚠️ Process.Kill() hiba: %v", err)
		} else {
			killed = true
			log.Printf("✅ Process killed via object: %s", name)
		}
	}
	
	// 2b. PID alapú kill (ha Process object nem működött)
	if !killed && slave.PID > 0 {
		log.Printf("💀 Killing via PID: %s (PID: %d)", name, slave.PID)
		process, err := os.FindProcess(slave.PID)
		if err != nil {
			log.Printf("⚠️ FindProcess hiba: %v", err)
		} else {
			// SIGKILL használata (kill -9)
			if err := process.Signal(syscall.SIGKILL); err != nil {
				log.Printf("⚠️ SIGKILL hiba: %v", err)
			} else {
				killed = true
				log.Printf("✅ Process killed via SIGKILL: %s (PID: %d)", name, slave.PID)
			}
		}
	}
	
	// 2c. pkill fallback (ha minden más sikertelen)
	if !killed && slave.PID > 0 {
		log.Printf("💀 Using pkill fallback: %s", name)
		cmd := exec.Command("pkill", "-9", "-P", fmt.Sprintf("%d", slave.PID))
		if err := cmd.Run(); err != nil {
			log.Printf("⚠️ pkill hiba: %v", err)
		} else {
			killed = true
			log.Printf("✅ Process killed via pkill: %s", name)
		}
	}
	
	// 3. Slave törlése a nyilvántartásból
	bm.mutex.Lock()
	delete(bm.slaves, name)
	bm.mutex.Unlock()
	
	if killed {
		log.Printf("✅ Slave '%s' sikeresen kilőve", name)
		return nil
	} else {
		return fmt.Errorf("nem sikerült kilőni a slave bot-ot (PID: %d)", slave.PID)
	}
}
func (bm *BotManagerPlugin) sendReplyToSlave(botName, channel, reply string) {
	bm.mutex.RLock()
	slave, exists := bm.slaves[botName]
	bm.mutex.RUnlock()
	
	if !exists || slave.Conn == nil {
		log.Printf("⚠️ Nem lehet válaszolni: %s (nincs kapcsolat)", botName)
		return
	}
	
	replyMsg := map[string]string{
		"type":    "reply",
		"channel": channel,
		"reply":   reply,
		"user":    "",
	}
	
	data, err := json.Marshal(replyMsg)
	if err != nil {
		log.Printf("❌ Reply JSON marshal hiba: %v", err)
		return
	}
	
	_, err = slave.Conn.Write(append(data, '\n'))
	if err != nil {
		log.Printf("❌ Reply küldési hiba (%s): %v", botName, err)
		slave.Conn.Close()
		slave.Conn = nil
	} else {
		log.Printf("📤 Válasz elküldve slave-nek [%s/%s]: %s", botName, channel, reply)
	}
}

func (bm *BotManagerPlugin) registerSlaveConnection(botName string, conn net.Conn) {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	slave, exists := bm.slaves[botName]
	if exists {
		if slave.Conn != nil {
			log.Printf("🔄 Régi kapcsolat lezárása: %s", botName)
			slave.Conn.Close()
		}
		slave.Conn = conn
		slave.Status = "running"
		log.Printf("✅ Futó slave újracsatlakozott: %s (PID: %d)", botName, slave.PID)
	} else {
		if cfg, configExists := bm.config.Slaves[botName]; configExists {
			bm.slaves[botName] = &ManagedSlave{
				Name:      botName,
				Config:    cfg,
				PID:       0,
				Status:    "running",
				StartedAt: time.Now(),
				Conn:      conn,
			}
			log.Printf("✅ Új slave regisztrálva: %s", botName)
		} else {
			log.Printf("⚠️ Ismeretlen slave próbál regisztrálni: %s", botName)
			conn.Close()
			return
		}
	}
	
	go bm.saveState()
}

func (bm *BotManagerPlugin) handleStatusUpdate(botName string, data interface{}) {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	slave, exists := bm.slaves[botName]
	if exists {
		slave.Status = "running"
		log.Printf("📊 Status update: %s - %v", botName, data)
	} else {
		log.Printf("⚠️ Status update ismeretlen slave botról: %s", botName)
	}
}

func (bm *BotManagerPlugin) handleLoginCommand(botName, channel, user, hostmask, message string) {
    log.Printf("🔐 Login attempt - bot: %s, sender: %s, hostmask: %s, message: %s", 
        botName, user, hostmask, message)
    
    parts := strings.Fields(message)
    
    if len(parts) < 3 {
        bm.sendLoginResponse(botName, channel, user, false, "Használat: login <felhasználónév> <jelszó>")
        return
    }
    
    targetUsername := parts[1]
    password := strings.Join(parts[2:], " ")
    
    log.Printf("🔐 Login details - username: %s, password: %s", targetUsername, password)
    
    // ✅ owner PLUGIN SESSION LÉTREHOZÁSA (de NE küldje a bot az üzenetet!)
    if bm.adminPlugin != nil {
        // Jelszó ellenőrzés
        valid, err := bm.adminPlugin.Db.VerifyPassword(targetUsername, password)
        if err != nil || !valid {
            log.Printf("🔐 ❌ Sikertelen login: %s as %s", user, targetUsername)
            bm.sendLoginResponse(botName, channel, user, false, "Hibás jelszó!")
            return
        }
        
        // Session létrehozása
        simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
        sessionID, sessionKey := bm.adminPlugin.CreateSession(simplifiedHostmask, targetUsername)
        _ = sessionID // használatlan
        
        log.Printf("🔐 ✅ Sikeres login: %s as %s (session: %s)", user, targetUsername, sessionKey)
        
        // ✅ VÁLASZ KÜLDÉSE A SLAVE BOTNAK (nem a master botnak!)
        successMsg := fmt.Sprintf("✅ Sikeres bejelentkezés %s felhasználóként!\nSession Key: %s (24 óráig érvényes)", 
            targetUsername, sessionKey)
        bm.sendLoginResponse(botName, channel, user, true, successMsg)
        
    } else {
        log.Printf("🔐 ❌ owner plugin nincs inicializálva!")
        bm.sendLoginResponse(botName, channel, user, false, "Login rendszer nem elérhető!")
    }
}
// handleLogoutCommand - Logout parancs kezelése
func (bm *BotManagerPlugin) handleLogoutCommand(botName, channel, user, hostmask string) {
    log.Printf("🔐 Logout request from %s (%s)", user, botName)
    
    // ✅ owner PLUGIN SESSION TÖRLÉSE
    if bm.adminPlugin != nil {
        simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
        
        // Ellenőrizzük, hogy van-e session
        if session, exists := bm.adminPlugin.GetSessionByHost(simplifiedHostmask); exists {
            // Session törlése
            bm.adminPlugin.DeleteSessionByHost(simplifiedHostmask)
            
            log.Printf("🔐 ✅ Logout successful for %s (was logged in as %s)", user, session.LoggedInAs)
            bm.sendLogoutResponse(botName, channel, user, true, "Sikeres kijelentkezés!")
        } else {
            log.Printf("🔐 ❌ No active session for %s", user)
            bm.sendLogoutResponse(botName, channel, user, false, "Nincs aktív session!")
        }
    } else {
        log.Printf("🔐 ❌ owner plugin nincs inicializálva!")
        bm.sendLogoutResponse(botName, channel, user, false, "Logout rendszer nem elérhető!")
    }
}



// sendLoginResponse - Login válasz küldése
func (bm *BotManagerPlugin) sendLoginResponse(botName, channel, user string, success bool, message string) {
	bm.mutex.RLock()
	slave, exists := bm.slaves[botName]
	bm.mutex.RUnlock()
	
	if !exists || slave.Conn == nil {
		log.Printf("⚠️ Nem lehet login választ küldeni: %s (nincs kapcsolat)", botName)
		return
	}
	
	responseMsg := map[string]interface{}{
		"type":     "session",
		"action":   "login_response", 
		"bot_name": botName,
		"user":     user,
		"channel":  channel,
		"data": map[string]interface{}{
			"success": success,
			"message": message,
		},
	}
	
	log.Printf("📤 Sending login response - success: %v, message: %s", success, message)
	
	data, err := json.Marshal(responseMsg)
	if err != nil {
		log.Printf("❌ Login response JSON marshal hiba: %v", err)
		return
	}
	
	data = append(data, '\n')
	_, err = slave.Conn.Write(data)
	if err != nil {
		log.Printf("❌ Login response küldési hiba (%s): %v", botName, err)
		slave.Conn.Close()
		slave.Conn = nil
	} else {
		log.Printf("📤 Login response sent to %s", botName)
	}
}

// sendLogoutResponse - Logout válasz küldése
func (bm *BotManagerPlugin) sendLogoutResponse(botName, channel, user string, success bool, message string) {
	bm.mutex.RLock()
	slave, exists := bm.slaves[botName]
	bm.mutex.RUnlock()
	
	if !exists || slave.Conn == nil {
		log.Printf("⚠️ Nem lehet logout választ küldeni: %s (nincs kapcsolat)", botName)
		return
	}
	
	responseMsg := map[string]interface{}{
		"type":     "session",
		"action":   "logout_response", 
		"bot_name": botName,
		"user":     user,
		"channel":  channel,
		"data": map[string]interface{}{
			"success": success,
			"message": message,
		},
	}
	
	log.Printf("📤 Sending logout response - success: %v, message: %s", success, message)
	
	data, err := json.Marshal(responseMsg)
	if err != nil {
		log.Printf("❌ Logout response JSON marshal hiba: %v", err)
		return
	}
	
	data = append(data, '\n')
	_, err = slave.Conn.Write(data)
	if err != nil {
		log.Printf("❌ Logout response küldési hiba (%s): %v", botName, err)
		slave.Conn.Close()
		slave.Conn = nil
	} else {
		log.Printf("📤 Logout response sent to %s", botName)
	}
}

func (bm *BotManagerPlugin) cmdList(channel string) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	if len(bm.config.Slaves) == 0 {
		bm.bot.SendMessage(channel, "Nincsenek konfigurált slave botok.")
		return
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("📋 Elérhető slave botok (%d db):", len(bm.config.Slaves)))

	for name, cfg := range bm.config.Slaves {
		status := "🔴 offline"
		socketStatus := "❌"
		
		if slave, ok := bm.slaves[name]; ok {
			if slave.Status == "running" {
				status = "🟢 online"
				if slave.Conn != nil {
					socketStatus = "✅"
				}
			}
		}
		
		lines = append(lines, fmt.Sprintf("  • %s (%s:%d) - %s | Socket: %s", 
			name, cfg.Server, cfg.Port, status, socketStatus))
	}

	for _, line := range lines {
		bm.bot.SendMessage(channel, line)
	}
}

func (bm *BotManagerPlugin) cmdStatus(channel string) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	if len(bm.slaves) == 0 {
		bm.bot.SendMessage(channel, "Nincsenek futó slave botok.")
		return
	}

	for name, slave := range bm.slaves {
		uptime := time.Since(slave.StartedAt).Round(time.Second)
		socketStatus := "❌"
		if slave.Conn != nil {
			socketStatus = "✅"
		}
		
		pidInfo := "PID: ismeretlen"
		if slave.PID > 0 {
			isAlive := "🔴"
			if slave.PID > 0 {
				process, err := os.FindProcess(slave.PID)
				if err == nil {
					if err := process.Signal(syscall.Signal(0)); err == nil {
						isAlive = "🟢"
					}
				}
			}
			pidInfo = fmt.Sprintf("PID: %d %s", slave.PID, isAlive)
		}
		
		msg := fmt.Sprintf("🤖 %s: %s | Socket: %s | Uptime: %v | %s",
			name, slave.Status, socketStatus, uptime, pidInfo)
		bm.bot.SendMessage(channel, msg)
	}
}

func (bm *BotManagerPlugin) cmdStart(channel, name string) {
	bm.mutex.Lock()
	
	cfg, exists := bm.config.Slaves[name]
	if !exists {
		bm.mutex.Unlock()
		bm.bot.SendMessage(channel, fmt.Sprintf("❌ Slave bot '%s' nem található a konfigban.", name))
		return
	}

	if slave, ok := bm.slaves[name]; ok && slave.Status == "running" {
		bm.mutex.Unlock()
		bm.bot.SendMessage(channel, fmt.Sprintf("⚠️ Slave bot '%s' már fut.", name))
		return
	}
	
	bm.mutex.Unlock()

	go func() {
		err := bm.startSlaveAsync(name, cfg)
		if err != nil {
			bm.bot.SendMessage(channel, fmt.Sprintf("❌ Slave bot indítási hiba: %v", err))
			return
		}

		bm.bot.SendMessage(channel, fmt.Sprintf("✅ Slave bot '%s' sikeresen elindult.", name))
		bm.saveState()
	}()
	
	bm.bot.SendMessage(channel, fmt.Sprintf("🔄 Slave bot '%s' indítása folyamatban...", name))
}

func (bm *BotManagerPlugin) startSlaveAsync(name string, cfg SlaveConfig) error {
	slaveBinary := "/home/bot/YnM-Go/YnMModuls/slaves/slave"

	log.Printf("🔍 [DEBUG] Starting slave process check...")
	log.Printf("   Slave binary path: %s", slaveBinary)

	if _, err := os.Stat(slaveBinary); os.IsNotExist(err) {
		log.Printf("❌ [DEBUG] Binary does not exist: %s", slaveBinary)
		return fmt.Errorf("slave bináris nem található: %s", slaveBinary)
	}

	fileInfo, err := os.Stat(slaveBinary)
	if err != nil {
		log.Printf("❌ [DEBUG] Stat error: %v", err)
		return fmt.Errorf("stat hiba: %v", err)
	}

	log.Printf("✅ [DEBUG] Binary found: %s", slaveBinary)
	log.Printf("   Size: %d bytes", fileInfo.Size())
	log.Printf("   Permissions: %s", fileInfo.Mode().String())
	log.Printf("   ModTime: %s", fileInfo.ModTime())

	testCmd := exec.Command(slaveBinary, "-help")
	if testErr := testCmd.Run(); testErr != nil {
		log.Printf("⚠️ [DEBUG] Binary test run failed: %v", testErr)
	}

	absSocketPath, err := filepath.Abs(bm.socketPath)
	if err != nil {
		return fmt.Errorf("socket path abs hiba: %v", err)
	}

	log.Printf("✅ [DEBUG] Socket path: %s", absSocketPath)

	// ✅ JAVÍTOTT CONFIG STRUKTÚRA - TOPIC MEZŐKKEL
	configWithName := struct {
		Name     string   `json:"name"`
		Server   string   `json:"server"`
		Port     int      `json:"port"`
		SSL      bool     `json:"ssl"`
		Nickname string   `json:"nickname"`
		Username string   `json:"username"`
		Realname string   `json:"realname"`
		Channels []string `json:"channels"`
		
		// ✅ ÚJ: Topic mezők
		TopicChannel        string `json:"topic_channel"`
		TopicUpdateInterval string `json:"topic_update_interval"`
	}{
		Name:     name,
		Server:   cfg.Server,
		Port:     cfg.Port,
		SSL:      cfg.SSL,
		Nickname: cfg.Nickname,
		Username: cfg.Username,
		Realname: cfg.Realname,
		Channels: cfg.Channels,
		
		// ✅ ÚJ: Topic értékek átadása
		TopicChannel:        cfg.TopicChannel,
		TopicUpdateInterval: cfg.TopicUpdateInterval,
	}

	configData, err := json.Marshal(configWithName)
	if err != nil {
		return fmt.Errorf("config JSON hiba: %v", err)
	}

	// ✅ Config fájl a data/ mappába
	configFile := filepath.Join(bm.dataDir, fmt.Sprintf("slave_%s_config.json", name))
	if err := os.WriteFile(configFile, configData, 0644); err != nil {
		return fmt.Errorf("config fájl írási hiba: %v", err)
	}

	log.Printf("📝 [DEBUG] Config file created: %s", configFile)
	
	// ✅ DEBUG: Logoljuk ki a config tartalmat
	log.Printf("📋 [DEBUG] Config content:")
	log.Printf("   TopicChannel: '%s'", cfg.TopicChannel)
	log.Printf("   TopicUpdateInterval: '%s'", cfg.TopicUpdateInterval)

	cmd := exec.Command(slaveBinary, 
		"-config", configFile, 
		"-socket", absSocketPath,
		"-name", name)
	
	cmd.Dir = "/home/bot/YnM-Go"
	
	cmd.Env = append(os.Environ(),
		"HOME=/home/bot",
		"USER=bot",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("🚀 [DEBUG] Executing command:")
	log.Printf("   Command: %s", cmd.Path)
	log.Printf("   Args: %v", cmd.Args)
	log.Printf("   Dir: %s", cmd.Dir)

	err = cmd.Start()
	if err != nil {
		log.Printf("❌ [DEBUG] Command start failed: %v", err)
		log.Printf("❌ [DEBUG] Error type: %T", err)
		
		if pathErr, ok := err.(*os.PathError); ok {
			log.Printf("❌ [DEBUG] PathError details: Op=%s, Path=%s, Err=%v", 
				pathErr.Op, pathErr.Path, pathErr.Err)
		}
		
		os.Remove(configFile)
		return fmt.Errorf("process indítási hiba: %v", err)
	}

	log.Printf("✅ [DEBUG] Slave process started successfully!")
	log.Printf("   PID: %d", cmd.Process.Pid)

	bm.mutex.Lock()
	bm.slaves[name] = &ManagedSlave{
		Name:      name,
		Config:    cfg,
		PID:       cmd.Process.Pid,
		Status:    "running",
		StartedAt: time.Now(),
		Process:   cmd.Process,
	}
	bm.mutex.Unlock()

	log.Printf("📝 [DEBUG] Slave registered in manager: %s", name)

	// ✅ NE TÖRÖLJÜK AZONNAL - várjunk 30 mp-et
	go func() {
		time.Sleep(30 * time.Second)
		if err := os.Remove(configFile); err == nil {
			log.Printf("🧹 [DEBUG] Config file cleaned: %s", configFile)
		}
	}()

	return nil
}


func (bm *BotManagerPlugin) cmdStop(channel, name string) {
	bm.mutex.RLock()
	
	_, exists := bm.slaves[name]
	if !exists {
		bm.mutex.RUnlock()
		bm.bot.SendMessage(channel, fmt.Sprintf("❌ Slave bot '%s' nem fut.", name))
		return
	}
	
	bm.mutex.RUnlock()

	go func() {
		err := bm.stopSlaveAsync(name)
		if err != nil {
			bm.bot.SendMessage(channel, fmt.Sprintf("❌ Slave bot leállítási hiba: %v", err))
			return
		}

		bm.bot.SendMessage(channel, fmt.Sprintf("✅ Slave bot '%s' sikeresen leállt.", name))
		bm.saveState()
	}()
	
	bm.bot.SendMessage(channel, fmt.Sprintf("🔄 Slave bot '%s' leállítása folyamatban...", name))
}

func (bm *BotManagerPlugin) stopSlaveAsync(name string) error {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	slave, exists := bm.slaves[name]
	if !exists {
		return fmt.Errorf("slave bot nem található")
	}

	log.Printf("🛑 Stopping slave bot '%s'", name)
	
	if slave.Conn != nil {
		slave.Conn.Close()
		slave.Conn = nil
	}
	
	if slave.Process != nil {
		log.Printf("   Killing process via Process object (PID: %d)", slave.PID)
		if err := slave.Process.Kill(); err != nil {
			log.Printf("⚠️ Process kill hiba: %v", err)
		}
	} else if slave.PID > 0 {
		log.Printf("   Killing process via PID (PID: %d)", slave.PID)
		process, err := os.FindProcess(slave.PID)
		if err != nil {
			log.Printf("⚠️ Process find error: %v", err)
		} else {
			if err := process.Kill(); err != nil {
				log.Printf("⚠️ PID kill error: %v", err)
			} else {
				log.Printf("✅ Process killed via PID: %d", slave.PID)
			}
		}
	} else {
		log.Printf("⚠️ No PID available for slave: %s", name)
	}

	delete(bm.slaves, name)
	
	go bm.saveState()
	
	log.Printf("✅ Slave bot '%s' stopped", name)
	return nil
}

func (bm *BotManagerPlugin) cmdRestart(channel, name string) {
	bm.bot.SendMessage(channel, fmt.Sprintf("🔄 Slave bot '%s' újraindítása...", name))
	
	err := bm.stopSlaveAsync(name)
	if err != nil {
		bm.bot.SendMessage(channel, fmt.Sprintf("❌ Leállítási hiba: %v", err))
		return
	}
	
	time.Sleep(2 * time.Second)
	
	bm.mutex.RLock()
	cfg, exists := bm.config.Slaves[name]
	bm.mutex.RUnlock()
	
	if !exists {
		bm.bot.SendMessage(channel, fmt.Sprintf("❌ Slave bot '%s' config nem található.", name))
		return
	}
	
	go func() {
		err := bm.startSlaveAsync(name, cfg)
		if err != nil {
			bm.bot.SendMessage(channel, fmt.Sprintf("❌ Indítási hiba: %v", err))
			return
		}
		bm.bot.SendMessage(channel, fmt.Sprintf("✅ Slave bot '%s' újraindult.", name))
		bm.saveState()
	}()
}

func (bm *BotManagerPlugin) autoStartSlaves() {
	for name, cfg := range bm.config.Slaves {
		if cfg.AutoStart {
			bm.mutex.RLock()
			slave, exists := bm.slaves[name]
			alreadyRunning := exists && slave.Status == "running"
			bm.mutex.RUnlock()
			
			if !alreadyRunning {
				log.Printf("🚀 Auto-start: %s", name)
				go func(n string, c SlaveConfig) {
					if err := bm.startSlaveAsync(n, c); err != nil {
						log.Printf("❌ Auto-start hiba (%s): %v", n, err)
					} else {
						log.Printf("✅ Auto-start sikeres: %s", n)
					}
				}(name, cfg)
			} else {
				log.Printf("ℹ️ Slave %s már fut, auto-start kihagyva", name)
			}
		}
	}
}

func (bm *BotManagerPlugin) loadState() {
	data, err := os.ReadFile(bm.stateFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️ State fájl betöltési hiba: %v", err)
		}
		return
	}

	var state StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("⚠️ State JSON parse hiba: %v", err)
		return
	}

	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	for name, slaveState := range state.Slaves {
		if cfg, exists := bm.config.Slaves[name]; exists {
			isRunning := false
			if slaveState.PID > 0 {
				process, err := os.FindProcess(slaveState.PID)
				if err == nil {
					if err := process.Signal(syscall.Signal(0)); err == nil {
						isRunning = true
						log.Printf("📥 Futó slave felismerve: %s (PID: %d)", name, slaveState.PID)
					}
				}
			}

			if isRunning {
				bm.slaves[name] = &ManagedSlave{
					Name:      name,
					Config:    cfg,
					PID:       slaveState.PID,
					Status:    "running",
					StartedAt: slaveState.StartedAt,
					Process:   nil,
					Conn:      nil,
				}
				log.Printf("✅ Futó slave betöltve state-ből: %s (PID: %d)", name, slaveState.PID)
			} else {
				log.Printf("ℹ️ Slave %s nem fut, state törlése", name)
			}
		}
	}

	log.Printf("📥 State betöltve: %d futó slave", len(bm.slaves))
}

func (bm *BotManagerPlugin) saveState() {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()

	state := StateFile{
		Slaves: make(map[string]SlaveState),
	}

	for name, slave := range bm.slaves {
		state.Slaves[name] = SlaveState{
			Name:            name,
			PID:             slave.PID,
			Status:          slave.Status,
			SocketConnected: slave.Conn != nil,
			StartedAt:       slave.StartedAt,
		}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("❌ State JSON marshal hiba: %v", err)
		return
	}

	if err := os.WriteFile(bm.stateFile, data, 0644); err != nil {
		log.Printf("❌ State fájl írási hiba: %v", err)
		return
	}
}

func (bm *BotManagerPlugin) handleModeCommand(botName, channel, user, hostmask, message, modeType string) {
    log.Printf("🔧 Mode command: %s (type: %s)", message, modeType)
    
    parts := strings.Fields(message)
    targetUser := user
    
    if len(parts) >= 2 {
        targetUser = parts[1]
    }
    
    log.Printf("🔧 Mode toggle: %s -> %s on %s", user, targetUser, channel)
    
    // ✅ Simplify hostmask BEFORE permission check
    simplifiedHostmask := YnMModule.SimplifyHostmask(hostmask)
    log.Printf("🔍 Permission check - Original: %s, Simplified: %s", hostmask, simplifiedHostmask)
    
    if bm.adminPlugin != nil {
        // ✅ Map mode type to command name for permission check
        commandName := modeType
        switch modeType {
        case "o":
            commandName = "op"
        case "h":
            commandName = "halfop"
        case "v":
            commandName = "voice"
        }
        
        log.Printf("🔍 Checking permission for command: %s (modeType: %s)", commandName, modeType)
        
        // ✅ Először ellenőrizzük az alap HasAccess-t (owner, admin, stb.)
        hasAccess := bm.adminPlugin.HasAccess(simplifiedHostmask, commandName)
        
        // ✅ Ha nincs alap hozzáférése, ellenőrizzük a session-t
        if !hasAccess {
            hasAccess = bm.adminPlugin.HasAccessWithSession(simplifiedHostmask, commandName)
        }
        
        if !hasAccess {
            log.Printf("❌ No permission for mode %s: %s (simplified: %s)", modeType, hostmask, simplifiedHostmask)
            return
        }
        log.Printf("✅ Permission granted for mode %s: %s", modeType, user)
    }
    
    // ✅ CHECK CURRENT STATE
    modeStatesMutex.Lock()
    
    // Initialize maps if needed
    if modeStates[channel] == nil {
        modeStates[channel] = make(map[string]map[string]bool)
    }
    if modeStates[channel][targetUser] == nil {
        modeStates[channel][targetUser] = make(map[string]bool)
    }
    
    // Toggle state
    currentState := modeStates[channel][targetUser][modeType]
    
    // ✅ FIX: Ha még nincs beállítva (első használat), akkor az első legyen +mode
    newState := true
    if currentState {
        newState = false
    }
    
    modeStates[channel][targetUser][modeType] = newState
    
    modeStatesMutex.Unlock()
    
    // ✅ SEND ONLY ONE MODE COMMAND
    modePrefix := "+"
    if !newState {
        modePrefix = "-"
    }
    
    commandMsg := map[string]interface{}{
        "type":     "command",
        "action":   "mode",
        "bot_name": botName,
        "channel":  channel,
        "user":     targetUser,
        "message":  fmt.Sprintf("%s%s", modePrefix, modeType),
    }
    
    bm.sendCommandToSlave(botName, commandMsg)
    log.Printf("📤 Mode toggle sent: %s%s for %s on %s (new state: %v)", 
        modePrefix, modeType, targetUser, channel, newState)
}