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
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"
    "runtime"
	"path/filepath"
    "strconv"
    "github.com/shirou/gopsutil/cpu"
    "github.com/shirou/gopsutil/process"
    _ "github.com/mattn/go-sqlite3"

)

// ===== STATUS FRISSÍTÉS LOOP =====

func (p *YnMApiPlugin) statusUpdateLoop() {
    normalInterval := 5 * time.Minute
    fastInterval := 60 * time.Second
    for {
        if p.isDatabaseReady() {
            break
        }
        time.Sleep(1 * time.Second)
    }
    ticker := time.NewTicker(normalInterval)
    defer ticker.Stop()
    
    p.updateStatus()
    
    highActivity := false
    lastActivity := time.Now()
    
    for {
        select {
        case <-ticker.C:
            // Ellenőrizzük, van-e magas aktivitás
            if time.Since(lastActivity) < 5*time.Minute {
                if !highActivity {
                    highActivity = true
                    ticker.Reset(fastInterval) // Gyors frissítés
                }
            } else {
                if highActivity {
                    highActivity = false
                    ticker.Reset(normalInterval) // Lassú frissítés
                }
            }
            
            p.updateStatus()
            
        case <-p.statusQuit:
            return
        }
    }
}


func (p *YnMApiPlugin) distinctNicks(db *sql.DB, query string, args ...any) ([]string, error) {
    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    out := make([]string, 0)
    for rows.Next() {
        var n string
        if err := rows.Scan(&n); err == nil && n != "" {
            out = append(out, n)
        }
    }
    return out, nil
}

