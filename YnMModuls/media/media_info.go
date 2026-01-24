// ==================================================
//  Szerzői jog © 2025 Markus (markus@ynm.hu)
//  Javított verzió - Discord kompatibilis
// ==================================================

package media

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	_ "github.com/mattn/go-sqlite3"
)

type JellyfinInfoPlugin struct {
	bot     *YnMIrC.Client
	dbPath  string
	mutex   sync.Mutex
}

func NewJellyfinInfoPlugin(bot *YnMIrC.Client, dbPath string) *JellyfinInfoPlugin {
	return &JellyfinInfoPlugin{
		bot:    bot,
		dbPath: dbPath,
	}
}

func (p *JellyfinInfoPlugin) Name() string {
	return "JellyfinInfoPlugin"
}

func (p *JellyfinInfoPlugin) Commands() []string {
	return []string{"!info"}
}

func (p *JellyfinInfoPlugin) Help() string {
	return "!info <cím> - Film vagy sorozat információk keresése a Jellyfin adatbázisban"
}

func (p *JellyfinInfoPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	
	if strings.HasPrefix(text, "!info") {
		title := strings.TrimSpace(strings.TrimPrefix(text, "!info"))
		if title == "" {
			return "Használat: !info Film címe"
		}
		
		// ✨ VÁLTOZÁS: Megkülönböztetjük az IRC és Discord csatornákat
		if strings.HasPrefix(msg.Channel, "#") {
			// IRC: goroutine-ban küldjük az üzeneteket
			go p.searchAndSendMedia(msg.Channel, title)
			return ""
		} else {
			// Discord: visszatérünk a válasszal
			return p.searchMedia(msg.Channel, title)
		}
	}
	
	return ""
}

func (p *JellyfinInfoPlugin) OnTick() []YnMIrC.Message {
	return nil
}

// MediaInfo struktúra a média információkhoz
type MediaInfo struct {
	OriginalTitle string
	Overview      string
	Runtime       string
	MediaType     string
}

// searchMedia - Discord kompatibilis verzió (visszatér a válasszal)
func (p *JellyfinInfoPlugin) searchMedia(channel, title string) string {
    info, err := p.getMovieInfo(title)
    if err != nil {
        return "A film vagy sorozat nem található az adatbázisban!"
    }

    mediaTypeHu := "Film"
    if info.MediaType == "MediaBrowser.Controller.Entities.TV.Series" {
        mediaTypeHu = "Sorozat"
    }

    // ✨ VÁLTOZÁS: Egyetlen stringben visszatérünk Discord számára
    response := fmt.Sprintf("[%s] megtalálva a YnM Media adatbázisban (%s):\n", title, mediaTypeHu)
    response += fmt.Sprintf("Cím: %s\n", info.OriginalTitle)
    response += fmt.Sprintf("Időtartam: %s\n", info.Runtime)
    response += fmt.Sprintf("Áttekintés: %s", truncateText(info.Overview, 1500)) // Korlátozzuk a hosszt
    
    return response
}

// searchAndSendMedia - IRC kompatibilis verzió (küldi az üzeneteket)
func (p *JellyfinInfoPlugin) searchAndSendMedia(channel, title string) {
    info, err := p.getMovieInfo(title)
    if err != nil {
        p.bot.SendMessage(channel, "A film vagy sorozat nem található az adatbázisban!")
        return
    }

    mediaTypeHu := "Film"
    if info.MediaType == "MediaBrowser.Controller.Entities.TV.Series" {
        mediaTypeHu = "Sorozat"
    }

    // IRC: közvetlen üzenetküldés
    p.bot.SendMessage(channel, fmt.Sprintf("[%s] megtalálva a YnM Media adatbázisban (%s):", title, mediaTypeHu))
    p.bot.SendMessage(channel, fmt.Sprintf("Cím: %s", info.OriginalTitle))
    p.bot.SendMessage(channel, fmt.Sprintf("Időtartam: %s", info.Runtime))
    p.sendLongMessage(channel, "Áttekintés: "+info.Overview)
}

// Segédfüggvény: szöveg rövidítése
func truncateText(text string, maxLength int) string {
    if len(text) <= maxLength {
        return text
    }
    return text[:maxLength-3] + "..."
}

// cleanSeriesTitle eltávolítja az SxxExx jellegű epizód azonosítót
func cleanSeriesTitle(title string) string {
    re := regexp.MustCompile(`(?i)\.S\d{1,2}E\d{1,2}.*$`)
    title = re.ReplaceAllString(title, "")
    title = strings.ReplaceAll(title, ".", " ")
    return strings.TrimSpace(title)
}

