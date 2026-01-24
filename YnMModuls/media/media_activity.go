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
package media

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	_ "github.com/mattn/go-sqlite3"
)

type MediaActivityPlugin struct {
	bot             *YnMIrC.Client
	cfg             *YnMConfig.MediaActivityConfig
	lastActivityID  int
	lastOnlineTimes map[string]time.Time
	mutex           sync.Mutex
	stopChan chan struct{}
}


func NewMediaActivityPlugin(bot *YnMIrC.Client, cfg *YnMConfig.MediaActivityConfig) *MediaActivityPlugin {
	return &MediaActivityPlugin{
		bot:            bot,
		cfg:           cfg,
		lastOnlineTimes: make(map[string]time.Time),
	}
}
func (p *MediaActivityPlugin) Start() {
    p.stopChan = make(chan struct{})
    go p.run()
}

func (p *MediaActivityPlugin) run() {
    ticker := time.NewTicker(time.Duration(p.cfg.CheckInterval) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if msgs := p.OnTick(); len(msgs) > 0 {
                for _, msg := range msgs {
                    p.bot.SendMessage(msg.Channel, msg.Text)
                }
            }
        case <-p.stopChan:
            return
        }
    }
}

func (p *MediaActivityPlugin) Stop() {
    close(p.stopChan)
}

func (p *MediaActivityPlugin) Name() string {
	return "MediaActivityPlugin"
}

func (p *MediaActivityPlugin) Commands() []string {
	return []string{}
}

func (p *MediaActivityPlugin) Help() string {
	return "Automatikus média aktivitás követés"
}

func (p *MediaActivityPlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

func (p *MediaActivityPlugin) OnTick() []YnMIrC.Message {
	if !p.cfg.Enabled {
		return nil
	}
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.initStatsDB()
	p.loadLastOnlineTimes()
	
	activity, err := p.getLastActivity()
	if err != nil {
		log.Printf("❌ [MediaActivity] Hiba az aktivitás lekérésekor: %v", err)
		return nil
	}
	
	if activity == nil {
		return nil
	}
	
	savedLastActivityID := p.readLastSentActivity()
	if savedLastActivityID != 0 && activity.ID <= savedLastActivityID {
		return nil
	}
	
	p.lastActivityID = activity.ID
	
	// ShortOverview kezelése
	var shortOverview string
	if activity.ShortOverview.Valid {
		shortOverview = activity.ShortOverview.String
	} else {
		// Ha nincs rövid leírás, ne írjunk ki semmit
		shortOverview = ""
	}
	
	dateCreatedStr, dtObj, err := p.parseDateTime(activity.DateCreated)
	if err != nil {
		log.Printf("⚠️ [MediaActivity] Dátum feldolgozás hiba: %v", err)
		dtObj = time.Now()
		dateCreatedStr = dtObj.Format("2006-01-02 15:04:05")
	}
	
	userName := p.extractUsername(activity.Name)
	isLogin := !strings.Contains(activity.Name, "kijelentkezett")
	status := p.getStatusEmoji(isLogin)
	
	p.updateUserStats(userName, dateCreatedStr, activity.Name, isLogin)
	
	// IRC üzenet formázása
	var ircMsg string
	if shortOverview != "" {
		ircMsg = fmt.Sprintf("👤 %s, %s, 🕒: %s, Status: %s", 
			activity.Name, shortOverview, dateCreatedStr, status)
	} else {
		ircMsg = fmt.Sprintf("👤 %s, 🕒: %s, Status: %s", 
			activity.Name, dateCreatedStr, status)
	}
	
	msgs := []YnMIrC.Message{
		{
			Channel: p.cfg.IRCChannel,
			Text:    ircMsg,
		},
	}
	
	// Online értesítés (ha szükséges)
	if isLogin && p.shouldNotifyOnline(userName, dtObj) {
		msgs = append(msgs, YnMIrC.Message{
			Channel: p.cfg.SecondaryChannel,
			Text:    fmt.Sprintf("👤 %s %s 🎥  %s", userName, status, p.cfg.NotificationURL),
		})
		p.lastOnlineTimes[userName] = dtObj
		p.saveLastOnlineTimes()
	}
	
	p.writeLastSentActivity(activity.ID)
	return msgs
}
// Helper methods
func (p *MediaActivityPlugin) getTrackerDBPath() string {
	return filepath.Join(p.cfg.BaseDataDir, "user_stats.db")
}

func (p *MediaActivityPlugin) getFlagFilePath() string {
	return filepath.Join(p.cfg.BaseDataDir, "last_sent_activity.txt")
}

func (p *MediaActivityPlugin) getLastOnlineFile() string {
	return filepath.Join(p.cfg.BaseDataDir, "last_online.json")
}

func (p *MediaActivityPlugin) extractUsername(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) > 0 {
		return parts[0]
	}
	return "Unknown"
}

