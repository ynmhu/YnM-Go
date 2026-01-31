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
    "time"
    "sync"
    "strings"
	"database/sql"
    "git.ynm.hu/markus/YnM-Go/YnMIrC"
    "git.ynm.hu/markus/YnM-Go/YnMAdmin"
    "git.ynm.hu/markus/YnM-Go/YnMConfig"
    "git.ynm.hu/markus/YnM-Go/YnMModule"
)

type TopicUpdaterPlugin struct {
    bot         *YnMIrC.Client
    cfg         *YnMConfig.Config
    adminPlugin *owner.YnmAdminPlugin
    startTime   time.Time
    lastUpdate  time.Time
    isOp        map[string]bool 
    mutex       sync.RWMutex
    updateInProgress map[string]bool 
    updateInterval time.Duration
    targetChannels []string 
    db          *sql.DB
}

func NewTopicUpdaterPlugin(bot *YnMIrC.Client, cfg *YnMConfig.Config, adminPlugin *owner.YnmAdminPlugin) *TopicUpdaterPlugin {
    updateInterval := 12 * time.Hour
    if cfg.TopicUpdateInterval != "" {
        if parsedInterval, err := time.ParseDuration(cfg.TopicUpdateInterval); err == nil {
            updateInterval = parsedInterval
        }
    }

    // Összes csatorna összegyűjtése
    targetChannels := []string{cfg.ConsoleChannel}
    if len(cfg.TopicOtherChannels) > 0 {
        targetChannels = append(targetChannels, cfg.TopicOtherChannels...)
    }
    
    var db *sql.DB
    if adminPlugin != nil && adminPlugin.Db != nil {
        db = adminPlugin.Db.SQL  // <-- ITT A JAVÍTÁS: SQL mező
    }

    return &TopicUpdaterPlugin{
        bot:              bot,
        cfg:              cfg,
        adminPlugin:      adminPlugin,
        db:               db,
        startTime:        time.Now(),
        updateInterval:   updateInterval,
        targetChannels:   targetChannels,
        isOp:             make(map[string]bool),  // INICIALIZÁLVA
        updateInProgress: make(map[string]bool),  // INICIALIZÁLVA
    }
}
// Initialize - Csatlakozáskor indítjuk
func (p *TopicUpdaterPlugin) Initialize() {
    go func() {
        time.Sleep(10 * time.Second)
        
        // 1. Ellenőrizzük és frissítjük a topic-ot
        p.checkAndUpdateTopic()
        
        // 2. Szinkronizáljuk az adatbázissal
        if p.db != nil {
            count, err := p.SyncAllTopicsToDatabase()
            if err != nil {
                fmt.Printf("[TopicUpdater] Initial database sync failed: %v\n", err)
            } else {
                fmt.Printf("[TopicUpdater] Initial database sync: %d channels updated\n", count)
            }
        }
    }()
}
// updateTopicInDatabase - Topic mentése az adatbázisba
func (p *TopicUpdaterPlugin) updateTopicInDatabase(channel string, topic string) error {
    if p.db == nil {
        return fmt.Errorf("database not available")
    }
    
    // Bot nick meghatározása
    botNick := p.bot.GetNick()
    
    // Ellenőrizzük, hogy létezik-e a csatorna az adatbázisban
    var exists bool
    err := p.db.QueryRow("SELECT EXISTS(SELECT 1 FROM channels WHERE name = ?)", channel).Scan(&exists)
    if err != nil {
        return fmt.Errorf("error checking channel existence: %v", err)
    }
    
    if exists {
        // Frissítés, ha már létezik
        _, err = p.db.Exec(`
            UPDATE channels 
            SET current_topic = ?,
                topic_set_by = ?,
                topic_set_at = datetime('now')
            WHERE name = ?
        `, topic, botNick, channel)
        if err != nil {
            return fmt.Errorf("error updating topic in database: %v", err)
        }
     //   fmt.Printf("[TopicUpdater] Topic updated in database for channel: %s\n", channel)
    } else {
        // Új bejegyzés, ha nem létezik
        // GetIdent és GetHost helyett fix értékek
        _, err = p.db.Exec(`
            INSERT INTO channels (
                name, auto_op, auto_voice, auto_halfop, owner, owner_hostmask,
                current_topic, topic_set_by, topic_set_at, current_modes, created_at
            ) VALUES (?, 0, 0, 0, ?, ?, ?, ?, datetime('now'), '', datetime('now'))
        `, 
            channel, 
            botNick, 
            botNick + "@bot.ynm.hu", // Egyszerű hostmask
            topic, 
            botNick)
        
        if err != nil {
            return fmt.Errorf("error inserting topic into database: %v", err)
        }
        fmt.Printf("[TopicUpdater] New channel added to database: %s\n", channel)
    }
    
    return nil
}
// checkAndUpdateTopic - Ellenőrzi, hogy OP-e és frissíti a topicot
func (p *TopicUpdaterPlugin) checkAndUpdateTopic() {
    for _, channel := range p.targetChannels {
        go p.updateChannelTopic(channel)
    }
}