func (p *YnMApiPlugin) updateStatus() {
	db, err := p.getDB()
    if err != nil || db == nil {
        return
    }

	pid := os.Getpid()
	execPath := p.getExecutablePath()

    server := p.client.GetConfig().Server
    nick := p.client.GetNick()
    connected := 0
    if p.client.IsConnected() {
        connected = 1
    }

	currentUptime := time.Since(p.startedAt).Truncate(time.Second)


	var maxUptimeStr, maxConnectStr sql.NullString
	_ = db.QueryRow(`
	  SELECT bot_max_uptime, bot_max_connect_time
	  FROM bot_stats
	  WHERE key = 'YnM-Go'
	  LIMIT 1
	`).Scan(&maxUptimeStr, &maxConnectStr)
	
	// parse régi max uptime
	maxUptime := time.Duration(0)
	if maxUptimeStr.Valid && maxUptimeStr.String != "" {
		if d, err := time.ParseDuration(maxUptimeStr.String); err == nil {
			maxUptime = d
		}
	}
	var totalUsers int
		_ = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers)
		
	if currentUptime > maxUptime {
		maxUptime = currentUptime
	}


	maxConnect := time.Duration(0)
	if maxConnectStr.Valid && maxConnectStr.String != "" {
		if d, err := time.ParseDuration(maxConnectStr.String); err == nil {
			maxConnect = d
		}
	}

	if connected == 1 && currentUptime > maxConnect {
		maxConnect = currentUptime
	}
	diskIOStr := getBotDiskIO(pid)
	netStr := getSystemNetworkTraffic()
	botUptimeStr := currentUptime.String()
	botMaxUptimeStr := maxUptime.String()
	botMaxConnectStr := maxConnect.String()
    serverUptime := getServerUptime()
    channels := p.client.GetChannels()
    channelsStr := strings.Join(channels, ", ")

    // RAM használat
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    ramUsedMB := float64(memStats.Alloc) / 1024.0 / 1024.0

    // CPU
    cpuPercent := 0.0
    if cpuUsage, err := cpu.Percent(time.Second, false); err == nil && len(cpuUsage) > 0 {
        cpuPercent = cpuUsage[0]
    }

    // Process memory
    procMemMB := 0.0
    if proc, err := process.NewProcess(int32(os.Getpid())); err == nil {
        if mem, err := proc.MemoryInfo(); err == nil {
            procMemMB = float64(mem.RSS) / 1024.0 / 1024.0
        }
    }

    // Load average
	loadStr := fmt.Sprintf("%d goroutines", runtime.NumGoroutine())

    // Disk usage
    diskStr := ""
	botDir, _ := os.Getwd()
	if botDir != "" {
		var dirSize int64
		err = filepath.Walk(botDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				dirSize += info.Size()
			}
			return nil
		})
		
		if err == nil {
			sizeMB := float64(dirSize) / 1024 / 1024
			if sizeMB < 1024 {
				diskStr = fmt.Sprintf("%.1f MB", sizeMB)
			} else {
				diskStr = fmt.Sprintf("%.1f GB", sizeMB/1024)
			}
		} else {
			diskStr = "N/A"
		}
	} else {
		diskStr = "N/A"
	}
	

    // Thread count
    threadCount := runtime.NumGoroutine()


	// GLOBAL owner (users.role alapján, 1 db)
	owner := ""
	_ = db.QueryRow(`SELECT nick FROM users WHERE role='owner' COLLATE NOCASE ORDER BY id LIMIT 1`).Scan(&owner)

	// GLOBAL listák (users.role alapján)
	globalAdmins, _ := p.distinctNicks(db,`SELECT DISTINCT nick FROM users WHERE role='admin' COLLATE NOCASE ORDER BY nick`)
	globalMods, _   := p.distinctNicks(db,`SELECT DISTINCT nick FROM users WHERE role='mod'   COLLATE NOCASE ORDER BY nick`)
	globalVips, _   := p.distinctNicks(db,`SELECT DISTINCT nick FROM users WHERE role='vip'   COLLATE NOCASE ORDER BY nick`)

	// LOCAL listák (channel_users.role alapján)
	admins, _ := p.distinctNicks(db,`SELECT DISTINCT nick FROM channel_users WHERE role='admin' COLLATE NOCASE ORDER BY nick`)
	mods, _   := p.distinctNicks(db,`SELECT DISTINCT nick FROM channel_users WHERE role='mod'   COLLATE NOCASE ORDER BY nick`)
	vips, _   := p.distinctNicks(db,`SELECT DISTINCT nick FROM channel_users WHERE role='vip'   COLLATE NOCASE ORDER BY nick`)

	// JSON stringek bot_stats TEXT mezőkbe
	globalAdminsJSON, _ := json.Marshal(globalAdmins)
	globalModsJSON, _   := json.Marshal(globalMods)
	globalVipsJSON, _   := json.Marshal(globalVips)

	adminsJSON, _ := json.Marshal(admins)
	modsJSON, _   := json.Marshal(mods)
	vipsJSON, _   := json.Marshal(vips)

    // Ellenőrizzük, van-e már ilyen nick a táblában

    statusKey := "YnM-Go"

    _, err = db.Exec(`
        INSERT OR REPLACE INTO bot_stats (
            key, value, ram_used_mb, cpu_percent, process_memory_mb, load_avg, 
            disk_usage, disk_io, network_traffic, thread_count, pid, exec_path, nick, version, go_version, 
            bot_uptime, bot_max_uptime, bot_max_connect_time, server_uptime, 
            channels, server, connected, last_updated, total_users, owner, globaladmins, 
            globalmods, globalvips, admins, mods, vips
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `,
        statusKey,
        0,
        ramUsedMB, cpuPercent, procMemMB, loadStr,
		diskStr, diskIOStr, netStr,
		threadCount, pid, execPath,
		nick, p.Version, p.GoVersion, botUptimeStr,
		botMaxUptimeStr, botMaxConnectStr,
		serverUptime,
		channelsStr, server, connected, time.Now(), totalUsers, owner,
		string(globalAdminsJSON), string(globalModsJSON), string(globalVipsJSON),
		string(adminsJSON), string(modsJSON), string(vipsJSON),
    )

    if err != nil {
        fmt.Println("[YNMAPIPLUGIN] Failed to update status:", err)
    }
}

