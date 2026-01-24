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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	_ "github.com/mattn/go-sqlite3"
)

type OraReminder struct {
	ID        int64
	Nick      string
	Message   string
	RemindAt  time.Time
	CreatedAt time.Time
}

type OraPlugin struct {
	db          *sql.DB
	mutex       sync.Mutex
	timers      map[int64]*time.Timer
	ircClient   *YnMIrC.Client
	usageCount  map[string]int
	channels    []string
	adminPlugin *owner.YnmAdminPlugin // Fixed package name
}

func NewOraPlugin(client *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, cfg *YnMConfig.Config) *OraPlugin {
	p := &OraPlugin{
		timers:      make(map[int64]*time.Timer),
		ircClient:   client,
		channels:    cfg.OraChan,
		adminPlugin: adminPlugin,
		usageCount:  make(map[string]int),
	}

	dbPath := cfg.OraDBFile
	if dbPath == "" {
		dbPath = filepath.Join("data", "ora_reminders.db")
	}
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(fmt.Errorf("Adatbázis megnyitási hiba: %v", err))
	}
	p.db = db

	db.Exec(`CREATE TABLE IF NOT EXISTS reminders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nick TEXT NOT NULL,
		message TEXT NOT NULL,
		remind_at TIMESTAMP NOT NULL
	);`)

	var statusExists int
	db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('reminders') WHERE name='status'").Scan(&statusExists)
	if statusExists == 0 {
		db.Exec(`ALTER TABLE reminders ADD COLUMN status TEXT DEFAULT 'active';`)
	}

	var createdAtExists int
	db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('reminders') WHERE name='created_at'").Scan(&createdAtExists)
	if createdAtExists == 0 {
		db.Exec(`ALTER TABLE reminders ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;`)
	}

	p.loadAndSchedule()
	return p
}

func (p *OraPlugin) Name() string { return "OraPlugin" }

var timeRegex = regexp.MustCompile(`(?i)(\d+d)?(\d+h)?(\d+m)?`)

func parseDuration(input string) (time.Duration, error) {
	m := timeRegex.FindStringSubmatch(input)
	if m == nil {
		return 0, fmt.Errorf("Hibás formátum")
	}
	d, h, min := 0, 0, 0
	for _, s := range m[1:] {
		if s == "" {
			continue
		}
		v, _ := strconv.Atoi(s[:len(s)-1])
		switch s[len(s)-1] {
		case 'd', 'D':
			d = v
		case 'h', 'H':
			h = v
		case 'm', 'M':
			min = v
		}
	}
	dur := time.Duration(d)*24*time.Hour + time.Duration(h)*time.Hour + time.Duration(min)*time.Minute
	if dur == 0 {
		return 0, fmt.Errorf("Időtartam nem lehet nulla")
	}
	return dur, nil
}

// Helper function to get user role directly from database
func (p *OraPlugin) getUserRole(nick, hostmask, channel string) string {
	if p.adminPlugin.Db != nil {
		role, err := p.adminPlugin.Db.GetUserGlobalRole(nick, hostmask)
		if err == nil {
			return role
		}
	}
	return ""
}

// Helper function to get admin level based on role
func (p *OraPlugin) getUserAdminLevel(nick, hostmask, channel string) int {
	role := p.getUserRole(nick, hostmask, channel)
	switch role {
	case "owner":
		return 4
	case "admin":
		return 3
	case "mod":
		return 2
	case "vip":
		return 1
	default:
		return 0
	}
}

// Helper function to check if user has minimum required admin level
func (p *OraPlugin) hasMinAdminLevel(nick, hostmask, channel string, minLevel int) bool {
	adminLevel := p.getUserAdminLevel(nick, hostmask, channel)
	return adminLevel >= minLevel
}

func (p *OraPlugin) loadAndSchedule() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	var createdAtExists int
	p.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('reminders') WHERE name='created_at'").Scan(&createdAtExists)

	var query string
	if createdAtExists > 0 {
		query = "SELECT id, nick, message, remind_at, created_at FROM reminders WHERE status = 'active'"
	} else {
		query = "SELECT id, nick, message, remind_at FROM reminders WHERE status = 'active'"
	}

	rows, err := p.db.Query(query)
	if err != nil {
		fmt.Printf("Hiba az emlékeztetők betöltésekor: %v\n", err)
		return
	}
	if rows == nil {
		return
	}
	defer rows.Close()

	now := time.Now()
	for rows.Next() {
		var r OraReminder
		var remindAtStr sql.NullString
		var createdAtStr sql.NullString

		var err error
		if createdAtExists > 0 {
			err = rows.Scan(&r.ID, &r.Nick, &r.Message, &remindAtStr, &createdAtStr)
		} else {
			err = rows.Scan(&r.ID, &r.Nick, &r.Message, &remindAtStr)
		}
		if err != nil {
			fmt.Printf("Hiba az emlékeztető beolvasásakor: %v\n", err)
			continue
		}

		if remindAtStr.Valid {
			r.RemindAt, _ = time.Parse(time.RFC3339, remindAtStr.String)
		} else {
			continue
		}

		if createdAtExists > 0 && createdAtStr.Valid && createdAtStr.String != "" {
			r.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr.String)
		} else {
			r.CreatedAt = r.RemindAt
		}

		if r.RemindAt.Before(now) {
			go p.sendReminder(r)
		} else {
			p.scheduleReminder(r)
		}
	}
}