func (p *TopicUpdaterPlugin) updateChannelTopic(channel string) {
    p.mutex.Lock()
    if p.updateInProgress[channel] {
        p.mutex.Unlock()
        return
    }
    p.updateInProgress[channel] = true
    p.mutex.Unlock()
    
    defer func() {
        p.mutex.Lock()
        p.updateInProgress[channel] = false
        p.mutex.Unlock()
    }()
    
    topic := p.generateTopic()
    
    // 1. Topic küldése az IRC-be
    p.bot.SendRaw(fmt.Sprintf("TOPIC %s :%s", channel, topic))
    
    time.Sleep(3 * time.Second) // Várunk egy kicsit
    
    // 2. Mentés az adatbázisba
    err := p.updateTopicInDatabase(channel, topic)
    if err != nil {
      //  fmt.Printf("[TopicUpdater] Database update failed for %s: %v\n", channel, err)
        // Visszaadjuk az eredményt a console channel-be (opció)
        if channel == p.cfg.ConsoleChannel {
            p.bot.SendRaw(fmt.Sprintf("PRIVMSG %s :Database update failed: %v", channel, err))
        }
    } else {
   //     fmt.Printf("[TopicUpdater] Topic saved to database for channel: %s\n", channel)
    }
    
    p.mutex.Lock()
    p.isOp[channel] = true
    if channel == p.cfg.ConsoleChannel {
        p.lastUpdate = time.Now()
    }
    p.mutex.Unlock()
}

func (p *TopicUpdaterPlugin) generateTopic() string {
    uptime := p.formatUptime()
    version := owner.YnMVersion
    website := "https://uptime.ynm.hu"
    updateEvery := formatDuration(p.updateInterval)

    return fmt.Sprintf(
        "\x02\x0314Uptime:\x0F \x0302%s\x0F | "+
            "\x02\x0304Update Every:\x0F \x0302%s\x0F | "+
            "\x02\x0317Version:\x0F \x0312%s\x0F | "+
            "\x02\x0302Web:\x0F \x1F%s\x0F",
        uptime, updateEvery, version, website,
    )
}

func (p *TopicUpdaterPlugin) formatUptime() string {
    duration := time.Since(p.startTime)
    
    days := int(duration.Hours()) / 24
    hours := int(duration.Hours()) % 24
    minutes := int(duration.Minutes()) % 60
    
    if days > 0 {
        return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
    } else if hours > 0 {
        return fmt.Sprintf("%dh %dm", hours, minutes)
    }
    return fmt.Sprintf("%dm", minutes)
}

func formatDuration(d time.Duration) string {
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60

    if hours >= 24 {
        return fmt.Sprintf("%dd", hours/24)
    } else if hours > 0 {
        return fmt.Sprintf("%dh", hours)
    }
    return fmt.Sprintf("%dm", minutes)
}

