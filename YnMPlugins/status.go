// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  Minden jog fenntartva.
// ==================================================

package ynm

import (
	"fmt"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
)

type StatusPlugin struct {
	client      *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	Db          *YnMDb.AdminDB
}

func NewStatusPlugin(client *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB) *StatusPlugin {
	return &StatusPlugin{
		client:      client,
		adminPlugin: adminPlugin,
		Db:          db,
	}
}

func (p *StatusPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	args := strings.Fields(text)
	
	if len(args) == 0 {
		return ""
	}
	
	// Admin ellenőrzés
	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	
	// Legalább 1-es szint kell (user)
	if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, 1) {
		return ""
	}

	switch args[0] {
	case "!status":
		if len(args) > 1 {
			switch args[1] {
			case "full", "detailed":
				p.DetailedStatusCommand(msg.Channel)
			case "network":
				p.NetworkStatusCommand(msg.Channel)
			case "disk":
				p.DiskStatusCommand(msg.Channel)
			case "system":
				p.SystemStatusCommand(msg.Channel)
			case "pid":
				p.PIDStatusCommand(msg.Channel)
			case "global":
				if len(args) > 2 {
					switch args[2] {
					case "admin", "admins":
						p.GlobalAdminsCommand(msg.Channel)
					case "mod", "mods":
						p.GlobalModsCommand(msg.Channel)
					case "vip", "vips":
						p.GlobalVipsCommand(msg.Channel)
					case "stats":
						p.GlobalStatsCommand(msg.Channel)
					default:
						p.BasicStatusCommand(msg.Channel)
					}
				} else {
					p.GlobalStatsCommand(msg.Channel)
				}
			case "admin", "admins":
				p.AdminsCommand(msg.Channel)
			case "mod", "mods":
				p.ModsCommand(msg.Channel)
			case "vip", "vips":
				p.VipsCommand(msg.Channel)
			case "owner":
				p.OwnerCommand(msg.Channel)
			case "help":
				p.showHelp(msg.Channel)
			default:
				p.BasicStatusCommand(msg.Channel)
			}
		} else {
			p.BasicStatusCommand(msg.Channel)
		}
		return ""
	case "!stats":
		if len(args) > 1 && args[1] == "global" {
			p.GlobalStatsCommand(msg.Channel)
		} else {
			p.BasicStatusCommand(msg.Channel)
		}
		return ""
	}
	return ""
}

// ALAP STATISZTIKA
func (p *StatusPlugin) BasicStatusCommand(channel string) {
	var (
		botUptimeStr     string
		botMaxUptimeStr  string
		botMaxConnectStr string
		serverUptimeStr  string
		ramUsedMB        float64
		cpuPercent       float64
		processMemoryMB  float64
		threadCount      int
		botNick          string
		version          string
		goVersion        string
		server           string
		connected        bool
		loadAvg          string
		diskUsage        string
		diskIO           string
		networkTraffic   string
		pid              int
		execPath         string
		channels         string
		owner            string
		totalUsers	int
	)

	err := p.Db.SQL.QueryRow(`
		SELECT 
			bot_uptime,
			bot_max_uptime,
			bot_max_connect_time,
			server_uptime,
			ram_used_mb,
			cpu_percent,
			process_memory_mb,
			thread_count,
			nick,
			version,
			go_version,
			server,
			connected,
			load_avg,
			disk_usage,
			disk_io,
			network_traffic,
			pid,
			exec_path,
			channels,
			total_users,
			owner
		FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC
		LIMIT 1
	`).Scan(
		&botUptimeStr,
		&botMaxUptimeStr,
		&botMaxConnectStr,
		&serverUptimeStr,
		&ramUsedMB,
		&cpuPercent,
		&processMemoryMB,
		&threadCount,
		&botNick,
		&version,
		&goVersion,
		&server,
		&connected,
		&loadAvg,
		&diskUsage,
		&diskIO,
		&networkTraffic,
		&pid,
		&execPath,
		&channels,
		&totalUsers,
		&owner,

	)

	if err != nil {
		p.client.SendMessage(channel, "❌ Hiba az adatok lekérdezésekor")
		return
	}

	// Connection status
	connStatus := "🔴"
	if connected {
		connStatus = "🟢"
	}

	// TLS status
	tlsStatus := "🔓"
	if p.client.IsTLS() {
		tlsStatus = "🔐"
	}

	// Csatornák száma
	channelCount := 0
	if strings.TrimSpace(channels) != "" {
		channelCount = len(strings.Split(channels, ", "))
	}

	p.client.SendMessage(channel, fmt.Sprintf("🤖 Bot: %s (%s) | %s %s", botNick, version, connStatus, tlsStatus))
	p.client.SendMessage(channel, fmt.Sprintf("🕒 Uptime: %s | Max: %s", botUptimeStr, botMaxUptimeStr))
	p.client.SendMessage(channel, fmt.Sprintf("🔌 Max connect: %s | 🖥️ Server: %s", botMaxConnectStr, server))
	p.client.SendMessage(channel, fmt.Sprintf("🧠 RAM: %.1fMB | 🔄 CPU: %.1f%% | 🧵 %s", ramUsedMB, cpuPercent, loadAvg))
	p.client.SendMessage(channel, fmt.Sprintf("🌐 Network: %s | 💾 Disk: %s", networkTraffic, diskUsage))
	p.client.SendMessage(channel, fmt.Sprintf("📡 Channels: %d | 🏆 Owner: %s | 👥 Total Users: %d", channelCount, owner, totalUsers))
}

