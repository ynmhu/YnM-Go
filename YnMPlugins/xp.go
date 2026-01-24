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
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"gopkg.in/yaml.v3"
)

// XPChannelConfig konfigurációs struktúra egy csatornához
type XPChannelConfig struct {
	Enabled   bool           `yaml:"enabled"`
	Ranks     map[int]string `yaml:"ranks"`
	XPPerMsg  int            `yaml:"xp_per_message"`
	LevelBase int            `yaml:"level_base"`
}

// XPConfig fő konfigurációs struktúra
type XPConfig struct {
	Database struct {
		File      string `yaml:"file"`
		BackupDir string `yaml:"backup_dir"`
	} `yaml:"database"`
	
	Cooldown        string                     `yaml:"cooldown"`
	BackupInterval  string                     `yaml:"backup_interval"`
	ExcludedNicks   []string                   `yaml:"excluded_nicks"`
	DefaultXPPerMsg int                        `yaml:"default_xp_per_message"`
	DefaultLevelBase int                       `yaml:"default_level_base"`
	DefaultRanks    map[int]string             `yaml:"default_ranks"`
	Channels        map[string]XPChannelConfig `yaml:"channels"`
}

// XPData tárolja egy user XP adatait egy csatornán
type XPData struct {
	XP          int
	LastXPGain  int64 // Unix timestamp másodpercben
	MsgCount    int
	Level       int
	LastSeen    int64
}

// XPManager kezeli az XP adatokat és mentést
type XPManager struct {
	sync.RWMutex
	db            map[string]map[string]*XPData // chan -> nick -> XPData
	config        *XPConfig
	dbFile        string
	backupDir     string
	cooldown      time.Duration
	backupTicker  *time.Ticker
	excludedNicks map[string]bool
}

// XPPlugin handles XP-related IRC commands
type XPPlugin struct {
    Manager *XPManager
}

// LoadXPConfig betölti a YAML konfigurációt
func LoadXPConfig(configFile string) (*XPConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("config fájl olvasási hiba: %v", err)
	}
	
	var config XPConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("YAML parsing hiba: %v", err)
	}
	
	// Default értékek beállítása
	if config.DefaultXPPerMsg <= 0 {
		config.DefaultXPPerMsg = 1
	}
	if config.DefaultLevelBase <= 0 {
		config.DefaultLevelBase = 100
	}
	
	// Csatornák konfigurációjának ellenőrzése
	for chanName, chanConfig := range config.Channels {
		if chanConfig.XPPerMsg <= 0 {
			config.Channels[chanName] = XPChannelConfig{
				Enabled:   chanConfig.Enabled,
				Ranks:     chanConfig.Ranks,
				XPPerMsg:  config.DefaultXPPerMsg,
				LevelBase: chanConfig.LevelBase,
			}
		}
		if chanConfig.LevelBase <= 0 {
			chanConfig := config.Channels[chanName]
			chanConfig.LevelBase = config.DefaultLevelBase
			config.Channels[chanName] = chanConfig
		}
	}
	
	return &config, nil
}

