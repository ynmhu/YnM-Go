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
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type MoviePlugin struct {
	bot             *YnMIrC.Client
	adminPlugin     *owner.YnmAdminPlugin
	db              *sql.DB
	jellyfinDB      *sql.DB
	lastHeckTime    map[string]time.Time
	movieRequests   []string
	usedPins        map[string]bool
	mutex           sync.RWMutex
	requestsChannel string
	jellyfinDBPath  string
	movieDBPath     string
	postTime        string
	postChan        []string
	postNick        string
}

type JellyfinMovie struct {
	Name          string
	CleanName     string
	OriginalTitle string
	RunTimeTicks  *int64
	DateCreated   string
	Overview      string
	Type          string
}

func NewMoviePlugin(
	bot *YnMIrC.Client,
	adminPlugin *owner.YnmAdminPlugin,
	jellyfinDBPath string,
	movieDBPath string,
	requestsChannel string,
	postTime string,
	postChan []string,
	postNick string,
) *MoviePlugin {

	p := &MoviePlugin{
		bot:             bot,
		adminPlugin:     adminPlugin,
		lastHeckTime:    make(map[string]time.Time),
		movieRequests:   make([]string, 0),
		usedPins:        make(map[string]bool),
		requestsChannel: requestsChannel,
		jellyfinDBPath:  jellyfinDBPath,
		movieDBPath:     movieDBPath,
		postTime:        postTime,
		postChan:        postChan,
		postNick:        postNick,
	}

	if err := p.initializeDatabases(); err != nil {
		log.Fatalf("Failed to initialize movie plugin databases: %v", err)
	}

	p.loadExistingPINs()
	go p.startRequestPosting()
	return p
}

func (p *MoviePlugin) Name() string { return "MoviePlugin" }

func (p *MoviePlugin) Commands() []string { return []string{"!kell"} }

func (p *MoviePlugin) Help() string {
	return "!kell <film címe> [évjárat] - Film kérés hozzáadása"
}

func (p *MoviePlugin) HandleMessage(msg YnMIrC.Message) string {
	var nick, hostmask string
	
	if msg.Sender != "" {
		fullHostmask := msg.Sender
		effectiveUser, effectiveHost := p.adminPlugin.GetEffectiveUser(fullHostmask)
		nick = effectiveUser
		hostmask = effectiveHost
	} else if msg.Nick != "" {
		userInfo, err := p.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
		if err != nil {
			return "❌ You need to link your Discord account first. Use !register"
		}
		nick = userInfo.Nick
		hostmask = userInfo.Hostmask
		
		effectiveUser, effectiveHost := p.adminPlugin.GetEffectiveUser(userInfo.Hostmask)
		if effectiveUser != userInfo.Nick {
			nick = effectiveUser
			hostmask = effectiveHost
		}
	} else {
		return ""
	}

	prefix := p.adminPlugin.GetPrefixForHost(hostmask)
	minLevel := 0

	if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
		return ""
	}

	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix+"kell")) {
		return ""
	}

	if strings.HasPrefix(msg.Channel, "#") {
		go p.handleMovieRequest(msg)
		return ""
	} else {
		return p.handleMovieRequestForDiscord(msg)
	}
}