// getMovieInfo keres a Jellyfin adatbázisban (marad változatlan)
func (p *JellyfinInfoPlugin) getMovieInfo(title string) (*MediaInfo, error) {
    p.mutex.Lock()
    defer p.mutex.Unlock()

    normalizedSearch := normalizeSearchTerm(title)

    db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", p.dbPath))
    if err != nil {
        return nil, fmt.Errorf("adatbázis megnyitási hiba: %v", err)
    }
    defer db.Close()

    query := `SELECT Name, CleanName, OriginalTitle, RunTimeTicks, DateCreated, Overview, Type
              FROM BaseItems 
              WHERE (type = 'MediaBrowser.Controller.Entities.Movies.Movie' OR 
                     type = 'MediaBrowser.Controller.Entities.TV.Series')`

    rows, err := db.Query(query)
    if err != nil {
        return nil, fmt.Errorf("lekérdezési hiba: %v", err)
    }
    defer rows.Close()

    for rows.Next() {
        var name, cleanName, originalTitle, overview, mediaType sql.NullString
        var runtimeTicks sql.NullInt64
        var dateCreated sql.NullString

        err := rows.Scan(&name, &cleanName, &originalTitle, &runtimeTicks, &dateCreated, &overview, &mediaType)
        if err != nil {
            continue
        }

        candidates := []string{name.String, cleanName.String, originalTitle.String}
        
        for _, candidate := range candidates {
            if candidate != "" {
                normalizedCandidate := normalizeSearchTerm(candidate)
                if strings.Contains(normalizedCandidate, normalizedSearch) {
                    result := &MediaInfo{}
                    
                    if originalTitle.Valid && originalTitle.String != "" {
                        result.OriginalTitle = originalTitle.String
                    } else {
                        result.OriginalTitle = cleanName.String
                    }
                    
                    if runtimeTicks.Valid {
                        result.Runtime = TicksToTime(runtimeTicks.Int64)
                    } else {
                        result.Runtime = "Ismeretlen"
                    }
                    
                    if overview.Valid && overview.String != "" && 
                       !strings.Contains(strings.ToLower(overview.String), "no overview available") &&
                       !strings.Contains(strings.ToLower(overview.String), "nincs áttekintés elérhető") {
                        result.Overview = overview.String
                    } else {
                        result.Overview = "Nincs áttekintés elérhető."
                    }
                    
                    result.MediaType = mediaType.String
                    
                    if mediaType.String == "MediaBrowser.Controller.Entities.TV.Series" {
                        result.OriginalTitle = cleanSeriesTitle(result.OriginalTitle)
                    }
                    
                    return result, nil
                }
            }
        }
    }
    
    return nil, fmt.Errorf("nem található")
}

// A többi segédfüggvény marad változatlan...
func (p *JellyfinInfoPlugin) sendLongMessage(channel, text string) {
    maxLength := 800
    words := strings.Fields(text)
    var buffer strings.Builder
    
    for _, word := range words {
        if buffer.Len()+len(word)+1 > maxLength {
            p.bot.SendMessage(channel, buffer.String())
            buffer.Reset()
        }
        if buffer.Len() > 0 {
            buffer.WriteString(" ")
        }
        buffer.WriteString(word)
    }
    
    if buffer.Len() > 0 {
        p.bot.SendMessage(channel, buffer.String())
    }
}

func normalizeSearchTerm(term string) string {
    term = strings.ToLower(term)
    term = removeAccents(term)
    
    replacer := strings.NewReplacer(
        ".", " ", "_", " ", "-", " ", ":", " ",
        "'", "", "\"", "", "!", "",
    )
    term = replacer.Replace(term)
    
    term = strings.TrimPrefix(term, "the ")
    term = strings.TrimPrefix(term, "a ")
    
    return strings.Join(strings.Fields(term), " ")
}

func removeAccents(text string) string {
    var result strings.Builder
    for _, r := range text {
        switch r {
        case 'ă', 'â': result.WriteRune('a')
        case 'î': result.WriteRune('i')
        case 'ș', 'ş': result.WriteRune('s')
        case 'ț', 'ţ': result.WriteRune('t')
        default:
            if !unicode.Is(unicode.Mn, r) {
                result.WriteRune(r)
            }
        }
    }
    return result.String()
}