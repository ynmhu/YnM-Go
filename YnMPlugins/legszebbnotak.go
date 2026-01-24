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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"  // ÚJ!
	"path/filepath"  // ÚJ!
	"log"
	"net/http"
	"sync"
	"strings"
	"time"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/telegram"
)

type LatestMusicPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	telegram        *telegram.TelegramAdapter
	channels        []string
	discordChannels []string
	interval        time.Duration
	lastHash        string
	mutex           sync.RWMutex
	ticker          *time.Ticker
	stopChan        chan struct{}
	apiURL          string
	// ÚJ: Facebook konfiguráció
	facebookEnabled    bool
	facebookScriptPath string
}

type MusicAPIResponse struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	SizeFormatted string `json:"size_formatted"`
	AddedDate     string `json:"added_date"`
}

// Új konstruktor: config-ból automatikusan szétválogatja IRC és Discord csatornákat
func NewLatestMusicPluginWithDiscord(bot *YnMIrC.Client, config *YnMConfig.Config,
	discordAdapter *discord.DiscordAdapter, telegramAdapter *telegram.TelegramAdapter) *LatestMusicPlugin {
	var discordChannels []string
	var ircChannels []string
	
	//log.Printf("🔍 LatestMusic csatornák feldolgozása...")
	
	// Csatornák szétválogatása
	for _, channel := range config.LatestMusicChannels {
		if isDiscordChannelMusic(channel) {
			discordChannels = append(discordChannels, channel)
		//	log.Printf("  🎮 Discord csatorna: %s", channel)
		} else {
			ircChannels = append(ircChannels, channel)
			//log.Printf("  📡 IRC csatorna: %s", channel)
		}
	}
	
	// Konzisztencia ellenőrzés: ha vannak Discord csatornák, de nincs adapter
	if len(discordChannels) > 0 && discordAdapter == nil {
		log.Printf("⚠️ FIGYELEM: Discord csatornák megadva, de Discord adapter nincs inicializálva!")
	}
	
	interval := parseDurationMusic(config.LatestMusicInterval)
	
	// ÚJ: Facebook konfiguráció beolvasása
	facebookEnabled := false
	facebookScriptPath := "/home/bot/facebook-poster/postfb.js"
	
	// Ha van Facebook konfig a config-ban, használjuk azt
	if config.FacebookEnabled {
		facebookEnabled = true
		if config.FacebookScriptPath != "" {
			facebookScriptPath = config.FacebookScriptPath
		}
		//log.Printf("  📘 Facebook posztolás: ENGEDÉLYEZVE")
		//log.Printf("     Script: %s", facebookScriptPath)
	}

	//log.Printf("⏰ LatestMusic beállítások:")
	//log.Printf("   📊 IRC csatornák: %d", len(ircChannels))
	//log.Printf("   📊 Discord csatornák: %d", len(discordChannels))
	//log.Printf("   📊 Telegram: %v", telegramAdapter != nil && telegramAdapter.IsEnabled())
	//log.Printf("   📘 Facebook: %v", facebookEnabled)
	//log.Printf("   ⏱️  Interval: %v", interval)

	return &LatestMusicPlugin{
		bot:                bot,
		discord:            discordAdapter,
		telegram:           telegramAdapter,
		channels:           ircChannels,
		discordChannels:    discordChannels,
		interval:           interval,
		stopChan:           make(chan struct{}),
		apiURL:             "https://ynm.hu/wp-json/legszebbnotak/v1/latest",
		facebookEnabled:    facebookEnabled,    // ÚJ!
		facebookScriptPath: facebookScriptPath, // ÚJ!
	}
}

// Eredeti IRC-only konstruktor (backward compatibility)
func NewLatestMusicPlugin(bot *YnMIrC.Client, channels []string, interval time.Duration) *LatestMusicPlugin {
	return &LatestMusicPlugin{
		bot:      bot,
		channels: channels,
		interval: interval,
		stopChan: make(chan struct{}),
		apiURL:   "https://ynm.hu/wp-json/legszebbnotak/v1/latest",
	}
}