func (p *MoviePlugin) handleMovieRequestForDiscord(msg YnMIrC.Message) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	text := strings.TrimSpace(msg.Text)
	parts := strings.Fields(text)
	
	if len(parts) < 2 {
		return "❌ **Használat:** `.kell <Film címe> [évjárat]`\n\n**Példák:**\n• `.kell Harry Potter 2001`\n• `.kell Avatar 2009`\n• `.kell Anna Bella` (évjárat elhagyható bizonyos feltételek mellett)"
	}

	requester := msg.Nick
	details := strings.TrimSpace(strings.TrimPrefix(text, ".kell "))
	parts = strings.Fields(details)

	var year int
	var title string
	
	lastPart := parts[len(parts)-1]
	yearNum, err := strconv.Atoi(lastPart)
	
	if err == nil && len(lastPart) == 4 {
		year = yearNum
		title = strings.Join(parts[:len(parts)-1], " ")
		
		if title == "" {
			return "❌ **Használat:** `.kell <Film címe> [évjárat]`\n\n**Példák:**\n• `.kell Harry Potter 2001`\n• `.kell Avatar 2009`"
		}
	} else {
		year = 0
		title = strings.Join(parts, " ")
		
		wordCount := len(parts)
		titleLength := len(strings.ReplaceAll(title, " ", ""))
		
		if wordCount < 2 {
			return "❌ **Évjárat megadása kötelező!**\n\nA film címének legalább **2 szóból** kell állnia, ha nincs évjárat megadva.\n\n**Használat:** `.kell <Film címe> <évjárat>`\n**Példa:** `.kell Avatar 2009`"
		}
		
		for _, word := range parts {
			if len(word) < 2 {
				return "❌ **Évjárat megadása kötelező!**\n\nMinden szónak legalább **2 karakterből** kell állnia, ha nincs évjárat megadva.\n\n**Használat:** `.kell <Film címe> <évjárat>`\n**Példa:** `.kell Avatar 2009`"
			}
		}
		
		if titleLength < 4 {
			return "❌ **Évjárat megadása kötelező!**\n\nA film címének összesen legalább **4 karakterből** kell állnia, ha nincs évjárat megadva.\n\n**Használat:** `.kell <Film címe> <évjárat>`\n**Példa:** `.kell Avatar 2009`"
		}
	}

	if exists, info := p.checkJellyfinMovie(title); exists {
		return fmt.Sprintf("❌ '*%s*' már fent van az *YnM Media*-n.\n*Cím*: %s\n*Feltöltés dátuma*: %s\n*Lejátszási idő*: %s\n*Áttekintés*: %s", 
			title, info.Name, p.parseDate(info.DateCreated), p.formatRuntime(info.RunTimeTicks), info.Overview)
	}

	if requested, existingRequester, date := p.isMovieRequestedWithDetails(title); requested {
		return fmt.Sprintf("❌ '%s' filmet már kérte <@%s> %s-án/én.", title, existingRequester, date)
	}

	pin := p.generatePIN()
	
	if err := p.addMovieToDatabase(title, pin, requester, year); err != nil {
		return fmt.Sprintf("❌ Adatbázis hiba: %v", err)
	}

	var request string
	if year == 0 {
		request = fmt.Sprintf("🎬 <@%s> új filmet kért: *%s* (📅 évjárat nincs megadva) – PIN: 🔑 %s", requester, title, pin)
	} else {
		request = fmt.Sprintf("🎬 <@%s> új filmet kért: *%s* (📅 %d) – PIN: 🔑 %s", requester, title, year, pin)
	}
	p.movieRequests = append(p.movieRequests, request)
	
	var yearDisplay string
	if year == 0 {
		yearDisplay = "nincs megadva (0000)"
	} else {
		yearDisplay = fmt.Sprintf("%d", year)
	}
	
	return fmt.Sprintf("✅ **Film kérés sikeresen rögzítve!**\n\n**📝 Részletek:**\n• **Cím:** %s\n• **Évjárat:** %s\n• **PIN kód:** %s\n• **Felkérte:** <@%s>\n\nKérések listája: https://bot.ynm.hu/media", 
		title, yearDisplay, pin, requester)
}

