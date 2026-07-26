// ==================================================
// Szerzői jog © 2025 Markus (markus@ynm.hu)
// Javított verzió - Több találat listázása
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
	bot *YnMIrC.Client
	dbPath string
	mutex sync.Mutex
}

func NewJellyfinInfoPlugin(bot *YnMIrC.Client, dbPath string) *JellyfinInfoPlugin {
	return &JellyfinInfoPlugin{
		bot: bot,
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
	return "!info <cím> - Film vagy sorozat keresése. Több találatnál listát ad."
}

func (p *JellyfinInfoPlugin) HandleMessage(msg YnMIrC.Message) string {
	text := strings.TrimSpace(msg.Text)
	
	if strings.HasPrefix(text, "!info") {
		title := strings.TrimSpace(strings.TrimPrefix(text, "!info"))
		if title == "" {
			return "Használat:!info Film címe"
		}
		
		// Idézőjelek levétele ha vannak
		title = strings.Trim(title, `"'`)
		
		if strings.HasPrefix(msg.Channel, "#") {
			go p.searchAndSendMedia(msg.Channel, title)
			return ""
		} else {
			return p.searchMedia(msg.Channel, title)
	}
	}
	
	return ""
}

func (p *JellyfinInfoPlugin) OnTick() []YnMIrC.Message {
	return nil
}

type MediaInfo struct {
	Name string
	OriginalTitle string
	Overview string
	Runtime string
	MediaType string
}

// searchMedia - Discord verzió
func (p *JellyfinInfoPlugin) searchMedia(channel, title string) string {
    results, exact, err := p.findMedia(title)
    if err!= nil {
        return "A film vagy sorozat nem található az adatbázisban!"
    }

    // Ha pontos találat van
    if exact!= nil {
        return p.formatSingleResult(*exact)
    }

    // Ha lista van
    return p.formatMultipleResults(title, results)
}

// IRC verzió
func (p *JellyfinInfoPlugin) searchAndSendMedia(channel, title string) {
    results, exact, err := p.findMedia(title)
    if err!= nil {
        p.bot.SendMessage(channel, "A film vagy sorozat nem található az adatbázisban!")
        return
    }

    if exact!= nil {
        p.sendSingleResult(channel, *exact)
        return
    }

    p.sendMultipleResults(channel, title, results)
}


// ÚJ: Keresés ami több eredményt ad vissza
func (p *JellyfinInfoPlugin) findMedia(title string) ([]MediaInfo, *MediaInfo, error) {
    p.mutex.Lock()
    defer p.mutex.Unlock()

    normalizedSearch := normalizeSearchTerm(title)

    db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", p.dbPath))
    if err!= nil {
        return nil, nil, err
    }
    defer db.Close()

    query := `SELECT Name, CleanName, OriginalTitle, RunTimeTicks, Overview, Type
              FROM BaseItems
              WHERE (type = 'MediaBrowser.Controller.Entities.Movies.Movie' OR
                     type = 'MediaBrowser.Controller.Entities.TV.Series')`

    rows, err := db.Query(query)
    if err!= nil {
        return nil, nil, err
    }
    defer rows.Close()

    var results []MediaInfo
    var exactMatch *MediaInfo
    seen := make(map[string]bool)

    for rows.Next() {
        var name, cleanName, originalTitle, overview, mediaType sql.NullString
        var runtimeTicks sql.NullInt64

        err := rows.Scan(&name, &cleanName, &originalTitle, &runtimeTicks, &overview, &mediaType)
        if err!= nil { continue }

        candidates := []string{name.String, cleanName.String, originalTitle.String}

        foundForThisItem := false // JAVÍTÁS: hogy minden candidate-et megnézzen

        for _, candidate := range candidates {
            if candidate == "" || foundForThisItem { continue } // már találtuk ennél a filmnél

            normalizedCandidate := normalizeSearchTerm(candidate)

            displayTitle := cleanName.String
            if originalTitle.Valid && originalTitle.String!= "" {
                displayTitle = originalTitle.String
            }
            if mediaType.String == "MediaBrowser.Controller.Entities.TV.Series" {
                displayTitle = cleanSeriesTitle(displayTitle)
            }

            if seen[displayTitle] {
                foundForThisItem = true
                continue
            }
            seen[displayTitle] = true

            runtime := "Ismeretlen"
            if runtimeTicks.Valid { runtime = TicksToTime(runtimeTicks.Int64) }

            ov := "Nincs áttekintés elérhető."
            if overview.Valid && overview.String!= "" &&
            !strings.Contains(strings.ToLower(overview.String), "no overview available") {
                ov = overview.String
            }

            media := MediaInfo{
                Name: displayTitle,
                OriginalTitle: displayTitle,
                Overview: ov,
                Runtime: runtime,
                MediaType: mediaType.String,
            }

            // 1. PONTOS EGYEZÉS ELLENŐRZÉS
            if normalizedCandidate == normalizedSearch {
                exactMatch = &media
                break // kilép candidate loopból
            }

            // 2. RÉSZLEGES EGYEZÉS
            if strings.Contains(normalizedCandidate, normalizedSearch) {
                results = append(results, media)
                foundForThisItem = true // ne vegye fel többször ugyanazt a filmet
            }
        }

        if exactMatch!= nil { break }
        if len(results) >= 10 { break }
    }

    if exactMatch!= nil {
        return nil, exactMatch, nil // pontos találat
    }
    if len(results) == 0 {
        return nil, nil, fmt.Errorf("nem található")
    }
    return results, nil, nil // lista
}




func (p *JellyfinInfoPlugin) formatMultipleResults(search string, results []MediaInfo) string {
    var titles []string
    for _, r := range results {
        titles = append(titles, fmt.Sprintf(`"%s"`, r.Name))
    }

    response := fmt.Sprintf("`%s` keresésre %d találat: %s", search, len(results), strings.Join(titles, " | "))
    // JAVÍTÁS: az első találatot ajánlja példának
    if len(results) > 0 {
        response += fmt.Sprintf("\nPontosításhoz: `!info \"%s\"`", results[0].Name)
    }
    return response
}

func (p *JellyfinInfoPlugin) sendMultipleResults(channel, search string, results []MediaInfo) {
    var titles []string
    for _, r := range results {
        titles = append(titles, fmt.Sprintf(`"%s"`, r.Name))
    }

    p.bot.SendMessage(channel, fmt.Sprintf("[%s] keresésre %d találat: %s", search, len(results), strings.Join(titles, " | ")))
    // JAVÍTÁS: az első találatot ajánlja példának
    if len(results) > 0 {
        p.bot.SendMessage(channel, fmt.Sprintf(`Pontosításhoz: !info %s `, results[0].Name))
    }
}



// Egy találat formázása
func (p *JellyfinInfoPlugin) formatSingleResult(r MediaInfo) string {
    mediaTypeHu := "Film"
    if r.MediaType == "MediaBrowser.Controller.Entities.TV.Series" {
        mediaTypeHu = "Sorozat"
    }

    response := fmt.Sprintf("[%s] megtalálva a YnM Media adatbázisban (%s):\n", r.Name, mediaTypeHu)
    response += fmt.Sprintf("Cím: %s\n", r.OriginalTitle)
    response += fmt.Sprintf("Időtartam: %s\n", r.Runtime)
    response += fmt.Sprintf("Áttekintés: %s", truncateText(r.Overview, 1500))
    return response
}

func (p *JellyfinInfoPlugin) sendSingleResult(channel string, r MediaInfo) {
    mediaTypeHu := "Film"
    if r.MediaType == "MediaBrowser.Controller.Entities.TV.Series" {
        mediaTypeHu = "Sorozat"
    }

    p.bot.SendMessage(channel, fmt.Sprintf("[%s] megtalálva a YnM Media adatbázisban (%s):", r.Name, mediaTypeHu))
    p.bot.SendMessage(channel, fmt.Sprintf("Cím: %s", r.OriginalTitle))
    p.bot.SendMessage(channel, fmt.Sprintf("Időtartam: %s", r.Runtime))
    p.sendLongMessage(channel, "Áttekintés: "+r.Overview)
}

// A többi függvény ugyanaz mint nálad
func truncateText(text string, maxLength int) string {
    if len(text) <= maxLength { return text }
    return text[:maxLength-3] + "..."
}

func cleanSeriesTitle(title string) string {
    re := regexp.MustCompile(`(?i)\.S\d{1,2}E\d{1,2}.*$`)
    title = re.ReplaceAllString(title, "")
    title = strings.ReplaceAll(title, ".", " ")
    return strings.TrimSpace(title)
}

func (p *JellyfinInfoPlugin) sendLongMessage(channel, text string) {
    maxLength := 800
    words := strings.Fields(text)
    var buffer strings.Builder
    for _, word := range words {
        if buffer.Len()+len(word)+1 > maxLength {
            p.bot.SendMessage(channel, buffer.String())
            buffer.Reset()
        }
        if buffer.Len() > 0 { buffer.WriteString(" ") }
        buffer.WriteString(word)
    }
    if buffer.Len() > 0 { p.bot.SendMessage(channel, buffer.String()) }
}

func normalizeSearchTerm(term string) string {
    term = strings.ToLower(term)
    term = removeAccents(term)
    replacer := strings.NewReplacer(".", " ", "_", " ", "-", " ", ":", " ", "'", "", "\"", "", "!", "")
    term = replacer.Replace(term)
    term = strings.TrimPrefix(term, "the ")
    term = strings.TrimPrefix(term, "a ")
    return strings.Join(strings.Fields(term), " ")
}

func removeAccents(text string) string {
    var result strings.Builder
    for _, r := range text {
        switch r {
        case 'á', 'à', 'â', 'ä', 'ã', 'å', 'ă', 'ā': result.WriteRune('a')
        case 'é', 'è', 'ê', 'ë', 'ē', 'ę': result.WriteRune('e')
        case 'í', 'ì', 'î', 'ï', 'ī': result.WriteRune('i')
        case 'ó', 'ò', 'ô', 'ö', 'ő', 'õ', 'ø', 'ō': result.WriteRune('o')
        case 'ú', 'ù', 'û', 'ü', 'ű', 'ū': result.WriteRune('u')
        case 'ç', 'ć': result.WriteRune('c')
        case 'ñ': result.WriteRune('n')
        case 'ș', 'ş': result.WriteRune('s')
        case 'ț', 'ţ': result.WriteRune('t')
        case 'ý', 'ÿ': result.WriteRune('y')
        default:
            if!unicode.Is(unicode.Mn, r) { result.WriteRune(r) }
        }
    }
    return result.String()
}