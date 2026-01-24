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
	"regexp"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)


type LinkPluginConfig struct {
	DbPath string `yaml:"URL_DB"` // pl: ./data/url.db
}

type LinkPlugin struct {
	bot      *YnMIrC.Client
	config   *LinkPluginConfig
	db       *sql.DB
	dbMutex  sync.Mutex
	urlRegex *regexp.Regexp
}

func LoadLinkPluginConfig() (*LinkPluginConfig, error) {
	configPath := "./YnMConfig/ynm.yaml"
	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni a config fájlt (%s): %w", configPath, err)
	}
	defer f.Close()

	var config LinkPluginConfig
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("YAML feldolgozási hiba: %w", err)
	}

	if config.DbPath == "" {
		config.DbPath = "./data/url.db"
	}

	return &config, nil
}

func NewLinkPlugin(bot *YnMIrC.Client) (*LinkPlugin, error) {
	cfg, err := LoadLinkPluginConfig()
	if err != nil {
		return nil, err
	}

	// Ellenőrizzük a mappát, ha nem létezik, létrehozzuk
	dir := filepath.Dir(cfg.DbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("nem sikerült létrehozni az adatbázis mappáját: %w", err)
	}

	db, err := sql.Open("sqlite3", cfg.DbPath)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni az adatbázist: %w", err)
	}

	// Tábla létrehozása ha nincs
	createTable := `
	CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel TEXT,
		nick TEXT,
		url TEXT,
		timestamp DATETIME
	);
	`
	if _, err := db.Exec(createTable); err != nil {
		return nil, fmt.Errorf("hiba a tábla létrehozásakor: %w", err)
	}

	// URL mintázat (egyszerű, http(s) linkeket keres)
	urlRegex := regexp.MustCompile(`https?://[^\s<>"]+`)

	return &LinkPlugin{
		bot:      bot,
		config:   cfg,
		db:       db,
		urlRegex: urlRegex,
	}, nil
}

// Üzenet feldolgozás - URL-ek keresése és tárolása
func (p *LinkPlugin) HandleMessage(msg YnMIrC.Message) string {
	urls := p.urlRegex.FindAllString(msg.Text, -1)
	if len(urls) == 0 {
		return ""
	}

	p.dbMutex.Lock()
	defer p.dbMutex.Unlock()

	for _, url := range urls {
		exists := false
		err := p.db.QueryRow("SELECT COUNT(*) FROM links WHERE url = ? AND channel = ?", url, msg.Channel).Scan(&exists)
		if err != nil {
			log.Printf("DB lekérdezési hiba: %v", err)
			continue
		}

		if exists {
			continue // már létezik, nem mentjük újra
		}

		_, err = p.db.Exec("INSERT INTO links (channel, nick, url, timestamp) VALUES (?, ?, ?, ?)",
			msg.Channel, msg.Nick, url, time.Now().Format("2006-01-02 15:04:05"))
		if err != nil {
			log.Printf("DB írási hiba: %v", err)
		}
	}

	return ""
}

// !links parancs kezelése - legutóbbi linkek lekérése és küldése
func (p *LinkPlugin) HandleCommand(command, channel, nick string, args []string) {
	if command != "links" {
		return
	}

	limit := 5
	if len(args) > 0 {
		// próbáljuk int-re konvertálni
		var n int
		_, err := fmt.Sscanf(args[0], "%d", &n)
		if err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}

	p.dbMutex.Lock()
	defer p.dbMutex.Unlock()

	rows, err := p.db.Query("SELECT nick, url, timestamp FROM links WHERE channel = ? ORDER BY timestamp DESC LIMIT ?", channel, limit)
	if err != nil {
		log.Printf("DB lekérdezési hiba: %v", err)
		return
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var nick, url, ts string
		if err := rows.Scan(&nick, &url, &ts); err != nil {
			log.Printf("DB sor beolvasási hiba: %v", err)
			return
		}
		p.bot.SendMessage(channel, fmt.Sprintf("[%s] %s: %s", ts, nick, url))
		found = true
	}

	if !found {
		p.bot.SendMessage(channel, "Nincsenek linkek.")
	}
}

func (p *LinkPlugin) Name() string {
	return "Link Logger"
}

// Kötelező a Plugin interfész metódusok:
func (p *LinkPlugin) OnTick() []YnMIrC.Message { return nil }
func (p *LinkPlugin) HandleMessageRaw(msg YnMIrC.Message) string {
	return p.HandleMessage(msg)
}
