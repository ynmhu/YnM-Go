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
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	_ "github.com/mattn/go-sqlite3"
)

type LearnPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	db          *sql.DB
	dbfile      string
}

func NewLearnPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, dbfile string) (*LearnPlugin, error) {
	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS learn (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT,
			keyword TEXT,
			definition TEXT,
			modified_at DATETIME,
			UNIQUE(category, keyword)
		)
	`)
	if err != nil {
		return nil, err
	}
	return &LearnPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		db:          db,
		dbfile:      dbfile,
	}, nil
}

func (p *LearnPlugin) Name() string {
	return "LearnPlugin"
}

func (p *LearnPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)

	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
	nick, hostmask := YnMModule.GetEffectiveNickAndHost(p.adminPlugin, msg.Sender)
	role := YnMModule.GetUserGlobalRoleWithDB(p.adminPlugin.Db, nick, hostmask)
	
	if role != "owner" {
		return ""
	}

 
	if strings.HasPrefix(text, "!learn ") {
		
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, 1) {
			return "" //return "❌ Hm."
		}
				
		if msg.Channel == "" || msg.Channel == nick {
			return ""
		}

		parts := strings.Fields(text)
		if len(parts) < 2 {
			return "Usage: !learn <add|del|rep> ..."
		}
		cmd := strings.ToLower(parts[1])

		switch cmd {
		case "add":
			if len(parts) < 5 {
				return "Usage: !learn add <category> <keyword> <definition>"
			}
			category := strings.ToLower(parts[2])
			keyword := strings.ToLower(parts[3])
			definition := strings.Join(parts[4:], " ")
			err := p.addOrUpdateEntry(category, keyword, definition)
			if err != nil {
				return fmt.Sprintf("⚠️ Error saving: %s", err.Error())
			}
			p.bot.SendMessage(msg.Channel, fmt.Sprintf("✅ Learned: [%s] %s = %s", category, keyword, definition))
		case "del":
			if len(parts) != 4 {
				return "Usage: !learn del <category> <keyword>"
			}
			category := strings.ToLower(parts[2])
			keyword := strings.ToLower(parts[3])
			err := p.deleteEntry(category, keyword)
			if err != nil {
				return fmt.Sprintf("⚠️ Error deleting: %s", err.Error())
			}
			p.bot.SendMessage(msg.Channel, fmt.Sprintf("🗑️ Deleted: [%s] %s", category, keyword))
		case "rep":
			if len(parts) < 5 {
				return "Usage: !learn rep <category> <keyword> <new definition>"
			}
			category := strings.ToLower(parts[2])
			keyword := strings.ToLower(parts[3])
			definition := strings.Join(parts[4:], " ")
			err := p.addOrUpdateEntry(category, keyword, definition) // same as add/update
			if err != nil {
				return fmt.Sprintf("⚠️ Error updating: %s", err.Error())
			}
			p.bot.SendMessage(msg.Channel, fmt.Sprintf("✏️ Updated: [%s] %s = %s", category, keyword, definition))
		default:
			return "Usage: !learn <add|del|rep> ..."
		}
		return ""
	}

	if strings.HasPrefix(text, "??") {
		if msg.Channel == "" || msg.Channel == nick {
			return ""
		}
		args := strings.Fields(text)
		if len(args) == 1 {
			// just "??" => list categories
			categories, err := p.listCategories()
			if err != nil {
				return fmt.Sprintf("⚠️ Error listing categories: %s", err.Error())
			}
			if len(categories) == 0 {
				p.bot.SendMessage(msg.Channel, "⚠️ No categories found.")
				return ""
			}
			p.bot.SendMessage(msg.Channel, "📚 Categories: " + strings.Join(categories, ", "))
			return ""
		}

		// ?? something => keyword or category search
		query := strings.ToLower(args[1])

		// Try keyword lookup first
		defs, err := p.getDefinitionsByKeyword(query)
		if err != nil {
			return fmt.Sprintf("⚠️ Error during lookup: %s", err.Error())
		}
		if len(defs) > 0 {
			for _, d := range defs {
				p.bot.SendMessage(msg.Channel, fmt.Sprintf("💡 [%s] %s = %s", d.Category, d.Keyword, d.Definition))
			}
			return ""
		}

		// If no keyword match, try listing keywords by category
		keywords, err := p.listKeywordsByCategory(query)
		if err != nil {
			return fmt.Sprintf("⚠️ Error listing category keywords: %s", err.Error())
		}
		if len(keywords) == 0 {
			p.bot.SendMessage(msg.Channel, fmt.Sprintf("❓ No result for keyword or category: %s", query))
			return ""
		}
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("📚 [%s] keywords: %s", query, strings.Join(keywords, ", ")))
		return ""
	}

	return ""
}

type definitionEntry struct {
	Category   string
	Keyword    string
	Definition string
}

func (p *LearnPlugin) addOrUpdateEntry(category, keyword, definition string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := p.db.Exec(`
		INSERT INTO learn (category, keyword, definition, modified_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(category, keyword) DO UPDATE SET
			definition=excluded.definition,
			modified_at=excluded.modified_at
	`, category, keyword, definition, now)
	return err
}

func (p *LearnPlugin) deleteEntry(category, keyword string) error {
	_, err := p.db.Exec(`DELETE FROM learn WHERE category = ? AND keyword = ?`, category, keyword)
	return err
}

func (p *LearnPlugin) listCategories() ([]string, error) {
	rows, err := p.db.Query(`SELECT DISTINCT category FROM learn ORDER BY category ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (p *LearnPlugin) getDefinitionsByKeyword(keyword string) ([]definitionEntry, error) {
	rows, err := p.db.Query(`SELECT category, keyword, definition FROM learn WHERE keyword = ? ORDER BY modified_at DESC`, keyword)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []definitionEntry{}
	for rows.Next() {
		var d definitionEntry
		if err := rows.Scan(&d.Category, &d.Keyword, &d.Definition); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, nil
}

func (p *LearnPlugin) listKeywordsByCategory(category string) ([]string, error) {
	rows, err := p.db.Query(`SELECT keyword FROM learn WHERE category = ? ORDER BY keyword ASC`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keywords := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keywords = append(keywords, k)
	}
	return keywords, nil
}

func (p *LearnPlugin) OnTick() []YnMIrC.Message {
	return nil
}