// RÉSZLETES STATISZTIKA
func (p *StatusPlugin) DetailedStatusCommand(channel string) {
	var (
		botUptimeStr     string
		botMaxUptimeStr  string
		botMaxConnectStr string
		serverUptimeStr  string
		ramUsedMB        float64
		cpuPercent       float64
		processMemoryMB  float64
		threadCount      int
		botNick          string
		version          string
		goVersion        string
		server           string
		connected        bool
		loadAvg          string
		diskUsage        string
		diskIO           string
		networkTraffic   string
		pid              int
		execPath         string
		channels         string
		owner            string
		totalUsers	int
		lastUpdated      time.Time
	)

	err := p.Db.SQL.QueryRow(`
		SELECT 
			bot_uptime,
			bot_max_uptime,
			bot_max_connect_time,
			server_uptime,
			ram_used_mb,
			cpu_percent,
			process_memory_mb,
			thread_count,
			nick,
			version,
			go_version,
			server,
			connected,
			load_avg,
			disk_usage,
			disk_io,
			network_traffic,
			pid,
			exec_path,
			channels,
			owner,
			total_users,
			last_updated
		FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC
		LIMIT 1
	`).Scan(
		&botUptimeStr,
		&botMaxUptimeStr,
		&botMaxConnectStr,
		&serverUptimeStr,
		&ramUsedMB,
		&cpuPercent,
		&processMemoryMB,
		&threadCount,
		&botNick,
		&version,
		&goVersion,
		&server,
		&connected,
		&loadAvg,
		&diskUsage,
		&diskIO,
		&networkTraffic,
		&pid,
		&execPath,
		&channels,
		&owner,
		&totalUsers,
		&lastUpdated,
	)

	if err != nil {
		p.client.SendMessage(channel, "❌ Hiba az adatok lekérdezésekor")
		return
	}

	// Connection status
	connStatus := "🔴 Nincs kapcsolat"
	if connected {
		connStatus = "🟢 Kapcsolódva"
	}
	// TLS status részletesen
	tlsStatus := "🔓 Insecure Connection"
	if p.client.IsTLS() {
		tlsStatus = "🔐 TLS Encrypted"
	}
	// Csatornák
	channelsList := []string{}
	if strings.TrimSpace(channels) != "" {
		channelsList = strings.Split(channels, ", ")
	}
	channelCount := len(channelsList)

	p.client.SendMessage(channel, "📊 *Részletes Bot Statisztika*")
	p.client.SendMessage(channel, fmt.Sprintf("🤖 Bot: %s | Verzió: %s | Go: %s", botNick, version, goVersion))
	p.client.SendMessage(channel, fmt.Sprintf("⚡ PID: %d | Útvonal: %s", pid, execPath))
	p.client.SendMessage(channel, fmt.Sprintf("🔗 Kapcsolat: %s | %s", connStatus, tlsStatus))
	p.client.SendMessage(channel, fmt.Sprintf("⏱️ Bot uptime: %s | Max uptime: %s", botUptimeStr, botMaxUptimeStr))
	p.client.SendMessage(channel, fmt.Sprintf("🔌 Max connect: %s | Server uptime: %s", botMaxConnectStr, serverUptimeStr))
	p.client.SendMessage(channel, fmt.Sprintf("🧠 RAM: %.1fMB (Process: %.1fMB) | 🔄 CPU: %.1f%%", ramUsedMB, processMemoryMB, cpuPercent))
	p.client.SendMessage(channel, fmt.Sprintf("🧵 Threads: %d | Load: %s", threadCount, loadAvg))
	p.client.SendMessage(channel, fmt.Sprintf("🌐 Hálózat: %s", networkTraffic))
	p.client.SendMessage(channel, fmt.Sprintf("💾 Tárhely: %s | I/O: %s", diskUsage, diskIO))
	p.client.SendMessage(channel, fmt.Sprintf("📡 Csatornák: %d db | 🏆 Owner: %s | 👥 Total Users: %d", channelCount, owner, totalUsers))
	p.client.SendMessage(channel, fmt.Sprintf("🕐 Utolsó frissítés: %s", lastUpdated.Format("15:04:05")))
	
	// Csatornalista (ha kevés)
	if channelCount > 0 && channelCount <= 8 {
		p.client.SendMessage(channel, fmt.Sprintf("📋 Csatornák: %s", strings.Join(channelsList, ", ")))
	}
}

