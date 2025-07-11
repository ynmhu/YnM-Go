// ============================================================================
//  Szerzői jog © 2024 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ============================================================================

package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"log"

	"github.com/ynmhu/YnM-Go/config"
	"github.com/ynmhu/YnM-Go/irc"
)

// Admin levels using numeric values
const (
	AdminLevelNone  = 0
	AdminLevelVIP   = 1
	AdminLevelAdmin = 2
	AdminLevelOwner = 3
)

// AdminPlugin handles administrative commands
type AdminPlugin struct {
	bot              *irc.Client
	cfg              *config.Config
	mu               sync.RWMutex
	userBanUntil     map[string]time.Time
	userRequestTimes map[string][]time.Time
	userBanNotified  map[string]bool
	store            *MultiAdminStore
	currentUsers     map[string]string 
	hasInitialOwner bool
}

func NewAdminPlugin(cfg *config.Config) *AdminPlugin {
	return &AdminPlugin{
		cfg:              cfg,
		userBanUntil:     make(map[string]time.Time),
		userRequestTimes: make(map[string][]time.Time),
		userBanNotified:  make(map[string]bool),
		currentUsers:     make(map[string]string),
	}
}

func (p *AdminPlugin) Initialize(bot *irc.Client) {
    p.bot = bot
	
	// Create data directory if it doesn't exist
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		if err := os.Mkdir("data", 0755); err != nil {
			fmt.Printf("Error creating data directory: %v\n", err)
		}
	}
	
	// Initialize the admin store
    p.store = NewMultiAdminStore()
    if err := p.store.Load(); err != nil {
        fmt.Printf("Error loading admin store: %v\n", err)
    }

    p.hasInitialOwner = p.store.HasOwner()
}

func init() {
    log.Println("Bot starting up...")
}

func (p *AdminPlugin) GetAdminLevel(nick, hostmask string) int {
    return p.store.GetAdminLevel(nick, hostmask)
}

func (p *AdminPlugin) HandleMessage(msg irc.Message) string {
	if !strings.HasPrefix(msg.Text, "!") {
		// Track user hostmasks for current users
		p.mu.Lock()
		if strings.Contains(msg.Sender, "!") {
			nick := strings.Split(msg.Sender, "!")[0]
			p.currentUsers[nick] = msg.Sender
		}
		p.mu.Unlock()
		return ""
	}

	parts := strings.Fields(msg.Text)
	if len(parts) == 0 {
		return ""
	}

	cmd := parts[0]
	fullHostmask := msg.Sender 
	nick := strings.Split(fullHostmask, "!")[0]

	// Handle !hello command (first-time setup)
	if cmd == "!hello" {
		if p.store.HasOwner() {
			return ""
		}

		hostmask := simplifyHostmask(fullHostmask) // egyszerűsítés

		info := AdminInfo{
			Nick:     nick,
			Hostmask: hostmask,
			Level:    AdminLevelOwner,
			AddedBy:  "system",
			AddedAt:  time.Now(),
		}
		if err := p.store.AddAdmin(info); err != nil {
			return "Error saving admin data"
		}
		return fmt.Sprintf("%s registered as bot owner (level 3)", nick)
	}

	// Check admin status
	adminLevel := p.store.GetAdminLevel(nick, fullHostmask)
	if adminLevel == AdminLevelNone {
		return ""
	}

	// Handle admin commands based on privilege level
	switch cmd {
	case "!die":
		if adminLevel >= AdminLevelOwner {
			p.bot.SendMessage(p.cfg.ConsoleChannel, "Shutting down by admin command...")
			go func() {
				time.Sleep(1 * time.Second)
				os.Exit(0)
			}()
			return "Shutting down..."
		}
		return "Insufficient privileges (requires level 3)"
		
	case "!restart":
		if adminLevel >= AdminLevelAdmin {
			p.bot.SendMessage(p.cfg.ConsoleChannel, "Restarting...")
			go p.restartBot()
			return "Restarting..."
		}
		return "Insufficient privileges (requires level 2)"
		
	case "!rehash":
		if adminLevel >= AdminLevelAdmin {
			newCfg, err := config.Load("config/config.yaml")
			if err != nil {
				return fmt.Sprintf("Config reload error: %v", err)
			}

			oldChannels := make(map[string]struct{})
			for _, ch := range p.cfg.Channels {
				oldChannels[ch] = struct{}{}
			}

			newChannels := make(map[string]struct{})
			for _, ch := range newCfg.Channels {
				newChannels[ch] = struct{}{}
			}

			// Kilépés azokról a csatornákról, amik törlődtek
			for ch := range oldChannels {
				if _, ok := newChannels[ch]; !ok {
					p.bot.SendRaw("PART " + ch)
					// vagy p.bot.Part(ch), ha van ilyen metódusod (ha nincs, implementáld)
				}
			}

			// Belépés az új csatornákba
			for ch := range newChannels {
				if _, ok := oldChannels[ch]; !ok {
					p.bot.Join(ch)
				}
			}

			p.cfg = newCfg

			return "Configuration reloaded, channels updated"
		}
		return "Insufficient privileges (requires level 2)"

		
	case "!addadmin":
		if adminLevel >= AdminLevelAdmin && len(parts) >= 2 {
			return p.handleAddAdmin(parts[1:], nick, adminLevel)
		}
		return "Usage: !addadmin <nick> [level] [hostmask] (requires level 2)"
		
	case "!deladmin":
		if adminLevel >= AdminLevelAdmin && len(parts) >= 2 {
			return p.handleDelAdmin(parts[1], nick, adminLevel)
		}
		return "Usage: !deladmin <nick> (requires level 2)"
		
	case "!listadmins":
		if adminLevel >= AdminLevelVIP {
			return p.handleListAdmins()
		}
		return "Insufficient privileges (requires level 1)"
		
	case "!admininfo":
		if adminLevel >= AdminLevelVIP {
			target := nick
			if len(parts) >= 2 {
				target = parts[1]
			}
			return p.handleAdminInfo(target)
		}
		return "Insufficient privileges (requires level 1)"
		
	case "!help":
		return p.handleHelp(adminLevel)
		
	case "!whoami":
		// Debug command to show user's admin status
		if adminLevel > AdminLevelNone {
			levelStr := p.getLevelString(adminLevel)
			return fmt.Sprintf("You are %s (%s - level %d). Hostmask: %s", 
				nick, levelStr, adminLevel, fullHostmask)
		}
		return fmt.Sprintf("You are %s with no admin privileges (level 0). Hostmask: %s", nick, fullHostmask)
	}

	return ""
}