func getServerUptime() string {
    data, err := os.ReadFile("/proc/uptime")
    if err != nil {
        return "unknown"
    }

    var uptime float64
    if _, err := fmt.Sscanf(string(data), "%f", &uptime); err != nil {
        return "unknown"
    }

    d := time.Duration(uptime) * time.Second
    days := int(d.Hours()) / 24
    hours := int(d.Hours()) % 24
    minutes := int(d.Minutes()) % 60
    seconds := int(d.Seconds()) % 60

    return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
}

// Handler: Aktuális bot státusz
func (p *YnMApiPlugin) handleBotStatus(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

		db, err := p.getDB()
		if err != nil || db == nil {
			respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"success": false,
				"error":   "Database not available",
			})
			return
		}
    row := db.QueryRow(`
        SELECT ram_used_mb, cpu_percent, process_memory_mb, load_avg, disk_usage,
               disk_io, network_traffic, thread_count, pid, exec_path,
               nick, version, go_version, total_users, 
               bot_uptime, bot_max_uptime, bot_max_connect_time,
               server_uptime, channels, server, connected, last_updated
        FROM bot_stats
        WHERE key = 'YnM-Go'
        ORDER BY last_updated DESC
        LIMIT 1
    `)

    var (
        ramUsedMB, cpuPercent, procMemMB float64
        loadStr, diskStr, diskIOStr, netStr string
		threadCount, pid int  
        execPath string    
        nick, version, goVersion string
		totalUsers int
        botUptime, botMaxUptime, botMaxConnectTime string
        serverUptime, channels, server string
        connected int
        lastUpdated time.Time
    )

    err = row.Scan(&ramUsedMB, &cpuPercent, &procMemMB, &loadStr, &diskStr,
        &diskIOStr, &netStr, &threadCount, &pid, &execPath,
        &nick, &version, &goVersion, &totalUsers,
        &botUptime, &botMaxUptime, &botMaxConnectTime,
        &serverUptime, &channels, &server, &connected, &lastUpdated)

    if err != nil {
        if err == sql.ErrNoRows {
            respondJSON(w, http.StatusOK, map[string]interface{}{
                "success": true,
                "message": "No status data available yet",
				"pid": pid,
				"exec_path": execPath,
                "nick": p.client.GetNick(),
                "version": p.Version,
                "go_version": p.GoVersion,
				"total_users": totalUsers,
                "server": p.client.GetConfig().Server,
                "connected": p.client.IsConnected(),
                "repository": p.repository,
            })
            return
        }
        respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
            "success": false,
            "error":   "Failed to get status: " + err.Error(),
        })
        return
    }

    // ---- csatornalista ----
    channelsList := []string{}
    if strings.TrimSpace(channels) != "" {
        channelsList = strings.Split(channels, ", ")
    }

    // ---- helper: distinct nick listák ----
    getDistinctNicks := func(query string, args ...any) ([]string, error) {
        rows, err := db.Query(query, args...)
        if err != nil { return nil, err }
        defer rows.Close()

        out := make([]string, 0)
        for rows.Next() {
            var n string
            if err := rows.Scan(&n); err == nil && n != "" {
                out = append(out, n)
            }
        }
        return out, nil
    }

    // =========================
    // GLOBAL (users táblából)
    // =========================

    // owner (mindig 1)
    globalOwner := ""
    _ = db.QueryRow(`SELECT nick FROM users WHERE role='owner' COLLATE NOCASE ORDER BY id LIMIT 1`).Scan(&globalOwner)

    globalAdmins, _ := getDistinctNicks(`SELECT DISTINCT nick FROM users WHERE role='admin' COLLATE NOCASE ORDER BY nick`)
    globalMods, _ := getDistinctNicks(`SELECT DISTINCT nick FROM users WHERE role='mod' COLLATE NOCASE ORDER BY nick`)
    globalVips, _ := getDistinctNicks(`SELECT DISTINCT nick FROM users WHERE role='vip' COLLATE NOCASE ORDER BY nick`)
    globalUsers, _ := getDistinctNicks(`SELECT DISTINCT nick FROM users WHERE role='user' COLLATE NOCASE ORDER BY nick`)

    // global countok (összes user)
    globalTotal := 0
    _ = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&globalTotal)

    // =========================
    // LOCAL (channel_users táblából)
    // =========================
    // (owner-t itt nem számoljuk, mert azt globálból akarod)
    localAdmins, _ := getDistinctNicks(`SELECT DISTINCT nick FROM channel_users WHERE role='admin' COLLATE NOCASE ORDER BY nick`)
    localMods, _ := getDistinctNicks(`SELECT DISTINCT nick FROM channel_users WHERE role='mod' COLLATE NOCASE ORDER BY nick`)
    localVips, _ := getDistinctNicks(`SELECT DISTINCT nick FROM channel_users WHERE role='vip' COLLATE NOCASE ORDER BY nick`)

    localTotal := 0
    _ = db.QueryRow(`SELECT COUNT(DISTINCT nick) FROM channel_users`).Scan(&localTotal)

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success": true,

        // bot metrikák
		"pid": pid,   
		"exec_path": execPath, 
        "nick": nick,
        "version": version,
        "go_version": goVersion,

        "bot_uptime": botUptime,
        "bot_max_uptime": botMaxUptime,
        "bot_max_connect_time": botMaxConnectTime,

        "server_uptime": serverUptime,
        "channels": channelsList,
        "server": server,
        "connected": connected == 1,
        "last_updated": lastUpdated.Format(time.RFC3339),

        "ram_used_mb": ramUsedMB,
        "cpu_percent": cpuPercent,
        "process_memory_mb": procMemMB,
        "load_avg": loadStr,
        "disk_usage": diskStr,
        "network_traffic": netStr,
        "thread_count": threadCount,

        // GLOBAL roles (users táblából)
		"total_users": totalUsers,
        "owner": globalOwner,
        "globaladmins": globalAdmins,
        "globalmods": globalMods,
        "globalvips": globalVips,
        "globalusers": globalUsers,

        "global_admin_count": len(globalAdmins),
        "global_mod_count": len(globalMods),
        "global_vip_count": len(globalVips),
        "global_user_count": len(globalUsers),
        "global_total_users": globalTotal,

        // LOCAL roles (channel_users táblából)
        "localadmins": localAdmins,
        "localmods": localMods,
        "localvips": localVips,

        "local_admin_count": len(localAdmins),
        "local_mod_count": len(localMods),
        "local_vip_count": len(localVips),
        "local_total_users": localTotal,

        "repository": p.repository,
    })
}
// Handler: Bot státusz történet
func (p *YnMApiPlugin) handleBotStatusHistory(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

	db, err := p.getDB()
	if err != nil || db == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Database not available",
		})
		return
	}

    limit := 100
    if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
        fmt.Sscanf(limitStr, "%d", &limit)
    }

    // FIX: 'status' helyett 'bot_stats' és hozzá kell adni a WHERE feltételt
    rows, err := db.Query(`
        SELECT ram_used_mb, cpu_percent, process_memory_mb, load_avg, disk_usage,
               network_traffic, thread_count, pid, exec_path,
               nick, version, go_version, bot_uptime, server_uptime, channels, 
               server, connected, last_updated, owner
        FROM bot_stats 
        WHERE key = 'YnM-Go'  // <-- FONTOS!
        ORDER BY last_updated DESC
        LIMIT ?
    `, limit)
    if err != nil {
        respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
            "success": false,
            "error":   "Database query failed",
        })
        return
    }
    defer rows.Close()

    var history []map[string]interface{}

    for rows.Next() {
        var (
            ramUsedMB, cpuPercent, procMemMB                                          float64
            loadStr, diskStr, netStr                                                  string
			threadCount, pid int  
			execPath string      
            nick, version, goVersion, botUptime, serverUptime, channels, server, owner string
            connected                                                                 int
            lastUpdated                                                               time.Time
        )

        if err := rows.Scan(&ramUsedMB, &cpuPercent, &procMemMB, &loadStr, &diskStr,
            &netStr, &threadCount, &pid, &execPath,
            &nick, &version, &goVersion, &botUptime, &serverUptime, &channels, 
            &server, &connected, &lastUpdated, &owner); err != nil {
            continue
        }

        channelsList := []string{}
        if channels != "" {
            channelsList = strings.Split(channels, ", ")
        }

        history = append(history, map[string]interface{}{
            "ram_used_mb":      ramUsedMB,
            "cpu_percent":      cpuPercent,
            "process_memory_mb": procMemMB,
            "load_avg":         loadStr,
            "disk_usage":       diskStr,
            "network_traffic":  netStr,
            "thread_count":     threadCount,
			"pid": pid,
			"exec_path": execPath,
            "nick":             nick,
            "version":          version,
            "go_version":       goVersion,
            "bot_uptime":       botUptime,
            "server_uptime":    serverUptime,
            "channels":         channelsList,
            "server":           server,
            "connected":        connected == 1,
            "last_updated":     lastUpdated.Format(time.RFC3339),
            "owner":            owner,
        })
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "history": history,
        "count":   len(history),
    })
}