// Új XPManager létrehozása konfigurációból
func NewXPManagerFromConfig(configFile string) (*XPManager, error) {
	config, err := LoadXPConfig(configFile)
	if err != nil {
		return nil, err
	}
	
	// Cooldown parsing
	cooldown, err := time.ParseDuration(config.Cooldown)
	if err != nil {
		log.Printf("Hibás cooldown formátum (%s), alapértelmezett 60s használata", config.Cooldown)
		cooldown = 60 * time.Second
	}
	
	// Backup interval parsing
	backupInterval, err := time.ParseDuration(config.BackupInterval)
	if err != nil {
		log.Printf("Hibás backup interval formátum (%s), alapértelmezett 1h használata", config.BackupInterval)
		backupInterval = time.Hour
	}
	
	xm := &XPManager{
		db:            make(map[string]map[string]*XPData),
		config:        config,
		dbFile:        config.Database.File,
		backupDir:     config.Database.BackupDir,
		cooldown:      cooldown,
		excludedNicks: make(map[string]bool),
	}

	// Excluded nicks beállítása
	for _, n := range config.ExcludedNicks {
		xm.excludedNicks[strings.ToLower(n)] = true
	}

	// VERIFY FILE BEFORE LOADING
	//log.Printf("🔍 Verifying XP database file: %s", xm.dbFile)
	if err := xm.VerifyDatabaseFile(); err != nil {
		log.Printf("❌ Database file verification failed: %v", err)
	} else {
		//log.Printf("✅ Database file verification passed")
	}

	// Betöltés
	//log.Printf("🔄 Starting XP database load...")
	if err := xm.load(); err != nil {
		log.Printf("❌ XP adatbázis betöltési hiba: %v", err)
	} else {
		//log.Printf("✅ XP database load completed")
	}

	// Backup timer indítása
	if backupInterval > 0 {
		xm.backupTicker = time.NewTicker(backupInterval)
		go func() {
			for range xm.backupTicker.C {
				if err := xm.backup(); err != nil {
					log.Printf("XP backup hiba: %v", err)
				}
			}
		}()
	}

	return xm, nil
}

// NewXPPlugin creates a new XP plugin instance
func NewXPPlugin(manager *XPManager) *XPPlugin {
	return &XPPlugin{Manager: manager}
}

// getChannelConfig visszaadja egy csatorna konfigurációját
func (xm *XPManager) getChannelConfig(chanName string) *XPChannelConfig {
    // Remove # prefix if present for config lookup
    cleanChanName := strings.TrimPrefix(strings.ToLower(chanName), "#")
    
    if config, exists := xm.config.Channels[cleanChanName]; exists {
        return &config
    }
    
    // Try with original name (including # if present)
    if config, exists := xm.config.Channels[chanName]; exists {
        return &config
    }
    
    // If no specific config, return default
    return &XPChannelConfig{
        Enabled:   true, // Default to enabled
        Ranks:     xm.config.DefaultRanks,
        XPPerMsg:  xm.config.DefaultXPPerMsg,
        LevelBase: xm.config.DefaultLevelBase,
    }
}

// load betölti az adatbázist fájlból
func (xm *XPManager) load() error {
    xm.Lock()
    defer xm.Unlock()

   // log.Printf("Attempting to load XP database from: %s", xm.dbFile)

    file, err := os.Open(xm.dbFile)
    if err != nil {
        if os.IsNotExist(err) {
            log.Printf("XP database file does not exist, starting with empty database")
            xm.db = make(map[string]map[string]*XPData)
            return nil
        }
        log.Printf("Error opening XP database: %v", err)
        return err
    }
    defer file.Close()

    // Initialize empty database
    xm.db = make(map[string]map[string]*XPData)
    
    scanner := bufio.NewScanner(file)
    lineNum := 0
    entriesLoaded := 0
    skippedLines := 0
    
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines
		if line == "" {
			skippedLines++
			continue
		}
		
		// Skip ONLY lines that start with # AND have only one field (pure comments)
		// OR lines that are clearly header comments (contain "Format:" or "Database")
		// Data lines like "#magyar ml 1 ..." should NOT be skipped
		if strings.HasPrefix(line, "#") {
			fields := strings.Fields(line)
			// Check if it's a header/comment line
			isComment := len(fields) < 5 || 
						strings.Contains(line, "Format:") || 
						strings.Contains(line, "XP Database")
			
			if isComment {
				skippedLines++
				continue
			}
		}
        
        // format: chan nick xp last_xp_gain msgcount level last_seen
        parts := strings.Fields(line)
        
        if len(parts) < 5 {
            skippedLines++
            continue
        }
        
        // KRITIKUS JAVÍTÁS: Normalizáljuk a csatorna nevet betöltéskor
        chanName := strings.ToLower(parts[0])
        nick := strings.ToLower(parts[1])
        
        xp, err1 := strconv.Atoi(parts[2])
        lastXpGain, err2 := strconv.ParseInt(parts[3], 10, 64)
        msgCount, err3 := strconv.Atoi(parts[4])
        
        if err1 != nil || err2 != nil || err3 != nil {
            skippedLines++
            continue
        }
        
        // Optional fields for backward compatibility
        level := 0
        lastSeen := time.Now().Unix()
        
        if len(parts) >= 6 {
            if l, err := strconv.Atoi(parts[5]); err == nil {
                level = l
            }
        }
        if len(parts) >= 7 {
            if ls, err := strconv.ParseInt(parts[6], 10, 64); err == nil {
                lastSeen = ls
            }
        }
        
        if xm.db[chanName] == nil {
            xm.db[chanName] = make(map[string]*XPData)
        }
        
        xm.db[chanName][nick] = &XPData{
            XP:         xp,
            LastXPGain: lastXpGain,
            MsgCount:   msgCount,
            Level:      level,
            LastSeen:   lastSeen,
        }
        
        entriesLoaded++
    }

    if err := scanner.Err(); err != nil {
        log.Printf("Scanner error: %v", err)
        return err
    }
    
    //log.Printf("XP database loaded successfully: %d entries from %d total lines (%d skipped)", entriesLoaded, lineNum, skippedLines)
    //log.Printf("Channels in memory: %d", len(xm.db))
    //for chanName, users := range xm.db {
       // log.Printf("  Channel '%s': %d users", chanName, len(users))
   // }
    
    return nil
}


