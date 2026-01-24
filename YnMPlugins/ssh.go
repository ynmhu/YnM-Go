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

package ynm

import (
	"fmt"
	"net"
	"strings"
	"time"
	"sync"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type SSHPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	timeout     time.Duration
	mu          sync.Mutex
}

func NewSSHPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, timeout time.Duration) *SSHPlugin {
	return &SSHPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		timeout:     timeout,
	}
}

func (p *SSHPlugin) HandleMessage(msg YnMIrC.Message) string {
	if !strings.HasPrefix(strings.ToLower(msg.Text), "!ssh") {
		return ""
	}
	
	var nick, hostmask string
	
	if msg.Sender != "" {
		// IRC user - session támogatással
		fullHostmask := msg.Sender
		effectiveUser, effectiveHost := p.adminPlugin.GetEffectiveUser(fullHostmask)
		nick = effectiveUser
		hostmask = effectiveHost
	} else if msg.Nick != "" {
		// Discord user - lookup by Discord ID
		userInfo, err := p.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
		if err != nil {
			fmt.Printf("[SSH Plugin] Discord user not found: %s\n", msg.Nick)
			return "❌ You need to link your Discord account first. Use !register"
		}
		nick = userInfo.Nick
		hostmask = userInfo.Hostmask
		fmt.Printf("[SSH Plugin] Discord user resolved: %s -> %s (role: %s)\n", msg.Nick, nick, userInfo.Role)
	} else {
		return ""
	}
	
	// Check permissions
	if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, 3) {
		fmt.Printf("[SSH Plugin] Permission denied for %s\n", nick)
		return "❌ You need VIP or higher permissions."
	}
	
	parts := strings.Fields(msg.Text)
	if len(parts) < 2 {
		return "Usage: !ssh <host> <port>"
	}
	if len(parts) < 3 {
		return "Usage: !ssh <host> <port>"
	}
	
	host := parts[1]
	port := parts[2]
	
	// Szinkron ellenőrzés - azonnal visszatér az eredménnyel
	return p.checkSSHPortSync(host, port)
}

// Új szinkron verzió
func (p *SSHPlugin) checkSSHPortSync(host, port string) string {
	address := net.JoinHostPort(host, port)
	
	conn, err := net.DialTimeout("tcp", address, p.timeout)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Sprintf("🔴 %s:%s - Connection refused", host, port)
		}
		return fmt.Sprintf("🔴 %s:%s - Port closed", host, port)
	}
	defer conn.Close()
	
	return fmt.Sprintf("🟢 %s:%s - Port open", host, port)
}

// A régi goroutine verzió maradhat IRC-hez, ha akarod
func (p *SSHPlugin) checkSSHPort(host, port, channel string) {
	address := net.JoinHostPort(host, port)
	
	conn, err := net.DialTimeout("tcp", address, p.timeout)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			p.bot.SendMessage(channel, fmt.Sprintf("🔴 %s:%s - Connection refused", host, port))
		} else {
			p.bot.SendMessage(channel, fmt.Sprintf("🔴 %s:%s - Port closed", host, port))
		}
		return
	}
	defer conn.Close()
	
	p.bot.SendMessage(channel, fmt.Sprintf("🟢 %s:%s - Port open", host, port))
}

func (p *SSHPlugin) OnTick() []YnMIrC.Message {
	return nil
}