// ===== CHANNEL HANDLEREK =====

// Handler: Összes csatorna listázása statisztikákkal
func (p *YnMApiPlugin) handleChannelsListWithStats(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
	db, err := p.getDB()
	if err != nil || db == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Database not available",
		})
		return
	}

    // Csatornák lekérése statisztikákkal
    rows, err := db.Query(`
        SELECT 
            c.id, c.name, c.auto_op, c.auto_voice, c.auto_halfop, 
            c.owner, c.owner_hostmask, c.created_at,
            COUNT(DISTINCT cu.id) as user_count,
            COUNT(DISTINCT CASE WHEN cu.role = 'owner' THEN cu.id END) as owner_count,
            COUNT(DISTINCT CASE WHEN cu.role = 'admin' THEN cu.id END) as admin_count,
            COUNT(DISTINCT CASE WHEN cu.role = 'mod' THEN cu.id END) as mod_count,
            COUNT(DISTINCT CASE WHEN cu.role = 'vip' THEN cu.id END) as vip_count
        FROM channels c
        LEFT JOIN channel_users cu ON c.name = cu.channel
        GROUP BY c.id, c.name
        ORDER BY c.name
    `)
    
    if err != nil {
        respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
            "success": false,
            "error":   "Database query failed",
        })
        return
    }
    defer rows.Close()

    var channels []ChannelWithStats
    for rows.Next() {
        var ch ChannelWithStats
        var owner, ownerHostmask sql.NullString
        err := rows.Scan(
            &ch.ID, &ch.Name, &ch.AutoOp, &ch.AutoVoice, &ch.AutoHalfop,
            &owner, &ownerHostmask, &ch.CreatedAt,
            &ch.UserCount, &ch.OwnerCount, &ch.AdminCount, &ch.ModCount, &ch.VipCount,
        )
        
        if err != nil {
            continue
        }
        
        if owner.Valid {
            ch.Owner = owner.String
        }
        if ownerHostmask.Valid {
            ch.OwnerHostmask = ownerHostmask.String
        }
        
        channels = append(channels, ch)
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success":  true,
        "channels": channels,
        "count":    len(channels),
    })
}

