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
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
)

type HunTorrentPlugin struct {
	bot         *YnMIrC.Client
	config      *YnMConfig.HunTorrentConfig
	lastMessages map[string]bool // üzenetek követése duplikáció ellen
	initialized bool            // első futás jelzése
}

func NewHunTorrentPlugin(bot *YnMIrC.Client, config *YnMConfig.HunTorrentConfig) *HunTorrentPlugin {
	return &HunTorrentPlugin{
		bot:         bot,
		config:      config,
		lastMessages: make(map[string]bool),
		initialized:  false,
	}
}

func (p *HunTorrentPlugin) Start() {
	go p.monitorChat()
}

func (p *HunTorrentPlugin) monitorChat() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	//log.Printf("[HunTorrentPlugin] Chat monitoring elindítva, ellenőrzés 60 másodpercenként")
	
	for {
		select {
		case <-ticker.C:
			p.checkNewMessages()
		}
	}
}

func (p *HunTorrentPlugin) checkNewMessages() {
	messages, err := p.fetchChatMessages()
	if err != nil {
		//log.Printf("[HunTorrentPlugin] Hiba az üzenetek lekérésekor: %v", err)
		return
	}

	if !p.initialized {
		// Első futás: csak a meglévő üzenetek regisztrálása, IRC-re nem küldés
		for _, msg := range messages {
			messageKey := fmt.Sprintf("%s:%s:%s", msg.Time, msg.Nick, msg.Message)
			p.lastMessages[messageKey] = true
		}
		p.initialized = true
		//log.Printf("[HunTorrentPlugin] Inicializálva, %d meglévő üzenet regisztrálva", len(messages))
		return
	}

	// Második futástól kezdve: új üzenetek küldése IRC-re
	newCount := 0
	for _, msg := range messages {
		messageKey := fmt.Sprintf("%s:%s:%s", msg.Time, msg.Nick, msg.Message)
		if !p.lastMessages[messageKey] {
			p.sendToChannels(fmt.Sprintf("[🔥HT] [%s] %s: %s", msg.Time, msg.Nick, msg.Message))
			p.lastMessages[messageKey] = true
			newCount++
		}
	}

	// Memória tisztítása (csak az utolsó 1000 üzenetet tartjuk meg)
	if len(p.lastMessages) > 1000 {
		p.cleanupMessageHistory()
	}

	//if newCount > 0 {
	//	log.Printf("[HunTorrentPlugin] %d új üzenet feldolgozva", newCount)
	//}
}

type ChatMessage struct {
	Time    string
	Nick    string
	Message string
}

func (p *HunTorrentPlugin) fetchChatMessages() ([]ChatMessage, error) {
	req, err := http.NewRequest("GET", p.config.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Cookie", p.config.Cookie)
	req.Header.Set("User-Agent", p.config.UserAgent)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return p.parseChatMessages(string(body))
}

func (p *HunTorrentPlugin) parseChatMessages(html string) ([]ChatMessage, error) {
	// Regex az üzenetekhez
	re := regexp.MustCompile(`\$\.chat_uzenet_beiras\(\d+,\s*\d+,\s*'([^']*)',\s*'([^']*)',\s*\d+,\s*'([^']*)'`)
	matches := re.FindAllStringSubmatch(html, -1)

	// HTML tag eltávolító regex
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	
	var messages []ChatMessage
	
	for _, m := range matches {
		nick := p.unescapeHTML(m[1])
		message := p.unescapeHTML(m[2])
		time := m[3]

		// HTML tagek eltávolítása
		message = htmlTagRegex.ReplaceAllString(message, "")
		
		// Többszörös szóközök normalizálása
		message = regexp.MustCompile(`\s+`).ReplaceAllString(message, " ")
		message = strings.TrimSpace(message)

		// Üres üzenetek kiszűrése
		if message != "" {
			messages = append(messages, ChatMessage{
				Time:    time,
				Nick:    nick,
				Message: message,
			})
		}
	}

	return messages, nil
}

func (p *HunTorrentPlugin) cleanupMessageHistory() {
	// Egyszerű cleanup: töröljük a fele üzenetet
	count := 0
	for k := range p.lastMessages {
		if count > 500 {
			delete(p.lastMessages, k)
		}
		count++
	}
	log.Printf("[HunTorrentPlugin] Üzenet história tisztítva, %d üzenet maradt", len(p.lastMessages))
}

func (p *HunTorrentPlugin) sendToChannels(msg string) {
	for _, ch := range p.config.Channels {
		p.bot.SendMessage(ch, msg)
	}
}

// SetCookie - Cookie frissítése futás közben
func (p *HunTorrentPlugin) SetCookie(newCookie string) {
	p.config.Cookie = newCookie
	log.Printf("[HunTorrentPlugin] Cookie frissítve")
}

// AddChannel - Új csatorna hozzáadása
func (p *HunTorrentPlugin) AddChannel(channel string) {
	p.config.Channels = append(p.config.Channels, channel)
	log.Printf("[HunTorrentPlugin] Új csatorna hozzáadva: %s", channel)
}

// unescapeHTML - HTML entitások dekódolása
func (p *HunTorrentPlugin) unescapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return s
}