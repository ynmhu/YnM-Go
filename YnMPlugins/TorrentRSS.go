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
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
//	"gopkg.in/yaml.v3"
//	"os"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
)

// Átnevezett és egységesített konfiguráció
type TorrentRSS struct {
	bot           *YnMIrC.Client
	lastRequest   map[string]time.Time
	mu            sync.Mutex
	config        YnMConfig.TorrentConfig  
	lastItems     []string 
	ticker        *time.Ticker
	stopChan      chan bool
	feedName      string 
}

type RSSItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
}

type RSS struct {
	Channel struct {
		Items []RSSItem `xml:"item"`
	} `xml:"channel"`
}

func NewTorrentRSS(bot *YnMIrC.Client, cfg YnMConfig.TorrentConfig) *TorrentRSS {
	h := &TorrentRSS{
		bot:         bot,
		lastRequest: make(map[string]time.Time),
		lastItems:   make([]string, 0),
		stopChan:    make(chan bool),
		config:      cfg,
		feedName:    cfg.Name,
	}

	// Konfiguráció validáció
	if h.config.MaxItems < 1 || h.config.MaxItems > 5 {
		h.config.MaxItems = 1
	}
	if h.config.StartHour < 0 || h.config.StartHour > 23 {
		h.config.StartHour = 8
	}
	if h.config.EndHour < 0 || h.config.EndHour > 23 {
		h.config.EndHour = 20
	}
	if h.config.CheckInterval < 10 {
		h.config.CheckInterval = 10
	}

	// Automatikus ellenőrzés indítása ha engedélyezve van és van RSS URL
	if h.config.Enabled && strings.TrimSpace(h.config.RSSUrl) != "" {
		h.startAutoCheck()
		//fmt.Printf("🚀 RSS automatikus ellenőrzés elindítva: %s\n", h.feedName)
	}

	return h
}

func (h *TorrentRSS) startAutoCheck() {
	if h.ticker != nil {
		h.ticker.Stop()
	}
	
	interval := time.Duration(h.config.CheckInterval) * time.Minute
	h.ticker = time.NewTicker(interval)
	
	go func() {
		for {
			select {
			case <-h.ticker.C:
				if h.isActiveTime() {
					h.checkForNewTorrents()
				}
			case <-h.stopChan:
				return
			}
		}
	}()
}

func (h *TorrentRSS) isActiveTime() bool {
	now := time.Now()
	hour := now.Hour()
	
	if h.config.StartHour <= h.config.EndHour {
		// Normál időszak (pl. 8-20)
		return hour >= h.config.StartHour && hour < h.config.EndHour
	} else {
		// Átnyúlik éjfélre (pl. 20-8)
		return hour >= h.config.StartHour || hour < h.config.EndHour
	}
}

func (h *TorrentRSS) checkForNewTorrents() {
	rssData, err := h.fetchRSSData()
	if err != nil {
		fmt.Printf("❌ RSS hiba (%s): %v\n", h.feedName, err)
		return
	}
	
	if len(rssData.Channel.Items) == 0 {
		return
	}
	
	newItems := h.findNewItems(rssData.Channel.Items)
	if len(newItems) > 0 {
		fmt.Printf("✅ %s - %d új torrent található\n", h.feedName, len(newItems))
		h.sendNewTorrentsToChannels(newItems)
	} else {
		fmt.Printf("ℹ️ %s - Nincsenek új torrentek\n", h.feedName)
	}
}

func (h *TorrentRSS) findNewItems(items []RSSItem) []RSSItem {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	var newItems []RSSItem
	
	for _, item := range items {
		isNew := true
		for _, lastTitle := range h.lastItems {
			if item.Title == lastTitle {
				isNew = false
				break
			}
		}
		if isNew {
			newItems = append(newItems, item)
			// Azonnal hozzáadjuk a lastItems-hoz, hogy ne jelenjen meg újra
			h.lastItems = append(h.lastItems, item.Title)
		}
		// Csak a korábban beállított maximum számú elemet tartjuk meg
		if len(h.lastItems) >= h.config.MaxItems * 2 {
			break
		}
	}
	
	return newItems
}