func (p *AdminPlugin) getLevelString(level int) string {
	switch level {
	case AdminLevelVIP:
		return "VIP"
	case AdminLevelAdmin:
		return "Admin"
	case AdminLevelOwner:
		return "Owner"
	default:
		return "Unknown"
	}
}

func (p *AdminPlugin) handleAddAdmin(args []string, requester string, requesterLevel int) string {
	if len(args) < 1 {
		return "Usage: !addadmin <nick> [level] [hostmask] - Levels: 1=VIP, 2=Admin, 3=Owner"
	}
	
	nick := args[0]
	level := AdminLevelVIP // Default level (1)
	hostmask := "" // Will be determined based on current connection or explicit input
	
	// Parse level if provided
	if len(args) >= 2 {
		parsedLevel, err := strconv.Atoi(args[1])
		if err != nil {
			// Try string format for backward compatibility
			switch strings.ToLower(args[1]) {
			case "vip", "1":
				level = AdminLevelVIP
			case "admin", "2":
				level = AdminLevelAdmin
			case "owner", "3":
				level = AdminLevelOwner
			default:
				return "Invalid level. Use: 1=VIP, 2=Admin, 3=Owner"
			}
		} else {
			switch parsedLevel {
			case 1:
				level = AdminLevelVIP
			case 2:
				level = AdminLevelAdmin
			case 3:
				level = AdminLevelOwner
			default:
				return "Invalid level. Use: 1=VIP, 2=Admin, 3=Owner"
			}
		}
	}
	
	// Check permissions for adding users at this level
	if requesterLevel == AdminLevelAdmin && level > AdminLevelVIP {
		return "As an admin, you can only add VIPs (level 1)"
	}
	
	if requesterLevel < AdminLevelOwner && level >= AdminLevelAdmin {
		return "Only owners can add admins (level 2) or other owners (level 3)"
	}
	
	// Prevent adding users with equal or higher level (except owners adding owners)
	if level >= requesterLevel && !(requesterLevel == AdminLevelOwner && level == AdminLevelOwner) {
		return "You cannot add users with equal or higher privileges"
	}
	
	// Parse hostmask if provided
		if len(args) >= 3 {
			hostmask = args[2]
		} else {
			p.mu.RLock()
			if currentHostmask, exists := p.currentUsers[nick]; exists {
				hostmask = currentHostmask
			} else {
				// Ha nincs hostmask, inkább jelezd, hogy adják meg explicit módon
				p.mu.RUnlock()
				return "Hostmask missing. ex !addadmin YnM vip *!*@YnM.ynm.hu."
			}
			p.mu.RUnlock()
		}
	
	// Add the admin with the determined hostmask
	info := AdminInfo{
		Nick:     nick,
		Hostmask: hostmask,
		Level:    level,
		AddedBy:  requester,
		AddedAt:  time.Now(),
	}
	if err := p.store.AddAdmin(info); err != nil {
		return "Error saving admin data"
	}
	if err := p.store.Save(); err != nil {
		return "Error saving admin data"
	}
	
	levelStr := p.getLevelString(level)
	return fmt.Sprintf("Added %s as %s (level %d)", nick, levelStr, level)
}