func (p *TopicUpdaterPlugin) HandleMessage(msg YnMIrC.Message) string {
    // Szerver üzenetek kiszűrése
    if YnMModule.IsServerMessage(msg.Sender) {
        return ""
    }
    
    botNick := p.bot.GetNick()
    
    // NAMES válasz (353) - csatornánként kezeljük
    if msg.Command == "353" && len(msg.Params) >= 3 {
        channel := msg.Params[2]
        if p.isTargetChannel(channel) && msg.Text != "" {
            isOp := strings.Contains(msg.Text, "@"+botNick)
            p.mutex.Lock()
            p.isOp[channel] = isOp
            p.mutex.Unlock()
        }
        return ""
    }
    
    // TOPIC válasz (332) - csatornánként
    if msg.Command == "332" && len(msg.Params) >= 2 {
        channel := msg.Params[1]
        if p.isTargetChannel(channel) {
            p.mutex.Lock()
            p.isOp[channel] = true
            p.updateInProgress[channel] = false
            p.mutex.Unlock()
            
            go func(ch string) {
                time.Sleep(2 * time.Second)
                p.updateChannelTopic(ch)
            }(channel)
        }
        return ""
    }
    
    // No topic set (331) - nincs topic, de le tudtuk kérni (OP vagyunk)
    if msg.Command == "331" && len(msg.Params) >= 2 {
        channel := msg.Params[1]
        if p.isTargetChannel(channel) {
            p.mutex.Lock()
            p.isOp[channel] = true
            p.updateInProgress[channel] = false
            p.mutex.Unlock()
            
            go func(ch string) {
                time.Sleep(2 * time.Second)
                p.updateChannelTopic(ch)
            }(channel)
        }
        return ""
    }
    
    // Nem vagyunk channel operator (482)
    if msg.Command == "482" && len(msg.Params) >= 2 {
        channel := msg.Params[1]
        if p.isTargetChannel(channel) {
            p.mutex.Lock()
            p.isOp[channel] = false
            p.updateInProgress[channel] = false
            p.mutex.Unlock()
        }
        return ""
    }
    
    // MODE változások - bot kapott/veszített OP jogot
    if msg.Command == "MODE" && strings.Contains(msg.Text, botNick) {
        // Meg kell találni, melyik csatornára vonatkozik a MODE
        if len(msg.Params) >= 1 && p.isTargetChannel(msg.Params[0]) {
            channel := msg.Params[0]
            if strings.Contains(msg.Text, "+o") {
                p.mutex.Lock()
                p.isOp[channel] = true
                p.mutex.Unlock()
                
                go func(ch string) {
                    time.Sleep(2 * time.Second)
                    p.updateChannelTopic(ch)
                }(channel)
            } else if strings.Contains(msg.Text, "-o") {
                p.mutex.Lock()
                p.isOp[channel] = false
                p.mutex.Unlock()
            }
        }
        return ""
    }
    
    // PRIVMSG parancsok
    if msg.Command == "PRIVMSG" && len(msg.Params) >= 2 {
        // Jogosultság ellenőrzés (csak owner)
        if p.adminPlugin != nil {
            nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
            role := YnMModule.GetUserGlobalRoleWithDB(p.adminPlugin.Db, nick, hostmask)
            if role != "owner" {
                return ""
            }
        } else {
            return ""
        }
        
    message := msg.Params[1]
    if strings.HasPrefix(message, ":") {
        message = message[1:]
    }
    
    if strings.HasPrefix(message, "!topic") {
        parts := strings.Fields(message)
        
        if len(parts) == 1 {
            // !topic - minden konfigurált csatornára
            p.mutex.RLock()
            consoleUpdateInProgress := p.updateInProgress[p.cfg.ConsoleChannel]
            targetChannels := p.targetChannels
            p.mutex.RUnlock()
            
            if consoleUpdateInProgress {
                return "Update in progress..."
            }
            
            // Adatbázis info hozzáadása a válaszhoz
            dbStatus := "DB: ✓"
            if p.db == nil {
                dbStatus = "DB: ✗ (not connected)"
            }
            
            response := fmt.Sprintf("Updating topic in %d channels: %v [%s]", 
                len(targetChannels), targetChannels, dbStatus)
            go p.checkAndUpdateTopic()
            return response
            
        } else if len(parts) >= 2 && parts[1] == "db" {
            // !topic db - adatbázis információ
            if p.db == nil {
                return "Database not connected"
            }
            
            // Számláljuk, hány csatorna van az adatbázisban
            var count int
            err := p.db.QueryRow("SELECT COUNT(*) FROM channels").Scan(&count)
            if err != nil {
                return fmt.Sprintf("Database error: %v", err)
            }
            
            return fmt.Sprintf("Database: %d channels stored", count)
            
        } else if len(parts) >= 2 && parts[1] == "status" {
            // !topic status - mutasd meg az állapotot
            p.mutex.RLock()
            
            dbStatus := "✓"
            if p.db == nil {
                dbStatus = "✗"
            }
            
            status := fmt.Sprintf(
                "Config: Console=%s, OtherChannels=%v, TargetChannels=%v | Database: %s", 
                p.cfg.ConsoleChannel,
                p.cfg.TopicOtherChannels,
                p.targetChannels,
                dbStatus,
            )
            p.mutex.RUnlock()
            return status
                
            } else if len(parts) >= 2 && parts[1] == "all" {
                // !topic all - minden csatornára (alias)
                p.mutex.RLock()
                consoleUpdateInProgress := p.updateInProgress[p.cfg.ConsoleChannel]
                targetChannels := p.targetChannels
                p.mutex.RUnlock()
                
                if consoleUpdateInProgress {
                    return "Update in progress..."
                }
                
                go p.checkAndUpdateTopic()
                return fmt.Sprintf("Updating topic in %d channels", len(targetChannels))
                
            } else if len(parts) >= 2 && strings.HasPrefix(parts[1], "#") {
                // !topic #channel - specifikus csatornára
                channel := parts[1]
                if p.isTargetChannel(channel) {
                    p.mutex.RLock()
                    channelUpdateInProgress := p.updateInProgress[channel]
                    p.mutex.RUnlock()
                    
                    if channelUpdateInProgress {
                        return "Update in progress for this channel..."
                    }
                    
                    go p.updateChannelTopic(channel)
                    return fmt.Sprintf("Updating topic in %s...", channel)
                }
                return "Channel not configured for topic updates"
            }
        }
    }
    
    return ""
}

