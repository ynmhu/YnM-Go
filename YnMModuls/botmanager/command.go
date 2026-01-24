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
	"strconv"
    "strings"
 //   "syscall"
    "time"
	"net"

)

func (bm *BotManagerPlugin) handleCycleCommand(botName, channel, user, hostmask, message string) {
    log.Printf("🔄 [BOTMANAGER] Cycle command received, forwarding to slave: %s", botName)
    
    bm.mutex.RLock()
    slave, exists := bm.slaves[botName]
    bm.mutex.RUnlock()
    
    if !exists || slave.Conn == nil {
        log.Printf("❌ Cannot send cycle command: %s (no connection)", botName)
        return
    }
    
    // ✅ PARANCS KÜLDÉSE A SLAVE BOTNAK
    commandMsg := map[string]interface{}{
        "type":     "command",
        "action":   "cycle", 
        "bot_name": botName,
        "channel":  channel,
        "user":     user,
    }
    
    log.Printf("📤 Sending CYCLE command to slave %s for channel %s", botName, channel)
    
    data, err := json.Marshal(commandMsg)
    if err != nil {
        log.Printf("❌ Cycle command JSON error: %v", err)
        return
    }
    
    data = append(data, '\n')
    _, err = slave.Conn.Write(data)
    if err != nil {
        log.Printf("❌ Cycle command send error (%s): %v", botName, err)
        slave.Conn.Close()
        slave.Conn = nil
    } else {
        log.Printf("✅ Cycle command sent to slave %s", botName)
    }
}
func (bm *BotManagerPlugin) handleOpCommand(botName, channel, user, hostmask, message string) {
    parts := strings.Fields(message)
    if len(parts) < 2 {
        return
    }
    
    targetUser := parts[1]
    
    commandMsg := map[string]interface{}{
        "type":     "command",
        "action":   "op", 
        "bot_name": botName,
        "channel":  channel,
        "user":     targetUser,
    }
    
    bm.sendCommandToSlave(botName, commandMsg)
}

func (bm *BotManagerPlugin) handleVoiceCommand(botName, channel, user, hostmask, message string) {
    log.Printf("🔊 Voice command from %s: %s", user, message)
    
    parts := strings.Fields(message)
    if len(parts) < 2 {
        log.Printf("❌ Voice command missing target user")
        return
    }
    
    targetUser := parts[1]
    
    // ✅ JOGOSULTSÁG ELLENŐRZÉS owner PLUGINNAL
    if bm.adminPlugin != nil {
        hasAccess := bm.adminPlugin.HasAccessWithSession(hostmask, "voice")
        if !hasAccess {
            log.Printf("❌ No permission for voice command: %s", hostmask)
            bm.sendReplyToSlave(botName, channel, "❌ Nincs jogosultságod a voice parancshoz!")
            return
        }
    }
    
    // ✅ PARANCS KÜLDÉSE A SLAVE BOTNAK
    commandMsg := map[string]interface{}{
        "type":     "command",
        "action":   "voice", 
        "bot_name": botName,
        "channel":  channel,
        "user":     targetUser,
    }
    
    bm.sendCommandToSlave(botName, commandMsg)
    log.Printf("📤 Voice command sent to slave %s: %s -> %s", botName, targetUser, channel)
}
// sendCommandToSlave - Általános parancs küldés
func (bm *BotManagerPlugin) sendCommandToSlave(botName string, commandMsg map[string]interface{}) {
    bm.mutex.RLock()
    slave, exists := bm.slaves[botName]
    bm.mutex.RUnlock()
    
    if !exists || slave.Conn == nil {
        return
    }
    
    data, err := json.Marshal(commandMsg)
    if err != nil {
        return
    }
    
    data = append(data, '\n')
    slave.Conn.Write(data)
}
func (bm *BotManagerPlugin) processSessionMessage(conn net.Conn, msg map[string]interface{}) {
    action := msg["action"].(string)
    botName := msg["bot_name"].(string)
    hostmask := msg["hostmask"].(string)
    channel := msg["channel"].(string)
    
    log.Printf("🔐 Session request from %s: %s for %s", botName, action, hostmask)
    
    switch action {
    case "get":
        // Session lekérése
        bm.handleGetSession(conn, botName, hostmask, channel)
        
    default:
        log.Printf("⚠️ Ismeretlen session action: %s", action)
    }
}
func (bm *BotManagerPlugin) handleGetSession(conn net.Conn, botName, hostmask, channel string) {
    log.Printf("🔐 Get session for %s (hostmask: %s)", botName, hostmask)
    
    var hasSession bool
    var username string
    
    // ✅ owner PLUGIN HASZNÁLATA session ellenőrzéshez
    if bm.adminPlugin != nil {
        session, exists := bm.adminPlugin.GetSessionByHost(hostmask)
        if exists {
            hasSession = true
            username = session.LoggedInAs
            log.Printf("🔐 Session found for %s: %s", hostmask, username)
        } else {
            log.Printf("🔐 No session for %s", hostmask)
        }
    }
    
    // ✅ VÁLASZ KÜLDÉSE A SLAVE BOTNAK
    responseMsg := map[string]interface{}{
        "type":     "session",
        "action":   "get_response", 
        "bot_name": botName,
        "hostmask": hostmask,
        "channel":  channel,
        "data": map[string]interface{}{
            "has_session": hasSession,
            "username":    username,
            "logged_in":   hasSession,
        },
    }
    
    data, err := json.Marshal(responseMsg)
    if err != nil {
        log.Printf("❌ Session response JSON marshal hiba: %v", err)
        return
    }
    
    data = append(data, '\n')
    _, err = conn.Write(data)
    if err != nil {
        log.Printf("❌ Session response küldési hiba (%s): %v", botName, err)
    } else {
        log.Printf("📤 Session response sent to %s: has_session=%v", botName, hasSession)
    }
}

