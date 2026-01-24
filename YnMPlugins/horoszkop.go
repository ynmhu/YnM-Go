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
	"io"
//	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

type HoroszkopPlugin struct {
	bot          *YnMIrC.Client
	csillagjegyek map[string]string
	titles       map[string]string
}

func NewHoroszkopPlugin(bot *YnMIrC.Client) *HoroszkopPlugin {
	return &HoroszkopPlugin{
		bot: bot,
		csillagjegyek: map[string]string{
			"vizonto":  "♒",
			"bika":     "♉",
			"bak":      "♑",
			"kos":      "♈",
			"ikrek":    "♊",
			"rak":      "♋",
			"oroszlan": "♌",
			"szuz":     "♍",
			"merleg":   "♎",
			"skorpio":  "♏",
			"nyilas":   "♐",
			"halak":    "⛎",
		},
		titles: map[string]string{
			"napi":    "Napi horoszkóp",
			"heti":    "Heti horoszkóp",
			"havi":    "Havi horoszkóp",
			"hetvegi": "Hétvégi horoszkóp",
		},
	}
}

func (h *HoroszkopPlugin) HandleMessage(msg YnMIrC.Message) string {
	// Regex pattern a !horoszkop parancshoz
	pattern := `^!horoszkop\s*(.*)`
	re := regexp.MustCompile(pattern)
	
	if !re.MatchString(msg.Text) {
		return ""
	}
	
	matches := re.FindStringSubmatch(msg.Text)
	if len(matches) < 2 {
		return ""
	}
	
	args := strings.Fields(matches[1])
	
	if len(args) == 0 {
		return "Helytelen, probáld meg így: !horoszkop bika napi, !horoszkop bika heti, !horoszkop bika havi vagy !horoszkop bika hétvégi."
	}
	
	csillagjegy := strings.ToLower(args[0])
	tipus := "napi" // Alapértelmezett típus
	
	if len(args) > 1 {
		tipus = strings.ToLower(args[1])
	}
	
	// Csillagjegy normalizálás
	csillagjegy = h.normalizeCsillagjegy(csillagjegy)
	
	// Ellenőrizzük, hogy létezik-e a csillagjegy
	if _, exists := h.csillagjegyek[csillagjegy]; !exists {
		validSigns := make([]string, 0, len(h.csillagjegyek))
		for k := range h.csillagjegyek {
			validSigns = append(validSigns, k)
		}
		return "Ismeretlen csillagjegy. Használhatók: " + strings.Join(validSigns, ", ")
	}
	
	// Típus normalizálás
	tipus = h.normalizeTipus(tipus)
	
	// ✅ EGYSZERŰBB MEGOLDÁS: Mindig szinkron módon adjuk vissza
	// A goroutine-t a hívó oldal kezeli (pl. commands.go)
	return h.fetchHoroszkop(msg.Channel, csillagjegy, tipus)
}
func (h *HoroszkopPlugin) fetchHoroszkop(channel, csillagjegy, tipus string) string {
	horoszkopData := h.getHoroszkop(csillagjegy, tipus)
	
	if horoszkopData == "" {
		return "Nem sikerült lekérni a horoszkópot. Próbáld újra később."
	}
	
	symbol := h.csillagjegyek[csillagjegy]
	
	// Discord esetén ne törjük több részre, küldjük egyben
	return fmt.Sprintf("*%s* (*%s*) *%s*: *%s*", 
		strings.Title(csillagjegy), 
		symbol, 
		strings.Title(tipus), 
		horoszkopData)
}

func (h *HoroszkopPlugin) OnTick() []YnMIrC.Message {
	return []YnMIrC.Message{}
}

func (h *HoroszkopPlugin) normalizeCsillagjegy(csillagjegy string) string {
	switch csillagjegy {
	case "szüz", "szűz", "szúz":
		return "szuz"
	case "vizöntő", "vizöntö", "vizőntő", "vizőntö":
		return "vizonto"
	case "rák":
		return "rak"
	case "oroszlán":
		return "oroszlan"
	case "mérleg":
		return "merleg"
	case "kós":
		return "kos"
	default:
		return csillagjegy
	}
}

