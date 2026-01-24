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

	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type UptimePlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	Db          *YnMDb.AdminDB
}

func NewUptimePlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB) *UptimePlugin {
	return &UptimePlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		Db:          db,
	}
}

func (p *UptimePlugin) HandleMessage(msg YnMIrC.Message) string {
	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
	prefix := p.adminPlugin.GetPrefixForHost(hostmask)
	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	minLevel := 1

	if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
		return ""
	}

	if !strings.HasPrefix(strings.ToLower(msg.Text), prefix+"uptime") {
		return ""
	}

	// CSAK UPTIME ADATOK AZ ADATBÁZISBÓL
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
		server           string
		connected        bool
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
			server,
			connected
		FROM bot_stats 
		WHERE key = 'YnM-Go'
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
		&server,
		&connected,
	)

	if err != nil {
		return fmt.Sprintf("❌ Hiba az adatok lekérdezésekor: %v", err)
	}

	// Connection status ikon
	connStatus := "🔴"
	if connected {
		connStatus = "🟢"
	}

	// Kapcsolat státusz szöveg
	connText := "Nincs kapcsolat"
	if connected {
		connText = "Kapcsolódva"
	}

	// Multi-line válasz
	line1 := fmt.Sprintf("🤖 Bot: %s | %s %s | 🧵 %d", 
		botNick, connStatus, connText, threadCount)
	line2 := fmt.Sprintf("🕒 Bot uptime: %s | 📈 Max uptime: %s", 
		botUptimeStr, botMaxUptimeStr)
	line3 := fmt.Sprintf("🔌 Max connect time: %s | 🖥️ Server uptime: %s", 
		botMaxConnectStr, serverUptimeStr)
	line4 := fmt.Sprintf("📊 RAM: %.1fMB (Go: %.1fMB) | 🔄 CPU: %.1f%%", 
		ramUsedMB, processMemoryMB, cpuPercent)
	line5 := fmt.Sprintf("🌐 Server: %s", server)

	// IRC-ben külön üzenetként küldjük
	p.bot.SendMessage(msg.Channel, line1)
	p.bot.SendMessage(msg.Channel, line2)
	p.bot.SendMessage(msg.Channel, line3)
	p.bot.SendMessage(msg.Channel, line4)
	
	return line5
}

func (p *UptimePlugin) OnTick() []YnMIrC.Message {
	return nil
}