func (xm *XPManager) VerifyDatabaseFile() error {
	if _, err := os.Stat(xm.dbFile); os.IsNotExist(err) {
		log.Printf("📄 Database file does not exist yet: %s", xm.dbFile)
		return nil
	}
	
	//log.Printf("📄 Database file exists: %s", xm.dbFile)
	
	content, err := os.ReadFile(xm.dbFile)
	if err != nil {
		log.Printf("❌ Error reading database file: %v", err)
		return err
	}
	
	//log.Printf("📊 Database file size: %d bytes", len(content))
	
	// Check if file is empty or only contains comments
	lines := strings.Split(string(content), "\n")
	dataLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			dataLines++
		}
	}
	
	//log.Printf("📊 Database file contains %d data lines", dataLines)
	
	return nil
}

// mentés fájlba
func (xm *XPManager) save() error {
	xm.Lock()
	defer xm.Unlock()

	return xm.saveUnsafe()
}

// Backup készítése
func (xm *XPManager) backup() error {
	xm.RLock()
	defer xm.RUnlock()

	if _, err := os.Stat(xm.backupDir); os.IsNotExist(err) {
		err = os.MkdirAll(xm.backupDir, 0755)
		if err != nil {
			return err
		}
	}
	backupFile := fmt.Sprintf("%s/xp_backup_%s.dat", xm.backupDir, time.Now().Format("20060102_150405"))

	input, err := os.ReadFile(xm.dbFile)
	if err != nil {
		return err
	}
	err = os.WriteFile(backupFile, input, 0644)
	if err != nil {
		return err
	}
	log.Printf("XP backup készült: %s", backupFile)
	return nil
}

// calculateLevel calculates the level based on XP for a specific channel
func (xm *XPManager) calculateLevel(chanName string, xp int) int {
	chanConfig := xm.getChannelConfig(chanName)
	levelBase := chanConfig.LevelBase
	
	if xp < levelBase {
		return 0
	}
	
	// Exponential level progression: level = floor(log2(xp / levelBase)) + 1
	level := 0
	requiredXP := levelBase
	
	for xp >= requiredXP {
		level++
		requiredXP *= 2 // Each level requires double the XP
	}
	
	return level
}

// getXPForLevel returns the XP required for a specific level in a channel
func (xm *XPManager) getXPForLevel(chanName string, level int) int {
	if level <= 0 {
		return 0
	}
	
	chanConfig := xm.getChannelConfig(chanName)
	xp := chanConfig.LevelBase
	for i := 1; i < level; i++ {
		xp *= 2
	}
	return xp
}

