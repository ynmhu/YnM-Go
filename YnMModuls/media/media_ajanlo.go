package media

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	_ "github.com/mattn/go-sqlite3"
)

type MediaAjanlatPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	adminPlugin     *owner.YnmAdminPlugin
	dbPath          string
	channels        []string
	discordChannels []string
	dailyTime       string
	mutex           sync.Mutex
	dailySchedule   *time.Timer
}

// Új konstruktor Discord támogatással
func NewMediaAjanlatPluginWithDiscord(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, dbPath string, channels []string, dailyTime string, discordAdapter *discord.DiscordAdapter) *MediaAjanlatPlugin {
	var ircChannels []string
	var discordChannels []string
	
	//log.Printf("🔍 Media Ajánlat csatornák feldolgozása...")
	
	// Csatornák szétválogatása
	for _, channel := range channels {
		if isDiscordChannelMedia(channel) {
			discordChannels = append(discordChannels, channel)
			//log.Printf("  🎮 Media Discord csatorna: %s", channel)
		} else {
			ircChannels = append(ircChannels, channel)
			//log.Printf("  📡 Media IRC csatorna: %s", channel)
		}
	}
	
	//log.Printf("📊 Media Ajánlat összesítő: %d IRC, %d Discord csatorna", len(ircChannels), len(discordChannels))
	
	p := &MediaAjanlatPlugin{
		bot:             bot,
		discord:         discordAdapter,
		adminPlugin:     adminPlugin,  // ← HOZZÁADNI
		dbPath:          dbPath,
		channels:        ircChannels,
		discordChannels: discordChannels,
		dailyTime:       dailyTime,
	}

	go p.scheduleDailyRecommendation()
	return p
}

// Eredeti konstruktor (backward compatibility)
func NewMediaAjanlatPlugin(bot *YnMIrC.Client, dbPath string, channels []string, dailyTime string) *MediaAjanlatPlugin {
	p := &MediaAjanlatPlugin{
		bot:       bot,
		dbPath:    dbPath,
		channels:  channels,
		dailyTime: dailyTime,
	}

	go p.scheduleDailyRecommendation()
	return p
}