func (p *AdminPlugin) handleDelAdmin(nick, requester string, requesterLevel int) string {
	info, exists := p.store.GetAdmin(nick)
	if !exists {
		return "User is not an admin"
	}
	
	// Check if requester can remove this admin
	if info.Level >= requesterLevel {
		return "You cannot remove admins with equal or higher privileges"
	}
	
	if nick == requester {
		return "You cannot remove yourself"
	}
	
	if p.store.RemoveAdmin(nick) {
		if err := p.store.Save(); err != nil {
			return "Error saving admin data"
		}
		return fmt.Sprintf("Removed %s from admin list", nick)
	}
	
	return "Failed to remove admin"
}

func (p *AdminPlugin) handleListAdmins() string {
	admins := p.store.ListAll()
	if len(admins) == 0 {
		return "No admins configured"
	}
	
	var result []string
	
	for _, admin := range admins {
		levelStr := p.getLevelString(admin.Level)
		result = append(result, fmt.Sprintf("%s (%s-%d)", admin.Nick, levelStr, admin.Level))
	}
	
	return "Admins: " + strings.Join(result, ", ")
}

func (p *AdminPlugin) handleAdminInfo(nick string) string {
	info, exists := p.store.GetAdmin(nick)
	if !exists {
		return fmt.Sprintf("%s is not an admin", nick)
	}
	
	levelStr := p.getLevelString(info.Level)
	
	return fmt.Sprintf("%s: %s (level %d), added by %s on %s, hostmask: %s",
		info.Nick, levelStr, info.Level, info.AddedBy,
		info.AddedAt.Format("2006-01-02 15:04:05"), info.Hostmask)
}

func (p *AdminPlugin) handleHelp(adminLevel int) string {
	var commands []string
	
	if adminLevel >= AdminLevelVIP {
		commands = append(commands, "!listadmins", "!admininfo", "!whoami")
	}
	
	if adminLevel >= AdminLevelAdmin {
		commands = append(commands, "!addadmin", "!deladmin", "!rehash", "!restart")
	}
	
	if adminLevel >= AdminLevelOwner {
		commands = append(commands, "!die")
	}
	
	if len(commands) == 0 {
		return "No admin commands available"
	}
	
	helpText := "Available admin commands: " + strings.Join(commands, ", ")
	helpText += fmt.Sprintf(" | Your level: %d (%s)", adminLevel, p.getLevelString(adminLevel))
	
	// Add hierarchy information based on user level
	if adminLevel == AdminLevelAdmin {
		helpText += " | You can add: VIPs (level 1)"
	} else if adminLevel == AdminLevelOwner {
		helpText += " | You can add: VIPs (level 1), Admins (level 2), Owners (level 3)"
	}
	
	helpText += " | Levels: 1=VIP, 2=Admin, 3=Owner"
	
	return helpText
}

func (p *AdminPlugin) restartBot() {
	// Send quit message to IRC
	p.bot.SendRaw("QUIT :Restarting...")
	
	// Give time for message to be sent
	time.Sleep(1 * time.Second)
	
	executable, err := os.Executable()
	if err != nil {
		p.bot.SendMessage(p.cfg.ConsoleChannel, "Restart error: "+err.Error())
		return
	}

	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		p.bot.SendMessage(p.cfg.ConsoleChannel, "Restart failed: "+err.Error())
		return
	}

	os.Exit(0)
}

func (p *AdminPlugin) OnTick() []irc.Message {
	return nil
}

func simplifyHostmask(fullHostmask string) string {
	atIndex := strings.Index(fullHostmask, "@")
	if atIndex == -1 {
		return "*!*@*"
	}
	host := fullHostmask[atIndex+1:]
	return "*!*@" + host
}


// Legacy method for backward compatibility
func (p *AdminPlugin) AddAdmin(nick string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	hostmask := "*!*@*"
	if fullHostmask, ok := p.currentUsers[nick]; ok {
		fmt.Println("DEBUG: fullHostmask before simplify:", fullHostmask)
		hostmask = simplifyHostmask(fullHostmask)
		fmt.Println("DEBUG: hostmask after simplify:", hostmask)
	}

	info := AdminInfo{
		Nick:     nick,
		Hostmask: hostmask,
		Level:    AdminLevelOwner,
		AddedBy:  "system",
		AddedAt:  time.Now(),
	}

	_ = p.store.AddAdmin(info)
	_ = p.store.Save()
}

