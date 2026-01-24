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
	"strings"
	"time"
	"fmt"
	"sync"
	"log"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type CyclePlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	StopChan    chan struct{}
	
	// Cycle beállítások
	autoCycleEnabled bool
	checkInterval    time.Duration
	lastCheck        time.Time
	maxUserLimit    int
	// Csatorna állapot követés (thread-safe)
	mu            sync.RWMutex
	ChannelUsers  map[string]map[string]bool // channel -> nick -> true
	ChannelOps    map[string]map[string]bool // channel -> opNick -> true
	
	// Bot nick cache
	botNick      string
	botNickLower string

}

func NewCyclePlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, stopChan chan struct{}) *CyclePlugin {
	botNick := bot.GetNick()
	plugin := &CyclePlugin{
		bot:              bot,
		adminPlugin:      adminPlugin,
		StopChan:         stopChan,
		autoCycleEnabled: true,
		checkInterval:    30 * time.Second, 
		 maxUserLimit:    20,
		ChannelUsers:     make(map[string]map[string]bool),
		ChannelOps:       make(map[string]map[string]bool),
		botNick:          botNick,
		botNickLower:     strings.ToLower(botNick),
	}
	
	go plugin.initializeChannels()
	return plugin
}

// NAMES lista kezelése (353 numeric) - optimált verzió
func (p *CyclePlugin) HandleNamesList(channel string, names []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	users := make(map[string]bool, len(names))
	ops := make(map[string]bool)
	
	for _, name := range names {
		nick := name
		isOp := false
		
		// Prefix ellenőrzése
		for len(nick) > 0 {
			firstChar := nick[0]
			if firstChar == '@' || firstChar == '&' || firstChar == '~' {
				isOp = true
				nick = nick[1:]
			} else if firstChar == '%' || firstChar == '+' {
				nick = nick[1:]
			} else {
				break
			}
		}
		
		if nick != "" {
			users[nick] = true
			if isOp {
				ops[nick] = true
			}
		}
	}
	
	p.ChannelUsers[channel] = users
	p.ChannelOps[channel] = ops
}

func (p *CyclePlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.ToLower(msg.Text)
	if !strings.HasPrefix(text, "!cycle") {
		return ""
	}
	
	// Jogosultság ellenőrzése
	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	role := YnMModule.GetUserGlobalRoleWithDB(p.adminPlugin.Db, nick, hostmask)
	if role != "owner" {
		return ""
	}
	
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}
	
	switch len(parts) {
	case 1: // !cycle
		return p.cycleChannel(msg.Channel)
		
	case 2: // !cycle #channel vagy !cycle auto
		target := parts[1]
		if strings.HasPrefix(target, "#") {
			return p.cycleChannel(target)
		}
		if target == "auto" {
			return "Használat: !cycle auto on/off"
		}
		
	case 3: // !cycle auto on/off
		if parts[1] == "auto" {
			switch parts[2] {
			case "on", "be", "enable":
				p.setAutoCycle(true)
				return "✅ Automatikus cycle bekapcsolva."
			case "off", "ki", "disable":
				p.setAutoCycle(false)
				return "✅ Automatikus cycle kikapcsolva."
			}
		}
	}
	
	return "❌ Használat: !cycle [#csatorna] vagy !cycle auto on/off"
}

func (p *CyclePlugin) cycleChannel(channel string) string {
	p.mu.RLock()
	ops := p.ChannelOps[channel]
	p.mu.RUnlock()
	
	// Gyors ellenőrzés - ha csak a bot van op
	if len(ops) == 1 && ops[p.botNick] {
		return "❌ Nem lehet cycle-t csinálni, mert csak nekem van @ jogom a csatornán."
	}
	
	// Async cycle indítása
	go func() {
		p.bot.SendRaw(fmt.Sprintf("PART %s :🔄 Channel cycle", channel))
		time.Sleep(1 * time.Second)
		p.bot.SendRaw(fmt.Sprintf("JOIN %s", channel))
	}()
	
	return fmt.Sprintf("🔄 Cycle indítása: %s", channel)
}

func (p *CyclePlugin) setAutoCycle(enabled bool) {
	p.autoCycleEnabled = enabled
}