func (h *HoroszkopPlugin) normalizeTipus(tipus string) string {
	switch tipus {
	case "hétvégi":
		return "hetvegi"
	case "napi", "heti", "havi", "hetvegi":
		return tipus
	default:
		return "napi"
	}
}

func (h *HoroszkopPlugin) fetchAndSendHoroszkop(channel, csillagjegy, tipus string) {
	horoszkopData := h.getHoroszkop(csillagjegy, tipus)
	
	if horoszkopData == "" {
		h.bot.SendMessage(channel, "Nem sikerült lekérni a horoszkópot. Próbáld újra később.")
		return
	}
	
	chunks := h.splitIntoChunks(horoszkopData, 350)
	symbol := h.csillagjegyek[csillagjegy]
	
	for _, chunk := range chunks {
		message := fmt.Sprintf("*%s* (*%s*) *%s*: *%s*", 
			strings.Title(csillagjegy), 
			symbol, 
			strings.Title(tipus), 
			chunk)
		
		h.bot.SendMessage(channel, message)
		time.Sleep(1 * time.Second) // 1 másodperc várakozás az üzenetek között
	}
}

func (h *HoroszkopPlugin) getHoroszkop(csillagjegy, tipus string) string {
	url := fmt.Sprintf("https://nlc.hu/horoszkop/%s/", csillagjegy)
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		//log.Printf("Horoszkóp HTTP kérés hiba: %v", err)
		return ""
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		//log.Printf("Horoszkóp HTTP válasz hiba: %v", err)
		return ""
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		//log.Printf("Horoszkóp HTTP státusz hiba: %d", resp.StatusCode)
		return ""
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		//log.Printf("Horoszkóp válasz olvasási hiba: %v", err)
		return ""
	}
	
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		//log.Printf("Horoszkóp HTML parsing hiba: %v", err)
		return ""
	}
	
	title, exists := h.titles[tipus]
	if !exists {
		//log.Printf("Horoszkóp ismeretlen típus: %s", tipus)
		return ""
	}
	
	//log.Printf("Horoszkóp keresés: %s címre: %s", title, url)
	
	// Debug: listázzuk az összes h2 címet
	doc.Find("h2").Each(func(i int, s *goquery.Selection) {
		//log.Printf("Talált h2 cím: '%s'", strings.TrimSpace(s.Text()))
	})
	
	// Keressük meg a megfelelő h2 címet
	var horoscopeContent string
	doc.Find("h2").Each(func(i int, s *goquery.Selection) {
		h2Text := strings.TrimSpace(s.Text())
		if h2Text == title {
			//log.Printf("Megtalált h2 cím: %s", h2Text)
			
			// Próbáljuk meg az összes következő testvér elemet végigmenni
			s.NextAll().Each(func(j int, nextSibling *goquery.Selection) {
				if horoscopeContent != "" {
					return // Ha már van tartalom, ugorjuk át
				}
				
				tagName := goquery.NodeName(nextSibling)
				//log.Printf("Vizsgált elem [%d]: <%s>", j, tagName)
				
				// Ha újabb h2-t találunk, akkor vége a keresésnek
				if tagName == "h2" {
				//	log.Printf("Újabb h2 elem találva, keresés befejezve")
					return
				}
				
				// Keressük a szöveget az elemben és az összes gyermekében
				elemText := strings.TrimSpace(nextSibling.Text())
				//log.Printf("Elem szövege: '%s'", elemText)
				
				// Csak akkor fogadjuk el, ha valódi szöveg (nem csak dátum)
				if elemText != "" && !isJustDate(elemText) && len(elemText) > 20 {
					horoscopeContent = elemText
					//log.Printf("Találat: %s", horoscopeContent[:min(100, len(horoscopeContent))])
					return
				}
				
				// Ha az elem maga nem tartalmaz szöveget, keressük a gyermekekben
				if elemText == "" || isJustDate(elemText) {
					nextSibling.Find("p, div, span").Each(func(k int, child *goquery.Selection) {
						if horoscopeContent != "" {
							return
						}
						childText := strings.TrimSpace(child.Text())
						//log.Printf("Gyermek elem szövege [%d]: '%s'", k, childText)
						
						if childText != "" && !isJustDate(childText) && len(childText) > 20 {
							horoscopeContent = childText
						//	log.Printf("Találat gyermek elemben: %s", horoscopeContent[:min(100, len(horoscopeContent))])
							return
						}
					})
				}
			})
			
			// Ha még mindig nincs tartalom, próbáljuk meg a szülő elem testvéreit
			if horoscopeContent == "" {
				//log.Printf("Szülő elem testvéreinek keresése...")
				s.Parent().NextAll().Each(func(j int, nextSibling *goquery.Selection) {
					if horoscopeContent != "" {
						return
					}
					
					tagName := goquery.NodeName(nextSibling)
				//	log.Printf("Szülő testvér elem [%d]: <%s>", j, tagName)
					
					if tagName == "h2" {
						return // Ha újabb h2-t találunk, vége
					}
					
					elemText := strings.TrimSpace(nextSibling.Text())
					if elemText != "" && !isJustDate(elemText) && len(elemText) > 20 {
						horoscopeContent = elemText
						//log.Printf("Találat szülő testvérben: %s", horoscopeContent[:min(100, len(horoscopeContent))])
						return
					}
					
					// Keresés a gyermekekben
					nextSibling.Find("p, div, span").Each(func(k int, child *goquery.Selection) {
						if horoscopeContent != "" {
							return
						}
						childText := strings.TrimSpace(child.Text())
						if childText != "" && !isJustDate(childText) && len(childText) > 20 {
							horoscopeContent = childText
							//log.Printf("Találat szülő testvér gyermekében: %s", horoscopeContent[:min(100, len(horoscopeContent))])
							return
						}
					})
				})
			}
		}
	})
	
	if horoscopeContent == "" {
		//log.Printf("Horoszkóp tartalom nem található: %s - %s", csillagjegy, tipus)
		
		// Utolsó próbálkozás: keressük meg az összes szöveget a dokumentumban
		// és próbáljuk meg azonosítani a horoszkóp tartalmat
		//log.Printf("Utolsó próbálkozás: teljes dokumentum keresése...")
		doc.Find("p, div").Each(func(i int, s *goquery.Selection) {
			if horoscopeContent != "" {
				return
			}
			text := strings.TrimSpace(s.Text())
			// Ha hosszabb szöveg és nem dátum, és valószínűleg horoszkóp tartalom
			if len(text) > 50 && !isJustDate(text) && containsHoroscopeKeywords(text) {
				horoscopeContent = text
				//log.Printf("Találat teljes keresésben: %s", horoscopeContent[:min(100, len(horoscopeContent))])
				return
			}
		})
	}
	
	return horoscopeContent
}

