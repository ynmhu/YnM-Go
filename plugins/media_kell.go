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
	bot              *irc.Client
	adminPlugin      *AdminPlugin
	db               *sql.DB
	jellyfinDB       *sql.DB
	lastHeckTime     map[string]time.Time
	movieRequests    []string
	usedPins         map[string]bool
	mutex            sync.RWMutex
	requestInterval  time.Duration
	activeHours      struct {
		start, end int
	}
	requestsChannel  string
	jellyfinDBPath   string
	movieDBPath      string
}


type JellyfinMovie struct {
	Name         string
	CleanName    string
	OriginalTitle string
	RunTimeTicks *int64
	DateCreated  string
	Overview     string
	Type         string
}

func NewMoviePlugin(bot *irc.Client, adminPlugin *AdminPlugin, jellyfinDBPath, movieDBPath, requestsChannel string) *MoviePlugin {
	plugin := &MoviePlugin{
		bot:             bot,
		adminPlugin:     adminPlugin,
		lastHeckTime:    make(map[string]time.Time),
		movieRequests:   make([]string, 0),
		usedPins:        make(map[string]bool),
		requestInterval: 14400 * time.Second, // 4 hours
		activeHours: struct {
			start, end int
		}{7, 22}, // 7 AM to 10 PM
		requestsChannel:  requestsChannel,
		jellyfinDBPath:   jellyfinDBPath,
		movieDBPath:      movieDBPath,
	}

	// Initialize databases
	if err := plugin.initializeDatabases(); err != nil {
		log.Fatalf("Failed to initialize movie plugin databases: %v", err)
	}

	// Load existing PINs
	plugin.loadExistingPINs()

	// Start periodic request posting
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
	// Check for movie request pattern: [username]!kell title year
	 movieRequestRegex := regexp.MustCompile(`^!kell\s+(.+)$`)
	
	if movieRequestRegex.MatchString(msg.Text) {
		return p.handleMovieRequest(msg)
	}

	return ""
}

func (p *MoviePlugin) handleMovieRequest(msg irc.Message) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Parse the request
    movieRequestRegex := regexp.MustCompile(`^!kell\s+(.+)$`)
	matches := movieRequestRegex.FindStringSubmatch(msg.Text)
	
	if len(matches) < 2 {
		return "Usage: !kell Film címe és dátum!"
	}

	requester := strings.Split(msg.Sender, "!")[0]
	details := strings.TrimSpace(matches[1])

	// Split title and year
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

	// Check if movie exists in Jellyfin
	if exists, info := p.checkJellyfinMovie(title); exists {
		response := fmt.Sprintf("'*%s*' már fel van töltve *YnM* *Media* -ra.", title)
		p.bot.SendMessage(msg.Channel, response)
		time.Sleep(1 * time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Cím*: %s", info.Name))
		time.Sleep(1 * time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Feltöltés dátuma*: %s *Lejátszási idő*: %s", 
			p.parseDate(info.DateCreated), p.formatRuntime(info.RunTimeTicks)))
		time.Sleep(1 * time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Áttekintés*: %s", info.Overview))
		return ""
	}

	// Check if movie already requested
	if requested, requester, date := p.isMovieRequestedWithDetails(title); requested {
		return fmt.Sprintf("'%s' Filmet már kérte @%s %s-án/én.", title, requester, date)
	}

	// Generate PIN and add to database
	pin := p.generatePIN()
	if err := p.addMovieToDatabase(title, pin, requester, year); err != nil {
		return fmt.Sprintf("Adatbázis hiba: %v", err)
	}

	// Add to requests queue
	request := fmt.Sprintf("[%s] Cim: '%s' (Évjárat: %d) - PIN: %s", requester, title, year, pin)
	p.movieRequests = append(p.movieRequests, request)
    nick := strings.Split(msg.Sender, "!")[0]
	response := fmt.Sprintf("@%s Cim: '%s' (Évjárat: %d) hozzáadva, PIN: %s.", nick, title, year, pin)
	p.bot.SendMessage(msg.Channel, response)
	time.Sleep(1 * time.Second)
	return "Kérések Listája: https://bot.ynm.hu/media"
}