// Rang meghatározása csatornánként
func (xm *XPManager) GetRank(chanName string, xp int) string {
	chanConfig := xm.getChannelConfig(chanName)
	
	if len(chanConfig.Ranks) == 0 {
		return "Nincs rang"
	}
	
	// Rangküszöbök csökkenő sorrendben
	var keys []int
	for k := range chanConfig.Ranks {
		keys = append(keys, k)
	}
	
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	for _, threshold := range keys {
		if xp >= threshold {
			return chanConfig.Ranks[threshold]
		}
	}
	return "Újoncok"
}

// XP lekérése adott nick és csatorna esetén
func (xm *XPManager) GetXP(chanName, nick string) int {
	xm.RLock()
	defer xm.RUnlock()
	
	if users, ok := xm.db[strings.ToLower(chanName)]; ok {
		if data, ok2 := users[strings.ToLower(nick)]; ok2 {
			return data.XP
		}
	}
	return 0
}

// GetUserData returns full user data
func (xm *XPManager) GetUserData(chanName, nick string) *XPData {
	xm.RLock()
	defer xm.RUnlock()

	chanLower := strings.ToLower(chanName)
	nickLower := strings.ToLower(nick)

	// Ha a nick pontot tartalmaz, érvénytelen – ne adjuk vissza
	if strings.Contains(nickLower, ".") {
		return nil
	}

	if users, ok := xm.db[chanLower]; ok {
		if data, ok2 := users[nickLower]; ok2 {
			return &XPData{
				XP:         data.XP,
				LastXPGain: data.LastXPGain,
				MsgCount:   data.MsgCount,
				Level:      data.Level,
				LastSeen:   data.LastSeen,
			}
		}
	}
	return nil
}


// XP hozzáadása cooldownnal
func (xm *XPManager) AddXP(chanName, nick string, amount int) (newXP int, levelUp bool, err error) {
	now := time.Now().Unix()
	
	// CRITICAL FIX: Normalize BEFORE any operations
	chanLower := strings.ToLower(chanName)
	nickLower := strings.ToLower(nick)
	
	if xm.excludedNicks[nickLower] {
		return 0, false, nil
	}
	
	// Check channel configuration - use ORIGINAL name for config lookup
	chanConfig := xm.getChannelConfig(chanName) // This handles normalization internally
	if !chanConfig.Enabled {
		return 0, false, nil
	}

	xm.Lock()
	defer xm.Unlock()

	// Use normalized names for database operations
	if xm.db[chanLower] == nil {
		xm.db[chanLower] = make(map[string]*XPData)
	}
	
	userData := xm.db[chanLower][nickLower]
	if userData == nil {
		userData = &XPData{
			Level: 0,
			LastSeen: now,
		}
		xm.db[chanLower][nickLower] = userData
	}
	
	// Cooldown check
	if now-int64(xm.cooldown.Seconds()) < userData.LastXPGain {
		return userData.XP, false, nil
	}

	oldLevel := userData.Level
	userData.XP += amount
	userData.MsgCount++
	userData.LastXPGain = now
	userData.LastSeen = now
	
	// Use ORIGINAL channel name for level calculation (it normalizes internally)
	userData.Level = xm.calculateLevel(chanName, userData.XP)
	
	levelUp = userData.Level > oldLevel

	// CRITICAL FIX: Use saveUnsafe since we already hold the lock
	if err := xm.saveUnsafe(); err != nil {
		return userData.XP, levelUp, err
	}
	
	return userData.XP, levelUp, nil
}