// isDiscordChannelMusic ellenőrzi, hogy a channel ID csak számokat tartalmaz-e (Discord channel ID)
func isDiscordChannelMusic(channel string) bool {
	if len(channel) == 0 {
		return false
	}
	for _, char := range channel {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// parseDurationMusic string időtartamot konvertál time.Duration-ra
func parseDurationMusic(durationStr string) time.Duration {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		//log.Printf("⚠️ LatestMusic: Érvénytelen időtartam (%s), alapértelmezett 5 perc használata", durationStr)
		return 5 * time.Minute
	}
	//log.Printf("✅ LatestMusic interval sikeresen beállítva: %v", duration)
	return duration
}

func (p *LatestMusicPlugin) Start() {
	if len(p.channels) > 0 {
		//log.Printf("📡 IRC csatornák: %v", p.channels)
	}
	if len(p.discordChannels) > 0 {
		//log.Printf("🎮 Discord csatornák: %v", p.discordChannels)
	}
	
	// Üres csatorna lista figyelmeztetés
	if len(p.channels) == 0 && len(p.discordChannels) == 0 {
	//	log.Printf("⚠️ FIGYELEM: LatestMusic plugin csatorna lista üres!")
	}
	
	// Hash betöltése egyszer, induláskor (race condition elkerülése)
	p.loadLastHash()
	
	p.ticker = time.NewTicker(p.interval)
	
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.checkAndSendMusic()
			case <-p.stopChan:
				p.ticker.Stop()
				return
			}
		}
	}()
}

func (p *LatestMusicPlugin) Stop() {
	close(p.stopChan)
	if p.ticker != nil {
		p.ticker.Stop()
	}
}

func (p *LatestMusicPlugin) checkAndSendMusic() {
	now := time.Now()
	log.Printf("🕒 LatestMusic ellenőrzés fut: %02d:%02d", now.Hour(), now.Minute())
	
	musicInfo, musicData, err := p.fetchLatestMusic()
	if err != nil {
		log.Printf("❌ LatestMusic API hiba: %v", err)
		return
	}
	
	if musicInfo == "" || musicData == nil {
		return
	}
	
	log.Printf("🎵 Új zene: %s", musicData.Name)
	
	// IRC csatornák
	for _, ch := range p.channels {
		p.bot.SendMessage(ch, musicInfo)
	}
	
	// Discord csatornák
	if p.discord != nil && len(p.discordChannels) > 0 {
		for _, ch := range p.discordChannels {
			err := p.discord.SendMessage(ch, musicInfo)
			if err != nil {
				log.Printf("❌ LatestMusic Discord hiba (%s): %v", ch, err)
			}
		}
	}
	
	// Telegram
	if p.telegram != nil && p.telegram.IsEnabled() {
		go func() {
			err := p.telegram.PostMusic(musicData.Name, musicData.Category, musicData.AddedDate)
			if err != nil {
				log.Printf("❌ Telegram hiba: %v", err)
			}
		}()
	}
	
	// FACEBOOK POSZTOLÁS - ÚJ!
	if p.facebookEnabled {
		go p.postToFacebook(musicData)
	}
}

