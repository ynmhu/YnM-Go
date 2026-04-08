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
	"sync"
	"time"

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

// maxSentPaths caps the in-memory + persisted sent-path list
const maxSentPaths = 2000

// suffixLabels maps path suffixes to display labels
var suffixLabels = map[string]string{
	"f":      "🎞️  Filmek 2023 után",
	"r":      "🎬 2023 előtti filmek",
	"Series": "📺 Sorozatok",
	"k":      "📜 Kérve",
	"c":      "🍿 Moziváltozat",
	"n":      "🧸 Rajzfilmek",
	"e":      "🐰 Rajzfilm Évadok",
	"m":      "🎞️ Filmek Hu 🇭🇺",
	"o":      "🎞️ Filmek Ro 🇷🇴",
	"d":      "🌍 Dokumentumfilmek",
	"h":      "✅ Hangoskönyvek 🎧 📰",
	"app":    "✅ Android App 🤖",
	"mp3":    "🎵 Mp3",
	"i":      "📝 Feliratos Filmek",
	"km":     "✅ KabareHu 🎧 🎭",
	"u":      "✅ KabareRo 🎧 🎭",
	"tv":     "📺 TV-műsorok",
}

var (
	customMessages   map[string]string
	blacklistedPaths []string
)

func init() {
	drives := []string{
		"x",
		"F1", "F2", "F3", "F4", "F5",
		"F6", "F7", "F8", "F9", "F10",
		"F11", "F12", "F13", "F14", "F15",
	}

	customMessages = make(map[string]string, len(drives)*len(suffixLabels))
	for _, drive := range drives {
		for suffix, label := range suffixLabels {
			customMessages["/media/"+drive+"/"+suffix] = label
		}
	}

	// Blacklisted paths: F0–F9 /x és /xm, plus /media/x/x
	blacklistedPaths = []string{"/media/x/x"}
	for i := 0; i <= 9; i++ {
		drive := fmt.Sprintf("F%d", i)
		blacklistedPaths = append(blacklistedPaths,
			"/media/"+drive+"/x",
			"/media/"+drive+"/xm",
		)
	}
}

// Shared SQL query base
const mediaQueryBase = `
	SELECT i.Name,
	       COALESCE(i.Genres, '') as Genres,
	       COALESCE(i.Overview, '') as Overview,
	       COALESCE(i.RunTimeTicks, 0) as RunTimeTicks,
	       COALESCE(i.ProductionYear, 0) as ProductionYear,
	       i.DateCreated,
	       COALESCE(i.Path, '') as Path,
	       CASE
	           WHEN i.Type = 'MediaBrowser.Controller.Entities.Movies.Movie' THEN 'Movie'
	           WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Series'    THEN 'Series'
	           WHEN i.Type = 'MediaBrowser.Controller.Entities.TV.Episode'   THEN 'Episode'
	           ELSE 'Other'
	       END as MediaType
	FROM BaseItems i`

// MediaUploadPlugin watches the Jellyfin DB and announces new media to IRC/Discord.
type MediaUploadPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	cfg             *YnMConfig.Config
	ticker          *time.Ticker
	stopChan        chan struct{}
	pending         map[string]*pendingMedia
	mu              sync.Mutex
	ircChannels     []string
	discordChannels []string

	// sentPaths is the single source of truth for deduplication.
	// Key = full file path; value = true if already sent.
	sentPaths     map[string]bool
	sentPathsList []string // ordered list for capped persistence
}

// NewMediaUploadPluginWithDiscord creates a plugin with both IRC and Discord support.
func NewMediaUploadPluginWithDiscord(bot *YnMIrC.Client, config *YnMConfig.Config, discordAdapter *discord.DiscordAdapter) *MediaUploadPlugin {
	var discordChannels, ircChannels []string
	for _, channel := range config.MediaUpload.Channels {
		if isDiscordChannelMedia(channel) {
			discordChannels = append(discordChannels, channel)
		} else {
			ircChannels = append(ircChannels, channel)
		}
	}
	return &MediaUploadPlugin{
		bot:             bot,
		discord:         discordAdapter,
		cfg:             config,
		ircChannels:     ircChannels,
		discordChannels: discordChannels,
		stopChan:        make(chan struct{}),
		pending:         make(map[string]*pendingMedia),
		sentPaths:       make(map[string]bool),
		sentPathsList:   []string{},
	}
}