func (xm *XPManager) saveUnsafe() error {
    log.Printf("Saving XP database to: %s", xm.dbFile)
    
    // Create temporary file first
    tempFile := xm.dbFile + ".tmp"
    f, err := os.Create(tempFile)
    if err != nil {
        log.Printf("Error creating temp file: %v", err)
        return err
    }
    
    // Header comment
    if _, err := f.WriteString("# XP Database - Format: channel nick xp last_xp_gain msgcount level last_seen\n"); err != nil {
        f.Close()
        os.Remove(tempFile)
        log.Printf("Error writing header: %v", err)
        return err
    }

    entriesWritten := 0
    for chanName, users := range xm.db {
        for nick, data := range users {
            line := fmt.Sprintf("%s %s %d %d %d %d %d\n",
                chanName, nick, data.XP, data.LastXPGain, data.MsgCount, data.Level, data.LastSeen)
            if _, err := f.WriteString(line); err != nil {
                f.Close()
                os.Remove(tempFile)
                log.Printf("Error writing line: %v", err)
                return err
            }
            entriesWritten++
        }
    }
    
    if err := f.Close(); err != nil {
        os.Remove(tempFile)
        log.Printf("Error closing file: %v", err)
        return err
    }
    
    // Atomic rename
    if err := os.Rename(tempFile, xm.dbFile); err != nil {
        os.Remove(tempFile)
        log.Printf("Error renaming file: %v", err)
        return err
    }
    
    log.Printf("XP database saved successfully: %d entries written", entriesWritten)
    return nil
}

// XP beállítása (admin funkció)
func (xm *XPManager) SetXP(chanName, nick string, amount int) error {
	chanLower := strings.ToLower(chanName)
	nickLower := strings.ToLower(nick)
	
	xm.Lock()
	defer xm.Unlock()

	if xm.db[chanLower] == nil {
		xm.db[chanLower] = make(map[string]*XPData)
	}
	
	userData := xm.db[chanLower][nickLower]
	if userData == nil {
		userData = &XPData{
			LastSeen: time.Now().Unix(),
		}
		xm.db[chanLower][nickLower] = userData
	}
	
	userData.XP = amount
	userData.Level = xm.calculateLevel(chanName, amount)
	
	return xm.save()
}

// XP törlése egy nicken
func (xm *XPManager) DeleteXP(nick string) error {
    nickLower := strings.ToLower(nick)
    
    xm.Lock()
    defer xm.Unlock()

    found := false
    for chanName, users := range xm.db {
        if _, ok := users[nickLower]; ok {
            delete(users, nickLower)
            found = true
            
            // If channel is now empty, remove it
            if len(users) == 0 {
                delete(xm.db, chanName)
            }
        }
    }
    
    if !found {
        return fmt.Errorf("user not found")
    }
    
    return xm.saveUnsafe()
}

// XP top lista egy csatornán
func (xm *XPManager) TopXP(chanName string, limit int) []struct {
	Nick    string
	XP      int
	Level   int
	MsgCount int
} {
	xm.RLock()
	defer xm.RUnlock()

	var list []struct {
		Nick    string
		XP      int
		Level   int
		MsgCount int
	}

	users, ok := xm.db[strings.ToLower(chanName)]
	if !ok {
		return list
	}
	
	for nick, data := range users {
		list = append(list, struct {
			Nick    string
			XP      int
			Level   int
			MsgCount int
		}{nick, data.XP, data.Level, data.MsgCount})
	}

	// Rendezés XP szerint csökkenő sorrendben
	sort.Slice(list, func(i, j int) bool {
		return list[i].XP > list[j].XP
	})

	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	
	return list
}

// GetStats returns channel statistics
func (xm *XPManager) GetStats(chanName string) (totalUsers, totalXP, totalMessages int) {
	xm.RLock()
	defer xm.RUnlock()
	
	users, ok := xm.db[strings.ToLower(chanName)]
	if !ok {
		return 0, 0, 0
	}
	
	totalUsers = len(users)
	for _, data := range users {
		totalXP += data.XP
		totalMessages += data.MsgCount
	}
	
	return totalUsers, totalXP, totalMessages
}

// HandleMessage implements the Plugin interface
func (xp *XPPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	
	// Handle XP commands first
	if strings.HasPrefix(text, "!xp") || text == "!xptop" || text == "!ranks" {
		return xp.HandleXPCommand(msg)
	}
	
	// Handle regular message XP gain
	return xp.HandleXPGain(msg)
}