// Segédfüggvény: ellenőrzi, hogy a szöveg csak dátum-e
func isJustDate(text string) bool {
	// Egyszerű dátum pattern ellenőrzés
	datePattern := `^\d{4}\.\d{2}\.\d{2}\.?$`
	matched, _ := regexp.MatchString(datePattern, text)
	return matched || len(text) < 15
}

// Segédfüggvény: ellenőrzi, hogy tartalmaz-e horoszkóp kulcsszavakat
func containsHoroscopeKeywords(text string) bool {
	keywords := []string{
		"ma", "mai", "nap", "szerelem", "karrier", "egészség", "pénz", 
		"munka", "kapcsolat", "energia", "bolygó", "hold", "nap", "mercury",
		"vénus", "mars", "jupiter", "szaturnusz", "uranus", "neptun", "pluto",
		"aspektus", "kvadrat", "trigon", "konjunkció", "szerencsés", "kedvező",
		"óvatosan", "figyelem", "lehetőség", "kihívás", "változás", "fejlődés",
	}
	
	lowerText := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}
	return false
}

func (h *HoroszkopPlugin) splitIntoChunks(text string, maxLength int) []string {
	var chunks []string
	var currentChunk strings.Builder
	
	sentences := strings.Split(text, ". ")
	
	for _, sentence := range sentences {
		testChunk := currentChunk.String() + sentence + ". "
		
		if len(testChunk) <= maxLength {
			currentChunk.WriteString(sentence + ". ")
		} else {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
			}
			currentChunk.WriteString(sentence + ". ")
		}
	}
	
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}
	
	return chunks
}