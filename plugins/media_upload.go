package plugins

// Szerzői jog: 2024, YnM Szerkesztette: Markus (markus@ynm.hu) Minden jog fenntartva.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ynmhu/YnM-Go/config"
	"github.com/ynmhu/YnM-Go/irc"
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

var (
	customMessages = map[string]string{
		"/media/f0/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f/f":       "✅ 2022-töl ⭐ 📺",
		"/media/f1/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f2/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f3/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f4/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f5/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f6/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f7/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f8/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f9/f":      "✅ 2022-töl ⭐ 📺",
		"/media/f0/r":      "✅ 2022-ig 📼 📺",
		"/media/f/r":       "✅ 2022-ig 📼 📺",
		"/media/f1/r":      "✅ 2022-ig 📼 📺",
		"/media/f2/r":      "✅ 2022-ig 📼 📺",
		"/media/f3/r":      "✅ 2022-ig 📼 📺",
		"/media/f4/r":      "✅ 2022-ig 📼 📺",
		"/media/f5/r":      "✅ 2022-ig 📼 📺",
		"/media/f6/r":      "✅ 2022-ig 📼 📺",
		"/media/f7/r":      "✅ 2022-ig 📼 📺",
		"/media/f8/r":      "✅ 2022-ig 📼 📺",
		"/media/f9/r":      "✅ 2022-ig 📼 📺",
		"/media/f0/Series": "✅ Sorozatok 🍿 📺",
		"/media/f/Series":  "✅ Sorozatok 🍿 📺",
		"/media/f1/Series": "✅ Sorozatok 🍿 📺",
		"/media/f2/Series": "✅ Sorozatok 🍿 📺",
		"/media/f3/Series": "✅ Sorozatok 🍿 📺",
		"/media/f4/Series": "✅ Sorozatok 🍿 📺",
		"/media/f5/Series": "✅ Sorozatok 🍿 📺",
		"/media/f8/Series": "✅ Sorozatok 🍿 📺",
		"/media/f9/Series": "✅ Sorozatok 🍿 📺",
		"/media/x/Series":  "✅ Sorozatok 🍿 📺",
		"/media/f0/k":      "✅ Kérve 🍿 📺",
		"/media/f1/k":      "✅ Kérve 🍿 📺",
		"/media/f2/k":      "✅ Kérve 🍿 📺",
		"/media/f3/k":      "✅ Kérve 🍿 📺",
		"/media/x/tv":      "✅ TV műsor 📺",
		"/media/f1/c":      "✅ Moziváltozat 📽",
		"/media/f2/c":      "✅ Moziváltozat 📽",
		"/media/f3/c":      "✅ Moziváltozat 📽",
		"/media/f4/c":      "✅ Moziváltozat 📽",
		"/media/f8/c":      "✅ Moziváltozat 📽",
		"/media/f/n":       "✅ Rajzfilmek 📽 🎭",
		"/media/f1/n":      "✅ Rajzfilmek 📽 🎭",
		"/media/f2/n":      "✅ Rajzfilmek 📽 🎭",
		"/media/f3/n":      "✅ Rajzfilmek 📽 🎭",
		"/media/f8/n":      "✅ Rajzfilmek 📽 🎭",
		"/media/f9/n":      "✅ Rajzfilmek 📽 🎭",
		"/media/x/e":       "✅ Rajzfilm Évadok 📽 🎭",
		"/media/f8/e":      "✅ Rajzfilm Évadok 📽 🎭",
		"/media/f9/e":      "✅ Rajzfilm Évadok 📽 🎭",
		"/media/x/n":       "✅ Rajzfilmek 📽 🎭",
		"/media/f0/o":      "✅ Román 📼 📺",
		"/media/f1/m":      "✅ Magyar 📼 📺",
		"/media/f6/m":      "✅ Magyar 📼 📺",
		"/media/x/app":     "✅ Android App 🤖",
		"/media/x":         "✅ XXX 📽 🔞",
		"/media/f8/x":      "✅ XXX 📽 🔞",
		"/media/mp3":       "✅ Mp3 🎵 🎧",
		"/media/f5/i":      "✅ Feliratos filmek 📺",
		"/media/f5/km":     "✅ KabareHu 🎧 🎭",
		"/media/f4/u":      "✅ KabareRo 🎧 🎭",
		"/media/f/d":       "✅ Dokumentum 📽️ 📺",
		"/media/f6/b":      "✅ Könyvek 📰",
	}
)

type MediaUploadPlugin struct {
	bot        *irc.Client
	cfg        *config.Config
	sentDates  []string
	lastDate   string
	ticker     *time.Ticker
	stopChan   chan struct{}
}

