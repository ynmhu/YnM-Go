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
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	"gopkg.in/yaml.v3"
)

// ─── Konfig ────────────────────────────────────────────────────────────────

type OnThisDayConfig struct {
	OnThisDayPlugin struct {
		Channel  []string `yaml:"channel"`
		PostTime string   `yaml:"postTime"`
	} `yaml:"OnThisDayPlugin"`
}

func LoadOnThisDayConfig(path string) (*OnThisDayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg OnThisDayConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ─── Wikipedia API struktúrák ───────────────────────────────────────────────

type WikiOnThisDay struct {
	Text  string     `json:"text"`
	Year  int        `json:"year"`
	Pages []WikiPage `json:"pages"`
}

type WikiPage struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

type WikiOnThisDayResponse struct {
	Selected []WikiOnThisDay `json:"selected"`
	Events   []WikiOnThisDay `json:"events"`
	Holidays []WikiOnThisDay `json:"holidays"`
}

// ─── MyMemory fordítás API ─────────────────────────────────────────────────

type MyMemoryResponse struct {
	ResponseData struct {
		TranslatedText string `json:"translatedText"`
	} `json:"responseData"`
	ResponseStatus int `json:"responseStatus"`
}

// ─── Szöveg tisztítás IRC-hez ─────────────────────────────────────────────

func sanitizeForIRC(text string) string {
    // Újsor és CR karakterek eltávolítása
    text = strings.ReplaceAll(text, "\n", " ")
    text = strings.ReplaceAll(text, "\r", " ")
    text = strings.ReplaceAll(text, "\x00", "")
    // Dupla szóközök összevonása
    for strings.Contains(text, "  ") {
        text = strings.ReplaceAll(text, "  ", " ")
    }
    // IRC max üzenethossz ~450 karakter (biztonság kedvéért)
    if len(text) > 450 {
        text = text[:447] + "..."
    }
    return strings.TrimSpace(text)
}

func translateToHungarian(text string) string {
	if text == "" {
		return text
	}

	encoded := url.QueryEscape(text)
	apiURL := fmt.Sprintf("https://api.mymemory.translated.net/get?q=%s&langpair=en|hu", encoded)

	body, err := httpGetOTD(apiURL)
	if err != nil {
		log.Printf("⚠️ [OnThisDay] Fordítás HTTP hiba: %v – eredeti szöveg marad", err)
		return text
	}

	var resp MyMemoryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Printf("⚠️ [OnThisDay] Fordítás parse hiba: %v – eredeti szöveg marad", err)
		return text
	}

	if resp.ResponseStatus != 200 || resp.ResponseData.TranslatedText == "" {
		log.Printf("⚠️ [OnThisDay] Fordítás sikertelen (status: %d) – eredeti szöveg marad", resp.ResponseStatus)
		return text
	}

	log.Printf("🌐 [OnThisDay] Fordítás kész: %q → %q", text, resp.ResponseData.TranslatedText)
	return sanitizeForIRC(resp.ResponseData.TranslatedText)
}

// ─── Plugin ────────────────────────────────────────────────────────────────

type OnThisDayPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	postTime        string
	ticker          *time.Ticker
	stopChan        chan bool
	channels        []string
	discordChannels []string
	lastPostedDate  string
}

func NewOnThisDayPlugin(bot *YnMIrC.Client, cfg struct {
	Channel  []string `yaml:"channel"`
	PostTime string   `yaml:"postTime"`
}, discordAdapter *discord.DiscordAdapter) (*OnThisDayPlugin, error) {

	var discordChannels []string
	var ircChannels []string

	for _, ch := range cfg.Channel {
		if isDiscordChannelOTD(ch) {
			discordChannels = append(discordChannels, ch)
		} else {
			ircChannels = append(ircChannels, ch)
		}
	}

	postTime := cfg.PostTime
	if postTime == "" {
		postTime = "08:00"
	}

	log.Printf("🔧 [OnThisDay] Init – postTime: %q", postTime)
	log.Printf("🔧 [OnThisDay] IRC csatornák: %v", ircChannels)
	log.Printf("🔧 [OnThisDay] Discord csatornák: %v", discordChannels)

	return &OnThisDayPlugin{
		bot:             bot,
		discord:         discordAdapter,
		postTime:        postTime,
		stopChan:        make(chan bool),
		channels:        ircChannels,
		discordChannels: discordChannels,
	}, nil
}

