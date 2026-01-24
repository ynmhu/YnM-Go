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
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
	"sync"
    "golang.org/x/text/encoding/charmap"
    "golang.org/x/text/transform"

   // "io"
	"github.com/PuerkitoBio/goquery"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

const (
	CACHE_DURATION     = 30 * time.Minute // 30 perc
	MAX_MESSAGE_LENGTH = 350              // Maximum üzenet hossz

)

// ViccPlugin struktura
type ViccPlugin struct {
	bot             *YnMIrC.Client
	viccCache       []string
	usedViccek      map[string]bool
	lastFetchTime   time.Time
	fallbackViccek  []string
	adminPlugin *owner.YnmAdminPlugin
	lastUsage       map[string]time.Time
    mu              sync.Mutex           
}

// NewViccPlugin létrehozza az új vicc plugin példányt
func NewViccPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin) *ViccPlugin {
	fallbackViccek := []string{
		"Offline.",
	}

	return &ViccPlugin{
		bot:            bot,
		viccCache:      []string{},
		usedViccek:     make(map[string]bool),
		lastFetchTime:  time.Time{},
		fallbackViccek: fallbackViccek,
		adminPlugin:     adminPlugin, 
		lastUsage:      make(map[string]time.Time), 
	}
}

func (v *ViccPlugin) canUseVicc(user string) bool {
    v.mu.Lock()
    defer v.mu.Unlock()
    
    if last, exists := v.lastUsage[user]; exists {
        if time.Since(last) < 30*time.Second {
            return false
        }
    }
    v.lastUsage[user] = time.Now()
    return true
}

func (v *ViccPlugin) Name() string {
	return "vicc"
}


// HandleMessage kezeli a bejövő üzeneteket
func (v *ViccPlugin) HandleMessage(msg YnMIrC.Message) string {
    viccPattern := regexp.MustCompile(`^!vicc$`)
    viccRefreshPattern := regexp.MustCompile(`^!vicc_refresh$`)
    viccTestPattern := regexp.MustCompile(`^!vicc_test$`)
    viccDebugPattern := regexp.MustCompile(`^!vicc_debug$`)
    viccLengthPattern := regexp.MustCompile(`^!vicc_length$`)
    
	var nick, hostmask string

	if YnMModule.IsServerMessage(msg.Sender) {
		return ""
	}

	if msg.Sender != "" {
		// IRC user - session támogatással
		fullHostmask := msg.Sender
		effectiveUser, effectiveHost := v.adminPlugin.GetEffectiveUser(fullHostmask)
		nick = effectiveUser
		hostmask = effectiveHost
	} else if msg.Nick != "" {
		// Discord user - lookup by Discord ID
		userInfo, err := v.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
		if err != nil {
			return "" //return "❌ You need to link your Discord account first. Use !register"
		}
		nick = userInfo.Nick
		hostmask = userInfo.Hostmask
		
		// Discord esetén is session ellenőrzés (ha szükséges)
		effectiveUser, effectiveHost := v.adminPlugin.GetEffectiveUser(userInfo.Hostmask)
		if effectiveUser != userInfo.Nick {
			nick = effectiveUser
			hostmask = effectiveHost
		}
	} else {
		return ""
	}

    switch {
    case viccPattern.MatchString(msg.Text):
        if !YnMModule.HasMinAdminLevelWithDB(v.adminPlugin.Db, nick, hostmask, msg.Channel, 1) {
           // fmt.Printf("[ViccPlugin] Permission denied for %s\n", nick)
            return "" //"❌ You need VIP or higher permissions."
        }
        return v.handleViccCommand(msg)

    case viccRefreshPattern.MatchString(msg.Text):
        // Admin szint (2) vagy magasabb
        if !YnMModule.HasMinAdminLevelWithDB(v.adminPlugin.Db, nick, hostmask, msg.Channel, 2) {
           // fmt.Printf("[ViccPlugin] Permission denied for %s\n", nick)
            return "❌ You need Admin or higher permissions."
        }
        return v.handleViccRefreshCommand(msg)

    case viccTestPattern.MatchString(msg.Text):
        // Admin szint (2) vagy magasabb
        if !YnMModule.HasMinAdminLevelWithDB(v.adminPlugin.Db, nick, hostmask, msg.Channel, 2) {
           // fmt.Printf("[ViccPlugin] Permission denied for %s\n", nick)
            return "❌ You need Admin or higher permissions."
        }
        return v.handleViccTestCommand(msg)

    case viccDebugPattern.MatchString(msg.Text):
        // Csak owner (1)
        if !YnMModule.HasMinAdminLevelWithDB(v.adminPlugin.Db, nick, hostmask, msg.Channel, 1) {
           // fmt.Printf("[ViccPlugin] Permission denied for %s\n", nick)
            return "❌ You need Owner permissions."
        }
        return v.handleViccDebugCommand(msg)

    case viccLengthPattern.MatchString(msg.Text):
        // Csak owner (1)
		if !YnMModule.HasMinAdminLevelWithDB(v.adminPlugin.Db, nick, hostmask, msg.Channel, 1) {
          //  fmt.Printf("[ViccPlugin] Permission denied for %s\n", nick)
            return "❌ You need Owner permissions."
		}
		if !v.canUseVicc(nick) {
            return "⏳ Várj egy kicsit a következő vicc előtt! (30 másodperc)"
        }
        
        return v.handleViccCommand(msg)
    }

    return ""
}

  