func (p *MoviePlugin) handleMovieRequest(msg YnMIrC.Message) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	hostmask := YnMModule.SimplifyHostmask(msg.Sender)
	prefix := p.adminPlugin.GetPrefixForHost(hostmask)
	text := strings.TrimSpace(msg.Text)
	
	//log.Printf("🔍 DEBUG IRC - Text: '%s' | Prefix: '%s'", text, prefix)
	
	if strings.ToLower(text) == strings.ToLower(prefix+"kell") {
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("Használat: %skell <Film címe> [évjárat] | Példa: %skell Harry Potter 2001 vagy %skell Anna Bella", prefix, prefix, prefix))
		return ""
	}
	
	movieRequestRegex := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(prefix) + `kell\s+(.+)$`)
	matches := movieRequestRegex.FindStringSubmatch(text)
	
	//log.Printf("🔍 REGEX - matches: %v", matches)
	
	if len(matches) < 2 {
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("Használat: %skell <Film címe> [évjárat] | Példa: %skell Harry Potter 2001", prefix, prefix))
		return ""
	}

	requester := strings.Split(msg.Sender, "!")[0]
	details := strings.TrimSpace(matches[1])
	parts := strings.Fields(details)
	
	//log.Printf("🔍 PARTS - details: '%s' | parts: %v | len: %d", details, parts, len(parts))
	
	if len(parts) == 0 {
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("Használat: %skell <Film címe> [évjárat]", prefix))
		return ""
	}

	var year int
	var title string
	lastPart := parts[len(parts)-1]
	yearNum, err := strconv.Atoi(lastPart)
	
	if err == nil && len(lastPart) == 4 {
		year = yearNum
		title = strings.Join(parts[:len(parts)-1], " ")
		
		if title == "" {
			p.bot.SendMessage(msg.Channel, fmt.Sprintf("Használat: %skell <Film címe> [évjárat]", prefix))
			return ""
		}
	} else {
		year = 0
		title = strings.Join(parts, " ")
		wordCount := len(parts)
		titleLength := len(strings.ReplaceAll(title, " ", ""))
		
		//log.Printf("🔍 VALIDATION - wordCount: %d | titleLength: %d | title: '%s'", wordCount, titleLength, title)
		
		if wordCount < 2 {
			p.bot.SendMessage(msg.Channel, fmt.Sprintf("Évjárat megadása kötelező! A film címének legalább 2 szóból kell állnia. Használat: %skell <Film címe> <évjárat>", prefix))
			return ""
		}
		
		for _, word := range parts {
			if len(word) < 2 {
				p.bot.SendMessage(msg.Channel, fmt.Sprintf("Évjárat megadása kötelező! Minden szónak legalább 2 karakterből kell állnia. Használat: %skell <Film címe> <évjárat>", prefix, prefix))
				return ""
			}
		}
		
		if titleLength < 4 {
			p.bot.SendMessage(msg.Channel, fmt.Sprintf("Évjárat megadása kötelező! A film címének összesen legalább 4 karakterből kell állnia. Használat: %skell <Film címe> <évjárat>", prefix, prefix))
			return ""
		}
	}

	if exists, info := p.checkJellyfinMovie(title); exists {
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("'*%s*' már fent van az *YnM Media*-n.", title))
		time.Sleep(time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Cím*: %s", info.Name))
		time.Sleep(time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Feltöltés dátuma*: %s *Lejátszási idő*: %s", p.parseDate(info.DateCreated), p.formatRuntime(info.RunTimeTicks)))
		time.Sleep(time.Second)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("*Áttekintés*: %s", info.Overview))
		return ""
	}

	if requested, requester, date := p.isMovieRequestedWithDetails(title); requested {
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("'%s' filmet már kérte @%s %s-án/én.", title, requester, date))
		return ""
	}

	pin := p.generatePIN()
	if err := p.addMovieToDatabase(title, pin, requester, year); err != nil {
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("Adatbázis hiba: %v", err))
		return ""
	}

	var request string
	if year == 0 {
		request = fmt.Sprintf("🎬 @%s új filmet kért: *%s* (📅 évjárat nincs megadva) – PIN: 🔑 %s", requester, title, pin)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("@%s Cím: '%s' (Évjárat: nincs megadva) hozzáadva, PIN: %s.", requester, title, pin))
	} else {
		request = fmt.Sprintf("🎬 @%s új filmet kért: *%s* (📅 %d) – PIN: 🔑 %s", requester, title, year, pin)
		p.bot.SendMessage(msg.Channel, fmt.Sprintf("@%s Cím: '%s' (Évjárat: %d) hozzáadva, PIN: %s.", requester, title, year, pin))
	}
	
	p.movieRequests = append(p.movieRequests, request)
	time.Sleep(time.Second)
	p.bot.SendMessage(msg.Channel, "Kérések listája: https://bot.ynm.hu/media")
	return ""
}

func (p *MoviePlugin) initializeDatabases() error {
	var err error
	p.db, err = sql.Open("sqlite3", p.movieDBPath)
	if err != nil {
		return fmt.Errorf("failed to open movie database: %v", err)
	}
	createTable := `CREATE TABLE IF NOT EXISTS movies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		pin TEXT NOT NULL UNIQUE,
		upload_date DATETIME DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now', 'localtime')),
		requested_by TEXT NOT NULL,
		year INTEGER NOT NULL,
		status TEXT DEFAULT 'Nem'
	);`
	if _, err := p.db.Exec(createTable); err != nil {
		return fmt.Errorf("failed to create movies table: %v", err)
	}
	if _, err := os.Stat(p.jellyfinDBPath); err == nil {
		p.jellyfinDB, err = sql.Open("sqlite3", p.jellyfinDBPath+"?mode=ro")
		if err != nil {
			log.Printf("Warning: Jellyfin DB megnyitási hiba: %v", err)
		}
	}
	return nil
}