// GLOBAL ADMINS
func (p *StatusPlugin) GlobalAdminsCommand(channel string) {
	var globalAdminsJSON string
	
	err := p.Db.SQL.QueryRow(`
		SELECT globaladmins FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&globalAdminsJSON)

	if err != nil || globalAdminsJSON == "" || globalAdminsJSON == "[]" {
		p.client.SendMessage(channel, "📋 Global Admins: Nincs admin")
		return
	}

	// Parse JSON
	admins := strings.Trim(globalAdminsJSON, "[]\"")
	adminList := []string{}
	if admins != "" {
		// Remove quotes and split
		admins = strings.ReplaceAll(admins, "\"", "")
		adminList = strings.Split(admins, ",")
		for i := range adminList {
			adminList[i] = strings.TrimSpace(adminList[i])
		}
	}

	if len(adminList) == 0 {
		p.client.SendMessage(channel, "📋 Global Admins: Nincs admin")
	} else {
		p.client.SendMessage(channel, fmt.Sprintf("📋 Global Admins (%d): %s", 
			len(adminList), strings.Join(adminList, ", ")))
	}
}

// GLOBAL MODS
func (p *StatusPlugin) GlobalModsCommand(channel string) {
	var globalModsJSON string
	
	err := p.Db.SQL.QueryRow(`
		SELECT globalmods FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&globalModsJSON)

	if err != nil || globalModsJSON == "" || globalModsJSON == "[]" {
		p.client.SendMessage(channel, "📋 Global Mods: Nincs mod")
		return
	}

	// Parse JSON
	mods := strings.Trim(globalModsJSON, "[]\"")
	modList := []string{}
	if mods != "" {
		mods = strings.ReplaceAll(mods, "\"", "")
		modList = strings.Split(mods, ",")
		for i := range modList {
			modList[i] = strings.TrimSpace(modList[i])
		}
	}

	if len(modList) == 0 {
		p.client.SendMessage(channel, "📋 Global Mods: Nincs mod")
	} else {
		p.client.SendMessage(channel, fmt.Sprintf("📋 Global Mods (%d): %s", 
			len(modList), strings.Join(modList, ", ")))
	}
}