func (p *OraPlugin) scheduleReminder(r OraReminder) {
	delay := time.Until(r.RemindAt)
	t := time.AfterFunc(delay, func() { p.sendReminder(r) })
	p.timers[r.ID] = t
}

func (p *OraPlugin) sendReminder(r OraReminder) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	msg := fmt.Sprintf("@%s emlékeztető: %s (beállítva: %s)", r.Nick, r.Message, r.CreatedAt.Format("2006-01-02 15:04:05"))

	for _, ch := range p.channels {
		p.ircClient.SendMessage(ch, msg)
	}

	p.db.Exec("UPDATE reminders SET status = 'expired' WHERE id = ?", r.ID)

	if t, ok := p.timers[r.ID]; ok {
		t.Stop()
		delete(p.timers, r.ID)
	}
}

func (p *OraPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	textLower := strings.ToLower(text)
	nick := strings.SplitN(msg.Sender, "!", 2)[0]
	channel := msg.Channel
	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
	prefix := p.adminPlugin.GetPrefixForHost(hostmask)
	effectiveNick, effectiveHostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	//role := YnMModule.GetUserGlobalRoleWithDB(p.adminPlugin.Db, nick, hostmask)

	minLevel := 1

	// Use effective values consistently
	_ = nick // Keep original for backward compatibility if needed
	nick = effectiveNick
	hostmask = effectiveHostmask

		switch {
		case textLower == strings.ToLower(prefix+"ora"):
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, channel, minLevel) {
			return ""
		}
		count := p.usageCount[nick]
		if count < 2 {
			p.usageCount[nick] = count + 1
			return fmt.Sprintf("@%s Használat: !ora <idő> <üzenet> (pl: !ora 1h30m Emlékeztető szöveg | !ora 2d Figyelmeztetés | !ora 15m Gyors emlékeztető)", nick)
		}
		return ""

	case strings.HasPrefix(textLower, strings.ToLower(prefix+"ora ")):
		// Require VIP level or higher (admin level 3 or lower)
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, channel, minLevel) {
			return ""
		}
		parts := strings.Fields(text)
		if len(parts) < 3 {
			return fmt.Sprintf("@%s Használat: !ora <idő> <üzenet>", nick)
		}

		dur, err := parseDuration(parts[1])
		if err != nil {
			return fmt.Sprintf("@%s Hiba: %v", nick, err)
		}

		message := strings.Join(parts[2:], " ")
		now := time.Now()
		remindAt := now.Add(dur)

		res, err := p.db.Exec(
			"INSERT INTO reminders (nick, message, remind_at, created_at) VALUES (?, ?, ?, ?)",
			nick, message, remindAt.Format(time.RFC3339), now.Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Sprintf("@%s Hiba az adatbázisba íráskor: %v", nick, err)
		}

		id, _ := res.LastInsertId()
		p.scheduleReminder(OraReminder{
			ID:        id,
			Nick:      nick,
			Message:   message,
			RemindAt:  remindAt,
			CreatedAt: now,
		})

		prettyDuration := formatDurationPretty(dur)
		return fmt.Sprintf("@%s Emlékeztető mentve 🌐 ->  https://bot.ynm.hu/memo  %s múlva, jelez %s-kor.", nick, prettyDuration, remindAt.Format("15:04:05"))

	case textLower == strings.ToLower(prefix+"orak"):
		// Only bot owner can list all reminders
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, channel, minLevel) {
			return ""
		}

		// Betöltjük, majd lekérdezzük az emlékeztetőket
		var createdAtExists int
		p.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('reminders') WHERE name='created_at'").Scan(&createdAtExists)

		var query string
		if createdAtExists > 0 {
			query = "SELECT id, nick, message, remind_at, status, created_at FROM reminders ORDER BY remind_at ASC"
		} else {
			query = "SELECT id, nick, message, remind_at, status FROM reminders ORDER BY remind_at ASC"
		}

		rows, err := p.db.Query(query)
		if err != nil {
			return fmt.Sprintf("@%s Hiba az adatbázis lekérdezéskor: %v", nick, err)
		}
		if rows == nil {
			return fmt.Sprintf("@%s Nincs emlékeztető.", nick)
		}
		defer rows.Close()

		var lines []string
		now := time.Now()
		for rows.Next() {
			var r OraReminder
			var remindAtStr, status sql.NullString
			var createdAtStr sql.NullString

			if createdAtExists > 0 {
				err = rows.Scan(&r.ID, &r.Nick, &r.Message, &remindAtStr, &status, &createdAtStr)
			} else {
				err = rows.Scan(&r.ID, &r.Nick, &r.Message, &remindAtStr, &status)
			}
			if err != nil || !remindAtStr.Valid {
				continue
			}
			r.RemindAt, _ = time.Parse(time.RFC3339, remindAtStr.String)
			if createdAtExists > 0 && createdAtStr.Valid && createdAtStr.String != "" {
				r.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr.String)
			} else {
				r.CreatedAt = r.RemindAt
			}

		if nick != r.Nick {
			userAdminLevel := p.getUserAdminLevel(nick, hostmask, channel)
			ownerAdminLevel := p.getUserAdminLevel(r.Nick, "", channel)
			
			switch userAdminLevel {
			case 1: // VIP - csak a sajátját látja
				continue
			case 2: // Mod - látja VIP és saját emlékeztetőket
				if ownerAdminLevel > 1 {
					continue
				}
			case 3: // Admin - látja mindenki kivéve owner
				if ownerAdminLevel >= 4 {
					continue
				}
			case 4: // Owner - látja mindenkiét
				// Nem kell continue
			default:
				continue
			}
		}

			dur := r.RemindAt.Sub(now)
			statusText := "Aktív"
			if status.Valid && status.String != "active" {
				statusText = "Lejárt"
			}

			lines = append(lines, fmt.Sprintf("ID:%d - @%s (%s) - Beállítva: %s - Állapot: %s - Üzenet: %s",
				r.ID, r.Nick, dur.Truncate(time.Second), r.CreatedAt.Format("2006-01-02 15:04:05"), statusText, r.Message))
		}

		if len(lines) == 0 {
			return fmt.Sprintf("@%s Nincs emlékeztető.", nick)
		}

		for _, line := range lines {
			p.ircClient.SendMessage(channel, line)
		}
		return ""
	// Fix the delora command condition

	case strings.HasPrefix(textLower, strings.ToLower(prefix+"delora")):

		
		parts := strings.Fields(text)
		if len(parts) != 2 {
			return "" //return fmt.Sprintf("@%s Használat: !delora <ID>", nick)
		}

		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return "" //return fmt.Sprintf("@%s Hibás ID: %v", nick, err)
		}
		
		var ownerNick string
		err = p.db.QueryRow("SELECT nick FROM reminders WHERE id = ?", id).Scan(&ownerNick)
		if err == sql.ErrNoRows {
			return "" //return fmt.Sprintf("@%s Nincs ilyen ID-jú emlékeztető.", nick)
		} else if err != nil {
			return fmt.Sprintf("@%s Hiba történt: %v", nick, err)
		}

		adminLevel := p.getUserAdminLevel(nick, hostmask, channel)
		if adminLevel != 4 { // Only owner (level 4) can delete
			return "" //return fmt.Sprintf("@%s Csak a tulajdonos törölhet emlékeztetőket.", nick)
		}

		_, err = p.db.Exec("DELETE FROM reminders WHERE id = ?", id)
		if err != nil {
			return fmt.Sprintf("@%s Hiba törlés közben: %v", nick, err)
		}

		if t, ok := p.timers[id]; ok {
			t.Stop()
			delete(p.timers, id)
		}

		return fmt.Sprintf("@%s Törölve: %d", nick, id)
	}
	return ""
}

func formatDurationPretty(d time.Duration) string {
	d = d.Round(time.Second)
	seconds := int(d.Seconds())

	if seconds < 60 {
		return fmt.Sprintf("%d másodperc", seconds)
	}
	minutes := seconds / 60
	seconds = seconds % 60
	if minutes < 60 {
		if seconds == 0 {
			return fmt.Sprintf("%d perc", minutes)
		}
		return fmt.Sprintf("%d perc %d másodperc", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	if minutes == 0 {
		return fmt.Sprintf("%d óra", hours)
	}
	return fmt.Sprintf("%d óra %d perc", hours, minutes)
}

func (p *OraPlugin) OnTick() []YnMIrC.Message { return nil }