func (p *CyclePlugin) shouldAutoCycle(channel string) bool {
    if !p.autoCycleEnabled {
        return false
    }
    
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    if ops, exists := p.ChannelOps[channel]; exists && ops[p.botNick] {
        return false
    }
    
    users := p.ChannelUsers[channel]
    userCount := len(users)
    
    if userCount == 0 {
        return true
    }

    ops := p.ChannelOps[channel]
    totalOps := len(ops)
    
    if userCount > 20 && totalOps == 0 {
        // Nagyszoba, senkinek sincs @ → felesleges próbálkozni
        return false
    }
    
    nonBotCount := 0
    for nick := range users {
        if nick != p.botNick {
            nonBotCount++
            if nonBotCount > 0 {
                break 
            }
        }
    }
    return nonBotCount == 0
}

func (p *CyclePlugin) OnTick() []YnMIrC.Message {
	now := time.Now()
	if now.Sub(p.lastCheck) < p.checkInterval {
		return nil
	}
	p.lastCheck = now
	
	if !p.autoCycleEnabled {
		return nil
	}
	
	// Összes csatorna begyűjtése
	p.mu.RLock()
	channels := make([]string, 0, len(p.ChannelUsers))
	for channel := range p.ChannelUsers {
		channels = append(channels, channel)
	}
	p.mu.RUnlock()
	
	// Async ellenőrzés minden csatornára
	for _, channel := range channels {
		if p.shouldAutoCycle(channel) {
			log.Printf("Automatikus cycle indítása: %s", channel)
			go p.autoCycleChannel(channel)
			break // Egyszerre csak egyet
		}
	}
	
	return nil
}

func (p *CyclePlugin) autoCycleChannel(channel string) {
	p.bot.SendRaw(fmt.Sprintf("PART %s :🔄 Auto cycle", channel))
	time.Sleep(1 * time.Second)
	p.bot.SendRaw(fmt.Sprintf("JOIN %s", channel))
}

func (p *CyclePlugin) initializeChannels() {
	time.Sleep(5 * time.Second)
}

// Optimált eseménykezelők
func (p *CyclePlugin) HandleJoin(nick, channel string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if users, exists := p.ChannelUsers[channel]; exists {
		users[nick] = true
	} else {
		p.ChannelUsers[channel] = map[string]bool{nick: true}
	}
}

func (p *CyclePlugin) HandlePart(nick, channel string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if users, exists := p.ChannelUsers[channel]; exists {
		delete(users, nick)
	}
	if ops, exists := p.ChannelOps[channel]; exists {
		delete(ops, nick)
	}
}

func (p *CyclePlugin) HandleQuit(nick string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for _, users := range p.ChannelUsers {
		delete(users, nick)
	}
	for _, ops := range p.ChannelOps {
		delete(ops, nick)
	}
}

func (p *CyclePlugin) HandleMode(channel, mode, nick, setBy string) {
 
    if !strings.Contains(mode, "o") || nick == "" {
        return
    }
    
    p.mu.Lock()
    defer p.mu.Unlock()

    ops, exists := p.ChannelOps[channel]
    if !exists {
        ops = make(map[string]bool)
        p.ChannelOps[channel] = ops
    }
    
    if strings.HasPrefix(mode, "+") {
        ops[nick] = true
    } else if strings.HasPrefix(mode, "-") {
        delete(ops, nick)
    }
    
    //log.Printf("DEBUG: HandleMode - channel: %s, mode: %s, nick: %s, setBy: %s", 
    //    channel, mode, nick, setBy)
}

// Callback-ek bekötése - javított verzió
func (p *CyclePlugin) RegisterCallbacks(bot *YnMIrC.Client) {
	bot.OnJoin = func(channel, nick, hostmask string) {
		p.HandleJoin(nick, channel)
		// Auto-op hozzáadva
		if nick != p.botNick && p.adminPlugin != nil {
			go func() {
				time.Sleep(300 * time.Millisecond)
				// p.adminPlugin.AutoApplyUserModes(nick, hostmask, channel)
			}()
		}
	}
	
	bot.OnPart = func(channel, nick, reason string) {
		p.HandlePart(nick, channel)
	}
	
	bot.OnQuit = func(nick, reason string) {
		p.HandleQuit(nick)
	}
	
	bot.OnMode = func(channel, modes, args, setBy string) {
		// Gyors szűrés
		if strings.Contains(modes, "o") {
			p.HandleMode(channel, modes, args, setBy)
		}
	}
	
	bot.OnNames = func(channel string, names []string) {
		p.HandleNamesList(channel, names)
	}
}