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
package media

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"sync"

	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	_ "github.com/mattn/go-sqlite3"
)

// MediaItem represents a media item from Jellyfin database
type MediaItem struct {
	Title          string      `json:"title"`
	Genres         string      `json:"genres"`
	Overview       string      `json:"overview"`
	RuntimeTicks   interface{} `json:"runtime_ticks"`
	ProductionYear int         `json:"production_year"`
	DateCreated    string      `json:"date_created"`
	Path           string      `json:"path"`
	MediaType      string      `json:"media_type"`
}

type pendingMedia struct {
	item      *MediaItem
	timestamp time.Time
}
var (
	customMessages = map[string]string{
        "/media/F1/f":      "🎞️  Filmek 2023 után",
        "/media/F2/f":      "🎞️  Filmek 2023 után",
        "/media/F3/f":      "🎞️  Filmek 2023 után",
        "/media/F4/f":      "🎞️  Filmek 2023 után",
        "/media/F5/f":      "🎞️  Filmek 2023 után",
        "/media/F6/f":      "🎞️  Filmek 2023 után",
        "/media/F7/f":      "🎞️  Filmek 2023 után",
        "/media/F8/f":      "🎞️  Filmek 2023 után",
        "/media/F9/f":      "🎞️  Filmek 2023 után",
        "/media/F10/f":     "🎞️  Filmek 2023 után",
        "/media/F11/f":     "🎞️  Filmek 2023 után",
        "/media/F12/f":     "🎞️  Filmek 2023 után",
        "/media/F13/f":     "🎞️  Filmek 2023 után",
        "/media/F14/f":     "🎞️  Filmek 2023 után",
        "/media/F15/f":     "🎞️  Filmek 2023 után",

        "/media/F1/r":      "🎬 2023 előtti filmek",
        "/media/F2/r":      "🎬 2023 előtti filmek",
        "/media/F3/r":      "🎬 2023 előtti filmek",
        "/media/F4/r":      "🎬 2023 előtti filmek",
        "/media/F5/r":      "🎬 2023 előtti filmek",
        "/media/F6/r":      "🎬 2023 előtti filmek",
        "/media/F7/r":      "🎬 2023 előtti filmek",
        "/media/F8/r":      "🎬 2023 előtti filmek",
        "/media/F9/r":      "🎬 2023 előtti filmek",
        "/media/F10/r":     "🎬 2023 előtti filmek",
        "/media/F11/r":     "🎬 2023 előtti filmek",
        "/media/F12/r":     "🎬 2023 előtti filmek",
        "/media/F13/r":     "🎬 2023 előtti filmek",
        "/media/F14/r":     "🎬 2023 előtti filmek",
        "/media/F15/r":     "🎬 2023 előtti filmek",

        "/media/F1/Series": "📺 Sorozatok",
        "/media/F2/Series": "📺 Sorozatok",
        "/media/F3/Series": "📺 Sorozatok",
        "/media/F4/Series": "📺 Sorozatok",
        "/media/F5/Series": "📺 Sorozatok",
        "/media/F6/Series": "📺 Sorozatok",
        "/media/F7/Series": "📺 Sorozatok",
        "/media/F8/Series": "📺 Sorozatok",
        "/media/F9/Series": "📺 Sorozatok",
        "/media/F10/Series":"📺 Sorozatok",
        "/media/F11/Series":"📺 Sorozatok",
        "/media/F12/Series":"📺 Sorozatok",
        "/media/F13/Series":"📺 Sorozatok",
        "/media/F14/Series":"📺 Sorozatok",
        "/media/F15/Series":"📺 Sorozatok",
		"/media/x/Series":     "📺 Sorozatok",
		

        "/media/F1/k":      "📜 Kérve",
        "/media/F2/k":      "📜 Kérve",
        "/media/F3/k":      "📜 Kérve",
        "/media/F4/k":      "📜 Kérve",
        "/media/F5/k":      "📜 Kérve",
        "/media/F6/k":      "📜 Kérve",
        "/media/F7/k":      "📜 Kérve",
        "/media/F8/k":      "📜 Kérve",
        "/media/F9/k":      "📜 Kérve",
        "/media/F10/k":     "📜 Kérve",
        "/media/F11/k":     "📜 Kérve",
        "/media/F12/k":     "📜 Kérve",
        "/media/F13/k":     "📜 Kérve",
        "/media/F14/k":     "📜 Kérve",
        "/media/F15/k":     "📜 Kérve",

        "/media/F1/c":      "🍿 Moziváltozat",
        "/media/F2/c":      "🍿 Moziváltozat",
        "/media/F3/c":      "🍿 Moziváltozat",
        "/media/F4/c":      "🍿 Moziváltozat",
        "/media/F5/c":      "🍿 Moziváltozat",
        "/media/F6/c":      "🍿 Moziváltozat",
        "/media/F7/c":      "🍿 Moziváltozat",
        "/media/F8/c":      "🍿 Moziváltozat",
        "/media/F9/c":      "🍿 Moziváltozat",
        "/media/F10/c":     "🍿 Moziváltozat",
        "/media/F11/c":     "🍿 Moziváltozat",
        "/media/F12/c":     "🍿 Moziváltozat",
        "/media/F13/c":     "🍿 Moziváltozat",
        "/media/F14/c":     "🍿 Moziváltozat",
        "/media/F15/c":     "🍿 Moziváltozat",

        "/media/F1/n":      "🧸 Rajzfilmek",
        "/media/F2/n":      "🧸 Rajzfilmek",
        "/media/F3/n":      "🧸 Rajzfilmek",
        "/media/F4/n":      "🧸 Rajzfilmek",
        "/media/F5/n":      "🧸 Rajzfilmek",
        "/media/F6/n":      "🧸 Rajzfilmek",
        "/media/F7/n":      "🧸 Rajzfilmek",
        "/media/F8/n":      "🧸 Rajzfilmek",
        "/media/F9/n":      "🧸 Rajzfilmek",
        "/media/F10/n":     "🧸 Rajzfilmek",
        "/media/F11/n":     "🧸 Rajzfilmek",
        "/media/F12/n":     "🧸 Rajzfilmek",
        "/media/F13/n":     "🧸 Rajzfilmek",
        "/media/F14/n":     "🧸 Rajzfilmek",
        "/media/F15/n":     "🧸 Rajzfilmek",
		"/media/x/n":     "🧸 Rajzfilmek",

    "/media/F1/e":  "🐰 Rajzfilm Évadok",
    "/media/F2/e":  "🐰 Rajzfilm Évadok",
    "/media/F3/e":  "🐰 Rajzfilm Évadok",
    "/media/F4/e":  "🐰 Rajzfilm Évadok",
    "/media/F5/e":  "🐰 Rajzfilm Évadok",
    "/media/F6/e":  "🐰 Rajzfilm Évadok",
    "/media/F7/e":  "🐰 Rajzfilm Évadok",
    "/media/F8/e":  "🐰 Rajzfilm Évadok",
    "/media/F9/e":  "🐰 Rajzfilm Évadok",
    "/media/F10/e": "🐰 Rajzfilm Évadok",
    "/media/F11/e": "🐰 Rajzfilm Évadok",
    "/media/F12/e": "🐰 Rajzfilm Évadok",
    "/media/F13/e": "🐰 Rajzfilm Évadok",
    "/media/F14/e": "🐰 Rajzfilm Évadok",
    "/media/F15/e": "🐰 Rajzfilm Évadok",
	"/media/x/e":     "🐰 Rajzfilm Évadok",

	"/media/F1/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F2/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F3/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F4/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F5/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F6/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F7/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F8/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F9/m":  "🎞️ Filmek Hu 🇭🇺",
    "/media/F10/m": "🎞️ Filmek Hu 🇭🇺",
    "/media/F11/m": "🎞️ Filmek Hu 🇭🇺",
    "/media/F12/m": "🎞️ Filmek Hu 🇭🇺",
    "/media/F13/m": "🎞️ Filmek Hu 🇭🇺",
    "/media/F14/m": "🎞️ Filmek Hu 🇭🇺",
    "/media/F15/m": "🎞️ Filmek Hu 🇭🇺",
	
	
	
	"/media/F1/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F2/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F3/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F4/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F5/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F6/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F7/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F8/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F9/o":  "🎞️ Filmek Ro 🇷🇴",
    "/media/F10/o": "🎞️ Filmek Ro 🇷🇴",
    "/media/F11/o": "🎞️ Filmek Ro 🇷🇴",
    "/media/F12/o": "🎞️ Filmek Ro 🇷🇴",
    "/media/F13/o": "🎞️ Filmek Ro 🇷🇴",
    "/media/F14/o": "🎞️ Filmek Ro 🇷🇴",
    "/media/F15/o": "🎞️ Filmek Ro 🇷🇴",

	
    "/media/F1/d":  "🌍 Dokumentumfilmek",
    "/media/F2/d":  "🌍 Dokumentumfilmek",
    "/media/F3/d":  "🌍 Dokumentumfilmek",
    "/media/F4/d":  "🌍 Dokumentumfilmek",
    "/media/F5/d":  "🌍 Dokumentumfilmek",
    "/media/F6/d":  "🌍 Dokumentumfilmek",
    "/media/F7/d":  "🌍 Dokumentumfilmek",
    "/media/F8/d":  "🌍 Dokumentumfilmek",
    "/media/F9/d":  "🌍 Dokumentumfilmek",
    "/media/F10/d": "🌍 Dokumentumfilmek",
    "/media/F11/d": "🌍 Dokumentumfilmek",
    "/media/F12/d": "🌍 Dokumentumfilmek",
    "/media/F13/d": "🌍 Dokumentumfilmek",
    "/media/F14/d": "🌍 Dokumentumfilmek",
    "/media/F15/d": "🌍 Dokumentumfilmek",
	
	
	"/media/F1/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F2/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F3/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F4/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F5/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F6/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F7/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F8/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F9/h":  "✅ Hangoskönyvek 🎧 📰",
    "/media/F10/h": "✅ Hangoskönyvek 🎧 📰",
    "/media/F11/h": "✅ Hangoskönyvek 🎧 📰",
    "/media/F12/h": "✅ Hangoskönyvek 🎧 📰",
    "/media/F13/h": "✅ Hangoskönyvek 🎧 📰",
    "/media/F14/h": "✅ Hangoskönyvek 🎧 📰",
    "/media/F15/h": "✅ Hangoskönyvek 🎧 📰",

        "/media/F1/app":   "✅ Android App 🤖",
        "/media/F2/app":   "✅ Android App 🤖",
        "/media/F3/app":   "✅ Android App 🤖",
        "/media/F4/app":   "✅ Android App 🤖",
        "/media/F5/app":   "✅ Android App 🤖",
        "/media/F6/app":   "✅ Android App 🤖",
        "/media/F7/app":   "✅ Android App 🤖",
        "/media/F8/app":   "✅ Android App 🤖",
        "/media/F9/app":   "✅ Android App 🤖",
        "/media/F10/app":  "✅ Android App 🤖",
        "/media/F11/app":  "✅ Android App 🤖",
        "/media/F12/app":  "✅ Android App 🤖",
        "/media/F13/app":  "✅ Android App 🤖",
        "/media/F14/app":  "✅ Android App 🤖",
        "/media/F15/app":  "✅ Android App 🤖",

        "/media/F1/mp3":   "🎵 Mp3",
        "/media/F2/mp3":   "🎵 Mp3",
        "/media/F3/mp3":   "🎵 Mp3",
        "/media/F4/mp3":   "🎵 Mp3",
        "/media/F5/mp3":   "🎵 Mp3",
        "/media/F6/mp3":   "🎵 Mp3",
        "/media/F7/mp3":   "🎵 Mp3",
        "/media/F8/mp3":   "🎵 Mp3",
        "/media/F9/mp3":   "🎵 Mp3",
        "/media/F10/mp3":  "🎵 Mp3",
        "/media/F11/mp3":  "🎵 Mp3",
        "/media/F12/mp3":  "🎵 Mp3",
        "/media/F13/mp3":  "🎵 Mp3",
        "/media/F14/mp3":  "🎵 Mp3",
        "/media/F15/mp3":  "🎵 Mp3",
		
        "/media/F1/i":     "📝 Feliratos Filmek",
        "/media/F2/i":     "📝 Feliratos Filmek",
        "/media/F3/i":     "📝 Feliratos Filmek",
        "/media/F4/i":     "📝 Feliratos Filmek",
        "/media/F5/i":     "📝 Feliratos Filmek",
        "/media/F6/i":     "📝 Feliratos Filmek",
        "/media/F7/i":     "📝 Feliratos Filmek",
        "/media/F8/i":     "📝 Feliratos Filmek",
        "/media/F9/i":     "📝 Feliratos Filmek",
        "/media/F10/i":    "📝 Feliratos Filmek",
        "/media/F11/i":    "📝 Feliratos Filmek",
        "/media/F12/i":    "📝 Feliratos Filmek",
        "/media/F13/i":    "📝 Feliratos Filmek",
        "/media/F14/i":    "📝 Feliratos Filmek",
        "/media/F15/i":    "📝 Feliratos Filmek",		
		
       "/media/F1/km":    "✅ KabareHu 🎧 🎭",
        "/media/F2/km":    "✅ KabareHu 🎧 🎭",
        "/media/F3/km":    "✅ KabareHu 🎧 🎭",
        "/media/F4/km":    "✅ KabareHu 🎧 🎭",
        "/media/F5/km":    "✅ KabareHu 🎧 🎭",
        "/media/F6/km":    "✅ KabareHu 🎧 🎭",
        "/media/F7/km":    "✅ KabareHu 🎧 🎭",
        "/media/F8/km":    "✅ KabareHu 🎧 🎭",
        "/media/F9/km":    "✅ KabareHu 🎧 🎭",
        "/media/F10/km":   "✅ KabareHu 🎧 🎭",
        "/media/F11/km":   "✅ KabareHu 🎧 🎭",
        "/media/F12/km":   "✅ KabareHu 🎧 🎭",
        "/media/F13/km":   "✅ KabareHu 🎧 🎭",
        "/media/F14/km":   "✅ KabareHu 🎧 🎭",
        "/media/F15/km":   "✅ KabareHu 🎧 🎭",

        "/media/F1/u":     "✅ KabareRo 🎧 🎭",
        "/media/F2/u":     "✅ KabareRo 🎧 🎭",
        "/media/F3/u":     "✅ KabareRo 🎧 🎭",
        "/media/F4/u":     "✅ KabareRo 🎧 🎭",
        "/media/F5/u":     "✅ KabareRo 🎧 🎭",
        "/media/F6/u":     "✅ KabareRo 🎧 🎭",
        "/media/F7/u":     "✅ KabareRo 🎧 🎭",
        "/media/F8/u":     "✅ KabareRo 🎧 🎭",
        "/media/F9/u":     "✅ KabareRo 🎧 🎭",
        "/media/F10/u":    "✅ KabareRo 🎧 🎭",
        "/media/F11/u":    "✅ KabareRo 🎧 🎭",
        "/media/F12/u":    "✅ KabareRo 🎧 🎭",
        "/media/F13/u":    "✅ KabareRo 🎧 🎭",
        "/media/F14/u":    "✅ KabareRo 🎧 🎭",
        "/media/F15/u":    "✅ KabareRo 🎧 🎭",

		"/media/x/tv":    "📺 TV-műsorok",
        "/media/F1/tv":    "📺 TV-műsorok",
        "/media/F2/tv":    "📺 TV-műsorok",
        "/media/F3/tv":    "📺 TV-műsorok",
        "/media/F4/tv":    "📺 TV-műsorok",
        "/media/F5/tv":    "📺 TV-műsorok",
        "/media/F6/tv":    "📺 TV-műsorok",
        "/media/F7/tv":    "📺 TV-műsorok",
        "/media/F8/tv":    "📺 TV-műsorok",
        "/media/F9/tv":    "📺 TV-műsorok",
        "/media/F10/tv":   "📺 TV-műsorok",
        "/media/F11/tv":   "📺 TV-műsorok",
        "/media/F12/tv":   "📺 TV-műsorok",
        "/media/F13/tv":   "📺 TV-műsorok",
        "/media/F14/tv":   "📺 TV-műsorok",
        "/media/F15/tv":   "📺 TV-műsorok",		
	
		"/media/x/app":     "✅ Android App 🤖",

	}

    blacklistedPaths = []string{
        "/media/x/x",
        "/media/F0/x",
        "/media/F1/x",
        "/media/F2/x",
        "/media/F3/x",
        "/media/F4/x",
        "/media/F5/x",
        "/media/F6/x",
        "/media/F7/x",
        "/media/F8/x",
        "/media/F9/x",
        "/media/F0/xm",
        "/media/F1/xm",
        "/media/F2/xm",
        "/media/F3/xm",
        "/media/F4/xm",
        "/media/F5/xm",
        "/media/F6/xm",
        "/media/F7/xm",
        "/media/F8/xm",
        "/media/F9/xm",
    }
)

type MediaUploadPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	cfg             *YnMConfig.Config
	sentDates       []string
	lastDate        string
	ticker          *time.Ticker
	stopChan        chan struct{}
	pending         map[string]*pendingMedia
	mu          sync.RWMutex
	ircChannels     []string
	discordChannels []string
}

// Új konstruktor: config-ból automatikusan szétválogatja IRC és Discord csatornákat
func NewMediaUploadPluginWithDiscord(bot *YnMIrC.Client, config *YnMConfig.Config, discordAdapter *discord.DiscordAdapter) *MediaUploadPlugin {
	var discordChannels []string
	var ircChannels []string
	
	//log.Printf("🔍 MediaUpload csatornák feldolgozása...")
	
	// Csatornák szétválogatása
	for _, channel := range config.MediaUpload.Channels {
		if isDiscordChannelMedia(channel) {
			discordChannels = append(discordChannels, channel)
			//log.Printf("  🎮 Discord csatorna: %s", channel)
		} else {
			ircChannels = append(ircChannels, channel)
			//log.Printf("  📡 IRC csatorna: %s", channel)
		}
	}
	
	//log.Printf("📊 MediaUpload csatorna összesítő: %d IRC, %d Discord", len(ircChannels), len(discordChannels))
	
	return &MediaUploadPlugin{
		bot:             bot,
		discord:         discordAdapter,
		cfg:             config,
		ircChannels:     ircChannels,
		discordChannels: discordChannels,
		stopChan:        make(chan struct{}),
		pending:         make(map[string]*pendingMedia),
	}
}

