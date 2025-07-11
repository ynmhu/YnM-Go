package plugins

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"io"

	"github.com/PuerkitoBio/goquery"
	"github.com/ynmhu/YnM-Go/irc"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/charmap"
)

type JokePlugin struct {
	bot        *irc.Client
	channels   []string
	sendAt     string
	statusFile string
}

func NewJokePlugin(bot *irc.Client, channels []string, sendAt string) *JokePlugin {
	statusPath := filepath.Join("data", "joke_status.json")
	return &JokePlugin{
		bot:        bot,
		channels:   channels,
		sendAt:     sendAt,
		statusFile: statusPath,
	}
}

func (p *JokePlugin) Start() {
	log.Printf("ℹ️ Vicc plugin elindult. Küldési idő: %s", p.sendAt)
	go p.scheduler()
}

func (p *JokePlugin) scheduler() {
	for {
		now := time.Now()
		sendTime, err := time.Parse("15:04", p.sendAt)
		if err != nil {
			log.Printf("⛔️ Vicc plugin - hibás időformátum: %v", err)
			return
		}

		target := time.Date(now.Year(), now.Month(), now.Day(), sendTime.Hour(), sendTime.Minute(), 0, 0, now.Location())
		if now.After(target) {
			target = target.Add(24 * time.Hour)
		}
		waitDuration := target.Sub(now)
		log.Printf("🕒 Vicc plugin vár %v-ot a következő küldésig", waitDuration)
		time.Sleep(waitDuration)
		p.sendDailyJoke()
	}
}

func (p *JokePlugin) sendDailyJoke() {
	today := time.Now().Format("2006-01-02")
	status := p.loadStatus()

	joke := p.getJoke()
	joke = cleanInvalidUTF8(joke)

	intro := "🤣 A nap vicce érkezik! 🎉"
	messages := splitMessage(joke, 320, 280)

	for _, ch := range p.channels {
		p.bot.SendMessage(ch, intro) // az üdvözlő üzenet egyszer
		time.Sleep(1 * time.Second)
		for i, part := range messages {
			if len(messages) > 1 {
				part += fmt.Sprintf(" (%d/%d)", i+1, len(messages))
			}
			p.bot.SendMessage(ch, part) // IDE kerüljön a part, nem az intro!
			log.Printf("Vicc rész elküldve csatornára %s: %s", ch, part)
			time.Sleep(1 * time.Second)
		}
	}

	status["last_sent"] = today
	status["last_joke"] = joke
	p.saveStatus(status)
}



func (p *JokePlugin) getJoke() string {
    url := "https://www.viccesviccek.hu/viccdoboz.php"
    client := &http.Client{
        Timeout: 10 * time.Second,
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        log.Printf("Hiba a vicc lekérésében: %v", err)
        return "Nem sikerült viccet lekérni. 😕"
    }
    req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; YnM-GoBot)")

    resp, err := client.Do(req)
    if err != nil {
        log.Printf("Hiba a vicc lekérésében: %v", err)
        return "Nem sikerült viccet lekérni. 😕"
    }
    defer resp.Body.Close()

    contentType := resp.Header.Get("Content-Type")
    log.Printf("Content-Type: %s", contentType)

    var reader io.Reader
    if strings.Contains(strings.ToLower(contentType), "iso-8859-2") {
        reader = charmap.ISO8859_2.NewDecoder().Reader(resp.Body)
    } else {
        // ha más charset, próbáljuk így
        reader, err = charset.NewReader(resp.Body, contentType)
        if err != nil {
            log.Printf("Charset konvertálás hiba: %v", err)
            return "Nem sikerült a karakterkódolás kezelése. 😕"
        }
    }

    // innen csak az UTF-8 konvertált olvasót adjuk tovább a goquery-nek
    doc, err := goquery.NewDocumentFromReader(reader)
    if err != nil {
        log.Printf("Hiba a HTML feldolgozásában: %v", err)
        return "Nem sikerült feldolgozni a vicc oldalt. 😕"
    }

    text := doc.Find("body").Text()
    text = strings.ReplaceAll(text, "\n", " ")
    if idx := strings.Index(text, "További viccek:"); idx != -1 {
        text = text[:idx]
    }
    text = strings.TrimSpace(text)

    text = cleanJokeText(text)

    if len(text) < 10 {
        return "Nem sikerült viccet találni az oldalon. 😕"
    }

    return text
}


func cleanInvalidUTF8(s string) string {
	var valid []rune
	for i, r := range s {
		if r == utf8.RuneError {
			_, size := utf8.DecodeRuneInString(s[i:])
			if size == 1 {
				continue
			}
		}
		valid = append(valid, r)
	}
	return string(valid)
}

func splitMessage(text string, maxLen int, minLen int) []string {
	var messages []string
	runes := []rune(text)

	for len(runes) > 0 {
		if len(runes) <= maxLen {
			messages = append(messages, strings.TrimSpace(string(runes)))
			break
		}

		// Keressünk mondat végét minimum minLen és maximum maxLen között
		splitPos := maxLen
		foundSplit := false

		// Először keressünk a minLen és maxLen között pontokat vagy más mondatvégi jeleket
		for i := minLen; i <= maxLen && i < len(runes); i++ {
			if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
				splitPos = i + 1
				foundSplit = true
				break
			}
		}

		// Ha nem találtunk mondatvégi jelet a megadott intervallumban, akkor keressünk sima szóközt maxLen közelében visszafelé
		if !foundSplit {
			for i := maxLen; i > minLen; i-- {
				if runes[i] == ' ' || runes[i] == '\n' {
					splitPos = i + 1
					foundSplit = true
					break
				}
			}
		}

		// Ha így sem találtunk, akkor egyszerűen maxLen-nél törjük a szöveget
		if !foundSplit {
			splitPos = maxLen
		}

		messages = append(messages, strings.TrimSpace(string(runes[:splitPos])))
		runes = runes[splitPos:]
	}

	return messages
}


func (p *JokePlugin) loadStatus() map[string]string {
	status := make(map[string]string)
	file, err := os.Open(p.statusFile)
	if err != nil {
		return status
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&status)
	if err != nil {
		return map[string]string{}
	}
	return status
}

func (p *JokePlugin) saveStatus(status map[string]string) {
	os.MkdirAll(filepath.Dir(p.statusFile), 0755)
	file, err := os.Create(p.statusFile)
	if err != nil {
		log.Printf("Hiba a státusz fájl létrehozásakor: %v", err)
		return
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(status); err != nil {
		log.Printf("Hiba a státusz mentésekor: %v", err)
	}
}

func (p *JokePlugin) HandleMessage(msg irc.Message) string {
	return ""
}

func (vp *JokePlugin) OnTick() []irc.Message {
    return nil
}




func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r)
}
