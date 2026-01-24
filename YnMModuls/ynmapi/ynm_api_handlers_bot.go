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
package ynmapi

import (

    "encoding/json"
    "fmt"
	"os"
    "net/http"
	"runtime"
    "strings"

    "time"


    _ "github.com/mattn/go-sqlite3"
)

func (p *YnMApiPlugin) handleBotControl(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        Command string `json:"command"`
        Token   string `json:"token"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    username := r.Header.Get("X-Username")
    userRole := p.getUserRole(username)
    
    // Csak owner használhatja
    if !strings.EqualFold(userRole, "owner") {
        p.logAudit(username, "🚫 BOT_CONTROL_DENIED", r.RemoteAddr, 
            fmt.Sprintf("Command: %s, Role: %s", req.Command, userRole))
        http.Error(w, "Only owners can control the bot", http.StatusForbidden)
        return
    }
    
    switch req.Command {
    case "restart":
        p.logAudit(username, "🔄 BOT_RESTART", r.RemoteAddr, "Bot restart initiated")
        
        // Válasz küldése MIELŐTT újraindulna
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Bot is restarting...",
            "command": "restart",
        })
        
        // Késleltetett restart
        go func() {
            time.Sleep(1 * time.Second)
            fmt.Println("[YnMApI] Bot restart requested by", username)
            
            // IRC disconnect
			if p.client != nil && p.client.IsConnected() {
				p.client.SendRaw("QUIT :Restarting...")
				p.client.Disconnect()
			}

            
            // Adatbázis bezárása
            if p.db != nil {
                p.db.Close()
            }
            
            time.Sleep(1 * time.Second)
            os.Exit(0) // Systemd vagy PM2 újraindítja
        }()
        
		
	 case "reload":  // ← IDE TEDD BE!
        p.logAudit(username, "♻️ BOT_RELOAD", r.RemoteAddr, "Configuration reload initiated")
        
        // Itt implementálhatod a config újratöltést
        // Például: pluginok újratöltése, beállítások frissítése
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "message": "Configuration reloaded",
            "command": "reload",
        })
    case "status":
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        ircConnected := p.client != nil && p.client.IsConnected()
        
        status := map[string]interface{}{
            "running":       true,
            "version":       "YnM-v1.0.37.15",
            "uptime":        time.Since(p.startTime).String(),
            "goroutines":    runtime.NumGoroutine(),
            "memory_mb":     m.Alloc / 1024 / 1024,
            "irc_connected": ircConnected,
            "database":      p.isDatabaseReady(),
        }
        
        if ircConnected {
            status["irc_nick"] = p.client.GetNick()
            status["irc_channels"] = len(p.client.GetChannels())
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success": true,
            "data":    status,
        })
        
    case "reconnect":
        if p.client != nil {
            p.logAudit(username, "🔌 IRC_RECONNECT", r.RemoteAddr, "IRC reconnect initiated")
            
            go func() {
				if p.client.IsConnected() {
					p.client.SendRaw("QUIT :Reconnecting...")
					p.client.Disconnect()
					time.Sleep(2 * time.Second)
				}


                // IRC reconnect logika
                p.client.Connect()
            }()
            
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]interface{}{
                "success": true,
                "message": "IRC reconnecting...",
            })
        } else {
            http.Error(w, "IRC client not available", http.StatusServiceUnavailable)
        }
        
    default:
        http.Error(w, "Unknown command", http.StatusBadRequest)
    }
}


func (p *YnMApiPlugin) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    stats := map[string]interface{}{
        "alloc_mb":       m.Alloc / 1024 / 1024,
        "total_alloc_mb": m.TotalAlloc / 1024 / 1024,
        "sys_mb":         m.Sys / 1024 / 1024,
        "num_gc":         m.NumGC,
        "goroutines":     runtime.NumGoroutine(),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "memory":  stats,
    })
} 

func (p *YnMApiPlugin) handleIRCChannels(w http.ResponseWriter, r *http.Request) {
    if p.client == nil || !p.client.IsConnected() {
        http.Error(w, "IRC not connected", http.StatusServiceUnavailable)
        return
    }
    
    channels := p.client.GetChannels()
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "channels": channels,
    })
}

func (p *YnMApiPlugin) handleIRCUsers(w http.ResponseWriter, r *http.Request) {
    channel := r.URL.Query().Get("channel")
    if channel == "" {
        http.Error(w, "Channel parameter required", http.StatusBadRequest)
        return
    }
    
    if p.client == nil || !p.client.IsConnected() {
        http.Error(w, "IRC not connected", http.StatusServiceUnavailable)
        return
    }
    
    users := p.client.GetChannelUsers(channel)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "channel": channel,
        "users":   users,
        "count":   len(users),
    })
}

func (p *YnMApiPlugin) handleIRCSend(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        Channel string `json:"channel"`
        Message string `json:"message"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if p.client == nil || !p.client.IsConnected() {
        http.Error(w, "IRC not connected", http.StatusServiceUnavailable)
        return
    }
    
    p.client.SendMessage(req.Channel, req.Message)
    
    username := r.Header.Get("X-Username")
    p.logAudit(username, "IRC_SEND", r.RemoteAddr, 
        fmt.Sprintf("Channel: %s, Message: %s", req.Channel, req.Message))
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "sent":    true,
    })
}

func (p *YnMApiPlugin) handleIRCJoin(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        Channel string `json:"channel"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if p.client == nil || !p.client.IsConnected() {
        http.Error(w, "IRC not connected", http.StatusServiceUnavailable)
        return
    }
    
    p.client.Join(req.Channel)
    
    username := r.Header.Get("X-Username")
    p.logAudit(username, "IRC_JOIN", r.RemoteAddr, 
        fmt.Sprintf("Channel: %s", req.Channel))
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "joined":  true,
    })
}

func (p *YnMApiPlugin) handleIRCPart(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        Channel string `json:"channel"`
        Message string `json:"message"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if p.client == nil || !p.client.IsConnected() {
        http.Error(w, "IRC not connected", http.StatusServiceUnavailable)
        return
    }
    
    p.client.Part(req.Channel, req.Message)
    
    username := r.Header.Get("X-Username")
    p.logAudit(username, "IRC_PART", r.RemoteAddr, 
        fmt.Sprintf("Channel: %s", req.Channel))
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "parted":  true,
    })
}