// NewMediaUploadPlugin creates an IRC-only plugin (backward compatibility).
func NewMediaUploadPlugin(bot *YnMIrC.Client, cfg *YnMConfig.Config) *MediaUploadPlugin {
	var ircChannels []string
	for _, channel := range cfg.MediaUpload.Channels {
		if !isDiscordChannelMedia(channel) {
			ircChannels = append(ircChannels, channel)
		}
	}
	return &MediaUploadPlugin{
		bot:           bot,
		cfg:           cfg,
		ircChannels:   ircChannels,
		stopChan:      make(chan struct{}),
		pending:       make(map[string]*pendingMedia),
		sentPaths:     make(map[string]bool),
		sentPathsList: []string{},
	}
}

func (p *MediaUploadPlugin) Name() string { return "MediaUpload" }

func (p *MediaUploadPlugin) HandleMessage(msg YnMIrC.Message) string { return "" }

func (p *MediaUploadPlugin) OnTick() []YnMIrC.Message { return nil }

// sentPathsFile returns the path to the persistence file for sent paths.
// Derived automatically from the existing SentDatesFile config key.
func (p *MediaUploadPlugin) sentPathsFile() string {
	return p.cfg.MediaUpload.SentDatesFile + ".paths"
}

func (p *MediaUploadPlugin) Start() error {
	if !p.cfg.MediaUpload.Enabled {
		return nil
	}

	if len(p.ircChannels) == 0 && len(p.discordChannels) == 0 {
		log.Printf("⚠️ FIGYELEM: MediaUpload plugin csatorna lista üres!")
	}

	// Load persisted sent paths and seed the in-memory map.
	paths, err := p.loadSentPaths()
	if err != nil {
		return err
	}
	p.sentPathsList = paths
	for _, path := range paths {
		p.sentPaths[path] = true
	}

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
	default:
		close(p.stopChan)
	}
}

// ─── Persistence ────────────────────────────────────────────────────────────

