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
	"sort"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)
func (p *YnMApiPlugin) handleStatus(w http.ResponseWriter, r *http.Request) {
    var activeCount, totalCount, totalUses int
    
	db, err := p.getDB()
	if err == nil && db != nil {
		row := db.QueryRow(`
			SELECT 
				COUNT(CASE WHEN password_expires > datetime('now') THEN 1 END) as active,
				COUNT(*) as total,
				COALESCE(SUM(password_uses), 0) as total_uses
			FROM users
			WHERE pass IS NOT NULL
		`)
		_ = row.Scan(&activeCount, &totalCount, &totalUses)
	}
    
    status := map[string]interface{}{
        "active_passwords": activeCount,
        "total_passwords":  totalCount,
        "total_uses":       totalUses,
        "server_time":      time.Now().Format("2006-01-02 15:04:05"),
        "database_ready":   p.isDatabaseReady(),
        "database_type":    "data/ynm.db",
        "max_password_age": "60 minutes",
        "max_uses":         10, 
        "api_version":      "YnM-1.4",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}
func (p *YnMApiPlugin) handleMaxUsesOptions(w http.ResponseWriter, r *http.Request) {
    // Manuálisan definiált opciók, mivel a configból már nincs
    options := map[int]string{
        10:  "10 uses (IRC default)",
        100: "100 uses",
        0:   "Unlimited (never expires)",
    }
    
    // Formázott válasz
    var formattedOptions []map[string]interface{}
    for value, description := range options {
        formattedOptions = append(formattedOptions, map[string]interface{}{
            "value":       value,
            "description": description,
            "display": func() string {
                if value == 0 {
                    return "unlimited"
                }
                return fmt.Sprintf("%d", value)
            }(),
        })
    }
    
    // Rendezés value szerint
    sort.Slice(formattedOptions, func(i, j int) bool {
        return formattedOptions[i]["value"].(int) < formattedOptions[j]["value"].(int)
    })
    
    response := map[string]interface{}{
        "success": true,
        "options": formattedOptions,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (p *YnMApiPlugin) handleBotStats(w http.ResponseWriter, r *http.Request) {
	db, err := p.getDB()
	if err != nil || db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}
    
    query := `SELECT key, value, ram_used_mb, cpu_percent, process_memory_mb, 
                     load_avg, disk_usage, network_traffic, thread_count,
                     nick, version, go_version, bot_uptime, bot_max_uptime,
                     bot_max_connect_time, server_uptime, channels, server,
                     connected, last_updated, total_users, owner, globaladmins, globalmods,
                     globalvips, admins, mods, vips
              FROM bot_stats
              LIMIT 1`
    
    row := db.QueryRow(query)
    
    var key, loadAvg, diskUsage, networkTraffic string
    var nick, version, goVersion, botUptime, botMaxUptime string
    var botMaxConnectTime, serverUptime, channels, server string
    var lastUpdated, owner, globalAdmins, globalMods, globalVips string
    var admins, mods, vips string
    var value, totalUsers, threadCount, connected int
    var ramUsedMB, cpuPercent, processMemoryMB float64
    
    err = row.Scan(
        &key, &value, &ramUsedMB, &cpuPercent, &processMemoryMB,
        &loadAvg, &diskUsage, &networkTraffic, &threadCount,
        &nick, &version, &goVersion, &botUptime, &botMaxUptime,
        &botMaxConnectTime, &serverUptime, &channels, &server,
        &connected, &lastUpdated, &totalUsers, &owner, &globalAdmins, &globalMods,
        &globalVips, &admins, &mods, &vips,
    )
    
    if err != nil {
        response := map[string]interface{}{
            "success": false,
            "error":   "No bot stats found",
            "message": err.Error(),
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
        return
    }
    
    stats := map[string]interface{}{
        "key":                  key,
        "value":                value,
        "ram_used_mb":          ramUsedMB,
        "cpu_percent":          cpuPercent,
        "process_memory_mb":    processMemoryMB,
        "load_avg":             loadAvg,
        "disk_usage":           diskUsage,
        "network_traffic":      networkTraffic,
        "thread_count":         threadCount,
        "nick":                 nick,
        "version":              version,
        "go_version":           goVersion,
        "bot_uptime":           botUptime,
        "bot_max_uptime":       botMaxUptime,
        "bot_max_connect_time": botMaxConnectTime,
        "server_uptime":        serverUptime,
        "channels":             channels,
        "server":               server,
        "connected":            connected,
        "last_updated":         lastUpdated,
        "owner":                owner,
		"total_users":          totalUsers,
        "globaladmins":         globalAdmins,
        "globalmods":           globalMods,
        "globalvips":           globalVips,
        "admins":               admins,
        "mods":                 mods,
        "vips":                 vips,
    }
    
    response := map[string]interface{}{
        "success": true,
        "stats":   stats,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}