// Eredeti IRC-only konstruktor (backward compatibility)
func NewMediaUploadPlugin(bot *YnMIrC.Client, cfg *YnMConfig.Config) *MediaUploadPlugin {
	var ircChannels []string
	
	for _, channel := range cfg.MediaUpload.Channels {
		if !isDiscordChannelMedia(channel) {
			ircChannels = append(ircChannels, channel)
		}
	}
	
	return &MediaUploadPlugin{
		bot:         bot,
		cfg:         cfg,
		ircChannels: ircChannels,
		stopChan:    make(chan struct{}),
		pending:     make(map[string]*pendingMedia),
	}
}


func (p *MediaUploadPlugin) Name() string {
	return "MediaUpload"
}

func (p *MediaUploadPlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

func (p *MediaUploadPlugin) isPathBlacklisted(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var basePath string
	if len(parts) >= 3 {
		basePath = "/" + strings.Join(parts[0:3], "/")
	} else {
		basePath = path
	}
	
	for _, blacklisted := range blacklistedPaths {
		if basePath == blacklisted {
			return true
		}
	}
	return false
}

func (p *MediaUploadPlugin) Start() error {
	if !p.cfg.MediaUpload.Enabled {
		return nil
	}

	//log.Printf("ℹ️ MediaUpload plugin elindult. Időzítés: %d perc", p.cfg.MediaUpload.IntervalMinutes)
	if len(p.ircChannels) > 0 {
		//log.Printf("📡 IRC csatornák: %v", p.ircChannels)
	}
	if len(p.discordChannels) > 0 {
		//log.Printf("🎮 Discord csatornák: %v", p.discordChannels)
	}
	
	// Üres csatorna lista figyelmeztetés
	if len(p.ircChannels) == 0 && len(p.discordChannels) == 0 {
		log.Printf("⚠️ FIGYELEM: MediaUpload plugin csatorna lista üres!")
	}

	// Betöltjük a már elküldött dátumokat
	var err error
	p.sentDates, err = p.loadSentDates()
	if err != nil {
		return err
	}

	// Indítjuk a ticker-t
	p.ticker = time.NewTicker(time.Duration(p.cfg.MediaUpload.IntervalMinutes) * time.Minute)
	
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.checkAndSendMedia()
			case <-p.stopChan:
				return
			}
		}
	}()

	return nil
}

