package media

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ynmhu/YnM-Go/irc"
	"github.com/ynmhu/YnM-Go/plugins/admin"
)

type MovieCompletionPlugin struct {
	bot         *irc.Client
	adminPlugin *admin.AdminPlugin
	db          *sql.DB
	mutex       sync.RWMutex
	movieDBPath string
}

func NewMovieCompletionPlugin(bot *irc.Client, adminPlugin *admin.AdminPlugin, movieDBPath string) *MovieCompletionPlugin {
	plugin := &MovieCompletionPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		movieDBPath: movieDBPath,
	}

	// Initialize database
	if err := plugin.initializeDatabase(); err != nil {
		//log.Printf("Failed to initialize movie completion plugin database: %v", err)
		return nil
	}

	//log.Printf("MovieCompletionPlugin initialized successfully")
	return plugin
}

func (p *MovieCompletionPlugin) Name() string {
	return "MovieCompletionPlugin"
}

func (p *MovieCompletionPlugin) Commands() []string {
	return []string{"!ok"}
}

func (p *MovieCompletionPlugin) Help() string {
	return "!ok <PIN> - Film kérés teljesítése"
}

func (p *MovieCompletionPlugin) HandleMessage(msg irc.Message) string {
    // Csak a !ok parancsra reagáljunk
    if !strings.HasPrefix(msg.Text, "!ok") {
        return ""
    }

    // Admin ellenőrzés
    nick := strings.Split(msg.Sender, "!")[0]
    hostmask := msg.Sender
    level := p.adminPlugin.GetAdminLevel(nick, hostmask)
    if level < 2 {
        return ""
    }

    // Naplózzuk csak a tényleges !ok parancsokat
   // log.Printf("[MovieCompletionPlugin] Command received: '%s' from '%s'", msg.Text, msg.Sender)

    // Feldolgozzuk a parancsot
    text := strings.TrimSpace(msg.Text)
    parts := strings.Fields(text)
    
    if len(parts) == 1 { // Csak !ok
        return "Használat: !ok <PIN>"
    }
    
    if len(parts) == 2 { // !ok <PIN>
        pin := parts[1]
        if !isValidPIN(pin) {
            return "Helytelen PIN formátum. 4-6 számjegy szükséges."
        }
        return p.handleMovieCompletion(pin, msg)
    }
    
    return "Túl sok paraméter. Használat: !ok <PIN>"
}

func (p *MovieCompletionPlugin) handleMovieCompletion(pin string, msg irc.Message) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	//log.Printf("[MovieCompletionPlugin] Processing completion for PIN: %s", pin)

	// Get movie details
	movie, err := p.getMovieByPIN(pin)
	if err != nil {
		//log.Printf("[MovieCompletionPlugin] Database error: %v", err)
		return fmt.Sprintf("Adatbázis hiba: %v", err)
	}

	if movie == nil {
		//log.Printf("[MovieCompletionPlugin] Movie not found for PIN: %s", pin)
		return fmt.Sprintf("Nincs film a(z) %s PIN-hez.", pin)
	}

		log.Printf("[MovieCompletionPlugin] Found movie: '%s' by %s, status: %s", 
		movie.Title, movie.RequestedBy, movie.Status)

	// Check if already completed
	if movie.Status == "Igen" {
		return fmt.Sprintf("A(z) %s PIN már teljesítve lett korábban.", pin)
	}

	// Mark as completed
	err = p.markMovieAsCompleted(pin)
	if err != nil {
		//log.Printf("[MovieCompletionPlugin] Error marking as completed: %v", err)
		return fmt.Sprintf("Hiba a teljesítés során: %v", err)
	}

	//log.Printf("[MovieCompletionPlugin] Successfully marked PIN %s as completed", pin)
	
	response := fmt.Sprintf("✅ PIN %s teljesítve! Film: '%s' (%d) - Kérő: @%s - Teljesítve: %s", 
		pin, movie.Title, movie.Year, movie.RequestedBy, time.Now().Format("2006-01-02 15:04:05"))
	
	return response
}