// HandleXPCommand handles IRC messages for XP commands
func (xp *XPPlugin) HandleXPCommand(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	
	// XP lekérés
	if text == "!xp" {
		userData := xp.Manager.GetUserData(msg.Channel, msg.Nick)
		if userData == nil {
			return fmt.Sprintf("%s, még nincs XP-d. Írj egy kicsit a csatornán! https://bot.ynm.hu/xp", msg.Nick)
		}
		
		rank := xp.Manager.GetRank(msg.Channel, userData.XP)
		nextLevelXP := xp.Manager.getXPForLevel(msg.Channel, userData.Level + 1)
		progress := ""
		
		if nextLevelXP > 0 {
			needed := nextLevelXP - userData.XP
			progress = fmt.Sprintf(" (következő szintig: %d XP)", needed)
		}
		
		return fmt.Sprintf("%s, XP: %d, Szint: %d, Rang: %s, Üzenetek: %d%s, https://bot.ynm.hu/xp", 
			msg.Nick, userData.XP, userData.Level, rank, userData.MsgCount, progress)
	}
	
	// In your HandleXPCommand function, add:
	if text == "!xp save" {
		if err := xp.Manager.ForceSave(); err != nil {
			return fmt.Sprintf("Save failed: %v", err)
		}
		return "XP database saved manually"
	}
	
	// Debug command to check loaded data
	if text == "!xp debug" {
		xp.Manager.RLock()
		totalChans := len(xp.Manager.db)
		totalUsers := 0
		for _, users := range xp.Manager.db {
			totalUsers += len(users)
		}
		xp.Manager.RUnlock()
		
		// Check if file exists
		fileExists := "NO"
		fileSize := int64(0)
		if info, err := os.Stat(xp.Manager.dbFile); err == nil {
			fileExists = "YES"
			fileSize = info.Size()
		}
		
		return fmt.Sprintf("DEBUG: %d channels in memory, %d total users | File: %s exists=%s size=%d bytes", 
			totalChans, totalUsers, xp.Manager.dbFile, fileExists, fileSize)
	}
	
	// Show actual file content
	if text == "!xp file" {
		content, err := os.ReadFile(xp.Manager.dbFile)
		if err != nil {
			return fmt.Sprintf("Error reading file: %v", err)
		}
		lines := strings.Split(string(content), "\n")
		preview := ""
		count := 0
		for _, line := range lines {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
				preview += line + " | "
				count++
				if count >= 3 {
					break
				}
			}
		}
		return fmt.Sprintf("File has %d lines. First entries: %s", len(lines), preview)
	}
	// Top lista
	if text == "!xp top" || text == "!xptop" {
		topList := xp.Manager.TopXP(msg.Channel, 10)
		if len(topList) == 0 {
			return "Még nincs XP adat ebben a csatornában."
		}
		
		response := "🏆 XP toplista: "
		for i, entry := range topList {
			medal := ""
			switch i {
			case 0:
				medal = "🥇"
			case 1:
				medal = "🥈"
			case 2:
				medal = "🥉"
			default:
				medal = fmt.Sprintf("%d.", i+1)
			}
			response += fmt.Sprintf("%s %s (%d XP, %d. szint) ", 
				medal, entry.Nick, entry.XP, entry.Level)
		}
		return response
	}
	
	// Statisztikák
	if text == "!xp stats" {
		totalUsers, totalXP, totalMessages := xp.Manager.GetStats(msg.Channel)
		if totalUsers == 0 {
			return "Még nincs XP adat ebben a csatornában."
		}
		
		avgXP := totalXP / totalUsers
		avgMsg := totalMessages / totalUsers
		
		return fmt.Sprintf("📊 Csatorna stats: %d felhasználó, %d összes XP, %d üzenet | Átlag: %d XP/fő, %d üzenet/fő https://bot.ynm.hu/xp", 
			totalUsers, totalXP, totalMessages, avgXP, avgMsg)
	}
	
	// Rangok listája
	if text == "!xp ranks" || text == "!ranks" {
		chanConfig := xp.Manager.getChannelConfig(msg.Channel)
		if len(chanConfig.Ranks) == 0 {
			return "Nincsenek definiált rangok ebben a csatornában."
		}
		
		var ranks []string
		var keys []int
		for k := range chanConfig.Ranks {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		
		for _, xpReq := range keys {
			ranks = append(ranks, fmt.Sprintf("%s (%d XP)", chanConfig.Ranks[xpReq], xpReq))
		}
		
		return "📋 Elérhető rangok: " + strings.Join(ranks, ", ")
	}
	
	
	// Add this in the HandleXPCommand function, before the final return ""
	if strings.HasPrefix(text, "!xp del ") || strings.HasPrefix(text, "!xp delete ") {
		parts := strings.Fields(text)
		if len(parts) >= 2 {
			targetNick := parts[2]
			// Add admin check here if needed
			if err := xp.Manager.DeleteXP(targetNick); err != nil {
				return fmt.Sprintf("Hiba %s törlésekor: %v", targetNick, err)
			}
			return fmt.Sprintf("%s XP adatai törölve lettek.", targetNick)
		}
	}
	
	
	
	// Más user XP-je
	if strings.HasPrefix(text, "!xp ") {
		parts := strings.Fields(text)
		if len(parts) == 2 {
			targetNick := parts[1]
			userData := xp.Manager.GetUserData(msg.Channel, targetNick)
			if userData == nil {
				return fmt.Sprintf("%s felhasználónak nincs XP-je ebben a csatornában.", targetNick)
			}
			
			rank := xp.Manager.GetRank(msg.Channel, userData.XP)
			return fmt.Sprintf("%s XP-je: %d, Szint: %d, Rang: %s, Üzenetek: %d, https://bot.ynm.hu/xp", 
				targetNick, userData.XP, userData.Level, rank, userData.MsgCount)
		}
	}
	
	return ""
}