// SyncAllTopicsToDatabase - Összes csatorna aktuális topic-jának szinkronizálása
func (p *TopicUpdaterPlugin) SyncAllTopicsToDatabase() (int, error) {
    if p.db == nil {
        return 0, fmt.Errorf("database not available")
    }
    
    count := 0
    for _, channel := range p.targetChannels {
        // Megpróbáljuk lekérni az aktuális topic-ot
        // (Ez az IRC-től függ, hogy van-e ilyen lehetőség)
        // Ha nincs közvetlen topic lekérés, akkor az ismert topic-ot mentjük
        topic := p.generateTopic() // Vagy lekérjük valahonnan
        
        err := p.updateTopicInDatabase(channel, topic)
        if err != nil {
            fmt.Printf("[TopicUpdater] Failed to sync %s: %v\n", channel, err)
        } else {
            count++
        }
    }
    
    return count, nil
}

// Public methods
func (p *TopicUpdaterPlugin) ForceUpdate() {
    go p.checkAndUpdateTopic()
}
func (p *TopicUpdaterPlugin) isTargetChannel(channel string) bool {
    for _, target := range p.targetChannels {
        if strings.EqualFold(target, channel) {
            return true
        }
    }
    return false
}

func (p *TopicUpdaterPlugin) IsOp() bool {
    p.mutex.RLock()
    defer p.mutex.RUnlock()
    if p.isOp == nil {
        return false
    }
    return p.isOp[p.cfg.ConsoleChannel]
}
func (p *TopicUpdaterPlugin) OnTick() []YnMIrC.Message {
    now := time.Now()
    
    p.mutex.RLock()
    // Biztonságos map olvasás
    consoleOp := false
    if p.isOp != nil {
        consoleOp = p.isOp[p.cfg.ConsoleChannel]
    }
    
    lastUpdate := p.lastUpdate
    
    consoleUpdateInProgress := false
    if p.updateInProgress != nil {
        consoleUpdateInProgress = p.updateInProgress[p.cfg.ConsoleChannel]
    }
    p.mutex.RUnlock()
    
    // Óránként ellenőrizzük, ha nem OP vagyunk a console channel-ben
    if !consoleOp && now.Minute() == 0 && now.Second() < 10 {
        go p.checkAndUpdateTopic()
    }
    
    // Rendszeres topic frissítés (ha OP vagyunk a console channel-ben)
    if consoleOp && !consoleUpdateInProgress && !lastUpdate.IsZero() {
        if time.Since(lastUpdate) >= p.updateInterval {
            go p.checkAndUpdateTopic()
        }
    }
    
    return nil
}