func (h *TorrentRSS) updateLastItems(items []RSSItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Új lista építése a legújabb elemekből
	newLastItems := make([]string, 0)
	maxTrack := h.config.MaxItems * 2
	
	for i, item := range items {
		if i >= maxTrack {
			break
		}
		newLastItems = append(newLastItems, item.Title)
	}
	
	h.lastItems = newLastItems
}

func (h *TorrentRSS) sendNewTorrentsToChannels(newItems []RSSItem) {
	maxShow := h.config.MaxItems
	if len(newItems) < maxShow {
		maxShow = len(newItems)
	}
	
	for _, channel := range h.config.Channels {
		go func(ch string) {
			// Feed névvel kiegészített header
			header := fmt.Sprintf("🆕 %s - Új Feltöltések Automatikusan", h.feedName)
			h.bot.SendMessage(ch, header)
			
			for i := 0; i < maxShow; i++ {
				item := newItems[i]
				message := h.formatTorrentMessage(i+1, item)
				h.bot.SendMessage(ch, message)
				time.Sleep(300 * time.Millisecond)
			}
		}(channel)
	}
}

func (h *TorrentRSS) isRateLimited(channel string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	now := time.Now()
	if last, exists := h.lastRequest[channel]; exists {
		if now.Sub(last) < 30*time.Second { 
			return true
		}
	}
	h.lastRequest[channel] = now
	return false
}

func (h *TorrentRSS) extractGenre(description string) string {
	re := regexp.MustCompile(`Műfaj:\s*([^|]+)`)
	matches := re.FindStringSubmatch(description)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return "N/A"
}

func (h *TorrentRSS) formatDate(pubDate string) string {
	t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", pubDate)
	if err != nil {
		return pubDate
	}
	return t.Format("2006.01.02 15:04")
}

func sanitizeIRCText(s string, maxLen int) string {
	// CR/LF/TAB eltávolítás
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")

	// Többszörös space javítás
	s = strings.Join(strings.Fields(s), " ")

	// UTF-8 biztonság + hossz limit
	runes := []rune(s)
	if len(runes) > maxLen {
		s = string(runes[:maxLen]) + "..."
	}

	return strings.TrimSpace(s)
}

func (h *TorrentRSS) formatTorrentMessage(index int, item RSSItem) string {
	cleanTitle := sanitizeIRCText(item.Title, 120)
	genre := sanitizeIRCText(h.extractGenre(item.Description), 40)
	category := sanitizeIRCText(item.Category, 20)

	if category == "" {
		category = "N/A"
	}

	formattedDate := h.formatDate(item.PubDate)

	msg := fmt.Sprintf("📁 [%d] %s | 🎭 %s | 📂 %s | ⏰ %s",
		index, cleanTitle, genre, category, formattedDate)

	return sanitizeIRCText(msg, 350)
}

