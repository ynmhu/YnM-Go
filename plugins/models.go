package plugins

import (
	"database/sql"
	"time"
	"sync"
	"github.com/ynmhu/YnM-Go/irc"
)

// MovieRequest - Filmkérés adatmodell
type MovieRequest struct {
	ID           int
	Title        string
	PIN          string
	RequestedBy  string
	Year         int
	Status       string
	UploadDate   time.Time
	CompletedDate *time.Time
}

// MovieRequestPlugin - A plugin fő struktúrája
type MovieRequestPlugin struct {
	bot         *irc.Client
	adminPlugin *AdminPlugin
	db          *sql.DB
	mutex       sync.RWMutex
	movieDBPath string
}