func isDiscordChannelOTD(channel string) bool {
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

// ─── Start / Stop ───────────────────────────────────────────────────────────

func (p *OnThisDayPlugin) Start() {
	p.ticker = time.NewTicker(1 * time.Minute)
	log.Printf("📅 [OnThisDay] Plugin elindult. Küldési idő: %q", p.postTime)

	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.checkAndPost()
			case <-p.stopChan:
				p.ticker.Stop()
				log.Printf("📅 [OnThisDay] Stop() – ticker leállt.")
				return
			}
		}
	}()
}

func (p *OnThisDayPlugin) Stop() {
	close(p.stopChan)
}

// ─── Fő logika ──────────────────────────────────────────────────────────────

func (p *OnThisDayPlugin) checkAndPost() {
	now := time.Now()
	currentTime := now.Format("15:04")
	today := now.Format("2006-01-02")

	if currentTime != p.postTime {
		return
	}
	if p.lastPostedDate == today {
		return
	}

	log.Printf("✅ [OnThisDay] Idő egyezik (%s), küldés indul...", currentTime)
	p.lastPostedDate = today
	p.postDailyMessage(now)
}

func (p *OnThisDayPlugin) postDailyMessage(t time.Time) {
	month := int(t.Month())
	day := t.Day()
	log.Printf("📅 [OnThisDay] postDailyMessage – %04d-%02d-%02d", t.Year(), month, day)

	lines := []string{}

	// 1. Ünnepek
	holiday := fetchHoliday(month, day)
	if holiday != "" {
		lines = append(lines, holiday)
	}

	// 2. Történelmi esemény
	event := fetchHistoricalEvent(month, day)
	if event != "" {
		lines = append(lines, event)
	}

	if len(lines) == 0 {
		log.Printf("⚠️ [OnThisDay] Nincs adat, semmi sem megy ki!")
		return
	}

	header := fmt.Sprintf("📅 \x02Ezen a napon\x02 – %s", magyarDatum(t))
	p.sendToAllChannels(header)
	for _, line := range lines {
		p.sendToAllChannels(line)
	}

	log.Printf("✅ [OnThisDay] Küldés kész (%d sor).", len(lines))
}

// ─── Ünnep lekérés ─────────────────────────────────────────────────────────

