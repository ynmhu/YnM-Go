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
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
	
	"github.com/mmcdole/gofeed"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

// MessageSender interface IRC és Discord közös kezelésére
type MessageSender interface {
	SendMessage(target, message string) error
}

type ForumPluginConfig struct {
	DbPath          string   `yaml:"DB_PATH"`           // pl: ./data/forum.db
	RssUrl          string   `yaml:"RSS_URL"`           // pl: https://forum.ynm.hu/latest.rss
	IrcChannels     []string `yaml:"IRC_CHANNELS"`      // pl: ["#Magyar", "YnM"]
	DiscordChannels []string `yaml:"DISCORD_CHANNELS"`  // Discord channel ID-k
	IntervalS       int      `yaml:"INTERVAL_S"`        // ellenőrzési idő másodpercben
}

type ForumPlugin struct {
	ircBot         *YnMIrC.Client
	discordAdapter MessageSender
	config         *ForumPluginConfig
	db             *sql.DB
}

func LoadForumPluginConfig() (*ForumPluginConfig, error) {
	cfgPath := "./YnMConfig/forum.yaml"
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni a config fájlt (%s): %w", cfgPath, err)
	}
	defer f.Close()

	cfg := &ForumPluginConfig{}
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("YAML feldolgozási hiba: %w", err)
	}

	// Alapértelmezések
	if cfg.DbPath == "" {
		cfg.DbPath = "./data/forum.db"
	}
	if cfg.RssUrl == "" {
		cfg.RssUrl = "https://forum.ynm.hu/latest.rss"
	}
	if len(cfg.IrcChannels) == 0 {
		cfg.IrcChannels = []string{"#Magyar", "YnM"}
	}
	if cfg.IntervalS <= 0 {
		cfg.IntervalS = 300
	}

	return cfg, nil
}

func NewForumPlugin(ircBot *YnMIrC.Client, discordAdapter MessageSender) (*ForumPlugin, error) {
	cfg, err := LoadForumPluginConfig()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(cfg.DbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("nem sikerült létrehozni az adatbázis mappáját: %w", err)
	}

	db, err := sql.Open("sqlite3", cfg.DbPath)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni az adatbázist: %w", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS seen_posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		link TEXT UNIQUE NOT NULL,
		posted_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("hiba a tábla létrehozásakor: %w", err)
	}

	return &ForumPlugin{
		ircBot:         ircBot,
		discordAdapter: discordAdapter,
		config:         cfg,
		db:             db,
	}, nil
}

func (p *ForumPlugin) StartPolling() {
	//log.Printf("[Forum Plugin] Indítás: %d másodperces frissítéssel", p.config.IntervalS)
	//log.Printf("[Forum Plugin] IRC csatornák: %v", p.config.IrcChannels)
	//log.Printf("[Forum Plugin] Discord csatornák: %v", p.config.DiscordChannels)
	
	go func() {
		// Első ellenőrzés azonnal
		p.CheckRSS()
		
		ticker := time.NewTicker(time.Duration(p.config.IntervalS) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			p.CheckRSS()
		}
	}()
}

func (p *ForumPlugin) Name() string {
	return "Forum RSS"
}

func (p *ForumPlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

// CheckRSS lekéri az RSS feedet, és az új bejegyzéseket IRC-re ÉS Discord-ra küldi
func (p *ForumPlugin) CheckRSS() {
	fp := gofeed.NewParser()

	feed, err := fp.ParseURL(p.config.RssUrl)
	if err != nil {
		log.Printf("[Forum Plugin] Nem sikerült lekérni az RSS feedet: %v", err)
		return
	}

	rows, err := p.db.Query("SELECT link FROM seen_posts")
	if err != nil {
		log.Printf("[Forum Plugin] DB lekérdezési hiba: %v", err)
		return
	}
	defer rows.Close()

	seenLinks := make(map[string]struct{})
	for rows.Next() {
		var link string
		if err := rows.Scan(&link); err == nil {
			seenLinks[link] = struct{}{}
		}
	}

	newPosts := 0
	for _, item := range feed.Items {
		if _, exists := seenLinks[item.Link]; exists {
			continue
		}

		_, err := p.db.Exec("INSERT INTO seen_posts (link) VALUES (?)", item.Link)
		if err != nil {
			log.Printf("[Forum Plugin] DB írási hiba: %v", err)
			continue
		}

		published := ""
		if item.PublishedParsed != nil {
			published = item.PublishedParsed.Format("2006-01-02 15:04")
		}

		// IRC üzenet (sima szöveg)
		ircMsg := fmt.Sprintf("📰: %s - Link: %s - Közzétéve: %s", 
			item.Title, item.Link, published)

		// Discord üzenet (formázott)
		discordMsg := fmt.Sprintf("📰 **Új fórum bejegyzés**\n**%s**\n🔗 %s\n📅 %s", 
			item.Title, item.Link, published)

		// Küldés IRC csatornákra
		if p.ircBot != nil {
			for _, ch := range p.config.IrcChannels {
				p.ircBot.SendMessage(ch, ircMsg)
				log.Printf("[Forum→IRC] %s: %s", ch, item.Title)
			}
		}

		// Küldés Discord csatornákra
		if p.discordAdapter != nil {
			for _, channelID := range p.config.DiscordChannels {
				if err := p.discordAdapter.SendMessage(channelID, discordMsg); err != nil {
					log.Printf("[Forum→Discord] Hiba (%s): %v", channelID, err)
				} else {
					log.Printf("[Forum→Discord] %s: %s", channelID, item.Title)
				}
			}
		}

		newPosts++
	}

	if newPosts > 0 {
		log.Printf("[Forum Plugin] %d új bejegyzés feldolgozva és továbbítva", newPosts)
	}
}

func (p *ForumPlugin) Stop() {
	if p.db != nil {
		p.db.Close()
		log.Println("[Forum Plugin] Leállítva")
	}
}

// OnTick - implements the Plugin interface
func (p *ForumPlugin) OnTick() []YnMIrC.Message {
	return nil
}

// HandleMessageRaw - implements the Plugin interface
func (p *ForumPlugin) HandleMessageRaw(msg YnMIrC.Message) string {
	return ""
}