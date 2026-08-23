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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

// QuotePlugin - a Python (Sopel) "napi idézet" plugin Go megfelelője.
// Naponta egyszer (QuoteTime-kor) kiküldi a napi idézetet a megadott
// csatorná(k)ra, illetve a !quote paranccsal manuálisan is lekérhető.
type QuotePlugin struct {
	bot     *YnMIrC.Client
	apiURL  string
	channels []string

	QuoteTime time.Time // csak az óra:perc számít belőle

	mu               sync.Mutex
	lastAnnounceDate string // "2026-08-23" formátum, duplikáció elleni védelem
}

type quoteAPIResponse struct {
	Quote string `json:"quote"`
}

// NewQuotePlugin létrehozza a plugint.
// timeStr formátuma "15:04" (pl. "07:05"), ugyanaz mint a Python cron időpontja.
func NewQuotePlugin(bot *YnMIrC.Client, apiURL string, channels []string, timeStr string) (*QuotePlugin, error) {
	loc := time.Now().Location()
	t, err := time.ParseInLocation("15:04", timeStr, loc)
	if err != nil {
		return nil, fmt.Errorf("QuotePlugin: érvénytelen idő formátum (%s): %v", timeStr, err)
	}

	return &QuotePlugin{
		bot:       bot,
		apiURL:    apiURL,
		channels:  channels,
		QuoteTime: t,
	}, nil
}

func (q *QuotePlugin) Name() string {
	return "QuotePlugin"
}

// fetchDailyQuote lekéri az idézetet az API-tól, és összerakja a végleges
// üzenetet - ugyanabban a formátumban, mint a régi Python plugin.
func (q *QuotePlugin) fetchDailyQuote() string {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(q.apiURL)
	if err != nil {
		return fmt.Sprintf("Hiba történt az idézet lekérésekor: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Nem sikerült lekérni az idézetet."
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Hiba történt az idézet lekérésekor: %v", err)
	}

	var data quoteAPIResponse
	quote := "Nincs idézet ma."
	if err := json.Unmarshal(body, &data); err == nil && strings.TrimSpace(data.Quote) != "" {
		quote = data.Quote
	}

	return fmt.Sprintf("📜 A nap idézete: \"%s\" - YnM AI 🤖", quote)
}

// HandleMessage - a !quote (illetve !idezet) parancsot kezeli.
func (q *QuotePlugin) HandleMessage(msg YnMIrC.Message) string {
	cmd := strings.TrimSpace(strings.ToLower(msg.Text))

	if cmd != "!quote" && cmd != "!idezet" && cmd != "!idézet" {
		return ""
	}

	return q.fetchDailyQuote()
}

// OnTick - percenként hívja a bot; ha elérkezett a beállított időpont,
// és ma még nem küldtük ki, akkor kiküldi minden csatornára.
func (q *QuotePlugin) OnTick() []YnMIrC.Message {
	q.mu.Lock()
	defer q.mu.Unlock()

	var messages []YnMIrC.Message

	now := time.Now()
	todayKey := now.Format("2006-01-02")

	if now.Hour() == q.QuoteTime.Hour() && now.Minute() == q.QuoteTime.Minute() && q.lastAnnounceDate != todayKey {
		text := q.fetchDailyQuote()

		for _, channel := range q.channels {
			messages = append(messages, YnMIrC.Message{
				Channel: channel,
				Text:    text,
			})
		}

		q.lastAnnounceDate = todayKey
	}

	return messages
}