func (p *MoviePlugin) loadExistingPINs() {
	rows, err := p.db.Query("SELECT pin FROM movies")
	if err != nil {
		//log.Printf("Error loading existing PINs: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var pin string
		if err := rows.Scan(&pin); err == nil {
			p.usedPins[pin] = true
		}
	}
}

func (p *MoviePlugin) generatePIN() string {
	for {
		pin := fmt.Sprintf("%05d", rand.Intn(90000)+10000)
		if !p.usedPins[pin] {
			p.usedPins[pin] = true
			return pin
		}
	}
}

func (p *MoviePlugin) checkJellyfinMovie(title string) (bool, JellyfinMovie) {
	if p.jellyfinDB == nil {
		return false, JellyfinMovie{}
	}
	query := `SELECT Name, CleanName, OriginalTitle, RunTimeTicks, DateCreated, Overview, Type
			  FROM BaseItems
			  WHERE (type = 'MediaBrowser.Controller.Entities.Movies.Movie' 
			      OR type = 'MediaBrowser.Controller.Entities.TV.Series')
			  AND (Name = ? COLLATE NOCASE OR CleanName = ? COLLATE NOCASE OR OriginalTitle = ? COLLATE NOCASE)`
	var movie JellyfinMovie
	err := p.jellyfinDB.QueryRow(query, title, title, title).Scan(&movie.Name, &movie.CleanName, &movie.OriginalTitle, &movie.RunTimeTicks, &movie.DateCreated, &movie.Overview, &movie.Type)
	if err == sql.ErrNoRows {
		return false, JellyfinMovie{}
	} else if err != nil {
		//log.Printf("Error querying Jellyfin DB: %v", err)
		return false, JellyfinMovie{}
	}
	return true, movie
}

func (p *MoviePlugin) isMovieRequestedWithDetails(title string) (bool, string, string) {
	var requester, uploadDate string
	err := p.db.QueryRow("SELECT requested_by, upload_date FROM movies WHERE title = ? LIMIT 1", title).Scan(&requester, &uploadDate)
	if err == sql.ErrNoRows {
		return false, "", ""
	} else if err != nil {
		//log.Printf("Error checking if movie requested: %v", err)
		return false, "", ""
	}
	return true, requester, p.parseDate(uploadDate)
}

func (p *MoviePlugin) addMovieToDatabase(title, pin, requester string, year int) error {
	uploadTime := time.Now().Format("2006-01-02 15:04:05")
	_, err := p.db.Exec("INSERT INTO movies (title, pin, upload_date, requested_by, year, status) VALUES (?, ?, ?, ?, ?, 'Nem')", title, pin, uploadTime, requester, year)
	return err
}

func (p *MoviePlugin) parseDate(dateString string) string {
	if dateString == "" {
		return "Ismeretlen dátum"
	}
	dateString = strings.ReplaceAll(dateString, "Z", "+00:00")
	formats := []string{"2006-01-02T15:04:05+00:00", "2006-01-02T15:04:05.000+00:00", "2006-01-02T15:04:05", "2006-01-02 15:04:05"}
	for _, f := range formats {
		if t, err := time.Parse(f, dateString); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return "Ismeretlen dátum"
}

func (p *MoviePlugin) formatRuntime(ticks *int64) string {
	if ticks == nil || *ticks == 0 {
		return "N/A"
	}
	seconds := *ticks / 10_000_000
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, (seconds%3600)/60, seconds%60)
}

func (p *MoviePlugin) startRequestPosting() {
	for {
		now := time.Now()
		scheduledTime, err := time.Parse("15:04", p.postTime)
		if err != nil {
			//log.Printf("❌ Hibás post_time formátum: %v", err)
			return
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))

		p.mutex.Lock()
		p.postMovieRequests()
		p.mutex.Unlock()
	}
}

func (p *MoviePlugin) postMovieRequests() {
	if len(p.movieRequests) == 0 {
		return
	}
	for _, chanName := range p.postChan {
		for _, request := range p.movieRequests {
			message := fmt.Sprintf("🚨 @%s 🚨: %s", p.postNick, request)
			p.bot.SendMessage(chanName, message)
		}
	}
	p.movieRequests = make([]string, 0)
}

func (p *MoviePlugin) Close() error {
	if p.db != nil {
		p.db.Close()
	}
	if p.jellyfinDB != nil {
		p.jellyfinDB.Close()
	}
	return nil
}

func (p *MoviePlugin) OnTick() []YnMIrC.Message { return nil }