// Handler: Egy csatorna részletes adatai (channels + modes + users)
func (p *YnMApiPlugin) handleChannelFullDetail(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    channelName := r.URL.Query().Get("channel")
    if channelName == "" {
        respondJSON(w, http.StatusBadRequest, map[string]interface{}{
            "success": false,
            "error":   "Channel name required",
        })
        return
    }

    // 1. Csatorna alapadatok
    var channel Channel
    var owner, ownerHostmask sql.NullString
    db, err := p.getDB()
	if err != nil || db == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Database not available",
		})
		return
	}
    err = db.QueryRow(`
        SELECT id, name, auto_op, auto_voice, auto_halfop, 
               owner, owner_hostmask, created_at 
        FROM channels 
        WHERE name = ? COLLATE NOCASE
    `, channelName).Scan(
        &channel.ID, &channel.Name, &channel.AutoOp, &channel.AutoVoice,
        &channel.AutoHalfop, &owner, &ownerHostmask, &channel.CreatedAt,
    )
    
    if err != nil {
        if err == sql.ErrNoRows {
            respondJSON(w, http.StatusNotFound, map[string]interface{}{
                "success": false,
                "error":   "Channel not found",
            })
        } else {
            respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
                "success": false,
                "error":   "Database error",
            })
        }
        return
    }
    
    if owner.Valid {
        channel.Owner = owner.String
    }
    if ownerHostmask.Valid {
        channel.OwnerHostmask = ownerHostmask.String
    }

    // 2. Csatorna módok
    var modes []ChannelMode
    modesRows, err := db.Query(`
        SELECT id, channel, modes, mode_params, set_by, 
               created_at, updated_at, active, enabled, mode 
        FROM channel_modes 
        WHERE channel = ? COLLATE NOCASE
    `, channelName)
    
    if err == nil {
        defer modesRows.Close()
        for modesRows.Next() {
            var mode ChannelMode
            modesRows.Scan(
                &mode.ID, &mode.Channel, &mode.Modes, &mode.ModeParams,
                &mode.SetBy, &mode.CreatedAt, &mode.UpdatedAt,
                &mode.Active, &mode.Enabled, &mode.Mode,
            )
            modes = append(modes, mode)
        }
    }

    // 3. Csatorna felhasználók
    var users []ChannelUser
    usersRows, err := db.Query(`
        SELECT id, nick, hostmask, channel, role, 
               auto_op, auto_voice, auto_halfop, created_at, added_by 
        FROM channel_users 
        WHERE channel = ? COLLATE NOCASE
        ORDER BY 
            CASE role 
                WHEN 'owner' THEN 1 
                WHEN 'admin' THEN 2 
                WHEN 'mod' THEN 3 
                WHEN 'vip' THEN 4 
                ELSE 5 
            END, nick
    `, channelName)
    
    if err == nil {
        defer usersRows.Close()
        for usersRows.Next() {
            var user ChannelUser
            usersRows.Scan(
                &user.ID, &user.Nick, &user.Hostmask, &user.Channel,
                &user.Role, &user.AutoOp, &user.AutoVoice,
                &user.AutoHalfop, &user.CreatedAt, &user.AddedBy,
            )
            users = append(users, user)
        }
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "channel": channel,
        "modes":   modes,
        "users":   users,
        "stats": map[string]int{
            "total_users":  len(users),
            "total_modes":  len(modes),
        },
    })
}