// ÚJ FÜGGVÉNY: Facebook posztolás Node.js script-tel
func (p *LatestMusicPlugin) postToFacebook(musicData *MusicAPIResponse) {
	log.Printf("📘 Facebook posztolás indítása...")
	
	// Ellenőrizzük, hogy létezik-e a script
	if _, err := os.Stat(p.facebookScriptPath); os.IsNotExist(err) {
		log.Printf("❌ Facebook script nem található: %s", p.facebookScriptPath)
		return
	}
	
	// Node.js futtatása
	cmd := exec.Command("node", p.facebookScriptPath)
	
	// Munkamappa beállítása
	cmd.Dir = filepath.Dir(p.facebookScriptPath)
	
	// Környezeti változók (ha szükséges)
	cmd.Env = os.Environ()
	
	// Kimenet csatolása (hogy lásd a Node script logját)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Futtatás (timeout-tal)
	done := make(chan error, 1)
	
	if err := cmd.Start(); err != nil {
		log.Printf("❌ Facebook script indítási hiba: %v", err)
		return
	}
	
	go func() {
		done <- cmd.Wait()
	}()
	
	// 5 perces timeout (Facebook belépés + posztolás)
	select {
	case err := <-done:
		if err != nil {
			log.Printf("❌ Facebook script futtatási hiba: %v", err)
		} else {
			log.Printf("✅ Facebook posztolás sikeres!")
		}
	case <-time.After(5 * time.Minute):
		log.Printf("⚠️ Facebook script timeout (5 perc)")
		if err := cmd.Process.Kill(); err != nil {
			log.Printf("❌ Process kill hiba: %v", err)
		}
	}
}

// parseLatestMusicInfo segédfüggvény a musicInfo string feldolgozásához
func parseLatestMusicInfo(musicInfo string) (name, category, addedDate string) {
	// Egyszerű string feldolgozás a formázott üzenetből
	parts := strings.Split(musicInfo, " | ")
	
	for _, part := range parts {
		if strings.Contains(part, "Legújabb zene:") {
			name = strings.TrimSpace(strings.Split(part, ":")[1])
		} else if strings.Contains(part, "Kategória:") {
			category = strings.TrimSpace(strings.Split(part, ":")[1])
		} else if strings.Contains(part, "Hozzáadva:") {
			addedDate = strings.TrimSpace(strings.Split(part, ":")[1])
		}
	}
	
	return
}

func (p *LatestMusicPlugin) fetchLatestMusic() (string, *MusicAPIResponse, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(p.apiURL)
	if err != nil {
		return "", nil, fmt.Errorf("API nem elérhető: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("válasz beolvasása sikertelen: %v", err)
	}
	
	var musicData MusicAPIResponse
	if err := json.Unmarshal(body, &musicData); err != nil {
		return "", nil, fmt.Errorf("JSON parse hiba: %v", err)
	}
	
	// Hash készítése
	currentHash := fmt.Sprintf("%s|%s", musicData.Name, musicData.AddedDate)
	
	p.mutex.Lock()
	
	// Ha ugyanaz a zene, ne küldjük újra
	if p.lastHash == currentHash {
		p.mutex.Unlock()
		return "", nil, nil
	}
	
	// Új zene - mentjük a hash-t
	p.lastHash = currentHash
	p.mutex.Unlock()
	
	// Hash mentése fájlba
	p.saveLastHash()
	
	// Formázott üzenet
	formatted := fmt.Sprintf("🎵 Legújabb zene: %s | Kategória: %s | Méret: %s | Hozzáadva: %s | További zenék: https://legszebbnotak.hu",
		musicData.Name,
		musicData.Category,
		musicData.SizeFormatted,
		musicData.AddedDate)
	
	return formatted, &musicData, nil
}

func (p *LatestMusicPlugin) loadLastHash() {
	data, err := os.ReadFile("data/latest_music.hash")
	if err != nil {
		// Ha a fájl nem létezik, ez normális (első futás)
		if !os.IsNotExist(err) {
			log.Printf("⚠️ LatestMusic: Hash betöltési hiba: %v", err)
		}
		return
	}
	
	p.mutex.Lock()
	p.lastHash = string(data)
	p.mutex.Unlock()
}

// saveLastHash elmenti az utolsó hash-t fájlba
func (p *LatestMusicPlugin) saveLastHash() {
	// Biztosítjuk, hogy a data mappa létezik
	if err := os.MkdirAll("data", 0755); err != nil {
		return
	}
	
	p.mutex.RLock()
	hashToSave := p.lastHash
	p.mutex.RUnlock()
	
	err := os.WriteFile("data/latest_music.hash", []byte(hashToSave), 0644)
	if err != nil {
		// Csendes hiba
	}
}

func (p *LatestMusicPlugin) Name() string {
	return "Latest Music"
}