// cleanJokeText tisztítja a vicc szövegét
func (v *ViccPlugin) cleanJokeText(text string) string {
    if text == "" {
        return ""
    }

    // Hibás karakterek javítása
    text = strings.ReplaceAll(text, "▒", "é")
    text = strings.ReplaceAll(text, "Ą", "á")
    text = strings.ReplaceAll(text, "ę", "ő")
    text = strings.ReplaceAll(text, "▒", "ű")
    text = strings.ReplaceAll(text, "Ó", "ó")
    text = strings.ReplaceAll(text, "▒", "ú")
    text = strings.ReplaceAll(text, "▒", "í")

	// HTML entitások dekódolása
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")

	// Többszörös szóközök, új sorok tisztítása
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Felesleges részek eltávolítása
	if strings.Contains(text, "Szerinted hány pontos volt") {
		parts := strings.Split(text, "Szerinted hány pontos volt")
		text = parts[0]
	}

	if strings.Contains(text, "Legbénább:") {
		parts := strings.Split(text, "Legbénább:")
		text = parts[0]
	}

	if strings.Contains(text, "Legjobb") {
		parts := strings.Split(text, "Legjobb")
		text = parts[0]
	}

	// Értékelési pontok eltávolítása
	re = regexp.MustCompile(`\(Eddig \d+ értékelés alapján [0-9.]+ pont\)`)
	text = re.ReplaceAllString(text, "")

	// Felesleges szóközök és pont a végén
	text = strings.TrimSpace(text)
	if strings.HasSuffix(text, ".") {
		text = text[:len(text)-1]
	}

	return strings.TrimSpace(text)
}

// isValidJokeContent ellenőrzi, hogy a szöveg valódi vicc-e
func (v *ViccPlugin) isValidJokeContent(text string) bool {
	if text == "" || len(text) < 30 {
		return false
	}

	// Kiszűrjük az értékelő rendszer maradványait
	invalidPhrases := []string{
		"szerinted hány pontos", "értékeld", "legbénább", "legjobb",
		"értékelés alapján", "pont)", "radio", "input", "onclick",
		"javascript", "érték", "border", "margin", "style", "label",
	}

	textLower := strings.ToLower(text)
	for _, phrase := range invalidPhrases {
		if strings.Contains(textLower, phrase) {
			return false
		}
	}

	// Túl rövid vagy túl hosszú
	if len(text) < 30 || len(text) > 1000 {
		return false
	}

	// Legalább 3 szónak kell lennie
	if len(strings.Fields(text)) < 3 {
		return false
	}

	return true
}

// splitLongMessage hosszú üzenetet feldarabol kisebb részekre
func (v *ViccPlugin) splitLongMessage(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var parts []string
	words := strings.Fields(text)
	currentPart := ""

	for _, word := range words {
		testPart := currentPart
		if currentPart != "" {
			testPart += " "
		}
		testPart += word

		if len(testPart) <= maxLength {
			currentPart = testPart
		} else {
			if currentPart != "" {
				parts = append(parts, currentPart)
				currentPart = word
			} else {
				// Ha egy szó önmagában túl hosszú
				parts = append(parts, word[:maxLength])
				currentPart = word[maxLength:]
			}
		}
	}

	if currentPart != "" {
		parts = append(parts, currentPart)
	}

	return parts
}

