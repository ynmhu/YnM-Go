package plugins

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ynmhu/YnM-Go/irc"
	_ "github.com/mattn/go-sqlite3"
)

type MediaAjanlatPlugin struct {
	bot           *irc.Client
	dbPath        string
	channel       string
	dailyTime     string
	mutex         sync.Mutex
	dailySchedule *time.Timer
}

func NewMediaAjanlatPlugin(bot *irc.Client, dbPath, channel, dailyTime string) *MediaAjanlatPlugin {
	p := &MediaAjanlatPlugin{
		bot:       bot,
		dbPath:    dbPath,
		channel:   channel,
		dailyTime: dailyTime,
	}

	go p.scheduleDailyRecommendation()
	return p
}

func (p *MediaAjanlatPlugin) Name() string {
	return "MediaAjanlatPlugin"
}

func (p *MediaAjanlatPlugin) Commands() []string {
	return []string{"!film"}
}

func (p *MediaAjanlatPlugin) Help() string {
	return "!film - Véletlenszerű film ajánlása YnM Media adatbázisból"
}

func (p *MediaAjanlatPlugin) HandleMessage(msg irc.Message) string {
	if strings.TrimSpace(msg.Text) == "!film" {
		return p.sendRecommendation(msg.Channel)
	}
	return ""
}

func (p *MediaAjanlatPlugin) scheduleDailyRecommendation() {
	for {
		now := time.Now()
		targetTime, err := time.ParseInLocation("15:04", p.dailyTime, now.Location())
		if err != nil {
			log.Printf("[MediaAjanlatPlugin] Hibás időformátum a konfigurációban: %v", err)
			return
		}

		nextRun := time.Date(now.Year(), now.Month(), now.Day(), targetTime.Hour(), targetTime.Minute(), 0, 0, now.Location())
		if nextRun.Before(now) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		duration := nextRun.Sub(now)
		log.Printf("[MediaAjanlatPlugin] Következő napi ajánlás időzítve: %v", nextRun)

		time.Sleep(duration)

		p.sendRecommendation(p.channel)
	}
}

func (p *MediaAjanlatPlugin) sendRecommendation(channel string) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	db, err := sql.Open("sqlite3", p.dbPath)
	if err != nil {
		log.Printf("[MediaAjanlatPlugin] DB megnyitási hiba: %v", err)
		return "Adatbázis hiba!"
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT Name, CleanName, OriginalTitle, RunTimeTicks, Overview, Path 
		FROM TypedBaseItems 
		WHERE type = 'MediaBrowser.Controller.Entities.Movies.Movie'
		AND (lower(Path) NOT LIKE '%/x/%' AND lower(Path) NOT LIKE '%/xxx/%')
	`)
	if err != nil {
		log.Printf("[MediaAjanlatPlugin] SQL hiba: %v", err)
		return "Lekérdezési hiba!"
	}
	defer rows.Close()

	var movies []Movie
	for rows.Next() {
		var m Movie
		if err := rows.Scan(&m.Name, &m.CleanName, &m.OriginalTitle, &m.RunTimeTicks, &m.Overview, &m.Path); err == nil {
			movies = append(movies, m)
		}
	}

	if len(movies) == 0 {
		p.bot.SendMessage(channel, "Nincs elérhető film az adatbázisban!")
		return ""
	}

	movie := movies[rand.Intn(len(movies))]
	runtimeStr := convertTicksToTime(movie.RunTimeTicks)

	p.bot.SendMessage(channel, fmt.Sprintf("🎬 Napi film ajánlat: %s", movie.OriginalTitle))
	time.Sleep(1 * time.Second)
	p.bot.SendMessage(channel, fmt.Sprintf("*Lejátszási idő*: %s", runtimeStr))
	time.Sleep(1 * time.Second)
	p.bot.SendMessage(channel, fmt.Sprintf("*Áttekintés*: %s", movie.Overview))

	return ""
}

// ─── Segédstruktúra ─────────────────────────────────────────────────
type Movie struct {
	Name          string
	CleanName     string
	OriginalTitle string
	RunTimeTicks  int64
	Overview      string
	Path          string
}

// ─── Tick konvertálás ───────────────────────────────────────────────
func convertTicksToTime(ticks int64) string {
	seconds := ticks / 10_000_000
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
func (p *MediaAjanlatPlugin) OnTick() []irc.Message {
	return nil
}