// GLOBAL VIPS
func (p *StatusPlugin) GlobalVipsCommand(channel string) {
	var globalVipsJSON string
	
	err := p.Db.SQL.QueryRow(`
		SELECT globalvips FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&globalVipsJSON)

	if err != nil || globalVipsJSON == "" || globalVipsJSON == "[]" {
		p.client.SendMessage(channel, "📋 Global VIPs: Nincs VIP")
		return
	}

	// Parse JSON
	vips := strings.Trim(globalVipsJSON, "[]\"")
	vipList := []string{}
	if vips != "" {
		vips = strings.ReplaceAll(vips, "\"", "")
		vipList = strings.Split(vips, ",")
		for i := range vipList {
			vipList[i] = strings.TrimSpace(vipList[i])
		}
	}

	if len(vipList) == 0 {
		p.client.SendMessage(channel, "📋 Global VIPs: Nincs VIP")
	} else {
		p.client.SendMessage(channel, fmt.Sprintf("📋 Global VIPs (%d): %s", 
			len(vipList), strings.Join(vipList, ", ")))
	}
}

// LOCAL ADMINS
func (p *StatusPlugin) AdminsCommand(channel string) {
	var adminsJSON string
	
	err := p.Db.SQL.QueryRow(`
		SELECT admins FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&adminsJSON)

	if err != nil || adminsJSON == "" || adminsJSON == "[]" {
		p.client.SendMessage(channel, "📋 Local Admins: Nincs admin")
		return
	}

	// Parse JSON
	admins := strings.Trim(adminsJSON, "[]\"")
	adminList := []string{}
	if admins != "" {
		admins = strings.ReplaceAll(admins, "\"", "")
		adminList = strings.Split(admins, ",")
		for i := range adminList {
			adminList[i] = strings.TrimSpace(adminList[i])
		}
	}

	if len(adminList) == 0 {
		p.client.SendMessage(channel, "📋 Local Admins: Nincs admin")
	} else {
		p.client.SendMessage(channel, fmt.Sprintf("📋 Local Admins (%d): %s", 
			len(adminList), strings.Join(adminList, ", ")))
	}
}

// LOCAL MODS
func (p *StatusPlugin) ModsCommand(channel string) {
	var modsJSON string
	
	err := p.Db.SQL.QueryRow(`
		SELECT mods FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&modsJSON)

	if err != nil || modsJSON == "" || modsJSON == "[]" {
		p.client.SendMessage(channel, "📋 Local Mods: Nincs mod")
		return
	}

	// Parse JSON
	mods := strings.Trim(modsJSON, "[]\"")
	modList := []string{}
	if mods != "" {
		mods = strings.ReplaceAll(mods, "\"", "")
		modList = strings.Split(mods, ",")
		for i := range modList {
			modList[i] = strings.TrimSpace(modList[i])
		}
	}

	if len(modList) == 0 {
		p.client.SendMessage(channel, "📋 Local Mods: Nincs mod")
	} else {
		p.client.SendMessage(channel, fmt.Sprintf("📋 Local Mods (%d): %s", 
			len(modList), strings.Join(modList, ", ")))
	}
}

// LOCAL VIPS
func (p *StatusPlugin) VipsCommand(channel string) {
	var vipsJSON string
	
	err := p.Db.SQL.QueryRow(`
		SELECT vips FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&vipsJSON)

	if err != nil || vipsJSON == "" || vipsJSON == "[]" {
		p.client.SendMessage(channel, "📋 Local VIPs: Nincs VIP")
		return
	}

	// Parse JSON
	vips := strings.Trim(vipsJSON, "[]\"")
	vipList := []string{}
	if vips != "" {
		vips = strings.ReplaceAll(vips, "\"", "")
		vipList = strings.Split(vips, ",")
		for i := range vipList {
			vipList[i] = strings.TrimSpace(vipList[i])
		}
	}

	if len(vipList) == 0 {
		p.client.SendMessage(channel, "📋 Local VIPs: Nincs VIP")
	} else {
		p.client.SendMessage(channel, fmt.Sprintf("📋 Local VIPs (%d): %s", 
			len(vipList), strings.Join(vipList, ", ")))
	}
}

