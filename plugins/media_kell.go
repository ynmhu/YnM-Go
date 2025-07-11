package plugins

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ynmhu/YnM-Go/irc"
)

type MoviePlugin struct {
	bot             *irc.Client
	adminPlugin     *AdminPlugin
	db              *sql.DB
	jellyfinDB      *sql.DB
	lastHeckTime    map[string]time.Time
	movieRequests   []string
	usedPins        map[string]bool
	mutex           sync.RWMutex
	requestsChannel string
	jellyfinDBPath  string
	movieDBPath     string
	postTime        string
	postChan        string
	postNick        string
}

type JellyfinMovie struct {
	Name          string
	CleanName     string
	OriginalTitle string
	RunTimeTicks  *int64
	DateCreated   string
	Overview      string
	Type          string
}

func NewMoviePlugin(bot *irc.Client, adminPlugin *AdminPlugin, jellyfinDBPath, movieDBPath, requestsChannel, postTime, postChan, postNick string) *MoviePlugin {
	plugin := &MoviePlugin{
		bot:             bot,
		adminPlugin:     adminPlugin,
		lastHeckTime:    make(map[string]time.Time),
		movieRequests:   make([]string, 0),
		usedPins:        make(map[string]bool),
		requestsChannel: requestsChannel,
		jellyfinDBPath:  jellyfinDBPath,
		movieDBPath:     movieDBPath,
		postTime:        postTime,
		postChan:        postChan,
		postNick:        postNick,
	}

	if err := plugin.initializeDatabases(); err != nil {
		log.Fatalf("Failed to initialize movie plugin databases: %v", err)
	}

	plugin.loadExistingPINs()
	go plugin.startRequestPosting()

	return plugin
}

func (p *MoviePlugin) Name() string {
	return "MoviePlugin"
}

func (p *MoviePlugin) Commands() []string {
	return []string{"!kell"}
}

func (p *MoviePlugin) Help() string {
	return "!kell <film címe> <évjárat> - Film kérés hozzáadása"
}

func (p *MoviePlugin) HandleMessage(msg irc.Message) string {
	movieRequestRegex := regexp.MustCompile(`^!kell\s+(.+)$`)
	if movieRequestRegex.MatchString(msg.Text) {
		return p.handleMovieRequest(msg)
	}
	return ""
}

func (p *MoviePlugin) handleMovieRequest(msg irc.Message) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	movieRequestRegex := regexp.MustCompile(`^!kell\s+(.+)$`)
	matches := movieRequestRegex.FindStringSubmatch(msg.Text)
	if len(matches) < 2 {
		return "Usage: !kell Film címe és dátum!"
	}

	requester := strings.Split(msg.Sender, "!")[0]
	details := strings.TrimSpace(matches[1])
	parts := strings.Split(details, " ")
	if len(parts) < 2 {
		return "Usage: !kell Film címe és dátum!"
	}

	yearStr := parts[len(parts)-1]
	year, err := strconv.Atoi(yearStr)
	if err != nil || len(yearStr) != 4 {
		return "Évjárat pl 1995 vagy 2024."
	}
	title := strings.Join(parts[:len(parts)-1], " ")
	if title == "" {
		return "Usage: !kell Film címe és dátum!"
	}

	if exists, info := p.checkJellyfinMovie(title); exists {
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("'*%s*' már fel van töltve *YnM* *Media* -ra.", title))
		time.Sleep(1 * time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Cím*: %s", info.Name))
		time.Sleep(1 * time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Feltöltés dátuma*: %s *Lejátszási idő*: %s", p.parseDate(info.DateCreated), p.formatRuntime(info.RunTimeTicks)))
		time.Sleep(1 * time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Áttekintés*: %s", info.Overview))
		return ""
	}

	if requested, requester, date := p.isMovieRequestedWithDetails(title); requested {
		return fmt.Sprintf("'%s' Filmet már kérte @%s %s-án/én.", title, requester, date)
	}

	pin := p.generatePIN()
	if err := p.addMovieToDatabase(title, pin, requester, year); err != nil {
		return fmt.Sprintf("Adatbázis hiba: %v", err)
	}

	request := fmt.Sprintf("🎬 @%s új filmet kért: *%s* (📅 %d) – PIN: 🔑 %s", requester, title, year, pin)
	p.movieRequests = append(p.movieRequests, request)
	nick := strings.Split(msg.Sender, "!")[0]
	p.bot.SendMessage(msg.Channel, fmt.Sprintf("@%s Cim: '%s' (Évjárat: %d) hozzáadva, PIN: %s.", nick, title, year, pin))
	time.Sleep(1 * time.Second)
	return "Kérések Listája: https://bot.ynm.hu/media"
}

func (p *MoviePlugin) initializeDatabases() error {
	var err error
	p.db, err = sql.Open("sqlite3", p.movieDBPath)
	if err != nil {
		return fmt.Errorf("failed to open movie database: %v", err)
	}

	createTableQuery := `CREATE TABLE IF NOT EXISTS movies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		pin TEXT NOT NULL UNIQUE,
		upload_date DATETIME DEFAULT CURRENT_TIMESTAMP,
		requested_by TEXT NOT NULL,
		year INTEGER NOT NULL,
		status TEXT DEFAULT 'Nem'
	);`

	if _, err := p.db.Exec(createTableQuery); err != nil {
		return fmt.Errorf("failed to create movies table: %v", err)
	}

	if _, err := os.Stat(p.jellyfinDBPath); err == nil {
		p.jellyfinDB, err = sql.Open("sqlite3", p.jellyfinDBPath+"?mode=ro")
		if err != nil {
			log.Printf("Warning: Failed to open Jellyfin database: %v", err)
		}
	}

	return nil
}

func (p *MoviePlugin) loadExistingPINs() {
	rows, err := p.db.Query("SELECT pin FROM movies")
	if err != nil {
		log.Printf("Error loading existing PINs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var pin string
		if err := rows.Scan(&pin); err != nil {
			log.Printf("Error scanning PIN: %v", err)
			continue
		}
		p.usedPins[pin] = true
	}
}

func (p *MoviePlugin) generatePIN() string {
	for {
		pin := fmt.Sprintf("%05d", rand.Intn(90000)+10000)
		if !p.usedPins[pin] {
			p.usedPins[pin] = true
			return pin
		}
	}
}

func (p *MoviePlugin) checkJellyfinMovie(title string) (bool, JellyfinMovie) {
	if p.jellyfinDB == nil {
		return false, JellyfinMovie{}
	}

	query := `SELECT Name, CleanName, OriginalTitle, RunTimeTicks, DateCreated, Overview, Type
		FROM TypedBaseItems
		WHERE (type = 'MediaBrowser.Controller.Entities.Movies.Movie' OR type = 'MediaBrowser.Controller.Entities.TV.Series')
		AND (Name = ? COLLATE NOCASE OR CleanName = ? COLLATE NOCASE OR OriginalTitle = ? COLLATE NOCASE)`

	var movie JellyfinMovie
	err := p.jellyfinDB.QueryRow(query, title, title, title).Scan(&movie.Name, &movie.CleanName, &movie.OriginalTitle, &movie.RunTimeTicks, &movie.DateCreated, &movie.Overview, &movie.Type)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Error querying Jellyfin database: %v", err)
		}
		return false, JellyfinMovie{}
	}

	return true, movie
}

