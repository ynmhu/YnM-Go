// plugins/media_keresek.go
package plugins

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
	"github.com/ynmhu/YnM-Go/irc"
)

func NewMovieRequestPlugin(bot *irc.Client, adminPlugin *AdminPlugin, movieDBPath string) *MovieRequestPlugin {
	plugin := &MovieRequestPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		movieDBPath: movieDBPath,
	}

	if err := plugin.initializeDatabase(); err != nil {
		log.Printf("Failed to initialize movie request plugin database: %v", err)
		return nil
	}

	log.Printf("MovieRequestPlugin initialized successfully")
	return plugin
}

func (p *MovieRequestPlugin) Name() string {
	return "MovieRequestPlugin"
}

func (p *MovieRequestPlugin) Commands() []string {
	return []string{"!keresek"}
}

func (p *MovieRequestPlugin) Help() string {
	return "!keresek - Függőben lévő filmkérések listázása"
}

func (p *MovieRequestPlugin) HandleMessage(msg irc.Message) string {
    if !strings.HasPrefix(msg.Text, "!keresek") {
        return ""
    }

    nick := strings.Split(msg.Sender, "!")[0]
    hostmask := msg.Sender
    level := p.adminPlugin.GetAdminLevel(nick, hostmask)
    if level < 2 {
        return ""
    }

    requests, err := p.getPendingRequests()
    if err != nil {
        log.Printf("[MovieRequestPlugin] Database error: %v", err)
        return fmt.Sprintf("Adatbázis hiba: %v", err)
    }

    if len(requests) == 0 {
        return "Nincs függőben lévő kérés."
    }

    // Küldjük külön üzenetként, hogy minden kérés új sorban legyen
    p.bot.SendMessage(msg.Channel, "Függőben lévő kérések:")
    for _, req := range requests {
        p.bot.SendMessage(msg.Channel, fmt.Sprintf(
            "Kérő: @%s  | Film: %s (%d) - PIN: %s ",
            req.RequestedBy, req.Title, req.Year, req.PIN,
        ))
        time.Sleep(500 * time.Millisecond) // Kis késleltetés, hogy ne floodoljon
    }
    return "" // Mivel már küldtük az üzeneteket
}

func (p *MovieRequestPlugin) getPendingRequests() ([]MovieRequest, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	query := `SELECT pin, requested_by, title, year, upload_date FROM movies WHERE status = 'Nem' ORDER BY upload_date DESC`
	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	var requests []MovieRequest
	for rows.Next() {
		var req MovieRequest
		var uploadDateStr string
		
		err := rows.Scan(&req.PIN, &req.RequestedBy, &req.Title, &req.Year, &uploadDateStr)
		if err != nil {
			log.Printf("[MovieRequestPlugin] Row scan error: %v", err)
			continue
		}

		if uploadDate, err := time.Parse("2006-01-02 15:04:05", uploadDateStr); err == nil {
			req.UploadDate = uploadDate
		}

		requests = append(requests, req)
	}

	return requests, nil
}

func (p *MovieRequestPlugin) initializeDatabase() error {
	var err error
	
	p.db, err = sql.Open("sqlite3", p.movieDBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	if err := p.db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	var count int
	err = p.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='movies'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for movies table: %v", err)
	}
	
	if count == 0 {
		return fmt.Errorf("movies table does not exist")
	}

	return nil
}

func (p *MovieRequestPlugin) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *MovieRequestPlugin) OnTick() []irc.Message {
	return nil
}