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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
)



type Mp3ScannerPlugin struct {
	bot             *YnMIrC.Client
	config          *YnMConfig.Config
	mp3Config       YnMConfig.Mp3ScannerConfig
	discord         *discord.DiscordAdapter
	mutex           sync.RWMutex
	ircChannels     []string
	discordChannels []string
	stopChan        chan struct{}  // <- EZ HIÁNYZOTT
	ticker          *time.Ticker   // <- EZ IS HIÁNYZHAT
}

func NewMp3ScannerPlugin(bot *YnMIrC.Client, config *YnMConfig.Config, discordAdapter *discord.DiscordAdapter) *Mp3ScannerPlugin {
    // MP3 scanner konfig betöltése
    mp3Config := loadMp3ScannerConfig(config)
    
    var ircChannels []string
    var discordChannels []string
    
    // Csatornák szétválogatása
    for _, channel := range mp3Config.ReportChan {
        if isDiscordChannel(channel) {
            discordChannels = append(discordChannels, channel)
            //log.Printf("  🎮 Discord csatorna: %s", channel)
        } else {
            ircChannels = append(ircChannels, channel)
            //log.Printf("  📡 IRC csatorna: %s", channel)
        }
    }
    
  //  log.Printf("🔧 Mp3Scanner beállítások:")
 //   log.Printf("   📊 IRC csatornák: %d", len(ircChannels))
  //  log.Printf("   📊 Discord csatornák: %d", len(discordChannels))
 //   log.Printf("   📁 Scan directory: %s", mp3Config.ScanDir)
  //  log.Printf("   🎯 Score threshold: %.2f", mp3Config.ScoreThreshold)

return &Mp3ScannerPlugin{
        bot:             bot,
        config:          config,
        mp3Config:       mp3Config,
        discord:         discordAdapter, // EZ HIÁNYZOTT!
        ircChannels:     ircChannels,
        discordChannels: discordChannels,
        stopChan:        make(chan struct{}),
    }
}

func (p *Mp3ScannerPlugin) HandleMessage(msg YnMIrC.Message) string {
    text := strings.TrimSpace(msg.Text)
    
    if text == "!mp3scan" {
        log.Printf("🔍 MP3 scan parancs: %s", msg.Nick)
        go p.CommandScan()
        return "🔍 MP3 fájlok ellenőrzése elindult..."
    }
    
    if text == "!mp3test" {
        log.Printf("🧪 MP3 test parancs: %s", msg.Nick)
        go p.CommandTestScan()
        return "🧪 MP3 teszt ellenőrzés elindult..."
    }
    
    if text == "!mp3info" {
        log.Printf("ℹ️ MP3 info parancs: %s", msg.Nick)
        info := p.DebugInfo()
        return info
    }
    
    if text == "!mp3debug" {
        log.Printf("🐛 MP3 debug parancs: %s", msg.Nick)
        return p.DebugInfo()
    }
    
    // ÚJ: API kulcs tesztelése
    if text == "!testacoustid" {
        log.Printf("🔑 AcoustID API teszt: %s", msg.Nick)
        result := p.testAcoustIDAPI()
        return result
    }
    
    return ""
}