func (p *MediaActivityPlugin) getStatusEmoji(isLogin bool) string {
	if isLogin {
		return "🟢"
	}
	return "🔴"
}

func (p *MediaActivityPlugin) shouldNotifyOnline(userName string, dt time.Time) bool {
	lastPrintTime, exists := p.lastOnlineTimes[userName]
	cooldown := time.Duration(p.cfg.OnlineCooldown) * time.Hour
	return !exists || time.Since(lastPrintTime) > cooldown
}

// Database methods
// Database methods
func (p *MediaActivityPlugin) initStatsDB() {
    dbPath := p.getTrackerDBPath()
    
    // CSAK READ ONLY módban nyissuk meg a STATS adatbázist is
    db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
    if err != nil {
        log.Printf("Database open error: %v", err)
        return
    }
    defer db.Close()

    // Ellenőrizzük, hogy létezik-e a tábla
    var tableExists bool
    err = db.QueryRow(`
        SELECT COUNT(*) FROM sqlite_master 
        WHERE type='table' AND name='user_activity'
    `).Scan(&tableExists)
    
    if err != nil {
        log.Printf("Table check error: %v", err)
        return
    }
    
    if !tableExists {
        log.Printf("⚠️ [MediaActivity] user_activity tábla nem létezik")
    }
}

func (p *MediaActivityPlugin) loadLastOnlineTimes() {
	data, err := os.ReadFile(p.getLastOnlineFile())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error reading timestamp file: %v", err)
		}
		return
	}

	var times map[string]string
	if err := json.Unmarshal(data, &times); err != nil {
		log.Printf("JSON decode error: %v", err)
		return
	}

	for user, dtStr := range times {
		dt, err := time.Parse(time.RFC3339, dtStr)
		if err != nil {
			log.Printf("Timestamp parse error: %v", err)
			continue
		}
		p.lastOnlineTimes[user] = dt
	}
}

type Activity struct {
	ID           int
	Name         string
	ShortOverview sql.NullString    // <-- Ez a változás
	DateCreated  string
}

func (p *MediaActivityPlugin) getLastActivity() (*Activity, error) {
    // URI formátum használata readonly módhoz
    dbURI := fmt.Sprintf("file:%s?mode=ro&immutable=1", p.cfg.JellyfinDBPath)
    
    db, err := sql.Open("sqlite3", dbURI)
    if err != nil {
        return nil, fmt.Errorf("database open error: %v", err)
    }
    defer db.Close()

    // Kapcsolat ellenőrzése
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("database ping error: %v", err)
    }

    var activity Activity
    err = db.QueryRow(`
        SELECT Id, Name, ShortOverview, DateCreated 
        FROM ActivityLogs 
        ORDER BY Id DESC 
        LIMIT 1`).Scan(&activity.ID, &activity.Name, &activity.ShortOverview, &activity.DateCreated)

    if err != nil {
        if err == sql.ErrNoRows {
            log.Printf("⚠️ [MediaActivity] Nincs aktivitás az adatbázisban")
            return nil, nil
        }
        return nil, fmt.Errorf("query error: %v", err)
    }

    return &activity, nil
}
func (p *MediaActivityPlugin) readLastSentActivity() int {
	data, err := os.ReadFile(p.getFlagFilePath())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error reading activity ID: %v", err)
		}
		return 0
	}

	var savedID int
	_, err = fmt.Sscanf(string(data), "%d", &savedID)
	if err != nil {
		log.Printf("Error parsing activity ID: %v", err)
		return 0
	}

	// Ellenőrizzük, hogy a mentett ID reális-e
	latestActivity, err := p.getLastActivity()
	if err != nil {
		log.Printf("Error checking latest activity: %v", err)
		return savedID // Hibás lekérdezés esetén maradjunk a mentett ID-nél
	}

	if latestActivity != nil && savedID > latestActivity.ID {
		log.Printf("⚠️ [MediaActivity] Mentett ID (%d) nagyobb, mint a legújabb aktivitás ID (%d), resetelés", savedID, latestActivity.ID)
		// Töröljük a fájlt, hogy legközelebb 0-ról induljon
		os.Remove(p.getFlagFilePath())
		return 0
	}

	return savedID
}
func (p *MediaActivityPlugin) parseDateTime(dtStr string) (string, time.Time, error) {
    // First check if the string contains a decimal point
    if idx := strings.Index(dtStr, "."); idx != -1 {
        // Ensure we have enough characters after the decimal point
        // We want to keep 6 digits (microseconds) plus the dot
        requiredLength := idx + 7
        if len(dtStr) < requiredLength {
            // If not enough characters, just take what we have
            dtStr = dtStr[:len(dtStr)]
        } else {
            dtStr = dtStr[:idx+7] // Keep microseconds
        }
    }

    dt, err := time.Parse("2006-01-02 15:04:05.999999", dtStr)
    if err != nil {
        return "", time.Time{}, fmt.Errorf("failed to parse datetime '%s': %v", dtStr, err)
    }

    return dt.Format("2006-01-02 15:04:05"), dt, nil
}