// OWNER
func (p *StatusPlugin) OwnerCommand(channel string) {
	var owner string
	
	err := p.Db.SQL.QueryRow(`
		SELECT owner FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&owner)

	if err != nil || owner == "" {
		p.client.SendMessage(channel, "👑 Owner: Nincs beállítva")
		return
	}

	p.client.SendMessage(channel, fmt.Sprintf("👑 Owner: %s", owner))
}

// GLOBAL STATS
func (p *StatusPlugin) GlobalStatsCommand(channel string) {
	var (
		globalAdminsJSON string
		globalModsJSON   string
		globalVipsJSON   string
		owner            string
		totalUsers       int
	)

	err := p.Db.SQL.QueryRow(`
		SELECT globaladmins, globalmods, globalvips, owner 
		FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&globalAdminsJSON, &globalModsJSON, &globalVipsJSON, &owner, &totalUsers)

	if err != nil {
		p.client.SendMessage(channel, "❌ Hiba a global adatok lekérdezésekor")
		return
	}

	// Parse counts
	countAdmins := 0
	if globalAdminsJSON != "" && globalAdminsJSON != "[]" {
		admins := strings.Trim(globalAdminsJSON, "[]\"")
		if admins != "" {
			admins = strings.ReplaceAll(admins, "\"", "")
			countAdmins = len(strings.Split(admins, ","))
		}
	}

	countMods := 0
	if globalModsJSON != "" && globalModsJSON != "[]" {
		mods := strings.Trim(globalModsJSON, "[]\"")
		if mods != "" {
			mods = strings.ReplaceAll(mods, "\"", "")
			countMods = len(strings.Split(mods, ","))
		}
	}

	countVips := 0
	if globalVipsJSON != "" && globalVipsJSON != "[]" {
		vips := strings.Trim(globalVipsJSON, "[]\"")
		if vips != "" {
			vips = strings.ReplaceAll(vips, "\"", "")
			countVips = len(strings.Split(vips, ","))
		}
	}

	p.client.SendMessage(channel, "🌍 *Global Statisztika*")
	p.client.SendMessage(channel, fmt.Sprintf("👑 Owner: %s", owner))
	p.client.SendMessage(channel, fmt.Sprintf("📊 Admins: %d | Mods: %d | VIPs: %d", countAdmins, countMods, countVips))
	p.client.SendMessage(channel, fmt.Sprintf("👥 Összes user: %d", totalUsers))
}

// PID INFO
func (p *StatusPlugin) PIDStatusCommand(channel string) {
	var (
		pid      int
		execPath string
		botNick  string
		version  string
		goVersion string
	)

	err := p.Db.SQL.QueryRow(`
		SELECT pid, exec_path, nick, version, go_version 
		FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&pid, &execPath, &botNick, &version, &goVersion)

	if err != nil {
		p.client.SendMessage(channel, "❌ Hiba a PID adatok lekérdezésekor")
		return
	}

	p.client.SendMessage(channel, "⚡ *Processz Információk*")
	p.client.SendMessage(channel, fmt.Sprintf("🤖 Bot: %s | Verzió: %s", botNick, version))
	p.client.SendMessage(channel, fmt.Sprintf("🔢 PID: %d", pid))
	p.client.SendMessage(channel, fmt.Sprintf("📁 Útvonal: %s", execPath))
	p.client.SendMessage(channel, fmt.Sprintf("🐹 Go verzió: %s", goVersion))
}

// HÁLÓZAT
func (p *StatusPlugin) NetworkStatusCommand(channel string) {
	var (
		networkTraffic string
		botNick        string
		server         string
		connected      bool
	)

	err := p.Db.SQL.QueryRow(`
		SELECT network_traffic, nick, server, connected 
		FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&networkTraffic, &botNick, &server, &connected)

	if err != nil {
		p.client.SendMessage(channel, "❌ Hiba a hálózati adatok lekérdezésekor")
		return
	}

	connStatus := "🔴"
	if connected {
		connStatus = "🟢"
	}

	p.client.SendMessage(channel, "🌐 *Hálózati Statisztika*")
	p.client.SendMessage(channel, fmt.Sprintf("🤖 Bot: %s | %s | 🖥️ %s", botNick, connStatus, server))
	p.client.SendMessage(channel, fmt.Sprintf("📊 Forgalom: %s", networkTraffic))
}