// ÚJ: AcoustID API tesztelése
func (p *Mp3ScannerPlugin) testAcoustIDAPI() string {
    client := &http.Client{Timeout: 10 * time.Second}
    
    // Teszt request érvénytelen fingerprint-del
    resp, err := client.Get(fmt.Sprintf("https://api.acoustid.org/v2/lookup?client=%s&meta=recordings&duration=180&fingerprint=test", p.mp3Config.AcoustIDAPIKey))
    if err != nil {
        return fmt.Sprintf("❌ API hiba: %v", err)
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    var result map[string]interface{}
    if err := json.Unmarshal(body, &result); err != nil {
        return fmt.Sprintf("❌ JSON parse hiba: %v", err)
    }
    
    if status, ok := result["status"].(string); ok {
        if status == "error" {
            if errorMsg, ok := result["error"].(map[string]interface{}); ok {
                if message, ok := errorMsg["message"].(string); ok {
                    if strings.Contains(message, "Invalid API key") {
                        return "❌ ÉRVÉNYTELEN API KULCS"
                    } else if strings.Contains(message, "Invalid fingerprint") {
                        return "✅ API KULCS ÉRVÉNYES (de a fingerprint hibás)"
                    }
                    return fmt.Sprintf("🔍 API válasz: %s", message)
                }
            }
        }
    }
    
    return fmt.Sprintf("📡 API válasz: %s", string(body))
}



func loadMp3ScannerConfig(mainConfig *YnMConfig.Config) YnMConfig.Mp3ScannerConfig {
    // 1. ELŐSZÖR PRÓBÁLJUK A FŐ CONFIG Mp3Scanner SZAKASZÁT
    // Ellenőrizzük, hogy van-e érték a ScanDir-ben (nem üres struct)
    if mainConfig.Mp3Scanner.ScanDir != "" {
        //log.Printf("✅ Mp3Scanner konfig betöltve a fő configból: ScanDir=%s", mainConfig.Mp3Scanner.ScanDir)
        return mainConfig.Mp3Scanner  // ← direkt érték, nem pointer
    }

    // 2. HA NINCS A FŐ CONFIGBAN, PRÓBÁLJUK A KÜLÖN FÁJLT
    configPath := "mp3scanner.yaml"
    if _, err := os.Stat(configPath); err == nil {
        log.Printf("🔧 Mp3Scanner konfig fájl betöltése: %s", configPath)
        data, err := os.ReadFile(configPath)
        if err != nil {
            log.Printf("❌ Mp3Scanner konfig olvasási hiba: %v", err)
            return getDefaultMp3Config()
        }

        root := make(map[string]YnMConfig.Mp3ScannerConfig)
        if err := yaml.Unmarshal(data, &root); err != nil {
            log.Printf("❌ Mp3Scanner YAML parse hiba: %v", err)
            return getDefaultMp3Config()
        }

        var ok bool
        var mp3Config YnMConfig.Mp3ScannerConfig
        if mp3Config, ok = root["Mp3Scanner"]; !ok {
            log.Printf("❌ Mp3Scanner kulcs nem található a YAML-ban")
            return getDefaultMp3Config()
        }
        
        log.Printf("✅ Mp3Scanner konfig betöltve: ScanDir=%s", mp3Config.ScanDir)
        return mp3Config
    }

    // 3. ALAPÉRTELMEZETT
    log.Printf("ℹ️ Mp3Scanner konfig nem található, alapértelmezett használata")
    return getDefaultMp3Config()
}

func getDefaultMp3Config() YnMConfig.Mp3ScannerConfig {
    return YnMConfig.Mp3ScannerConfig{
        ScanDir:        "/media/f8/zsolt",
        LogFile:        "/home/bot/ID/mp3.log",
        TestLogFile:    "/home/bot/ID/mp3_test.log",
        ReportChan:     []string{"#YnM", "1425034642914021399"},
        FpcalcPath:     "/usr/bin/fpcalc",
        ScoreThreshold: 0.8,
        SupportedExt:   []string{".mp3", ".flac", ".wav", ".m4a", ".aac", ".ogg"},
        AcoustIDAPIKey: "I5omICCueZ",
        // ÚJ: Alapértelmezett automatikus szkennelés beállítások
        AutoScanEnabled: false, // Alapértelmezetten kikapcsolva
        ScanInterval:    "24h",
        ScanTime:        "02:00",
        QuietMode:       true,  // Alapértelmezetten csendes mód
    }
}



func (p *Mp3ScannerPlugin) Start() {
	log.Printf("🔧 Mp3Scanner plugin indítva")
	
	// Automatikus szkennelés indítása, ha engedélyezve van
	if p.mp3Config.AutoScanEnabled {
		log.Printf("⏰ Automatikus MP3 szkennelés beállítva: %s időközönként, %s-kor", 
			p.mp3Config.ScanInterval, p.mp3Config.ScanTime)
		go p.StartScheduler()
	} else {
		log.Printf("⏰ Automatikus MP3 szkennelés letiltva")
	}
}

func (p *Mp3ScannerPlugin) Stop() {
	log.Printf("🔧 Mp3Scanner plugin leállítva")
	if p.stopChan != nil {
		close(p.stopChan)
	}
	if p.ticker != nil {
		p.ticker.Stop()
	}
}

func (p *Mp3ScannerPlugin) Name() string {
	return "MP3 Scanner"
}

func (p *Mp3ScannerPlugin) DebugInfo() string {
    var lines []string
    
    lines = append(lines, "🎵 MP3 Scanner Info:")
    lines = append(lines, fmt.Sprintf("📁 ScanDir: %s", p.mp3Config.ScanDir))
    lines = append(lines, fmt.Sprintf("📊 IRC: %d, Discord: %d", len(p.ircChannels), len(p.discordChannels)))
    lines = append(lines, fmt.Sprintf("🎯 Score: %.2f", p.mp3Config.ScoreThreshold))
    
    // Automatikus szkennelés információk
    if p.mp3Config.AutoScanEnabled {
        lines = append(lines, "⏰ Auto: ✅ BEKAPCSOLVA")
        lines = append(lines, fmt.Sprintf("📅 Interval: %s", p.mp3Config.ScanInterval))
        if p.mp3Config.ScanTime != "" {
            lines = append(lines, fmt.Sprintf("🕐 Time: %s", p.mp3Config.ScanTime))
        }
        lines = append(lines, fmt.Sprintf("🔇 Quiet mode: %v", p.mp3Config.QuietMode))
        
        if nextRun, err := p.calculateNextRun(); err == nil {
            lines = append(lines, fmt.Sprintf("🚀 Next: %s", nextRun.Format("01-02 15:04")))
        }
    } else {
        lines = append(lines, "⏰ Auto: ❌ KIKAPCSOLVA")
    }
    
    return strings.Join(lines, " | ")
}
// Segédfüggvény a következő futás időpontjának kiszámolásához
func (p *Mp3ScannerPlugin) calculateNextRun() (time.Time, error) {
    if !p.mp3Config.AutoScanEnabled {
        return time.Time{}, fmt.Errorf("automatikus szkennelés nincs bekapcsolva")
    }
    
    now := time.Now()
    
    // Ha van specifikus idő megadva
    if p.mp3Config.ScanTime != "" {
        targetTime, err := time.Parse("15:04", p.mp3Config.ScanTime)
        if err != nil {
            return time.Time{}, err
        }
        
        // Következő futás ma
        nextRun := time.Date(now.Year(), now.Month(), now.Day(), 
            targetTime.Hour(), targetTime.Minute(), 0, 0, now.Location())
        
        // Ha ma már elmúlt az idő, holnapra számoljuk
        if nextRun.Before(now) {
            interval, err := time.ParseDuration(p.mp3Config.ScanInterval)
            if err != nil {
                return time.Time{}, err
            }
            nextRun = nextRun.Add(interval)
        }
        
        return nextRun, nil
    }
    
    // Interval alapú számítás
    interval, err := time.ParseDuration(p.mp3Config.ScanInterval)
    if err != nil {
        return time.Time{}, err
    }
    
    return now.Add(interval), nil
}

// ÚJ: Részletes schedule információ
func (p *Mp3ScannerPlugin) getScheduleInfo() string {
    if !p.mp3Config.AutoScanEnabled {
        return "⏰ MP3 Auto Scan: ❌ KIKAPCSOLVA | Use !mp3scan"
    }
    
    var lines []string
    lines = append(lines, "⏰ MP3 Auto Scan: ✅ ON")
    lines = append(lines, fmt.Sprintf("Interval: %s", p.mp3Config.ScanInterval))
    
    if p.mp3Config.ScanTime != "" {
        lines = append(lines, fmt.Sprintf("Time: %s", p.mp3Config.ScanTime))
    }
    
    if nextRun, err := p.calculateNextRun(); err == nil {
        timeUntil := time.Until(nextRun)
        hoursUntil := int(timeUntil.Hours())
        minutesUntil := int(timeUntil.Minutes()) % 60
        lines = append(lines, fmt.Sprintf("Next: %s", nextRun.Format("01-02 15:04")))
        lines = append(lines, fmt.Sprintf("In: %dh %dm", hoursUntil, minutesUntil))
    }
    
    return strings.Join(lines, " | ")
}
// 🔹 Manuális parancs
func (p *Mp3ScannerPlugin) CommandScan() {
	p.sendMessageToAllChannels(fmt.Sprintf("📂 MP3 fájlok ellenőrzése elindult: %s...", p.mp3Config.ScanDir))
	p.appendLog(fmt.Sprintf("\n=== Manuális ellenőrzés kezdete: %s ===\n", time.Now().Format("2006-01-02 15:04:05")), p.mp3Config.LogFile)
	total, processed, deleted := p.scanDirectory(p.mp3Config.ScanDir, p.mp3Config.LogFile, false) // false = nem quiet mode
	
	summary := fmt.Sprintf("📊 Összesen talált audio fájlok: %d, Feldolgozott: %d, Törölve: %d", total, processed, deleted)
	p.sendMessageToAllChannels(summary)
	p.sendMessageToAllChannels("✅ Az MP3 fájlok ellenőrzése befejeződött.")
}
// 🔹 Automatikus szkennelés - csendes mód
func (p *Mp3ScannerPlugin) AutoScan() {
	log.Printf("🔍 Automatikus MP3 szkennelés indítása...")
	p.appendLog(fmt.Sprintf("\n=== Automatikus ellenőrzés kezdete: %s ===\n", time.Now().Format("2006-01-02 15:04:05")), p.mp3Config.LogFile)
	
	total, processed, deleted := p.scanDirectory(p.mp3Config.ScanDir, p.mp3Config.LogFile, p.mp3Config.QuietMode)
	
	// Csak logoljuk, nem küldünk üzenetet a csatornákba
	log.Printf("🔍 Automatikus MP3 szkennelés befejezve: %d fájl, %d feldolgozott, %d törölve", total, processed, deleted)
	
	// Ha találtunk védett tartalmat, akkor küldjünk összefoglalót
	if deleted > 0 {
		summary := fmt.Sprintf("🛡️ Automatikus MP3 ellenőrzés: %d védett fájl törölve", deleted)
		p.sendMessageToAllChannels(summary)
	} else if !p.mp3Config.QuietMode {
		// Ha nincs quiet mode, akkor küldjük az eredményt
		summary := fmt.Sprintf("✅ Automatikus MP3 ellenőrzés: %d fájl, 0 védett tartalom", total)
		p.sendMessageToAllChannels(summary)
	}
}

// 🔹 Gyors teszt - mindig teljes jelentéssel
func (p *Mp3ScannerPlugin) CommandTestScan() {
	p.sendMessageToAllChannels(fmt.Sprintf("🧪 Teszt ellenőrzés: %s", p.mp3Config.ScanDir))
	p.appendLog(fmt.Sprintf("\n=== Teszt ellenőrzés: %s ===\n", time.Now().Format("2006-01-02 15:04:05")), p.mp3Config.TestLogFile)
	total, processed, deleted := p.scanDirectory(p.mp3Config.ScanDir, p.mp3Config.TestLogFile, false) // false = nem quiet mode
	
	summary := fmt.Sprintf("🧪 Teszt eredmény: Fájlok: %d, Feldolgozott: %d, Törölve: %d", total, processed, deleted)
	p.sendMessageToAllChannels(summary)
	p.sendMessageToAllChannels("✅ Teszt befejezve.")
}

// 🔹 Fájl ellenőrzés AcoustID-vel
// 🔹 Fájl ellenőrzés AcoustID-vel
func (p *Mp3ScannerPlugin) checkAudio(filePath string, logFile string, quietMode bool) bool {
	log.Printf("🔍 Ellenőrzés folyamatban: %s", filePath)

	// 1️⃣ fpcalc hívás
	cmd := exec.Command(p.mp3Config.FpcalcPath, "-json", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		logLine := fmt.Sprintf("❌ fpcalc hiba: %s - %v\n", filePath, err)
		p.appendLog(logLine, logFile)
		log.Print(logLine)
		return false
	}

	// 2️⃣ AcoustID API hívás
	resp, err := http.PostForm("https://api.acoustid.org/v2/lookup", map[string][]string{
		"client":      {p.mp3Config.AcoustIDAPIKey},
		"duration":    {p.extractDuration(out.Bytes())},
		"fingerprint": {p.extractFingerprint(out.Bytes())},
		"meta":        {"recordings"},
	})
	if err != nil {
		logLine := fmt.Sprintf("❌ AcoustID API hiba: %s - %v\n", filePath, err)
		p.appendLog(logLine, logFile)
		log.Print(logLine)
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		logLine := fmt.Sprintf("❌ JSON parse hiba: %s - %v\n", filePath, err)
		p.appendLog(logLine, logFile)
		log.Print(logLine)
		return false
	}

	if matches, ok := result["results"].([]interface{}); ok {
		for _, m := range matches {
			if matchMap, ok := m.(map[string]interface{}); ok {
				if score, ok := matchMap["score"].(float64); ok && score >= p.mp3Config.ScoreThreshold {
					if recordings, ok := matchMap["recordings"].([]interface{}); ok && len(recordings) > 0 {
						for _, r := range recordings {
							rec := r.(map[string]interface{})
							title := rec["title"].(string)
							artist := ""
							if artists, ok := rec["artists"].([]interface{}); ok && len(artists) > 0 {
								artistMap := artists[0].(map[string]interface{})
								artist = artistMap["name"].(string)
							}
							warning := fmt.Sprintf("⚠️ Jogvédett fájl (%.2f): %s - %s - %s\n", score, filePath, title, artist)
							p.appendLog(warning, logFile)
							log.Print(warning)
							
							// MINDIG küldjük el a védett tartalomról az értesítést
							p.sendMessageToAllChannels(fmt.Sprintf("⚠️ Jogvédett tartalom (%.2f)! -> %s - %s (%s)", score, title, artist, filepath.Base(filePath)))

							// fájl törlése
							if err := os.Remove(filePath); err != nil {
								errMsg := fmt.Sprintf("❌ Törlési hiba: %s - %v\n", filePath, err)
								p.appendLog(errMsg, logFile)
								log.Print(errMsg)
								p.sendMessageToAllChannels(fmt.Sprintf("❌ Törlési hiba: %s - %v", filepath.Base(filePath), err))
							} else {
								deleteMsg := fmt.Sprintf("🗑️ Törölve: %s\n", filePath)
								p.appendLog(deleteMsg, logFile)
								log.Print(deleteMsg)
								p.sendMessageToAllChannels(fmt.Sprintf("🗑️ Törölve: %s", filepath.Base(filePath)))
							}
							return true
						}
					}
				}
			}
		}
	}

	p.appendLog(fmt.Sprintf("ℹ️ Alacsony vagy nincs egyezés: %s\n", filePath), logFile)
	time.Sleep(400 * time.Millisecond) // rate limiting
	return false
}
// 🔹 Log írás
func (p *Mp3ScannerPlugin) appendLog(line string, logFile string) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("❌ Log írási hiba: %v", err)
		return
	}
	defer f.Close()
	f.WriteString(line)
}