func (p *MediaUploadPlugin) Stop() {
	if p.ticker != nil {
		p.ticker.Stop()
	}
	select {
	case <-p.stopChan:
		// már zárt
	default:
		close(p.stopChan)
	}
}

func (p *MediaUploadPlugin) loadSentDates() ([]string, error) {
	if _, err := os.Stat(p.cfg.MediaUpload.SentDatesFile); os.IsNotExist(err) {
		// Fájl nem létezik, üres listát adunk vissza
		return []string{}, nil
	}
	
	data, err := os.ReadFile(p.cfg.MediaUpload.SentDatesFile)
	if err != nil {
		return nil, err
	}
	
	var dates []string
	if len(data) > 0 {
		err = json.Unmarshal(data, &dates)
		if err != nil {
			return nil, err
		}
	}
	return dates, nil
}

func (p *MediaUploadPlugin) saveSentDates(dates []string) error {
	data, err := json.Marshal(dates)
	if err != nil {
		return err
	}
	return os.WriteFile(p.cfg.MediaUpload.SentDatesFile, data, 0644)
}

func (p *MediaUploadPlugin) checkAndSendMedia() {
	m, err := p.getLatestMedia()
	if err != nil || m == nil {
		return
	}

	created := strings.Split(m.DateCreated, ".")[0]
	if p.contains(p.sentDates, created) {
		return
	}

	// Tiltólista ellenőrzés
	if strings.Contains(strings.ToLower(m.Title), "xxx") || p.isPathBlacklisted(m.Path) {
		return
	}

	// Ha van leírás, küldjük ki rögtön
	if m.Overview != "" {
		p.sendMedia(m)
		return
	}

	// Ha nincs leírás, pending listára tesszük
	key := m.Path
	if _, exists := p.pending[key]; !exists {
		p.pending[key] = &pendingMedia{
			item:      m,
			timestamp: time.Now(),
		}
	}

	// Ellenőrizzük a pending filmeket
	for key, pm := range p.pending {
		// újra lekérjük a leírást
		latest, err := p.getLatestMediaByPath(pm.item.Path)
		if err != nil || latest == nil {
			continue
		}

		// Ha van leírás, küldjük ki
		if latest.Overview != "" {
			p.sendMedia(latest)
			delete(p.pending, key)
			continue
		}

		// Ha eltelt 3 perc, küldjük ki "Nincs elérhető leírás."-szal
		if time.Since(pm.timestamp) > 3*time.Minute {
			pm.item.Overview = "Nincs elérhető leírás."
			p.sendMedia(pm.item)
			delete(p.pending, key)
		}
	}
}

