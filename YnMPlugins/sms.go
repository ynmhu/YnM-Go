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
)

const (
    MaxMessageLength    = 300
    MaxMessagesPerUser  = 5
)

type SmsPlugin struct {
	bot     *YnMIrC.Client
	db      *sql.DB
	dbMutex sync.RWMutex
}

func NewSmsPlugin(bot *YnMIrC.Client, dbPath string) (*SmsPlugin, error) {
	p := &SmsPlugin{
		bot: bot,
	}
	if err := p.initDB(dbPath); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *SmsPlugin) initDB(dbPath string) error {
    absPath, err := filepath.Abs(dbPath)
    if err != nil {
        return fmt.Errorf("nem sikerült abszolút útvonalat képezni: %w", err)
    }

    dir := filepath.Dir(absPath)

    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("nem sikerült könyvtárat létrehozni: %w", err)
    }

    db, err := sql.Open("sqlite3", absPath+"?_journal_mode=WAL&_sync=NORMAL&_timeout=5000")
    if err != nil {
        return fmt.Errorf("adatbázis megnyitási hiba: %w", err)
    }
    p.db = db

    _, err = p.db.Exec(`
        CREATE TABLE IF NOT EXISTS sms (
            target TEXT NOT NULL,
            sender TEXT NOT NULL,
            message TEXT NOT NULL,
            timestamp DATETIME NOT NULL
        );
        CREATE INDEX IF NOT EXISTS idx_sms_target ON sms(target);
    `)
    return err
}

func (p *SmsPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)

	// SMS parancsok kezelése
	if strings.HasPrefix(text, "!sms ") {
		// !sms help parancs
		if text == "!sms help" {
			return "📧 SMS parancsok: !sms <nick> <üzenet> | !sms list | !sms list detail | !sms clear | !sms help"
		}

		// !sms list parancs
		if text == "!sms list" {
			target := strings.ToLower(msg.Nick)
			count, _ := p.countMessages(target)
			return fmt.Sprintf("📧 %d várakozó üzeneted van.", count)
		}

		// !sms list detail parancs
		if text == "!sms list detail" {
			target := strings.ToLower(msg.Nick)
			return p.getDetailedMessageList(target)
		}

		// !sms clear parancs
		if text == "!sms clear" {
			target := strings.ToLower(msg.Nick)
			p.dbMutex.Lock()
			defer p.dbMutex.Unlock()
			
			result, err := p.db.Exec("DELETE FROM sms WHERE target = ?", target)
			if err != nil {
				return "Hiba az üzenetek törlésekor."
			}
			
			affected, _ := result.RowsAffected()
			return fmt.Sprintf("🗑️ %d üzenet törölve.", affected)
		}

		// !sms <nick> <üzenet> parancs
		parts := strings.SplitN(text, " ", 3)
		if len(parts) < 3 {
			return "Használat: !sms <nick> <üzenet> vagy !sms help"
		}
		
		target := strings.ToLower(parts[1])
		message := parts[2]
		sender := msg.Nick

		// Üzenet hossz ellenőrzés
		if len(message) > MaxMessageLength {
			return fmt.Sprintf("⚠️ Az üzenet túl hosszú! Max %d karakter.", MaxMessageLength)
		}

		// Felhasználónkénti üzenet limit ellenőrzés
		if count, _ := p.countMessages(target); count >= MaxMessagesPerUser {
			return fmt.Sprintf("⚠️ @%s postaládája megtelt! (%d/%d)", parts[1], count, MaxMessagesPerUser)
		}

		if err := p.storeSms(target, sender, message); err != nil {
			return "Hiba az üzenet mentésekor."
		}
		return fmt.Sprintf("✉️ Üzenet elmentve @%s számára.", parts[1])
	}

	// Ha a címzett ír bármit, küldjük vissza az üzenetet, ha van elmentve
	if msg.Nick != "" && text != "" && !strings.HasPrefix(text, "!") {
		target := strings.ToLower(msg.Nick)

		p.dbMutex.Lock()
		defer p.dbMutex.Unlock()

		rows, err := p.db.Query("SELECT sender, message, timestamp FROM sms WHERE target = ?", target)
		if err != nil {
			return ""
		}
		defer rows.Close()

		var responses []string
		for rows.Next() {
			var sender, message, timestampStr string
			if err := rows.Scan(&sender, &message, &timestampStr); err == nil {
				// Először próbáljuk a TimeFormat konstanssal
				if ts, err := time.Parse(TimeFormat, timestampStr); err == nil {
					timeFormatted := ts.Format(DisplayTimeFormat)
					responses = append(responses, fmt.Sprintf("✉️ @%s, @%s üzenetet hagyott: \"%s\" (%s)", msg.Nick, sender, message, timeFormatted))
				} else if ts, err := time.Parse(time.RFC3339, timestampStr); err == nil {
					// Ha az nem megy, próbáljuk RFC3339 formátummal
					timeFormatted := ts.Format(DisplayTimeFormat)
					responses = append(responses, fmt.Sprintf("✉️ @%s, @%s üzenetet hagyott: \"%s\" (%s)", msg.Nick, sender, message, timeFormatted))
				} else {
					// Fallback - időbélyeg nélkül is küldjük el
					responses = append(responses, fmt.Sprintf("✉️ @%s, @%s üzenetet hagyott: \"%s\"", msg.Nick, sender, message))
				}
			}
		}

		if len(responses) > 0 {
			// Üzenetek törlése a küldés után
			_, _ = p.db.Exec("DELETE FROM sms WHERE target = ?", target)
			return strings.Join(responses, " | ")
		}
	}

	return ""
}

