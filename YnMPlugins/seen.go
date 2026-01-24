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

package ynm

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)
const (
	MaxSearchResults       = 5
	DBTimeout              = 5 * time.Second	
	TimeFormat            = "2006-01-02 15:04:05"
	DisplayTimeFormat     = "2006-01-02 15:04:05"
)



type SeenPlugin struct {
	bot         *YnMIrC.Client
	db          *sql.DB
	dbMutex     sync.RWMutex
	searchTrack map[string][]searchRecord // Tracks who searched for whom and when
	searchMutex sync.RWMutex
	searchNotificationDelay time.Duration
	adminPlugin *owner.YnmAdminPlugin
}

type SeenRecord struct {
	User        string
	LastMessage string
	Timestamp   time.Time
}

type searchRecord struct {
	searcher string
	time     time.Time
} 

func NewSeenPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, dbPath string, delay time.Duration) (*SeenPlugin, error) {
	plugin := &SeenPlugin{
		bot:         bot,
		searchNotificationDelay: delay,
		adminPlugin: adminPlugin,
		searchTrack: make(map[string][]searchRecord),
	}

	if err := plugin.initDB(dbPath); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return plugin, nil
}

func (p *SeenPlugin) initDB(dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_sync=NORMAL&_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	p.db = db

	if err := p.createTable(); err != nil {
		db.Close()
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

func (p *SeenPlugin) createTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS seen (
			user TEXT PRIMARY KEY COLLATE NOCASE,
			last_message TEXT NOT NULL,
			timestamp DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_seen_timestamp ON seen(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_seen_user_lower ON seen(LOWER(user));

		CREATE TABLE IF NOT EXISTS searches (
			target TEXT NOT NULL,
			searcher TEXT NOT NULL,
			timestamp DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_searches_target ON searches(target);
	`
	_, err := p.db.Exec(query)
	return err
}
func (p *SeenPlugin) getUserAdminLevel(nick, hostmask, channel string) int {
	if p.adminPlugin != nil && p.adminPlugin.Db != nil {
		role, err := p.adminPlugin.Db.GetUserRoleInChannel(nick, hostmask, channel)
		if err == nil {
			switch role {
			case "owner":
				return 1
			case "admin":
				return 2
			case "vip":
				return 3
			default:
				return 0
			}
		}
	}
	return 0
}

// hasMinAdminLevel ellenőrzi, hogy a felhasználónak van-e minimum szintű jogosultsága
func (p *SeenPlugin) hasMinAdminLevel(nick, hostmask, channel string, minLevel int) bool {
	adminLevel := p.getUserAdminLevel(nick, hostmask, channel)
	return adminLevel > 0 && adminLevel <= minLevel
}


func (p *SeenPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	senderNick := strings.SplitN(msg.Sender, "!", 2)[0]
	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
	prefix := p.adminPlugin.GetPrefixForHost(hostmask)

	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	
	// ========== BOT ELLENŐRZÉS ==========
	// Bot nick lekérése
	var botNick string
	if p.adminPlugin != nil && p.adminPlugin.Bot != nil {
		botNick = p.adminPlugin.Bot.GetNick()
	}
	if botNick == "" && p.adminPlugin != nil && p.adminPlugin.Cfg != nil {
		botNick = p.adminPlugin.Cfg.NickName
	}
	
	// Csak ha nem bot és valószínűleg valódi felhasználó
	shouldCheckPermission := true
	
	// 1. Bot ellenőrzés
	if botNick != "" && strings.EqualFold(nick, botNick) {
		shouldCheckPermission = false
	}
	
	// 2. Szerver ellenőrzés
	if YnMModule.IsServerHostmask(hostmask) || YnMModule.IsServerHostmask(nick) {
		shouldCheckPermission = false
	}
	
	// 3. Ha nickben van pont, akkor hostnév (nem nick)
	if strings.Contains(nick, ".") {
		shouldCheckPermission = false
	}
	
	// 4. Ha üres a nick
	if strings.TrimSpace(nick) == "" {
		shouldCheckPermission = false
	}
	
	// 5. Ha hostmask tartalmaz "services", "chanserv" stb.
	lowerHost := strings.ToLower(hostmask)
	serverKeywords := []string{"services", "chanserv", "nickserv", "authserv", "irc.server"}
	for _, keyword := range serverKeywords {
		if strings.Contains(lowerHost, keyword) {
			shouldCheckPermission = false
			break
		}
	}
	
	// Csak ha ellenőriznünk kell a jogosultságot
	if shouldCheckPermission {
		minLevel := 1
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
			return ""
		}
	}
	
	// ========== SEEN COMMAND ==========
	if strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix+"seen")) {
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return "Usage: " + prefix + "seen <nick>"
		}
		targetNick := parts[1]

		if err := p.storeSearch(targetNick, senderNick); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to store search: %v\n", err)
		}

		return p.querySeen(targetNick)
	}

	// ========== PASSZÍV MENTÉS & ÉRTESÍTÉS ==========
	// csak csatornás üzenetet mentsünk
	// DE: ne mentsük a bot üzeneteit sem!
	isBotMessage := (botNick != "" && strings.EqualFold(msg.Nick, botNick)) ||
	                strings.Contains(strings.ToLower(msg.Sender), "irc.ynm.hu")
	
	if !isBotMessage && msg.Nick != "" && text != "" && msg.Channel != "" && strings.HasPrefix(msg.Channel, "#") {
		if err := p.storeSeen(msg.Nick, text, msg.Time); err != nil {
			return ""
		}

		originalNick := msg.Nick
		lowerNick := strings.ToLower(msg.Nick)
		var notifications []string
		now := time.Now()

		p.dbMutex.Lock()
		rows, err := p.db.Query(`
			SELECT searcher, timestamp FROM searches
			WHERE target = ?
		`, lowerNick)

		if err == nil {
			for rows.Next() {
				var searcher string
				var timestampStr string
				if err := rows.Scan(&searcher, &timestampStr); err == nil {
					ts, _ := time.Parse(TimeFormat, timestampStr)
					if now.Sub(ts) > p.searchNotificationDelay {
						notifications = append(notifications,
							fmt.Sprintf("🚨 @%s, @%s searched for you (%s)", originalNick, searcher, ts.Format(DisplayTimeFormat)))
					}
				}
			}
			rows.Close()
			p.db.Exec(`DELETE FROM searches WHERE target = ?`, lowerNick)
		}
		p.dbMutex.Unlock()

		if len(notifications) > 0 {
			return strings.Join(notifications, " | ")
		}
	}

	return ""
}


func (p *SeenPlugin) storeSearch(target, searcher string) error {
	p.dbMutex.Lock()
	defer p.dbMutex.Unlock()

	query := `
		INSERT INTO searches (target, searcher, timestamp)
		VALUES (?, ?, ?)
	`
	_, err := p.db.Exec(query, strings.ToLower(target), searcher, time.Now().UTC().Format(TimeFormat))
	return err
}

func (p *SeenPlugin) storeSeen(nick, message string, timestamp time.Time) error {
	p.dbMutex.Lock()
	defer p.dbMutex.Unlock()

	if len(message) > 400 {
		message = message[:397] + "..."
	}

	query := `
		INSERT OR REPLACE INTO seen (user, last_message, timestamp)
		VALUES (?, ?, ?)
	`
	_, err := p.db.Exec(query, nick, message, timestamp.UTC().Format(TimeFormat))
	return err
}

func (p *SeenPlugin) querySeen(nick string) string {
	p.dbMutex.RLock()
	defer p.dbMutex.RUnlock()

	var record SeenRecord
	var timestampStr string

	err := p.db.QueryRow(`
		SELECT user, last_message, timestamp FROM seen
		WHERE LOWER(user) = LOWER(?)
		ORDER BY timestamp DESC
		LIMIT 1
	`, nick).Scan(&record.User, &record.LastMessage, &timestampStr)

	if err != nil {
		return fmt.Sprintf("No data found for '@%s'.", nick)
	}

	storedTime, err := time.Parse(TimeFormat, timestampStr)
	if err != nil {
		storedTime = time.Now().UTC()
	}

	return fmt.Sprintf("%s's last message: '%s' (%s, %s)",
		record.User, record.LastMessage, storedTime.Format(DisplayTimeFormat), p.formatTimeAgo(storedTime))
}

func (p *SeenPlugin) formatTimeAgo(timestamp time.Time) string {
	duration := time.Since(timestamp)

	switch {
	case duration < time.Second:
		return "just now"
	case duration < time.Minute:
		return fmt.Sprintf("%d seconds ago", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	}
}

func (p *SeenPlugin) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *SeenPlugin) OnTick() []YnMIrC.Message {
	return nil
}