// fetchViccek lekéri a vicceket a weboldalról
func (v *ViccPlugin) fetchViccek() []string {
	currentTime := time.Now()

	// Cache ellenőrzés
	if currentTime.Sub(v.lastFetchTime) < CACHE_DURATION && len(v.viccCache) > 0 {
		return v.viccCache
	}

	var viccek []string

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	// Több oldal lekérése
	pagesToFetch := []int{0, 10, 20, 30, 40}

	for _, pageNum := range pagesToFetch {
		var url string
		if pageNum == 0 {
			url = "https://www.viccesviccek.hu/vicces_viccek"
		} else {
			url = fmt.Sprintf("https://www.viccesviccek.hu/vicces_viccek&honnan=%d", pageNum)
		}

		log.Printf("Lekérem az oldalt: %s", url)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("Hiba a request létrehozásakor: %v", err)
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Hiba a %s oldal lekérése során: %v", url, err)
			continue
		}

		if resp.StatusCode == 200 {
			// Próbáljuk meg Windows-1250 kódolással dekódolni
			decoder := charmap.Windows1250.NewDecoder()
			reader := transform.NewReader(resp.Body, decoder)
			
			doc, err := goquery.NewDocumentFromReader(reader)
			if err != nil {
				log.Printf("Hiba a HTML parse-olás során: %v", err)
				resp.Body.Close()
				continue
			}

			// Keresünk minden olyan táblázatot, ami viccet tartalmaz
			doc.Find("table[style*='BACKGROUND: white'][style*='FONT-SIZE: 18px']").Each(func(i int, table *goquery.Selection) {
				// A vicc szövege a második sorban van
				rows := table.Find("tr")
				if rows.Length() >= 2 {
					jokeRow := rows.Eq(1)
					jokeCell := jokeRow.Find("td")

					if jokeCell.Length() > 0 {
						jokeText := jokeCell.Text()
						cleanedJoke := v.cleanJokeText(jokeText)

						if v.isValidJokeContent(cleanedJoke) {
							// Duplikáció ellenőrzés
							found := false
							for _, existingJoke := range viccek {
								if existingJoke == cleanedJoke {
									found = true
									break
								}
							}

							if !found {
								viccek = append(viccek, cleanedJoke)
								log.Printf("Vicc hozzáadva: %s...", cleanedJoke[:min(80, len(cleanedJoke))])
							}
						}
					}
				}
			})
		}

		resp.Body.Close()
		time.Sleep(1 * time.Second) // Szünet az oldalak között
	}

	if len(viccek) > 0 {
		// Keverés
		for i := len(viccek) - 1; i > 0; i-- {
			j := rand.Intn(i + 1)
			viccek[i], viccek[j] = viccek[j], viccek[i]
		}
		log.Printf("Összesen %d viccet találtam", len(viccek))
	} else {
		log.Println("Nem találtam vicceket, fallback viccek használata")
		viccek = v.fallbackViccek
	}

	// Cache frissítése
	v.viccCache = viccek
	v.lastFetchTime = currentTime

	return viccek
}

// getUnusedVicc visszaad egy még nem használt viccet
func (v *ViccPlugin) getUnusedVicc() string {
	viccek := v.fetchViccek()

	// Ha minden viccet használtunk, nullázzuk a listát
	if len(v.usedViccek) >= len(viccek) {
		v.usedViccek = make(map[string]bool)
		log.Println("Minden vicc elhangzott, újrakezdés...")
	}

	// Keresünk egy még nem használt viccet
	var availableViccek []string
	for _, vicc := range viccek {
		if !v.usedViccek[vicc] {
			availableViccek = append(availableViccek, vicc)
		}
	}

	if len(availableViccek) > 0 {
		selectedVicc := availableViccek[rand.Intn(len(availableViccek))]
		v.usedViccek[selectedVicc] = true
		return selectedVicc
	}

	if len(viccek) > 0 {
		return viccek[rand.Intn(len(viccek))]
	}

	return "Sajnos nincs elérhető vicc!"
}

// handleViccCommand kezeli a !vicc parancsot
// handleViccCommand kezeli a !vicc parancsot
func (v *ViccPlugin) handleViccCommand(msg YnMIrC.Message) string {
    vicc := v.getUnusedVicc()

    if vicc != "" && vicc != "Sajnos nincs elérhető vicc!" {
        // Discord esetén küldjük egyben (nincs darabolás)
        if msg.Nick != "" {
            // Discord - egy üzenetben
            return fmt.Sprintf("🤣 %s", vicc)
        } else {
            // IRC - régi logika darabolással
            viccParts := v.splitLongMessage(vicc, MAX_MESSAGE_LENGTH)
            if len(viccParts) > 0 {
                response := fmt.Sprintf("🤣 %s", viccParts[0])
                if len(viccParts) > 1 {
                    go func() {
                        for i := 1; i < len(viccParts); i++ {
                            time.Sleep(500 * time.Millisecond)
                            v.bot.SendMessage(msg.Channel, fmt.Sprintf("   %s", viccParts[i]))
                        }
                    }()
                }
                return response
            }
        }
    }

    return "😅 Sajnos most nincs elérhető vicc, próbáld később!"
}