func (p *SmsPlugin) countMessages(target string) (int, error) {
    p.dbMutex.RLock()
    defer p.dbMutex.RUnlock()
    
    var count int
    err := p.db.QueryRow("SELECT COUNT(*) FROM sms WHERE target = ?", target).Scan(&count)
    return count, err
}

func (p *SmsPlugin) getDetailedMessageList(target string) string {
    p.dbMutex.RLock()
    defer p.dbMutex.RUnlock()
    
    rows, err := p.db.Query("SELECT sender, message, timestamp FROM sms WHERE target = ? ORDER BY timestamp", target)
    if err != nil {
        return "Hiba az üzenetek lekérésekor."
    }
    defer rows.Close()
    
    var messages []string
    for rows.Next() {
        var sender, message, timestampStr string
        if err := rows.Scan(&sender, &message, &timestampStr); err == nil {
            if ts, err := time.Parse(TimeFormat, timestampStr); err == nil {
                timeFormatted := ts.Format(DisplayTimeFormat)
                preview := message
                if len(preview) > 50 {
                    preview = preview[:50] + "..."
                }
                messages = append(messages, fmt.Sprintf("@%s: \"%s\" (%s)", sender, preview, timeFormatted))
            } else if ts, err := time.Parse(time.RFC3339, timestampStr); err == nil {
                timeFormatted := ts.Format(DisplayTimeFormat)
                preview := message
                if len(preview) > 50 {
                    preview = preview[:50] + "..."
                }
                messages = append(messages, fmt.Sprintf("@%s: \"%s\" (%s)", sender, preview, timeFormatted))
            } else {
                // Fallback - időbélyeg nélkül
                preview := message
                if len(preview) > 50 {
                    preview = preview[:50] + "..."
                }
                messages = append(messages, fmt.Sprintf("@%s: \"%s\" (időbélyeg hiba)", sender, preview))
            }
        }
    }
    
    if len(messages) == 0 {
        return "📧 Nincs várakozó üzeneted."
    }
    
    return fmt.Sprintf("📧 Várakozó üzenetek (%d): %s", len(messages), strings.Join(messages, " | "))
}

func (p *SmsPlugin) storeSms(target, sender, message string) error {
	p.dbMutex.Lock()
	defer p.dbMutex.Unlock()

	_, err := p.db.Exec(
		"INSERT INTO sms (target, sender, message, timestamp) VALUES (?, ?, ?, ?)",
		target, sender, message, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (p *SmsPlugin) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *SmsPlugin) OnTick() []YnMIrC.Message {
	return nil
}