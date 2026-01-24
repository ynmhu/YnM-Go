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
	"os/exec"
	"os/user"
	"strings"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
)

// Rate limiter thread-safe implementáció
type RateLimiter struct {
	mu            sync.Mutex
	lastExecution map[string]time.Time
	defaultCD     time.Duration
}

func NewRateLimiter(defaultCooldown time.Duration) *RateLimiter {
	return &RateLimiter{
		lastExecution: make(map[string]time.Time),
		defaultCD:     defaultCooldown,
	}
}

func (rl *RateLimiter) CanExecute(userID, command string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, command)

	if last, exists := rl.lastExecution[key]; exists {
		remaining := rl.defaultCD - time.Since(last)
		if remaining > 0 {
			return false, remaining
		}
	}
	rl.lastExecution[key] = time.Now()
	return true, 0
}

// Service Manager plugin
type ServiceManager struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	Db          *YnMDb.AdminDB
	rateLimiter *RateLimiter
	timeout     time.Duration
}

func NewServiceManager(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB) *ServiceManager {
	return &ServiceManager{
		bot:         bot,
		adminPlugin: adminPlugin,
		Db:          db,
		rateLimiter: NewRateLimiter(5 * time.Second),
		timeout:     30 * time.Second,
	}
}

func (sm *ServiceManager) HandleMessage(msg YnMIrC.Message) string {
    var nick, hostmask string
    if msg.Sender != "" {
        // IRC user
        nick = strings.Split(msg.Sender, "!")[0]
        hostmask = YnMModule.SimplifyHostmask(msg.Sender)
    } else if msg.Nick != "" {
        // Discord user - lookup by Discord ID
        userInfo, err := sm.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
        if err != nil {
            //fmt.Printf("[ServiceManager] Discord user not found: %s\n", msg.Nick)
            return "❌ You need to link your Discord account first. Use !register"
        }
        nick = userInfo.Nick
        hostmask = userInfo.Hostmask
       // fmt.Printf("[ServiceManager] Discord user resolved: %s -> %s (role: %s)\n", msg.Nick, nick, userInfo.Role)
    } else {
        return ""
    }
    
    prefix := sm.adminPlugin.GetPrefixForHost(hostmask)
    
    if !strings.HasPrefix(strings.ToLower(msg.Text), prefix+"service") {
        return ""
    }

    // ✅ FIX: Bot saját parancsai mindig engedélyezettek
    botNick := sm.bot.GetNick() // vagy hardcode: "YnM-Beta"
    isSelfCommand := (nick == botNick) || strings.HasSuffix(hostmask, "Beta.ynm.hu")
    
    minLevel := 3 
    
    if !isSelfCommand && !YnMModule.HasMinAdminLevelWithDB(sm.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
     //   fmt.Printf("[ServiceManager] Permission denied for %s\n", nick)
        return "❌ You need VIP or higher permissions."
    }
    
  //  fmt.Printf("[ServiceManager] ✅ Access granted for %s (hostmask: %s, self: %v)\n", nick, hostmask, isSelfCommand)

    parts := strings.Fields(msg.Text)
    if len(parts) != 3 {
        return "❌ Használat: !service <szolgáltatás> <művelet>"
    }

    service := sm.resolveServiceAlias(strings.ToLower(parts[1]))
    action := strings.ToLower(parts[2])

    // Rate limit - Discord esetén a Discord ID-t használjuk
    userID := nick
    if msg.Nick != "" {
        userID = msg.Nick
    }
    
    if can, remaining := sm.rateLimiter.CanExecute(userID, action+"_"+service); !can {
        return fmt.Sprintf("❌ Rate limit: várj még %.1f másodpercet", remaining.Seconds())
    }

    if !sm.isValidAction(action) {
        return fmt.Sprintf("❌ Érvénytelen művelet. Engedélyezett: start, stop, restart, status, reload")
    }

    return sm.executeCommand(action, service)
}

func (sm *ServiceManager) executeCommand(action, service string) string {
	var cmd *exec.Cmd
	
	// status parancs nem igényel sudo-t
	if action == "status" {
		cmd = exec.Command("systemctl", "--no-pager", "--plain", action, service)
	} else {
		// start, stop, restart, reload igényli a sudo-t
		cmd = exec.Command("sudo", "systemctl", "--no-pager", "--plain", action, service)
	}
	
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		// Ha sudo jogosultság hiányzik
		if strings.Contains(outputStr, "sudo") || strings.Contains(outputStr, "password") {
			currentUser, userErr := user.Current()
			username := "felhasználó"
			if userErr == nil {
				username = currentUser.Username
			}
			return fmt.Sprintf("❌ Hiányzó sudo jogosultság! Konfiguráld: %s ALL=(ALL) NOPASSWD: /bin/systemctl", username)
		}
		return fmt.Sprintf("❌ Hiba: %v - %s", err, outputStr)
	}

	resp := sm.formatSuccessMessage(action, service)

	if action == "status" {
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			lineTrim := strings.TrimSpace(line)
			if strings.HasPrefix(lineTrim, "Active:") {
				resp += " | " + lineTrim
				break
			}
		}
	}

	return resp
}

func (sm *ServiceManager) resolveServiceAlias(service string) string {
	if service == "apache" {
		return "apache2"
	}
	return service
}

func (sm *ServiceManager) isValidAction(action string) bool {
	switch action {
	case "start", "stop", "restart", "status", "reload":
		return true
	default:
		return false
	}
}

func (sm *ServiceManager) formatSuccessMessage(action, service string) string {
	switch action {
	case "status":
		return fmt.Sprintf("📊 %s státusza:", service)
	case "start":
		return fmt.Sprintf("✅ %s elindítva", service)
	case "stop":
		return fmt.Sprintf("⏹️ %s leállítva", service)
	case "restart":
		return fmt.Sprintf("🔄 %s újraindítva", service)
	case "reload":
		return fmt.Sprintf("🔃 %s konfiguráció újratöltve", service)
	default:
		return fmt.Sprintf("✅ %s %s művelet végrehajtva", service, action)
	}
}

func (sm *ServiceManager) OnTick() []YnMIrC.Message {
	return nil
}