// HandleXPGain processes regular messages for XP gain
func (xp *XPPlugin) HandleXPGain(msg YnMIrC.Message) string {
	// Skip commands and very short messages
	if strings.HasPrefix(msg.Text, "!") || len(strings.TrimSpace(msg.Text)) < 3 {
		return ""
	}
	
	// Csatorna specifikus XP mennyiség
	chanConfig := xp.Manager.getChannelConfig(msg.Channel)
	
	newXP, levelUp, err := xp.Manager.AddXP(msg.Channel, msg.Nick, chanConfig.XPPerMsg)
	if err != nil {
		return ""
	}
	
	// Level up üzenet
	if levelUp {
		userData := xp.Manager.GetUserData(msg.Channel, msg.Nick)
		if userData != nil {
			rank := xp.Manager.GetRank(msg.Channel, userData.XP)
			return fmt.Sprintf("🎉 Gratulálok %s! Elérted a %d. szintet! (%d XP, %s rang) https://bot.ynm.hu/xp", 
				msg.Nick, userData.Level, newXP, rank)
		}
	}
	
	return ""
}

// OnTick implements the Plugin interface - called periodically
func (xp *XPPlugin) OnTick() []YnMIrC.Message {
    return nil
}

// Shutdown meghívása program leállításkor
func (xm *XPManager) Shutdown() {
	if xm.backupTicker != nil {
		xm.backupTicker.Stop()
	}
	
	// Final save with proper error handling
	if err := xm.save(); err != nil {
		log.Printf("XP final save error: %v", err)
	}
}

// IsChannelEnabled checks if XP tracking is enabled for a channel
func (xm *XPManager) IsChannelEnabled(chanName string) bool {
	chanConfig := xm.getChannelConfig(chanName)
	return chanConfig.Enabled
}

// IsNickExcluded checks if a nick is excluded from XP tracking
func (xm *XPManager) IsNickExcluded(nick string) bool {
	xm.RLock()
	defer xm.RUnlock()
	return xm.excludedNicks[strings.ToLower(nick)]
}


func (xm *XPManager) ForceSave() error {
	if err := xm.save(); err != nil {
		return err
	}
	return nil
}