// isDiscordChannelMedia ellenőrzi, hogy Discord csatorna ID-e (csak számok)
func isDiscordChannelMedia(channel string) bool {
	if len(channel) == 0 {
		return false
	}
	for _, char := range channel {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
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

func (p *MediaAjanlatPlugin) HandleMessage(msg YnMIrC.Message) string {
	if strings.TrimSpace(msg.Text) == "!film" {
		// Session támogatással
		var nick, hostmask string
		 if YnMModule.IsServerMessage(msg.Sender) {
			return ""
		}

		if msg.Sender != "" {
			// IRC user - session támogatással
			fullHostmask := msg.Sender
			effectiveUser, effectiveHost := p.adminPlugin.GetEffectiveUser(fullHostmask)
			nick = effectiveUser
			hostmask = effectiveHost
		} else if msg.Nick != "" {
			// Discord user - session támogatással
			userInfo, err := p.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
			if err != nil {
				return "❌ You need to link your Discord account first. Use !register"
			}
			nick = userInfo.Nick
			hostmask = userInfo.Hostmask
			
			// Discord esetén is session ellenőrzés
			effectiveUser, effectiveHost := p.adminPlugin.GetEffectiveUser(userInfo.Hostmask)
			if effectiveUser != userInfo.Nick {
				nick = effectiveUser
				hostmask = effectiveHost
			}
		} else {
			return ""
		}
		
		// VIP jogosultság ellenőrzése (minLevel 3 = VIP)
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, 1) {
			return "" //return "❌ Csak VIP vagy magasabb szintű felhasználók használhatják a !film parancsot."
		}
		
		// Megállapítjuk, hogy ez Discord vagy IRC üzenet
		isDiscord := p.isDiscordMessage(msg.Channel)
		return p.sendRecommendation(msg.Channel, isDiscord)
	}
	return ""
}

// isDiscordMessage megállapítja, hogy a csatorna Discord-e
func (p *MediaAjanlatPlugin) isDiscordMessage(channel string) bool {
	// Ha nincs Discord adapter, akkor biztosan nem Discord
	if p.discord == nil {
		return false
	}
	
	// Ha a csatorna számjegyekből áll (Discord ID), akkor Discord
	if isDiscordChannelMedia(channel) {
		return true
	}
	
	// Ellenőrizzük, hogy a csatorna szerepel-e a Discord csatornák listájában
	for _, discordChannel := range p.discordChannels {
		if channel == discordChannel {
			return true
		}
	}
	
	return false
}

func (p *MediaAjanlatPlugin) scheduleDailyRecommendation() {
	for {
		now := time.Now()
		targetTime, err := time.ParseInLocation("15:04", p.dailyTime, now.Location())
		if err != nil {
			log.Printf("❌ [MediaAjanlatPlugin] Hibás időformátum a konfigurációban: %v", err)

			return
		}

		nextRun := time.Date(now.Year(), now.Month(), now.Day(), targetTime.Hour(), targetTime.Minute(), 0, 0, now.Location())
		if nextRun.Before(now) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		duration := nextRun.Sub(now)
		//log.Printf("⏰ [MediaAjanlatPlugin] Következő napi ajánlás időzítve: %v", nextRun.Format("2006-01-02 15:04:05"))

		time.Sleep(duration)

		// IRC csatornákra küldés
		for _, ch := range p.channels {
			p.sendRecommendation(ch, false)
			time.Sleep(2 * time.Second)
		}
		
		// Discord csatornákra küldés
		if p.discord != nil {
			for _, ch := range p.discordChannels {
				p.sendRecommendation(ch, true)
				time.Sleep(2 * time.Second)
			}
		}
	}
}

func (p *MediaAjanlatPlugin) sendRecommendation(channel string, isDiscord bool) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Log az adatbázis útvonaláról
	//log.Printf("🔍 [MediaAjanlatPlugin] DB útvonal: %s", p.dbPath)

	db, err := sql.Open("sqlite3", "file:"+p.dbPath+"?mode=ro")
	if err != nil {
		//log.Printf("❌ [MediaAjanlatPlugin] DB megnyitási hiba: %v", err)
		return "Adatbázis hiba!"
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT Name, CleanName, OriginalTitle, RunTimeTicks, Overview, Path
FROM BaseItems
WHERE Type = 'MediaBrowser.Controller.Entities.Movies.Movie'
AND (lower(Path) NOT LIKE '%/x/%' AND lower(Path) NOT LIKE '%/xxx/%')
    `)
	if err != nil {
		log.Printf("❌ [MediaAjanlatPlugin] SQL hiba: %v", err)
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
		msg := "Nincs elérhető film az adatbázisban!"
		if isDiscord {
			if p.discord != nil {
				p.discord.SendMessage(channel, msg)
			}
		} else {
			p.bot.SendMessage(channel, msg)
		}
		return ""
	}

	movie := movies[rand.Intn(len(movies))]
	runtimeStr := convertTicksToTime(movie.RunTimeTicks)
	overview := truncateToSentence(movie.Overview, 500)

	// Üzenetek küldése
	messages := []string{
		fmt.Sprintf("🎬 Napi film ajánlat: %s", movie.OriginalTitle),
		fmt.Sprintf("*Lejátszási idő*: %s", runtimeStr),
		fmt.Sprintf("*Áttekintés*: %s", overview),
	}

	if isDiscord {
		if p.discord != nil {
			for _, msg := range messages {
				err := p.discord.SendMessage(channel, msg)
				if err != nil {
					log.Printf("❌ [MediaAjanlatPlugin] Discord küldési hiba (%s): %v", channel, err)
				}
				time.Sleep(1 * time.Second)
			}
			//log.Printf("✅ [MediaAjanlatPlugin] Film ajánlat elküldve Discord %s csatornára: %s", channel, movie.OriginalTitle)
		}
	} else {
		for _, msg := range messages {
			p.bot.SendMessage(channel, msg)
			time.Sleep(1 * time.Second)
		}
		//log.Printf("✅ [MediaAjanlatPlugin] Film ajánlat elküldve IRC %s csatornára: %s", channel, movie.OriginalTitle)
	}

	return ""
}

// Helper function to truncate text to maxLength ending at last complete sentence
func truncateToSentence(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	truncated := text[:maxLength]

	lastDot := strings.LastIndex(truncated, ".")
	lastQuestion := strings.LastIndex(truncated, "?")
	lastExclamation := strings.LastIndex(truncated, "!")

	end := maxLength
	if lastDot > 0 && lastDot > lastQuestion && lastDot > lastExclamation {
		end = lastDot + 1
	} else if lastQuestion > 0 && lastQuestion > lastExclamation {
		end = lastQuestion + 1
	} else if lastExclamation > 0 {
		end = lastExclamation + 1
	}

	if end <= 0 {
		return truncated
	}

	return strings.TrimSpace(truncated[:end])
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

func (p *MediaAjanlatPlugin) OnTick() []YnMIrC.Message {
	return nil
}