func (p *MovieCompletionPlugin) initializeDatabase() error {
	var err error
	
	//log.Printf("[MovieCompletionPlugin] Opening database: %s", p.movieDBPath)
	
	p.db, err = sql.Open("sqlite3", p.movieDBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test database connection
	if err := p.db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Check if movies table exists
	var count int
	err = p.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='movies'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for movies table: %v", err)
	}
	
	if count == 0 {
		return fmt.Errorf("movies table does not exist")
	}

	// Try to add completed_date column if it doesn't exist
	_, err = p.db.Exec("ALTER TABLE movies ADD COLUMN completed_date DATETIME")
	if err != nil {
		// Column might already exist, check if it's there
		rows, err := p.db.Query("PRAGMA table_info(movies)")
		if err != nil {
			return fmt.Errorf("failed to get table info: %v", err)
		}
		defer rows.Close()
		
		hasCompletedDate := false
		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue sql.NullString
			
			err = rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
			if err != nil {
				continue
			}
			
			if name == "completed_date" {
				hasCompletedDate = true
				break
			}
		}
		
		if !hasCompletedDate {
			return fmt.Errorf("failed to add completed_date column: %v", err)
		}
	}

	//log.Printf("[MovieCompletionPlugin] Database initialized successfully")
	return nil
}

func (p *MovieCompletionPlugin) getMovieByPIN(pin string) (*MovieRequest, error) {
	query := `SELECT id, title, pin, requested_by, year, status, upload_date, completed_date FROM movies WHERE pin = ?`
	
	//log.Printf("[MovieCompletionPlugin] Querying database for PIN: %s", pin)
	
	row := p.db.QueryRow(query, pin)
	
	var movie MovieRequest
	var uploadDateStr string
	var completedDateStr sql.NullString
	
	err := row.Scan(&movie.ID, &movie.Title, &movie.PIN, &movie.RequestedBy, 
		&movie.Year, &movie.Status, &uploadDateStr, &completedDateStr)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Movie not found
		}
		return nil, fmt.Errorf("database scan error: %v", err)
	}

	// Parse upload date
	if uploadDate, err := time.Parse("2006-01-02 15:04:05", uploadDateStr); err == nil {
		movie.UploadDate = uploadDate
	}

	// Parse completed date
	if completedDateStr.Valid && completedDateStr.String != "N/A" {
		if completedDate, err := time.Parse("2006-01-02 15:04:05", completedDateStr.String); err == nil {
			movie.CompletedDate = &completedDate
		}
	}

	return &movie, nil
}

func (p *MovieCompletionPlugin) markMovieAsCompleted(pin string) error {
	// Use Go's time formatting to ensure consistent format
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	query := `UPDATE movies SET status = 'Igen', completed_date = ? WHERE pin = ?`
	
	//log.Printf("[MovieCompletionPlugin] Updating movie status for PIN: %s with completion date: %s", pin, currentTime)
	
	result, err := p.db.Exec(query, currentTime, pin)
	if err != nil {
		return fmt.Errorf("failed to update movie: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no movie found with PIN %s", pin)
	}

	//log.Printf("[MovieCompletionPlugin] Successfully updated %d row(s)", rowsAffected)
	return nil
}

// Add this function to debug what's in the database
func (p *MovieCompletionPlugin) debugMovieRecord(pin string) {
	query := `SELECT id, title, pin, requested_by, year, status, upload_date, completed_date FROM movies WHERE pin = ?`
	
	row := p.db.QueryRow(query, pin)
	
	var id, year int
	var title, pinDB, requestedBy, status, uploadDate string
	var completedDate sql.NullString
	
	err := row.Scan(&id, &title, &pinDB, &requestedBy, &year, &status, &uploadDate, &completedDate)
	if err != nil {
		//log.Printf("[DEBUG] Error scanning row: %v", err)
		return
	}
	
	//log.Printf("[DEBUG] Movie record - ID: %d, Title: %s, PIN: %s, Status: %s", id, title, pinDB, status)
	//log.Printf("[DEBUG] Upload Date: %s", uploadDate)
	//log.Printf("[DEBUG] Completed Date Valid: %v, Value: '%s'", completedDate.Valid, completedDate.String)
}

// Call this in your handleMovieCompletion function after marking as completed:
// p.debugMovieRecord(pin)


func (p *MovieCompletionPlugin) Close() error {
	//log.Printf("[MovieCompletionPlugin] Closing database connection")
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *MovieCompletionPlugin) OnTick() []irc.Message {
	return nil
}

// Helper function to validate PIN format
func isValidPIN(pin string) bool {
	if len(pin) < 4 || len(pin) > 6 {
		return false
	}
	
	// Check if all characters are digits
	for _, char := range pin {
		if char < '0' || char > '9' {
			return false
		}
	}
	
	return true
}