// ============================================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  https://ynm.hu
//  YnM-Go IRC bot plugin: Emlékeztető (!ora) plugin admin jogosultságokkal
// ============================================================================

package plugins

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

	"github.com/ynmhu/YnM-Go/irc"
	"github.com/ynmhu/YnM-Go/config"
	_ "github.com/mattn/go-sqlite3"
)

type OraReminder struct {
	ID        int64
	Nick      string
	Message   string
	RemindAt  time.Time
	CreatedAt time.Time  // Új mező a beállítás időpontjának tárolásához
}

type OraPlugin struct {
	db          *sql.DB
	mutex       sync.Mutex
	timers      map[int64]*time.Timer
	ircClient   *irc.Client
	usageCount  map[string]int       // nick -> hányszor kapott használati útmutatót
	channels    []string             // configból jövő csatornák
	adminPlugin *AdminPlugin         // admin szint ellenőrzéshez
}

func NewOraPlugin(client *irc.Client, admin *AdminPlugin, cfg *config.Config) *OraPlugin {
	p := &OraPlugin{
		timers:      make(map[int64]*time.Timer),
		ircClient:   client,
		channels:    cfg.OraChan,
		adminPlugin: admin,
		usageCount:  make(map[string]int),
	}

	dbPath := cfg.OraDBFile
	if dbPath == "" {
		dbPath = filepath.Join("data", "ora_reminders.db")
	}
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	// Nyisd meg az adatbázist
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(fmt.Errorf("Adatbázis megnyitási hiba: %v", err))
	}
	p.db = db

	// Táblák létrehozása (ha nem léteznek)
	db.Exec(`CREATE TABLE IF NOT EXISTS reminders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nick TEXT NOT NULL,
		message TEXT NOT NULL,
		remind_at TIMESTAMP NOT NULL
	);`)

	// Új oszlopok hozzáadása, ha még nem léteznek
	// Először ellenőrizzük, hogy létezik-e a status oszlop
	var statusExists int
	db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('reminders') WHERE name='status'").Scan(&statusExists)
	if statusExists == 0 {
		db.Exec(`ALTER TABLE reminders ADD COLUMN status TEXT DEFAULT 'active';`)
	}

	// Ellenőrizzük, hogy létezik-e a created_at oszlop
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

func (p *OraPlugin) loadAndSchedule() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Először ellenőrizzük, hogy létezik-e a created_at oszlop
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
			continue // Ha nincs remind_at, akkor ugorjuk át
		}
		
		// Ha a created_at NULL vagy nem létezik az oszlop, akkor a remind_at-et használjuk alapértelmezésként
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

	// Itt használjuk a CreatedAt mezőt a beállítás időpontjának megjelenítésére
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