func (p *MediaUploadPlugin) sendMedia(m *MediaItem) {
	messages := p.FormatMediaMessage(m)
	
	// IRC csatornák
	for _, msg := range messages {
		for _, ch := range p.ircChannels {
			p.bot.SendMessage(ch, msg)
			//log.Printf("✅ MediaUpload üzenet elküldve IRC %s csatornára: %s", ch, m.Title)
			time.Sleep(1 * time.Second)
		}
	}
	
	// Discord csatornák
	if p.discord != nil && len(p.discordChannels) > 0 {
		// Discordra formázott üzenet
		discordMsg := p.FormatDiscordMediaMessage(m)
		for _, ch := range p.discordChannels {
			err := p.discord.SendMessage(ch, discordMsg)
			if err != nil {
				log.Printf("❌ MediaUpload Discord hiba (%s): %v", ch, err)
			} else {
				//log.Printf("✅ MediaUpload üzenet elküldve Discord %s csatornára: %s", ch, m.Title)
			}
			time.Sleep(1 * time.Second)
		}
	}

	created := strings.Split(m.DateCreated, ".")[0]
	p.sentDates = append(p.sentDates, created)
	_ = p.saveSentDates(p.sentDates)
	p.lastDate = m.DateCreated
}

// Discordra optimalizált formázás
func (p *MediaUploadPlugin) FormatDiscordMediaMessage(m *MediaItem) string {
	parts := strings.Split(strings.Trim(m.Path, "/"), "/")
	var basePath string
	if len(parts) >= 3 {
		basePath = "/" + strings.Join(parts[0:3], "/")
	} else {
		basePath = m.Path
	}
	
	custom := customMessages[basePath]
	if custom == "" {
		custom = "✅ Media" // alapértelmezett üzenet
	}

	runtime := ""
	if ticks, err := p.parseRuntimeTicks(m.RuntimeTicks); err == nil {
		runtime = ticks
	}
	
	overview := m.Overview
	if len(overview) > 1000 {
		if idx := strings.LastIndex(overview[:1000], "."); idx > 0 {
			overview = overview[:idx+1]
		} else {
			overview = overview[:1000]
		}
	}

	created := strings.Split(m.DateCreated, ".")[0]

	return fmt.Sprintf("**「 ✦ %s ✦ 」**\n🎭 **Műfaj:** %s\n👆 **Feltöltve:** %s | 📂 **Kategória:** %s \n⏰ **Időtartam:** %s | 📅 **Év:** %d\n📝 **Leírás:** %s",
		m.Title, m.Genres, created, custom, runtime, m.ProductionYear, overview)
}