func (p *MediaActivityPlugin) updateUserStats(username, dateStr, fullActivity string, isLogin bool) {
    // CSAK akkor próbáljunk írni, ha a stats DB írható
    dbPath := p.getTrackerDBPath()
    
    // Ellenőrizzük, hogy írható-e a fájl
    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        log.Printf("⚠️ [MediaActivity] Stats DB nem létezik: %s", dbPath)
        return
    }
    
    // Írási jog ellenőrzése
    if err := os.WriteFile(dbPath+".test", []byte("test"), 0644); err != nil {
        log.Printf("⚠️ [MediaActivity] Stats DB nem írható: %s", dbPath)
        os.Remove(dbPath + ".test")
        return
    }
    os.Remove(dbPath + ".test")
    
    var activityDate, loginTime string
    dt, err := time.Parse("2006-01-02 15:04:05", dateStr)
    if err != nil {
        dt = time.Now()
        activityDate = dt.Format("2006-01-02")
        loginTime = dt.Format("15:04:05")
    } else {
        activityDate = dt.Format("2006-01-02")
        loginTime = dt.Format("15:04:05")
    }

    status := "online"
    if !isLogin {
        status = "offline"
    }

    // READ-WRITE módban nyissuk meg a stats DB-t
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        log.Printf("Database open error: %v", err)
        return
    }
    defer db.Close()

    var counter int
    err = db.QueryRow(`
        SELECT counter FROM user_activity 
        WHERE username = ? AND activity_date = ?`, 
        username, activityDate).Scan(&counter)

    if err == nil {
        _, err = db.Exec(`
            UPDATE user_activity 
            SET counter = counter + 1, 
                login_time = ?, 
                status = ?,
                activity_description = ? 
            WHERE username = ? AND activity_date = ?`,
            loginTime, status, fullActivity, username, activityDate)
    } else if err == sql.ErrNoRows {
        _, err = db.Exec(`
            INSERT INTO user_activity 
            (username, activity_date, login_time, status, activity_description, counter)
            VALUES (?, ?, ?, ?, ?, 1)`,
            username, activityDate, loginTime, status, fullActivity)
    }

    if err != nil {
        log.Printf("User stats update error: %v", err)
    }
}

func (p *MediaActivityPlugin) saveLastOnlineTimes() {
	times := make(map[string]string)
	for user, dt := range p.lastOnlineTimes {
		times[user] = dt.Format(time.RFC3339)
	}

	data, err := json.Marshal(times)
	if err != nil {
		log.Printf("JSON encode error: %v", err)
		return
	}

	if err := os.WriteFile(p.getLastOnlineFile(), data, 0644); err != nil {
		log.Printf("Error writing timestamp file: %v", err)
	}
}

func (p *MediaActivityPlugin) writeLastSentActivity(activityID int) {
	if err := os.WriteFile(p.getFlagFilePath(), []byte(fmt.Sprintf("%d", activityID)), 0644); err != nil {
		log.Printf("Error writing activity ID: %v", err)
	}
}