func (h *TorrentRSS) fetchRSSData() (*RSS, error) {
	if strings.TrimSpace(h.config.RSSUrl) == "" {
		return nil, fmt.Errorf("RSS URL nincs konfigurálva")
	}

	resp, err := http.Get(h.config.RSSUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// XML cleanup - EZ HIÁNYZIK!
	cleanBody := cleanXML(string(body))

	var rss RSS
	if err := xml.Unmarshal([]byte(cleanBody), &rss); err != nil {
		return nil, err
	}

	return &rss, nil
}

// XML cleanup függvény - ADD HOOZZÁ EZT
func cleanXML(xmlData string) string {
	// Javítjuk a hibás HTML entitásokat
	cleaned := strings.ReplaceAll(xmlData, "&torrent_pass", "&amp;torrent_pass")
	cleaned = strings.ReplaceAll(cleaned, "&amp;amp;", "&amp;") // dupla escape javítása
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")       // space entitás javítása
	return cleaned
}

func (h *TorrentRSS) HandleMessage(msg YnMIrC.Message) string {
	// Konfigurációs parancsok
	if strings.HasPrefix(msg.Text, "!torrentconfig") || strings.HasPrefix(msg.Text, "!tconfig") {
		return h.handleConfigCommand(msg)
	}
	
	// RSS lekérdezési parancsok - több variáció támogatása
	validCommands := []string{"!torrent", "!rss", "!huntorrent", "!ht", "!ncore", "!nc"}
	isValidCommand := false
	
	for _, cmd := range validCommands {
		if strings.HasPrefix(msg.Text, cmd) {
			isValidCommand = true
			break
		}
	}
	
	if !isValidCommand {
		return ""
	}

	if h.isRateLimited(msg.Channel) {
		return "⌛ Kérlek várj 30 másodpercet a következő lekérdezés előtt!"
	}

	rssData, err := h.fetchRSSData()
	if err != nil {
		return fmt.Sprintf("❌ RSS hiba (%s): %v", h.feedName, err)
	}

	if len(rssData.Channel.Items) == 0 {
		return fmt.Sprintf("ℹ️ Nincsenek új torrentek (%s).", h.feedName)
	}

	// Header küldése feed névvel
	header := fmt.Sprintf("🔥 %s - Legújabb Feltöltések", h.feedName)
	go h.bot.SendMessage(msg.Channel, header)

	maxItems := h.config.MaxItems
	if len(rssData.Channel.Items) < maxItems {
		maxItems = len(rssData.Channel.Items)
	}

	go func() {
		for i := 0; i < maxItems; i++ {
			item := rssData.Channel.Items[i]
			message := h.formatTorrentMessage(i+1, item)
			h.bot.SendMessage(msg.Channel, message)
			time.Sleep(300 * time.Millisecond)
		}
	}()

	return ""
}

func (h *TorrentRSS) handleConfigCommand(msg YnMIrC.Message) string {
	parts := strings.Fields(msg.Text)
	if len(parts) < 2 {
		rssStatus := "konfigurálva"
		if strings.TrimSpace(h.config.RSSUrl) == "" {
			rssStatus = "NINCS BEÁLLÍTVA!"
		}
		return fmt.Sprintf("⚙️ %s konfig: Csatornák: %v | RSS URL: %s | Max elemek: %d | Időszak: %d-%d óra | Intervallum: %d perc | Állapot: %t", 
			h.feedName, h.config.Channels, rssStatus, h.config.MaxItems, h.config.StartHour, h.config.EndHour, h.config.CheckInterval, h.config.Enabled)
	}

	switch parts[1] {
	case "enable":
		if strings.TrimSpace(h.config.RSSUrl) == "" {
			return fmt.Sprintf("❌ %s: RSS URL nincs konfigurálva! Először add meg az RSS linket a rss.yaml fájlban!", h.feedName)
		}
		h.config.Enabled = true
		h.startAutoCheck()
		return fmt.Sprintf("✅ %s automatikus ellenőrzés bekapcsolva", h.feedName)
		
	case "disable":
		h.config.Enabled = false
		if h.ticker != nil {
			h.ticker.Stop()
		}
		return fmt.Sprintf("❌ %s automatikus ellenőrzés kikapcsolva", h.feedName)
		
	case "status":
		status := "kikapcsolva"
		if h.config.Enabled {
			status = "bekapcsolva"
		}
		return fmt.Sprintf("📊 %s állapot: %s | RSS URL: %s | Ellenőrzés: %d percenként", 
			h.feedName, status, 
			func() string {
				if h.config.RSSUrl == "" { return "NINCS" }
				return "VAN"
			}(), h.config.CheckInterval)
		
	case "test":
		// Teszt lekérés
		rssData, err := h.fetchRSSData()
		if err != nil {
			return fmt.Sprintf("❌ %s teszt hiba: %v", h.feedName, err)
		}
		if len(rssData.Channel.Items) == 0 {
			return fmt.Sprintf("⚠️ %s: RSS feed üres", h.feedName)
		}
		return fmt.Sprintf("✅ %s teszt sikeres - %d elem található", h.feedName, len(rssData.Channel.Items))
		
	default:
		return "ℹ️ Használat: !tconfig [enable|disable|status|test] vagy !tconfig a beállításokhoz"
	}
}

func (h *TorrentRSS) OnTick() []YnMIrC.Message {
	return nil
}

// Cleanup függvény
func (h *TorrentRSS) Stop() {
	if h.ticker != nil {
		h.ticker.Stop()
	}
	close(h.stopChan)
}