func (p *MoviePlugin) initializeDatabases() error {
	// Initialize movie requests database
	var err error
	p.db, err = sql.Open("sqlite3", p.movieDBPath)
	if err != nil {
		return fmt.Errorf("failed to open movie database: %v", err)
	}

	// Create movies table if it doesn't exist
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS movies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			pin TEXT NOT NULL UNIQUE,
			upload_date DATETIME DEFAULT CURRENT_TIMESTAMP,
			requested_by TEXT NOT NULL,
			year INTEGER NOT NULL,
			status TEXT DEFAULT 'Nem'
		);
	`
	if _, err := p.db.Exec(createTableQuery); err != nil {
		return fmt.Errorf("failed to create movies table: %v", err)
	}

	// Initialize Jellyfin database connection (read-only)
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

	query := `
		SELECT Name, CleanName, OriginalTitle, RunTimeTicks, DateCreated, Overview, Type
		FROM TypedBaseItems 
		WHERE (type = 'MediaBrowser.Controller.Entities.Movies.Movie' OR 
			   type = 'MediaBrowser.Controller.Entities.TV.Series')
		AND (Name = ? COLLATE NOCASE OR CleanName = ? COLLATE NOCASE OR OriginalTitle = ? COLLATE NOCASE)
	`

	var movie JellyfinMovie
	err := p.jellyfinDB.QueryRow(query, title, title, title).Scan(
		&movie.Name, &movie.CleanName, &movie.OriginalTitle, 
		&movie.RunTimeTicks, &movie.DateCreated, &movie.Overview, &movie.Type,
	)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Error querying Jellyfin database: %v", err)
		}
		return false, JellyfinMovie{}
	}

	return true, movie
}

func (p *MoviePlugin) isMovieRequestedWithDetails(title string) (bool, string, string) {
    var requester string
    var uploadDate string
    
    query := "SELECT requested_by, upload_date FROM movies WHERE title = ? LIMIT 1"
    err := p.db.QueryRow(query, title).Scan(&requester, &uploadDate)
    
    if err != nil {
        if err == sql.ErrNoRows {
            return false, "", ""
        }
        log.Printf("Error checking if movie is requested: %v", err)
        return false, "", ""
    }
    
    // Parse and format the date properly
    formattedDate := p.parseDate(uploadDate)
    
    return true, requester, formattedDate
}
func (p *MoviePlugin) addMovieToDatabase(title, pin, requester string, year int) error {
	query := `
		INSERT INTO movies (title, pin, requested_by, year, status) 
		VALUES (?, ?, ?, ?, 'Nem')
	`
	_, err := p.db.Exec(query, title, pin, requester, year)
	return err
}

func (p *MoviePlugin) parseDate(dateString string) string {
    if dateString == "" {
        return "Ismeretlen dátum"
    }

    // Handle ISO format with Z suffix
    dateString = strings.ReplaceAll(dateString, "Z", "+00:00")
    
    // Try to parse various date formats
    formats := []string{
        "2006-01-02T15:04:05+00:00",
        "2006-01-02T15:04:05.000+00:00",
        "2006-01-02T15:04:05",
        "2006-01-02 15:04:05",
    }

    for _, format := range formats {
        if t, err := time.Parse(format, dateString); err == nil {
            return t.Format("2006-01-02") // Only return date, not time
        }
    }

    return "Ismeretlen dátum"
}

func (p *MoviePlugin) formatRuntime(runtimeTicks *int64) string {
	if runtimeTicks == nil || *runtimeTicks == 0 {
		return "N/A"
	}

	totalSeconds := *runtimeTicks / 10_000_000 // Convert .NET ticks to seconds
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func (p *MoviePlugin) startRequestPosting() {
	ticker := time.NewTicker(p.requestInterval)
	defer ticker.Stop()

	for range ticker.C {
		p.postMovieRequests()
	}
}

func (p *MoviePlugin) postMovieRequests() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	currentHour := time.Now().Hour()
	if currentHour < p.activeHours.start || currentHour >= p.activeHours.end {
		return
	}

	if len(p.movieRequests) == 0 {
		return
	}

	for _, request := range p.movieRequests {
		message := fmt.Sprintf("🚨 @Markus 🚨: %s", request)
		p.bot.SendMessage(p.requestsChannel, message)
	}

	// Clear requests after posting
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