func NewMediaUploadPlugin(bot *irc.Client, cfg *config.Config) *MediaUploadPlugin {
	return &MediaUploadPlugin{
		bot:      bot,
		cfg:      cfg,
		stopChan: make(chan struct{}),
	}
}

func (p *MediaUploadPlugin) Name() string {
	return "MediaUpload"
}

func (p *MediaUploadPlugin) HandleMessage(msg irc.Message) string {
	// Ez a plugin nem reagál parancsokra
	return ""
}

func (p *MediaUploadPlugin) Start() error {
	if !p.cfg.MediaUpload.Enabled {
		return nil
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
	close(p.stopChan)
}

func (p *MediaUploadPlugin) loadSentDates() ([]string, error) {
	data, err := os.ReadFile(p.cfg.MediaUpload.SentDatesFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Ha a fájl nem létezik, létrehozzuk üres listával
			_ = p.saveSentDates([]string{})
			return []string{}, nil
		}
		return nil, err
	}
	var dates []string
	err = json.Unmarshal(data, &dates)
	return dates, err
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
	if err != nil || m == nil || m.Overview == "" {
		return
	}

	created := strings.Split(m.DateCreated, ".")[0]
	if p.contains(p.sentDates, created) {
		return
	}

	// Üzenetek küldése
	for _, msg := range p.FormatMediaMessage(m) {
		for _, ch := range p.cfg.MediaUpload.Channels {
			p.bot.SendMessage(ch, msg)
			time.Sleep(1 * time.Second)
		}
	}

	// Dátum hozzáadása a küldött listához
	p.sentDates = append(p.sentDates, created)
	_ = p.saveSentDates(p.sentDates)
	p.lastDate = m.DateCreated
}

func (p *MediaUploadPlugin) getLatestMedia() (*MediaItem, error) {
	db, err := sql.Open("sqlite3", p.cfg.MediaUpload.JellyfinDB)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
		SELECT i.Name, i.Genres, i.Overview, i.RunTimeTicks, i.ProductionYear, i.DateCreated, i.Path,
		CASE
			WHEN i.Type = 'MediaBrowser.Controller.Entities.Movies.Movie' THEN 'Movie'
			WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Series' THEN 'Series'
			WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Episode' THEN 'Episode'
			ELSE 'Other'
		END
		FROM TypedBaseItems i
		WHERE i.Type IN ('MediaBrowser.Controller.Entities.Movies.Movie', 'MediaBrowser.Controller.Entities.TV.Series', 'MediaBrowser.Controller.Entities.TV.Episode')
		ORDER BY i.DateCreated DESC
		LIMIT 1`

	row := db.QueryRow(query)
	var m MediaItem
	if err := row.Scan(&m.Title, &m.Genres, &m.Overview, &m.RuntimeTicks, &m.ProductionYear, &m.DateCreated, &m.Path, &m.MediaType); err != nil {
		return nil, err
	}
	return &m, nil
}

func (p *MediaUploadPlugin) FormatMediaMessage(m *MediaItem) []string {
	parts := strings.Split(m.Path, "/")
	basePath := m.Path
	if len(parts) >= 4 {
		basePath = "/" + strings.Join(parts[1:4], "/")
	}
	custom := customMessages[basePath]

	runtime := ""
	if ticks, err := p.parseRuntimeTicks(m.RuntimeTicks); err == nil {
		runtime = ticks
	}
	overview := m.Overview
	if len(overview) > 350 {
		if idx := strings.LastIndex(overview[:350], "."); idx > 0 {
			overview = overview[:idx+1]
		} else {
			overview = overview[:350]
		}
	}

	created := strings.Split(m.DateCreated, ".")[0]
	mediaLabel := map[string]string{"Movie": "Film", "Series": "Sorozat"}[m.MediaType]

	return []string{
		fmt.Sprintf(" 「 ✦ %s ✦ 」 | 🎭: %s", m.Title, m.Genres),
		fmt.Sprintf("👆: %s | 📂: %s %s", created, custom, mediaLabel),
		fmt.Sprintf("⏰: %s | 📅: %d 🎥", runtime, m.ProductionYear),
		fmt.Sprintf("📝: %s", overview),
	}
}

func (p *MediaUploadPlugin) parseRuntimeTicks(ticks any) (string, error) {
	t, ok := ticks.(int64)
	if !ok {
		return "", fmt.Errorf("invalid ticks")
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


func (p *MediaUploadPlugin) OnTick() []irc.Message {
    return nil
}