func (p *MediaUploadPlugin) getLatestMediaByPath(path string) (*MediaItem, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", p.cfg.MediaUpload.JellyfinDB))

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	query := `
		SELECT i.Name, 
		       COALESCE(i.Genres, '') as Genres,
		       COALESCE(i.Overview, '') as Overview, 
		       COALESCE(i.RunTimeTicks, 0) as RunTimeTicks,
		       COALESCE(i.ProductionYear, 0) as ProductionYear,
		       i.DateCreated,
		       COALESCE(i.Path, '') as Path,
		       CASE
		           WHEN i.Type = 'MediaBrowser.Controller.Entities.Movies.Movie' THEN 'Movie'
		           WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Series' THEN 'Series'
		           WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Episode' THEN 'Episode'
		           ELSE 'Other'
		       END as MediaType
		FROM BaseItems i
		WHERE i.Path = ?
		LIMIT 1`

	row := db.QueryRow(query, path)
	var m MediaItem
	err = row.Scan(&m.Title, &m.Genres, &m.Overview, &m.RuntimeTicks,
		&m.ProductionYear, &m.DateCreated, &m.Path, &m.MediaType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	return &m, nil
}

func (p *MediaUploadPlugin) getLatestMedia() (*MediaItem, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", p.cfg.MediaUpload.JellyfinDB))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Tesztelni kellene a kapcsolatot
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	query := `
		SELECT i.Name, 
		       COALESCE(i.Genres, '') as Genres,
		       COALESCE(i.Overview, '') as Overview, 
		       COALESCE(i.RunTimeTicks, 0) as RunTimeTicks,
		       COALESCE(i.ProductionYear, 0) as ProductionYear,
		       i.DateCreated,
		       COALESCE(i.Path, '') as Path,
		       CASE
		           WHEN i.Type = 'MediaBrowser.Controller.Entities.Movies.Movie' THEN 'Movie'
		           WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Series' THEN 'Series'
		           WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Episode' THEN 'Episode'
		           ELSE 'Other'
		       END as MediaType
		FROM BaseItems i
		WHERE i.Type IN ('MediaBrowser.Controller.Entities.Movies.Movie', 
		                 'MediaBrowser.Controller.Entities.TV.Series', 
		                 'MediaBrowser.Controller.Entities.TV.Episode')
		  AND i.DateCreated IS NOT NULL
		ORDER BY i.DateCreated DESC
		LIMIT 1`

	row := db.QueryRow(query)
	var m MediaItem
	
	err = row.Scan(&m.Title, &m.Genres, &m.Overview, &m.RuntimeTicks,
		&m.ProductionYear, &m.DateCreated, &m.Path, &m.MediaType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // nincs új media
		}
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	return &m, nil
}

func (p *MediaUploadPlugin) FormatMediaMessage(m *MediaItem) []string {
	parts := strings.Split(strings.Trim(m.Path, "/"), "/")
	var basePath string
	if len(parts) >= 3 {
		basePath = "/" + strings.Join(parts[0:3], "/")
	} else {
		basePath = m.Path
	}
	
	custom := customMessages[basePath]
	if custom == "" {
		custom = "✅ Media" // alapértelmezett üzenet
	}

	runtime := ""
	if ticks, err := p.parseRuntimeTicks(m.RuntimeTicks); err == nil {
		runtime = ticks
	}
	overview := m.Overview
	if len(overview) > 600 {
		if idx := strings.LastIndex(overview[:600], "."); idx > 0 {
			overview = overview[:idx+1]
		} else {
			overview = overview[:600] + "..."
		}
	}

	created := strings.Split(m.DateCreated, ".")[0]

	return []string{
		fmt.Sprintf(" 「 ✦ %s ✦ 」 | 🎭: %s", m.Title, m.Genres),
		fmt.Sprintf("👆: %s | 📂: %s ", created, custom),
		fmt.Sprintf("⏰: %s | 📅: %d 🎥", runtime, m.ProductionYear),
		fmt.Sprintf("📝: %s", overview),
	}
}

func (p *MediaUploadPlugin) parseRuntimeTicks(ticks any) (string, error) {
	var t int64
	switch v := ticks.(type) {
	case int64:
		t = v
	case float64:
		t = int64(v)
	case int:
		t = int64(v)
	case nil:
		return "00:00:00", nil
	default:
		return "", fmt.Errorf("unexpected type for ticks: %T", ticks)
	}
	
	sec := t / 10000000
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s), nil
}

func (p *MediaUploadPlugin) contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func (p *MediaUploadPlugin) OnTick() []YnMIrC.Message {
	return nil
}