// handleViccStatCommand kezeli a !vicc_stat parancsot
func (v *ViccPlugin) handleViccStatCommand(msg YnMIrC.Message) string {
	viccek := v.fetchViccek()
	totalViccek := len(viccek)
	usedCount := len(v.usedViccek)
	remaining := totalViccek - usedCount

	return fmt.Sprintf("📊 Vicc statisztika: %d vicc van cache-ben, %d már elhangzott, %d maradt", totalViccek, usedCount, remaining)
}

// handleViccRefreshCommand kezeli a !vicc_refresh parancsot
func (v *ViccPlugin) handleViccRefreshCommand(msg YnMIrC.Message) string {
	v.lastFetchTime = time.Time{}
	v.viccCache = []string{}
	v.usedViccek = make(map[string]bool)

	viccek := v.fetchViccek()
	return fmt.Sprintf("🔄 Cache frissítve! %d vicc betöltve.", len(viccek))
}

// handleViccTestCommand kezeli a !vicc_test parancsot
func (v *ViccPlugin) handleViccTestCommand(msg YnMIrC.Message) string {
	go func() {
		for i := 0; i < 3; i++ {
			vicc := v.getUnusedVicc()
			if vicc != "" && vicc != "Sajnos nincs elérhető vicc!" {
				viccParts := v.splitLongMessage(vicc, MAX_MESSAGE_LENGTH)

				for j, part := range viccParts {
					var message string
					if j == 0 {
						message = fmt.Sprintf("🤣 Teszt %d: %s", i+1, part)
					} else {
						message = fmt.Sprintf("       %s", part)
					}

					v.bot.SendMessage(msg.Channel, message)

					if j < len(viccParts)-1 {
						time.Sleep(500 * time.Millisecond)
					}
				}
			} else {
				v.bot.SendMessage(msg.Channel, fmt.Sprintf("😅 Teszt %d: Nincs elérhető vicc", i+1))
			}

			if i < 2 {
				time.Sleep(2 * time.Second)
			}
		}
	}()

	return "🧪 Teszt indítva, 3 vicc következik..."
}

// handleViccDebugCommand kezeli a !vicc_debug parancsot
func (v *ViccPlugin) handleViccDebugCommand(msg YnMIrC.Message) string {
	go func() {
		url := "https://www.viccesviccek.hu/vicces_viccek"
		client := &http.Client{Timeout: 10 * time.Second}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			v.bot.SendMessage(msg.Channel, fmt.Sprintf("😔 Debug hiba: %v", err))
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			v.bot.SendMessage(msg.Channel, fmt.Sprintf("😔 Debug hiba: %v", err))
			return
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			v.bot.SendMessage(msg.Channel, fmt.Sprintf("😔 Debug hiba: %v", err))
			return
		}

		jokeTables := doc.Find("table[style*='BACKGROUND: white'][style*='FONT-SIZE: 18px']")
		v.bot.SendMessage(msg.Channel, fmt.Sprintf("🐛 Debug: %d vicc táblázat találva az oldalon", jokeTables.Length()))

		if jokeTables.Length() > 0 {
			firstTable := jokeTables.First()
			rows := firstTable.Find("tr")
			if rows.Length() >= 2 {
				jokeCell := rows.Eq(1).Find("td")
				if jokeCell.Length() > 0 {
					rawText := jokeCell.Text()
					if len(rawText) > 100 {
						rawText = rawText[:100]
					}
					v.bot.SendMessage(msg.Channel, fmt.Sprintf("🐛 Első vicc nyers szöveg: %s...", rawText))
				}
			}
		}
	}()

	return "🐛 Debug folyamatban..."
}

// handleViccLengthCommand kezeli a !vicc_length parancsot
func (v *ViccPlugin) handleViccLengthCommand(msg YnMIrC.Message) string {
	go func() {
		// Tesztelés céljából egy hosszú viccet készítünk
		longJoke := strings.Repeat("Ez egy nagyon hosszú vicc lesz, ami több mint 450 karaktert tartalmaz, hogy teszteljük a feldarabolás funkcióját. ", 5)

		v.bot.SendMessage(msg.Channel, fmt.Sprintf("📏 Teszt vicc hossza: %d karakter", len(longJoke)))

		// Feldaraboljuk
		parts := v.splitLongMessage(longJoke, MAX_MESSAGE_LENGTH)
		v.bot.SendMessage(msg.Channel, fmt.Sprintf("📏 Feldarabolva %d részre", len(parts)))

		for i, part := range parts {
			preview := part
			if len(preview) > 50 {
				preview = preview[:50]
			}
			v.bot.SendMessage(msg.Channel, fmt.Sprintf("📏 %d. rész (%d kar): %s...", i+1, len(part), preview))
		}
	}()

	return "📏 Hossz teszt folyamatban..."
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (vp *ViccPlugin) OnTick() []YnMIrC.Message {
    return nil
}
