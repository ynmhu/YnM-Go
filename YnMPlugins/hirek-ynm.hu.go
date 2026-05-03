// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldal
//
//  Minden jog fenntartva.
//  YnM-Go rendszer része.
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


// ================= CONFIG =================

type HirekPluginConfig struct {
	DbPath          string   `yaml:"DB_PATH"`
	RssUrl          string   `yaml:"RSS_URL"`
	IrcChannels     []string `yaml:"CHANNELS"`
	DiscordChannels []string `yaml:"DISCORD_CHANNELS"`
	IntervalS       int      `yaml:"INTERVAL_S"`
}

// ================= PLUGIN =================

type HirekPlugin struct {
	ircBot         *YnMIrC.Client
	discordAdapter MessageSender
	config         *HirekPluginConfig
	db             *sql.DB
}

// ================= CONFIG LOAD =================

func LoadHirekPluginConfig() (*HirekPluginConfig, error) {
	cfgPath := "./YnMConfig/hirek.yaml"

	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("config hiba: %w", err)
	}
	defer f.Close()

	cfg := &HirekPluginConfig{}
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("YAML hiba: %w", err)
	}

	if cfg.DbPath == "" {
		cfg.DbPath = "./data/hirek.db"
	}
	if cfg.RssUrl == "" {
		cfg.RssUrl = "https://ynm.hu/hirek/feed/"
	}
	if cfg.IntervalS <= 0 {
		cfg.IntervalS = 300
	}

	return cfg, nil
}

// ================= INIT =================

func NewHirekPlugin(ircBot *YnMIrC.Client, discordAdapter MessageSender) (*HirekPlugin, error) {
	cfg, err := LoadHirekPluginConfig()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(cfg.DbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", cfg.DbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS seen_hirek (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		link TEXT UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`)
	if err != nil {
		return nil, err
	}

	return &HirekPlugin{
		ircBot:         ircBot,
		discordAdapter: discordAdapter,
		config:         cfg,
		db:             db,
	}, nil
}

// ================= LOOP =================

func (p *HirekPlugin) seedDB() {
    fp := gofeed.NewParser()
    feed, err := fp.ParseURL(p.config.RssUrl)
    if err != nil {
        log.Println("[Hirek] Seed RSS hiba:", err)
        return
    }
    for _, item := range feed.Items {
        _, _ = p.db.Exec("INSERT OR IGNORE INTO seen_hirek(link) VALUES(?)", item.Link)
    }
    log.Printf("[Hirek] Seed kész: %d elem beírva (küldés nélkül)", len(feed.Items))
}

func (p *HirekPlugin) StartPolling() {
    go func() {
        var count int
        p.db.QueryRow("SELECT COUNT(*) FROM seen_hirek").Scan(&count)

        if count == 0 {
            // Üres DB → várunk a csatlakozásra, aztán küldjük ki
            log.Println("[Hirek] Üres DB, várakozás az IRC csatlakozásra...")
            time.Sleep(60 * time.Second)
            log.Println("[Hirek] Hírek kiküldése...")
            p.CheckRSS()
        } else {
            // Újraindítás → seed, nincs küldés
            log.Printf("[Hirek] %d rekord a DB-ben, seed mód", count)
            p.seedDB()
        }

        ticker := time.NewTicker(time.Duration(p.config.IntervalS) * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            p.CheckRSS()
        }
    }()
}

// ================= RSS CHECK =================

func (p *HirekPlugin) CheckRSS() {
	log.Println("[Hirek] CheckRSS FUT")
	log.Println("[Hirek] RSS URL:", p.config.RssUrl)
	fp := gofeed.NewParser()

	feed, err := fp.ParseURL(p.config.RssUrl)
	if err != nil {
		log.Println("[Hirek] RSS hiba:", err)
		return
	}
	log.Println("[Hirek] feed title:", feed.Title)
	log.Println("[Hirek] feed items:", len(feed.Items))

	rows, err := p.db.Query("SELECT link FROM seen_hirek")
	if err != nil {
		log.Println("[Hirek] DB hiba:", err)
		return
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var l string
		rows.Scan(&l)
		seen[l] = true
	}

	for _, item := range feed.Items {
		log.Println("[Hirek] item:", item.Title)
		if seen[item.Link] {
			log.Println("[Hirek] SKIP (already seen):", item.Title)
			continue
		}

		_, err := p.db.Exec("INSERT INTO seen_hirek(link) VALUES(?)", item.Link)
		if err != nil {
			log.Println("[Hirek] DB insert hiba:", err)
		}

		msg := fmt.Sprintf("📰 HÍR: %s | %s", item.Title, item.Link)

		// IRC küldés
		if p.ircBot != nil {
			for _, ch := range p.config.IrcChannels {
				p.ircBot.SendMessage(ch, msg)
			}
		}

		// Discord küldés
		if p.discordAdapter != nil {
			for _, ch := range p.config.DiscordChannels {
				_ = p.discordAdapter.SendMessage(ch, msg)
			}
		}

		log.Println("[Hirek] Új hír:", item.Title)
	}
}

// ================= STOP =================
func (p *HirekPlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

func (p *HirekPlugin) OnTick() []YnMIrC.Message {
	return nil
}

func (p *HirekPlugin) Stop() {
	if p.db != nil {
		_ = p.db.Close()
	}
}