// command.go - helyettesítsd a handleSlaveUptimeCommand függvényt ezzel:

func (bm *BotManagerPlugin) handleSlaveUptimeCommand(botName, channel, user string) {
    bm.mutex.RLock()
    slave, exists := bm.slaves[botName]
    bm.mutex.RUnlock()
    
    if !exists {
        log.Printf("❌ Unknown slave bot: %s", botName)
        bm.sendReplyToSlave(botName, channel, "❌ Ismeretlen slave bot")
        return
    }
    
    // 1. SLAVE UPTIME
    slaveUptime := time.Since(slave.StartedAt).Round(time.Second)
    socketStatus := "❌"
    if slave.Conn != nil {
        socketStatus = "✅"
    }
    
    // 2. MASTER UPTIME (kérdezzük az UptimePlugin-től)
    // Ehhez a master startTime-ja kell, ami az UptimePlugin-ben van
    // Egyszerűsítés: a master uptime-ja az BotManager startTime-ja
    masterUptime := time.Since(bm.startTime).Round(time.Second)
    
    // 3. SLAVE RAM HASZNÁLAT (kérdezzük a slave processztől)
    slaveRAM := bm.getSlaveRAMUsage(slave.PID)
    
    // 4. SLAVE PID ÉS PATH
    slavePID := slave.PID
    slavePath := bm.getSlavePath(slave.PID)
    
    // Sorok
    line1 := fmt.Sprintf("🤖 Slave Bot: %s", botName)
    line2 := fmt.Sprintf("🕒 Slave Uptime: %s | 🔌 Socket: %s", slaveUptime, socketStatus)
    line3 := fmt.Sprintf("👑 Master: ✅ Online | 🕒 Uptime: %s", masterUptime)
    line4 := fmt.Sprintf("📊 RAM: %s | 🔢 PID: %d", slaveRAM, slavePID)
    line5 := fmt.Sprintf("📍 Path: %s", slavePath)
    
    // Küldés
    fullResponse := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", 
        line1, line2, line3, line4, line5)
    
    bm.sendMultiLineResponse(botName, channel, user, fullResponse)
}

// Helper függvények:
func (bm *BotManagerPlugin) getSlaveRAMUsage(pid int) string {
    if pid <= 0 {
        return "unknown"
    }
    
    // Linux: /proc/[pid]/statm
    statmPath := fmt.Sprintf("/proc/%d/statm", pid)
    data, err := os.ReadFile(statmPath)
    if err != nil {
        return "unknown"
    }
    
    fields := strings.Fields(string(data))
    if len(fields) < 2 {
        return "unknown"
    }
    
    // page size * pages (általában 4096 bytes/page)
    pages, _ := strconv.ParseInt(fields[1], 10, 64)
    ramKB := (pages * 4) // 4KB per page
    
    return fmt.Sprintf("%dKB", ramKB)
}

func (bm *BotManagerPlugin) getSlavePath(pid int) string {
    if pid <= 0 {
        return "unknown"
    }
    
    procPath := fmt.Sprintf("/proc/%d/exe", pid)
    path, err := os.Readlink(procPath)
    if err != nil {
        return "unknown"
    }
    
    return path
}


func (bm *BotManagerPlugin) sendMultiLineResponse(botName, channel, user, response string) {
    var lines []string
    
    // ✅ ~~~ szeparátort elsőbbséggel kezeljük
    if strings.Contains(response, "~~~") {
        lines = strings.Split(response, "~~~")
        log.Printf("🔍 sendMultiLineResponse: %d sor (~~~ szeparátor)", len(lines))
    } else if strings.Contains(response, "\n") {
        lines = strings.Split(response, "\n")
        log.Printf("🔍 sendMultiLineResponse: %d sor (\\n szeparátor)", len(lines))
    } else {
        lines = []string{response}
        log.Printf("🔍 sendMultiLineResponse: 1 sor (nincs szeparátor)", len(lines))
    }
    
    for i, line := range lines {
        line = strings.TrimSpace(line)
        if line != "" {
            log.Printf("🔍 Küldés %d. sor: %q", i+1, line)
            bm.sendReplyToSlave(botName, channel, line)
            
            if i < len(lines)-1 {
                time.Sleep(200 * time.Millisecond)
            }
        }
    }
}