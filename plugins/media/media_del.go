package media

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	_ "github.com/mattn/go-sqlite3"
	"github.com/ynmhu/YnM-Go/irc"
	 "github.com/ynmhu/YnM-Go/plugins/admin"

)

type MovieDeletionPlugin struct {
	bot         *irc.Client
	adminPlugin *admin.AdminPlugin
	db          *sql.DB
	mutex       sync.RWMutex
	movieDBPath string
}

func NewMovieDeletionPlugin(bot *irc.Client, adminPlugin *admin.AdminPlugin, movieDBPath string) *MovieDeletionPlugin {
	plugin := &MovieDeletionPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		movieDBPath: movieDBPath,
	}

	// Initialize database
	if err := plugin.initializeDatabase(); err != nil {
		////log.Printf("Failed to initialize movie deletion plugin database: %v", err)
		return nil
	}

	log.Printf("MovieDeletionPlugin initialized successfully")
	return plugin
}

func (p *MovieDeletionPlugin) Name() string {
	return "MovieDeletionPlugin"
}

func (p *MovieDeletionPlugin) Commands() []string {
	return []string{"!del"}
}

func (p *MovieDeletionPlugin) Help() string {
	return "!del <PIN> - Film törlése PIN alapján"
}

func (p *MovieDeletionPlugin) HandleMessage(msg irc.Message) string {
    // Csak a !del parancsra reagáljunk
    if !strings.HasPrefix(msg.Text, "!del") {
        return ""
    }

    // Admin ellenőrzés
    nick := strings.Split(msg.Sender, "!")[0]
    hostmask := msg.Sender
    level := p.adminPlugin.GetAdminLevel(nick, hostmask)
    if level < 2 {
        return ""
    }

    // Naplózzuk csak a tényleges !del parancsokat
    //log.Printf("[MovieDeletionPlugin] Command received: '%s' from '%s'", msg.Text, msg.Sender)

    // Feldolgozzuk a parancsot
    text := strings.TrimSpace(msg.Text)
    parts := strings.Fields(text)
    
    if len(parts) == 1 { // Csak !del
        return "Használat: !del <PIN>"
    }
    
    if len(parts) == 2 { // !del <PIN>
        pin := parts[1]
        if !isValidPIN(pin) {
            return "Helytelen PIN formátum. 4-6 számjegy szükséges."
        }
        return p.handleMovieDeletion(pin)
    }
    
    return "Túl sok paraméter. Használat: !del <PIN>"
}


func (p *MovieDeletionPlugin) handleMovieDeletion(pin string) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	//log.Printf("[MovieDeletionPlugin] Processing deletion for PIN: %s", pin)

	// Delete movie by PIN
	deleted, err := p.deleteMovieByPIN(pin)
	if err != nil {
		//log.Printf("[MovieDeletionPlugin] Database error: %v", err)
		return fmt.Sprintf("Adatbázis hiba: %v", err)
	}

	if deleted {
		//log.Printf("[MovieDeletionPlugin] Successfully deleted movie with PIN: %s", pin)
		return fmt.Sprintf("%s sikeresen törölve.", pin)
	} else {
		//log.Printf("[MovieDeletionPlugin] Movie not found for PIN: %s", pin)
		return fmt.Sprintf("Nem tudom törölni %s. Lehetséges nem létezik.", pin)
	}
}

func (p *MovieDeletionPlugin) initializeDatabase() error {
	var err error
	
	//log.Printf("[MovieDeletionPlugin] Opening database: %s", p.movieDBPath)
	
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

	//log.Printf("[MovieDeletionPlugin] Database initialized successfully")
	return nil
}

func (p *MovieDeletionPlugin) deleteMovieByPIN(pin string) (bool, error) {
	query := `DELETE FROM movies WHERE pin = ?`
	
	//log.Printf("[MovieDeletionPlugin] Deleting movie with PIN: %s", pin)
	
	result, err := p.db.Exec(query, pin)
	if err != nil {
		return false, fmt.Errorf("failed to delete movie: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %v", err)
	}

	//log.Printf("[MovieDeletionPlugin] Deleted %d row(s)", rowsAffected)
	return rowsAffected > 0, nil
}

func (p *MovieDeletionPlugin) Close() error {
	//log.Printf("[MovieDeletionPlugin] Closing database connection")
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *MovieDeletionPlugin) OnTick() []irc.Message {
	return nil
}