func (p *OraPlugin) HandleMessage(msg irc.Message) string {
	text := strings.TrimSpace(msg.Text)
	nick := strings.SplitN(msg.Sender, "!", 2)[0]
	channel := msg.Channel
	hostmask := msg.Sender
	level := p.adminPlugin.GetAdminLevel(nick, hostmask)

	switch {
	case text == "!ora":
		if level < 1 || level > 3 {
			return "" //return fmt.Sprintf("@%s Nincs jogosultságod emlékeztetőt hozzáadni.", nick)
		}
		count := p.usageCount[nick]
		if count < 2 {
			p.usageCount[nick] = count + 1
			return fmt.Sprintf("@%s Használat: !ora <idő> <üzenet> (pl:  !ora 1h30m Emlékeztető szöveg  |  !ora 2d Figyelmeztetés | !ora 15m Gyors emlékeztető)", nick)
		}
		return ""

	case strings.HasPrefix(text, "!ora "):
		if level < 1 || level > 3 {
			return "" //return fmt.Sprintf("@%s Nincs jogosultságod emlékeztetőt hozzáadni.", nick)
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

		// Most már a created_at mezőt is mentjük
		res, err := p.db.Exec("INSERT INTO reminders (nick, message, remind_at, created_at) VALUES (?, ?, ?, ?)", 
			nick, message, remindAt.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			return fmt.Sprintf("@%s Hiba az adatbázisba íráskor: %v", nick, err)
		}

		id, _ := res.LastInsertId()
		p.scheduleReminder(OraReminder{
			ID: id, 
			Nick: nick, 
			Message: message, 
			RemindAt: remindAt,
			CreatedAt: now,
		})

		prettyDuration := formatDurationPretty(dur)

		return fmt.Sprintf("@%s Emlékeztető mentve %s múlva, jelez %s-kor.", nick, prettyDuration, remindAt.Format("15:04:05"))

	case text == "!orak":
		if level < 1 || level > 3 {
			return fmt.Sprintf("@%s Nincs jogosultságod emlékeztetők lekérésére.", nick)
		}
		
		// Ellenőrizzük, hogy létezik-e a created_at oszlop
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
			
			var err error
			if createdAtExists > 0 {
				err = rows.Scan(&r.ID, &r.Nick, &r.Message, &remindAtStr, &status, &createdAtStr)
			} else {
				err = rows.Scan(&r.ID, &r.Nick, &r.Message, &remindAtStr, &status)
			}
			
			if err != nil {
				continue // Ha hiba van, ugorjuk át ezt a sort
			}
			
			if !remindAtStr.Valid {
				continue // Ha nincs remind_at, akkor ugorjuk át
			}
			r.RemindAt, _ = time.Parse(time.RFC3339, remindAtStr.String)
			
			// Ha a created_at NULL vagy nem létezik az oszlop, akkor a remind_at-et használjuk alapértelmezésként
			if createdAtExists > 0 && createdAtStr.Valid && createdAtStr.String != "" {
				r.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr.String)
			} else {
				r.CreatedAt = r.RemindAt
			}

			ownerLevel := p.adminPlugin.GetAdminLevel(r.Nick, "") 

			if nick == r.Nick {
				// Saját emlékeztető mindig látható
			} else {
				switch level {
				case 1:
					continue // 1-es szint nem láthat másokat
				case 2:
					if ownerLevel != 1 {
						continue // 2-es szint csak 1-es szintűeket láthat
					}
				case 3:
					// 3-as szint mindent lát
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

	case strings.HasPrefix(text, "!delora"):
		parts := strings.Fields(text)
		if len(parts) != 2 {
			return fmt.Sprintf("@%s Használat: !delora <ID>", nick)
		}

		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return fmt.Sprintf("@%s Hibás ID: %v", nick, err)
		}

		var owner string
		err = p.db.QueryRow("SELECT nick FROM reminders WHERE id = ?", id).Scan(&owner)
		if err == sql.ErrNoRows {
			return fmt.Sprintf("@%s Nincs ilyen ID-jú emlékeztető.", nick)
		} else if err != nil {
			return fmt.Sprintf("@%s Hiba történt: %v", nick, err)
		}

		ownerLevel := p.adminPlugin.GetAdminLevel(owner, "")

		// Törlési szabályok:
		if owner == nick {
			// Sajátját mindenki törölheti, aki 1-3 szintű
			if level < 1 || level > 3 {
				return fmt.Sprintf("@%s Nincs jogosultságod az emlékeztetők törlésére.", nick)
			}
		} else {
			switch level {
			case 1:
				return fmt.Sprintf("@%s Nem jogosult mások emlékeztetőjének törlésére.", nick)
			case 2:
				if ownerLevel != 1 {
					return fmt.Sprintf("@%s Nem jogosult törölni ezt az emlékeztetőt.", nick)
				}
			case 3:
				// 3-as szint törölhet bárkit
			default:
				return fmt.Sprintf("@%s Nincs jogosultságod az emlékeztetők törlésére.", nick)
			}
		}

		res, err := p.db.Exec("DELETE FROM reminders WHERE id = ?", id)
		if err != nil {
			return fmt.Sprintf("@%s Hiba törlés közben: %v", nick, err)
		}

		affected, _ := res.RowsAffected()
		if affected == 0 {
			return fmt.Sprintf("@%s Nincs ilyen ID-jú emlékeztető.", nick)
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

func (p *OraPlugin) OnTick() []irc.Message { return nil }