// Handler: Csatorna + Felhasználók lekérése (egyszerűsített)
func (p *YnMApiPlugin) handleChannelWithUsers(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
	db, err := p.getDB()
	if err != nil || db == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Database not available",
		})
		return
	}
    // Összes csatorna és hozzájuk tartozó felhasználók
    rows, err := db.Query(`
        SELECT 
            c.id, c.name, c.auto_op, c.auto_voice, c.auto_halfop,
            c.owner, c.owner_hostmask, c.created_at,
            cu.id, cu.nick, cu.hostmask, cu.role, 
            cu.auto_op, cu.auto_voice, cu.auto_halfop, 
            cu.created_at, cu.added_by
        FROM channels c
        LEFT JOIN channel_users cu ON c.name = cu.channel
        ORDER BY c.name, 
            CASE cu.role 
                WHEN 'owner' THEN 1 
                WHEN 'admin' THEN 2 
                WHEN 'mod' THEN 3 
                WHEN 'vip' THEN 4 
                ELSE 5 
            END, cu.nick
    `)
    
    if err != nil {
        respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
            "success": false,
            "error":   "Database query failed",
        })
        return
    }
    defer rows.Close()

    channelsMap := make(map[string]*ChannelWithStats)
    
    for rows.Next() {
        var chID int
        var chName string
        var chAutoOp, chAutoVoice, chAutoHalfop bool
        var chOwner, chOwnerHostmask sql.NullString
        var chCreatedAt time.Time
        
        var cuID sql.NullInt64
        var cuNick, cuHostmask, cuRole, cuAddedBy sql.NullString
        var cuAutoOp, cuAutoVoice, cuAutoHalfop sql.NullBool
        var cuCreatedAt sql.NullTime
        
        err := rows.Scan(
            &chID, &chName, &chAutoOp, &chAutoVoice, &chAutoHalfop,
            &chOwner, &chOwnerHostmask, &chCreatedAt,
            &cuID, &cuNick, &cuHostmask, &cuRole,
            &cuAutoOp, &cuAutoVoice, &cuAutoHalfop,
            &cuCreatedAt, &cuAddedBy,
        )
        
        if err != nil {
            continue
        }
        
        // Ha még nincs a map-ben, hozzáadjuk a csatornát
        if _, exists := channelsMap[chName]; !exists {
            ch := &ChannelWithStats{}
            ch.ID = chID
            ch.Name = chName
            ch.AutoOp = chAutoOp
            ch.AutoVoice = chAutoVoice
            ch.AutoHalfop = chAutoHalfop
            ch.CreatedAt = chCreatedAt
            
            if chOwner.Valid {
                ch.Owner = chOwner.String
            }
            if chOwnerHostmask.Valid {
                ch.OwnerHostmask = chOwnerHostmask.String
            }
            
            channelsMap[chName] = ch
        }
        
        // Ha van felhasználó, hozzáadjuk a statisztikához
        if cuID.Valid {
            channelsMap[chName].UserCount++
            
            if cuRole.Valid {
                switch strings.ToLower(cuRole.String) {
                case "owner":
                    channelsMap[chName].OwnerCount++
                case "admin":
                    channelsMap[chName].AdminCount++
                case "mod":
                    channelsMap[chName].ModCount++
                case "vip":
                    channelsMap[chName].VipCount++
                }
            }
        }
    }

    // Map -> Slice
    var channels []ChannelWithStats
    for _, ch := range channelsMap {
        channels = append(channels, *ch)
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success":  true,
        "channels": channels,
        "count":    len(channels),
    })
}
func (p *YnMApiPlugin) getExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	return execPath
}