func fetchHoliday(month, day int) string {
	apiURL := fmt.Sprintf(
		"https://en.wikipedia.org/api/rest_v1/feed/onthisday/holidays/%02d/%02d",
		month, day,
	)
	log.Printf("🌐 [OnThisDay] GET: %s", apiURL)
	body, err := httpGetOTD(apiURL)
	if err != nil {
		log.Printf("❌ [OnThisDay] Ünnep HTTP hiba: %v", err)
		return ""
	}

	var resp struct {
		Holidays []WikiOnThisDay `json:"holidays"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Holidays) == 0 {
		log.Printf("⚠️ [OnThisDay] Ünnep: nincs adat vagy parse hiba")
		return ""
	}

	h := resp.Holidays[0]
	log.Printf("🌍 [OnThisDay] Ünnep fordítás előtt: %q", h.Text)
	translated := translateToHungarian(h.Text)
	return fmt.Sprintf("🌍 \x02Nemzetközi nap:\x02 %s", translated)
}

// ─── Történelmi esemény lekérés ─────────────────────────────────────────────

func fetchHistoricalEvent(month, day int) string {
	// Először "selected" (fontosabb, szerkesztett) események
	apiURL := fmt.Sprintf(
		"https://en.wikipedia.org/api/rest_v1/feed/onthisday/selected/%02d/%02d",
		month, day,
	)
	log.Printf("🌐 [OnThisDay] GET (selected): %s", apiURL)
	body, err := httpGetOTD(apiURL)
	if err != nil {
		log.Printf("⚠️ [OnThisDay] selected hiba: %v – events fallback...", err)
		apiURL = fmt.Sprintf(
			"https://en.wikipedia.org/api/rest_v1/feed/onthisday/events/%02d/%02d",
			month, day,
		)
		log.Printf("🌐 [OnThisDay] GET (events): %s", apiURL)
		body, err = httpGetOTD(apiURL)
		if err != nil {
			log.Printf("❌ [OnThisDay] events fallback is hiba: %v", err)
			return ""
		}
	}

	var resp WikiOnThisDayResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Printf("❌ [OnThisDay] Esemény JSON parse hiba: %v", err)
		return ""
	}

	events := resp.Selected
	if len(events) == 0 {
		events = resp.Events
	}
	if len(events) == 0 {
		log.Printf("⚠️ [OnThisDay] Nincs egyetlen esemény sem!")
		return ""
	}

	// Véletlenszerű esemény (max 5-ből)
	pool := events
	if len(pool) > 5 {
		pool = pool[:5]
	}
	e := pool[rand.Intn(len(pool))]
	log.Printf("📖 [OnThisDay] Esemény fordítás előtt: %d – %q", e.Year, e.Text)

	translated := translateToHungarian(e.Text)

	link := ""
	if len(e.Pages) > 0 {
		link = " → " + e.Pages[0].ContentURLs.Desktop.Page
	}

	return fmt.Sprintf("📖 \x02%d-ben:\x02 %s%s", e.Year, translated, link)
}

// ─── HTTP helper ─────────────────────────────────────────────────────────────

func httpGetOTD(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YnM-Go IRC Bot/1.0 (https://bot.ynm.hu)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("🌐 [OnThisDay] HTTP %d ← %s", resp.StatusCode, rawURL)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// ─── Küldés ──────────────────────────────────────────────────────────────────

func (p *OnThisDayPlugin) sendToAllChannels(message string) {
	for _, ch := range p.channels {
		log.Printf("📡 [OnThisDay] IRC → %s", ch)
		p.bot.SendMessage(ch, message)
	}
	if p.discord != nil {
		for _, ch := range p.discordChannels {
			log.Printf("🎮 [OnThisDay] Discord → %s", ch)
			if err := p.discord.SendMessage(ch, message); err != nil {
				log.Printf("❌ [OnThisDay] Discord hiba (%s): %v", ch, err)
			}
		}
	}
}

// ─── Segédfüggvény ───────────────────────────────────────────────────────────

func magyarDatum(t time.Time) string {
	honapok := []string{
		"január", "február", "március", "április", "május", "június",
		"július", "augusztus", "szeptember", "október", "november", "december",
	}
	return fmt.Sprintf("%d. %s %d.", t.Year(), honapok[t.Month()-1], t.Day())
}

// ─── IRC parancsok ───────────────────────────────────────────────────────────

func (p *OnThisDayPlugin) HandleMessage(msg YnMIrC.Message) string {
	switch strings.TrimSpace(msg.Text) {
	case "!ma", "!onthisday":
		log.Printf("💬 [OnThisDay] Manuális parancs: %q", msg.Text)
		// Csak abba a csatornába küldjük vissza ahol a parancs érkezett
		go p.postToChannel(time.Now(), msg.Channel)
		return ""
	}
	return ""
}

func (p *OnThisDayPlugin) postToChannel(t time.Time, channel string) {
	month := int(t.Month())
	day := t.Day()

	header := fmt.Sprintf("📅 \x02Ezen a napon\x02 – %s", magyarDatum(t))
	p.bot.SendMessage(channel, header)

	holiday := fetchHoliday(month, day)
	if holiday != "" {
		p.bot.SendMessage(channel, holiday)
	}

	event := fetchHistoricalEvent(month, day)
	if event != "" {
		p.bot.SendMessage(channel, event)
	}
}

func (p *OnThisDayPlugin) OnTick() []YnMIrC.Message {
	return nil
}

func (p *OnThisDayPlugin) Name() string {
	return "On This Day"
}