func (p *MoviePlugin) isMovieRequestedWithDetails(title string) (bool, string, string) {
	var requester, uploadDate string
	err := p.db.QueryRow("SELECT requested_by, upload_date FROM movies WHERE title = ? LIMIT 1", title).Scan(&requester, &uploadDate)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, "", ""
		}
		log.Printf("Error checking if movie is requested: %v", err)
		return false, "", ""
	}
	return true, requester, p.parseDate(uploadDate)
}

func (p *MoviePlugin) addMovieToDatabase(title, pin, requester string, year int) error {
	_, err := p.db.Exec("INSERT INTO movies (title, pin, requested_by, year, status) VALUES (?, ?, ?, ?, 'Nem')", title, pin, requester, year)
	return err
}

func (p *MoviePlugin) parseDate(dateString string) string {
	if dateString == "" {
		return "Ismeretlen dátum"
	}
	dateString = strings.ReplaceAll(dateString, "Z", "+00:00")
	formats := []string{"2006-01-02T15:04:05+00:00", "2006-01-02T15:04:05.000+00:00", "2006-01-02T15:04:05", "2006-01-02 15:04:05"}
	for _, format := range formats {
		if t, err := time.Parse(format, dateString); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return "Ismeretlen dátum"
}

func (p *MoviePlugin) formatRuntime(runtimeTicks *int64) string {
	if runtimeTicks == nil || *runtimeTicks == 0 {
		return "N/A"
	}
	totalSeconds := *runtimeTicks / 10_000_000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func (p *MoviePlugin) startRequestPosting() {
	for {
		now := time.Now()
		scheduledTime, err := time.Parse("15:04", p.postTime)
		if err != nil {
			log.Printf("❌ Hibás post_time formátum: %v", err)
			return
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))
		p.mutex.Lock()
		p.postMovieRequests()
		p.mutex.Unlock()
	}
}

func (p *MoviePlugin) postMovieRequests() {
	if len(p.movieRequests) == 0 {
		return
	}
	for _, request := range p.movieRequests {
		message := fmt.Sprintf("🚨 @%s 🚨: %s", p.postNick, request)
		p.bot.SendMessage(p.postChan, message)
	}
	p.movieRequests = make([]string, 0)
}

func (p *MoviePlugin) Close() error {
	if p.db != nil {
		p.db.Close()
	}
	if p.jellyfinDB != nil {
		p.jellyfinDB.Close()
	}
	return nil
}

func (p *MoviePlugin) OnTick() []irc.Message {
	return nil
}