func getBotDiskIO(pid int) string {
    ioPath := fmt.Sprintf("/proc/%d/io", pid)
    data, err := os.ReadFile(ioPath)
    if err != nil {
        return "📖 0.0 MB | 📝 0.0 MB"
    }

    var readBytes, writeBytes uint64
    lines := strings.Split(string(data), "\n")
    
    for _, line := range lines {
        if strings.HasPrefix(line, "read_bytes:") {
            fmt.Sscanf(strings.TrimSpace(line[11:]), "%d", &readBytes)
        }
        if strings.HasPrefix(line, "write_bytes:") {
            fmt.Sscanf(strings.TrimSpace(line[12:]), "%d", &writeBytes)
        }
    }

    readMB := float64(readBytes) / 1024 / 1024
    writeMB := float64(writeBytes) / 1024 / 1024

    return fmt.Sprintf("📖 %.1f MB | 📝 %.1f MB", readMB, writeMB)
}
func getSystemNetworkTraffic() string {
    data, err := os.ReadFile("/proc/net/dev")
    if err != nil {
        return "↑ N/A MB ↓ N/A MB"
    }
    
    var rxBytes, txBytes uint64
    lines := strings.Split(string(data), "\n")
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if strings.Contains(line, ":") && !strings.HasPrefix(line, "lo:") {
            parts := strings.Split(line, ":")
            if len(parts) >= 2 {
                fields := strings.Fields(parts[1])
                if len(fields) >= 16 {
                    if rx, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
                        rxBytes += rx
                    }
                    if tx, err := strconv.ParseUint(fields[8], 10, 64); err == nil {
                        txBytes += tx
                    }
                }
            }
        }
    }
    
    rxMB := float64(rxBytes) / 1024 / 1024
    txMB := float64(txBytes) / 1024 / 1024
    
    return fmt.Sprintf("↑ %.1f MB ↓ %.1f MB", txMB, rxMB)
}
// ===== SEGÉDFÜGGVÉNYEK =====

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}
