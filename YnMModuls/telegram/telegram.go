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

package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type TelegramAdapter struct {
	enabled      bool
	lastPostTime time.Time
	minInterval  time.Duration
	botToken     string
	chatID       string // Privát chat (admin notification)
	channelID    string // Publikus csatorna (auto-post)
	postCount    int
	lastPostName string
}

// NewTelegramAdapter létrehoz egy új Telegram adaptert
func NewTelegramAdapter(enabled bool, minIntervalStr string, botToken string, chatID string, channelID string) *TelegramAdapter {
	if !enabled {
		log.Printf("ℹ️ Telegram modul kikapcsolva")
		return nil
	}

	if botToken == "" {
		log.Printf("⚠️ Telegram bot token hiányzik!")
		return nil
	}

	if chatID == "" && channelID == "" {
		log.Printf("⚠️ Telegram chat ID vagy channel ID hiányzik!")
		return nil
	}

	minInterval, err := time.ParseDuration(minIntervalStr)
	if err != nil {
		log.Printf("⚠️ Érvénytelen Telegram interval (%s), alapértelmezett 30m használata", minIntervalStr)
		minInterval = 30 * time.Minute
	}

	//log.Printf("✅ Telegram adapter inicializálva")
	if chatID != "" {
//		log.Printf("   💬 Privát chat (admin): %s", chatID)
	}
	if channelID != "" {
//		log.Printf("   📢 Publikus csatorna: %s", channelID)
	}
//	log.Printf("   ⏰ Min. interval: %v", minInterval)

	return &TelegramAdapter{
		enabled:     true,
		minInterval: minInterval,
		botToken:    botToken,
		chatID:      chatID,
		channelID:   channelID,
		postCount:   0,
	}
}

// PostMusic posztol új zenéről a Telegram csatornára
func (ta *TelegramAdapter) PostMusic(name, category, addedDate string) error {
	if !ta.enabled {
		return fmt.Errorf("Telegram adapter kikapcsolva")
	}

	// Rate limiting
	now := time.Now()
	if now.Sub(ta.lastPostTime) < ta.minInterval {
		remaining := ta.minInterval - now.Sub(ta.lastPostTime)
		log.Printf("⏳ Telegram: túl korai posztolás, várakozás: %v", remaining)
		return nil
	}

	log.Printf("📤 Telegram üzenet készítése...")
	log.Printf("   🎵 Zene: %s", name)
	log.Printf("   📂 Kategória: %s", category)

	// Szép formázás - .mp3 eltávolítása
	cleanName := strings.TrimSuffix(name, ".mp3")

	// Kategória emoji
	categoryEmoji := map[string]string{
		"Mulatos": "🎺",
		"Notak":   "🎤",
		"Vegyes":  "🎵",
	}
	emoji := categoryEmoji[category]
	if emoji == "" {
		emoji = "🎵"
	}

	// PUBLIKUS POSZT (csatornára)
	publicPost := fmt.Sprintf(`%s *Új zene érkezett\!*

🎼 *%s*
📂 %s
📅 %s

🎧 Hallgasd meg: https://legszebbnotak\.hu

\#LegszebNóták \#ÚjZene \#%s`,
		emoji,
		escapeMarkdown(cleanName),
		escapeMarkdown(category),
		escapeMarkdown(addedDate),
		category)

	// PRIVÁT NOTIFICATION (admin-nak)
	privateNotification := fmt.Sprintf(`🔔 *Új zene posztolva a csatornára\!*

📀 %s
📂 %s

📢 Csatorna: https://t\.me/ynm\_hu`,
		escapeMarkdown(cleanName),
		escapeMarkdown(category))

	// Küldés a CSATORNÁRA (publikus)
	if ta.channelID != "" {
		err := ta.sendMessage(ta.channelID, publicPost, true)
		if err != nil {
			log.Printf("❌ Csatorna küldési hiba: %v", err)
			return err
		}
		log.Printf("✅ Telegram CSATORNA poszt sikeres!")
		ta.postCount++
		ta.lastPostName = cleanName
	}

	// Küldés PRIVÁTBA (admin notification)
	if ta.chatID != "" {
		err := ta.sendMessage(ta.chatID, privateNotification, true)
		if err != nil {
			log.Printf("⚠️ Admin notification hiba: %v", err)
		} else {
			log.Printf("✅ Telegram ADMIN notification sikeres!")
		}
	}

	ta.lastPostTime = now
	return nil
}

// sendMessage küld egy üzenetet Telegram-ra
func (ta *TelegramAdapter) sendMessage(chatID string, message string, useMarkdown bool) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ta.botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    message,
	}

	if useMarkdown {
		payload["parse_mode"] = "MarkdownV2"
	}

	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("HTTP hiba: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("Telegram API hiba (%d): %v", resp.StatusCode, result)
	}

	return nil
}

// escapeMarkdown escape-eli a MarkdownV2 speciális karaktereket
func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}

// IsEnabled visszaadja, hogy engedélyezve van-e az adapter
func (ta *TelegramAdapter) IsEnabled() bool {
	return ta != nil && ta.enabled
}

// GetStats visszaadja a statisztikákat
func (ta *TelegramAdapter) GetStats() string {
	if ta == nil || !ta.enabled {
		return "Telegram: kikapcsolva"
	}
	return fmt.Sprintf("📊 Telegram posztok: %d db | Utolsó: %s", ta.postCount, ta.lastPostName)
}

// Name visszaadja a modul nevét
func (ta *TelegramAdapter) Name() string {
	return "Telegram Adapter"
}