// DISK
func (p *StatusPlugin) DiskStatusCommand(channel string) {
	var (
		diskUsage string
		diskIO    string
		botNick   string
	)

	err := p.Db.SQL.QueryRow(`
		SELECT disk_usage, disk_io, nick 
		FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&diskUsage, &diskIO, &botNick)

	if err != nil {
		p.client.SendMessage(channel, "❌ Hiba a disk adatok lekérdezésekor")
		return
	}

	p.client.SendMessage(channel, "💾 *Tárhely Statisztika*")
	p.client.SendMessage(channel, fmt.Sprintf("🤖 Bot: %s", botNick))
	p.client.SendMessage(channel, fmt.Sprintf("📦 Tárhely használat: %s", diskUsage))
	p.client.SendMessage(channel, fmt.Sprintf("⚡ Disk I/O: %s", diskIO))
}

// RENDSZER
func (p *StatusPlugin) SystemStatusCommand(channel string) {
	var (
		ramUsedMB       float64
		cpuPercent      float64
		processMemoryMB float64
		threadCount     int
		loadAvg         string
		botNick         string
	)

	err := p.Db.SQL.QueryRow(`
		SELECT ram_used_mb, cpu_percent, process_memory_mb, thread_count, load_avg, nick 
		FROM bot_stats 
		WHERE key = 'YnM-Go'
		ORDER BY last_updated DESC LIMIT 1
	`).Scan(&ramUsedMB, &cpuPercent, &processMemoryMB, &threadCount, &loadAvg, &botNick)

	if err != nil {
		p.client.SendMessage(channel, "❌ Hiba a rendszer adatok lekérdezésekor")
		return
	}

	p.client.SendMessage(channel, "🖥️ *Rendszer Statisztika*")
	p.client.SendMessage(channel, fmt.Sprintf("🤖 Bot: %s", botNick))
	p.client.SendMessage(channel, fmt.Sprintf("🧠 RAM: %.1fMB (Process: %.1fMB)", ramUsedMB, processMemoryMB))
	p.client.SendMessage(channel, fmt.Sprintf("🔄 CPU: %.1f%% | 🧵 Threads: %d", cpuPercent, threadCount))
	p.client.SendMessage(channel, fmt.Sprintf("📈 Load: %s", loadAvg))
}

// SEGÍTSÉG
func (p *StatusPlugin) showHelp(channel string) {
	p.client.SendMessage(channel, "📖 *Status Parancsok*")
	p.client.SendMessage(channel, "!status - Alap bot állapot")
	p.client.SendMessage(channel, "!status full - Részletes statisztika")
	p.client.SendMessage(channel, "!status global - Global statisztika")
	p.client.SendMessage(channel, "!status global admin - Global admins")
	p.client.SendMessage(channel, "!status global mod - Global mods")
	p.client.SendMessage(channel, "!status global vip - Global VIPs")
	p.client.SendMessage(channel, "!status admin - Local admins")
	p.client.SendMessage(channel, "!status mod - Local mods")
	p.client.SendMessage(channel, "!status vip - Local VIPs")
	p.client.SendMessage(channel, "!status owner - Owner")
	p.client.SendMessage(channel, "!status pid - Processz információk")
	p.client.SendMessage(channel, "!status network - Hálózati forgalom")
	p.client.SendMessage(channel, "!status disk - Tárhely információk")
	p.client.SendMessage(channel, "!status system - Rendszer terhelés")
	p.client.SendMessage(channel, "!stats global - Global statisztika")
	p.client.SendMessage(channel, "!status help - Ez a segítség")
}

func (p *StatusPlugin) OnTick() []YnMIrC.Message {
	return nil
}