func (p *MediaUploadPlugin) loadSentPaths() ([]string, error) {
	file := p.sentPathsFile()
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return []string{}, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var paths []string
	if len(data) > 0 {
		if err = json.Unmarshal(data, &paths); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func (p *MediaUploadPlugin) saveSentPaths(paths []string) error {
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	return os.WriteFile(p.sentPathsFile(), data, 0644)
}

// markSent records a path as sent (thread-safe).
// Returns false if the path was already marked, true if newly marked.
// MUST be called without p.mu held.
func (p *MediaUploadPlugin) markSent(path string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sentPaths[path] {
		return false
	}
	p.sentPaths[path] = true
	p.sentPathsList = append(p.sentPathsList, path)

	// Cap the list and remove oldest entries from the map.
	if len(p.sentPathsList) > maxSentPaths {
		excess := p.sentPathsList[:len(p.sentPathsList)-maxSentPaths]
		for _, old := range excess {
			delete(p.sentPaths, old)
		}
		p.sentPathsList = p.sentPathsList[len(p.sentPathsList)-maxSentPaths:]
	}

	_ = p.saveSentPaths(p.sentPathsList)
	return true
}

func (p *MediaUploadPlugin) isSent(path string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sentPaths[path]
}

// ─── Core logic ─────────────────────────────────────────────────────────────

func (p *MediaUploadPlugin) checkAndSendMedia() {
	m, err := p.getLatestMedia()
	if err != nil || m == nil {
		return
	}

	// Fast exit: already sent.
	if p.isSent(m.Path) {
		return
	}

	// Blacklist / adult content filter.
	if strings.Contains(strings.ToLower(m.Title), "xxx") || p.isPathBlacklisted(m.Path) {
		return
	}

	// Has description → send immediately.
	if m.Overview != "" {
		p.sendMedia(m)
		return
	}

	// No description yet → add to pending queue.
	p.mu.Lock()
	if _, exists := p.pending[m.Path]; !exists {
		p.pending[m.Path] = &pendingMedia{item: m, timestamp: time.Now()}
	}
	// Snapshot pending entries so we can process them without holding the lock.
	type snap struct {
		key string
		pm  *pendingMedia
	}
	snapshots := make([]snap, 0, len(p.pending))
	for k, pm := range p.pending {
		snapshots = append(snapshots, snap{k, pm})
	}
	p.mu.Unlock()

	for _, s := range snapshots {
		// Already sent in a previous iteration?
		if p.isSent(s.pm.item.Path) {
			p.mu.Lock()
			delete(p.pending, s.key)
			p.mu.Unlock()
			continue
		}

		// Re-query DB to see if description has appeared.
		latest, err := p.getLatestMediaByPath(s.pm.item.Path)
		if err != nil || latest == nil {
			continue
		}

		if latest.Overview != "" {
			p.sendMedia(latest)
			p.mu.Lock()
			delete(p.pending, s.key)
			p.mu.Unlock()
			continue
		}

		// Timeout: send without description.
		if time.Since(s.pm.timestamp) > 3*time.Minute {
			s.pm.item.Overview = "Nincs elérhető leírás."
			p.sendMedia(s.pm.item)
			p.mu.Lock()
			delete(p.pending, s.key)
			p.mu.Unlock()
		}
	}
}

// sendMedia delivers the item to all configured channels.
// Internally calls markSent — duplicate calls are safe and are silently ignored.
func (p *MediaUploadPlugin) sendMedia(m *MediaItem) {
	// Atomic check-and-mark: if already sent, abort.
	if !p.markSent(m.Path) {
		return
	}

	messages := p.FormatMediaMessage(m)
	for _, msg := range messages {
		for _, ch := range p.ircChannels {
			p.bot.SendMessage(ch, msg)
			time.Sleep(1 * time.Second)
		}
	}

	if p.discord != nil && len(p.discordChannels) > 0 {
		discordMsg := p.FormatDiscordMediaMessage(m)
		for _, ch := range p.discordChannels {
			if err := p.discord.SendMessage(ch, discordMsg); err != nil {
				log.Printf("❌ MediaUpload Discord hiba (%s): %v", ch, err)
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// ─── Database ───────────────────────────────────────────────────────────────

func (p *MediaUploadPlugin) openDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", p.cfg.MediaUpload.JellyfinDB))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}

func scanMediaRow(row *sql.Row) (*MediaItem, error) {
	var m MediaItem
	err := row.Scan(&m.Title, &m.Genres, &m.Overview, &m.RuntimeTicks,
		&m.ProductionYear, &m.DateCreated, &m.Path, &m.MediaType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}
	return &m, nil
}

func (p *MediaUploadPlugin) getLatestMedia() (*MediaItem, error) {
	db, err := p.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	q := mediaQueryBase + `
		WHERE i.Type IN (
		    'MediaBrowser.Controller.Entities.Movies.Movie',
		    'MediaBrowser.Controller.Entities.TV.Series',
		    'MediaBrowser.Controller.Entities.TV.Episode')
		  AND i.DateCreated IS NOT NULL
		ORDER BY i.DateCreated DESC
		LIMIT 1`

	return scanMediaRow(db.QueryRow(q))
}

func (p *MediaUploadPlugin) getLatestMediaByPath(path string) (*MediaItem, error) {
	db, err := p.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	q := mediaQueryBase + " WHERE i.Path = ? LIMIT 1"
	return scanMediaRow(db.QueryRow(q, path))
}

// ─── Path / category helpers ────────────────────────────────────────────────

func (p *MediaUploadPlugin) isPathBlacklisted(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var base string
	if len(parts) >= 3 {
		base = "/" + strings.Join(parts[0:3], "/")
	} else {
		base = path
	}
	for _, bl := range blacklistedPaths {
		if base == bl {
			return true
		}
	}
	return false
}

// basePath extracts the first 3 path components, e.g. /media/F1/f
func basePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 {
		return "/" + strings.Join(parts[0:3], "/")
	}
	return path
}

// categoryLabel returns the display label for a media path.
func categoryLabel(path string) string {
	if label, ok := customMessages[basePath(path)]; ok {
		return label
	}
	return "✅ Media"
}

// ─── Formatting ─────────────────────────────────────────────────────────────

func (p *MediaUploadPlugin) FormatMediaMessage(m *MediaItem) []string {
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
		fmt.Sprintf("👆: %s | 📂: %s ", created, categoryLabel(m.Path)),
		fmt.Sprintf("⏰: %s | 📅: %d 🎥", runtime, m.ProductionYear),
		fmt.Sprintf("📝: %s", overview),
	}
}

func (p *MediaUploadPlugin) FormatDiscordMediaMessage(m *MediaItem) string {
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

	return fmt.Sprintf(
		"**「 ✦ %s ✦ 」**\n🎭 **Műfaj:** %s\n👆 **Feltöltve:** %s | 📂 **Kategória:** %s \n⏰ **Időtartam:** %s | 📅 **Év:** %d\n📝 **Leírás:** %s",
		m.Title, m.Genres, created, categoryLabel(m.Path), runtime, m.ProductionYear, overview,
	)
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

	sec := t / 10_000_000
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s), nil
}