// 🔹 Mappa bejárása
func (p *Mp3ScannerPlugin) scanDirectory(dir string, logFile string, quietMode bool) (total, processed, deleted int) {
	log.Printf("🔍 Keresés a könyvtárban: %s (quiet mode: %v)", dir, quietMode)
	
	// Ellenőrizd, hogy a könyvtár létezik-e
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("❌ A könyvtár nem létezik: %s", dir)
		log.Printf(errorMsg)
		if !quietMode {
			p.sendMessageToAllChannels(errorMsg)
		}
		return
	}
	
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			p.appendLog(fmt.Sprintf("❌ WalkDir hiba: %v\n", err), logFile)
			return nil
		}
		if !d.IsDir() && p.hasSupportedExt(path) {
			total++
			if p.checkAudio(path, logFile, quietMode) {
				deleted++
			}
			processed++
		}
		return nil
	})
	
	if err != nil {
		p.appendLog(fmt.Sprintf("❌ ScanDirectory hiba: %v\n", err), logFile)
	}
	
	summary := fmt.Sprintf("📊 Összesen talált audio fájlok: %d, Feldolgozott: %d, Törölve: %d\n", total, processed, deleted)
	p.appendLog(summary, logFile)
	log.Printf(summary)
	
	return
}

func (p *Mp3ScannerPlugin) hasSupportedExt(filename string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range p.mp3Config.SupportedExt {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// 🔹 Üzenet küldése minden csatornára
func (p *Mp3ScannerPlugin) sendMessageToAllChannels(message string) {
	// IRC csatornák
	for _, ch := range p.ircChannels {
		p.bot.SendMessage(ch, message)
	}
	
	// Discord csatornák
	if p.discord != nil && len(p.discordChannels) > 0 {
		for _, ch := range p.discordChannels {
			err := p.discord.SendMessage(ch, message)
			if err != nil {
				log.Printf("❌ Mp3Scanner Discord hiba (%s): %v", ch, err)
			}
		}
	}
}

// 🔹 fpcalc JSON segédfüggvények
func (p *Mp3ScannerPlugin) extractDuration(fpcalcJSON []byte) string {
	var data map[string]interface{}
	json.Unmarshal(fpcalcJSON, &data)
	if dur, ok := data["duration"].(float64); ok {
		return fmt.Sprintf("%.0f", dur)
	}
	return "0"
}

func (p *Mp3ScannerPlugin) extractFingerprint(fpcalcJSON []byte) string {
	var data map[string]interface{}
	json.Unmarshal(fpcalcJSON, &data)
	if fp, ok := data["fingerprint"].(string); ok {
		return fp
	}
	return ""
}


// 🔹 Automatikus időzített futás
func (p *Mp3ScannerPlugin) StartScheduler() {
	// Parse interval
	interval, err := time.ParseDuration(p.mp3Config.ScanInterval)
	if err != nil {
		log.Printf("❌ Érvénytelen scan_interval: %s, alapértelmezett 24h használata", p.mp3Config.ScanInterval)
		interval = 24 * time.Hour
	}

	// Ha van specifikus idő megadva, számoljuk ki a következő futást
	if p.mp3Config.ScanTime != "" {
		p.scheduleAtSpecificTime(interval)
	} else {
		// Egyszerű interval alapú ütemezés
		p.scheduleWithInterval(interval)
	}
}

func (p *Mp3ScannerPlugin) scheduleAtSpecificTime(interval time.Duration) {
	for {
		now := time.Now()
		
		// Parse the target time (pl: "02:00")
		targetTime, err := time.Parse("15:04", p.mp3Config.ScanTime)
		if err != nil {
			log.Printf("❌ Érvénytelen scan_time: %s, interval alapú ütemezés használata", p.mp3Config.ScanTime)
			p.scheduleWithInterval(interval)
			return
		}
		
		// Create next run time for today
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 
			targetTime.Hour(), targetTime.Minute(), 0, 0, now.Location())
		
		// If the time already passed today, schedule for tomorrow
		if nextRun.Before(now) {
			nextRun = nextRun.Add(interval)
		}
		
		sleepDuration := nextRun.Sub(now)
		log.Printf("⏰ Következő MP3 szkennelés: %s (%.0f perc múlva)", 
			nextRun.Format("2006-01-02 15:04"), sleepDuration.Minutes())
		
		// Wait until the next run time
		time.Sleep(sleepDuration)
		
		// Execute the auto scan (not CommandScan!)
		log.Printf("🔍 Automatikus MP3 szkennelés indítása...")
		p.AutoScan() // ← MÓDOSÍTOTT: AutoScan helyett CommandScan
	}
}

func (p *Mp3ScannerPlugin) scheduleWithInterval(interval time.Duration) {
	p.ticker = time.NewTicker(interval)
	
	log.Printf("⏰ MP3 szkennelés beállítva: %v időközönként", interval)
	
	go func() {
		for {
			select {
			case <-p.ticker.C:
				log.Printf("🔍 Automatikus MP3 szkennelés indítása...")
				p.AutoScan() // ← MÓDOSÍTOTT: AutoScan helyett CommandScan
			case <-p.stopChan:
				p.ticker.Stop()
				return
			}
		}
	}()
}



func (p *Mp3ScannerPlugin) OnTick() []YnMIrC.Message {
	return nil
}