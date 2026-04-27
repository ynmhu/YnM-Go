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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
)

type ChatGPTPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	Db          *YnMDb.AdminDB
	apiKey      string
	timeout     time.Duration
}

func NewChatGPTPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB, apiKey string, timeout time.Duration) *ChatGPTPlugin {
	return &ChatGPTPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		apiKey:      apiKey,
		Db:          db,
		timeout:     timeout,
	}
}

func (p *ChatGPTPlugin) HandleMessage(msg YnMIrC.Message) string {
    var nick, hostmask string
    
    if msg.Sender != "" {
        // IRC user
        nick = strings.Split(msg.Sender, "!")[0]
        simplifiedHostmask := YnMModule.SimplifyHostmask(msg.Sender)
        
        if session, exists := p.adminPlugin.GetSessionByHost(simplifiedHostmask); exists {
            hostmask = session.LoggedInHost
        } else {
            // Ha nincs session, használjuk az eredeti hostmask-ot
            hostmask = simplifiedHostmask
        }
        hostmask = p.adminPlugin.GetEffectiveHostmask(simplifiedHostmask)
    } else if msg.Nick != "" {
        // Discord user
        userInfo, err := p.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
        if err != nil {
            return ""
        }
        nick = userInfo.Nick
        hostmask = userInfo.Hostmask
    } else {
        return ""
    }
    
    prefix := p.adminPlugin.GetPrefixForHost(hostmask)
    minLevel := 1
    
    if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, minLevel) {
        return ""
    }
	text := strings.TrimSpace(msg.Text)
    if !(strings.ToLower(text) == strings.ToLower(prefix+"gpt") || strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix+"gpt "))) {
        return ""
    }

    parts := strings.Fields(msg.Text)
    if len(parts) < 2 {
        return "Usage: !chatgpt <question>"
    }
    prompt := strings.Join(parts[1:], " ")
    
    return p.askChatGPTSync(prompt)
}

// ✅ ÚJ szinkron függvény
func (p *ChatGPTPlugin) askChatGPTSync(prompt string) string {
	type requestBody struct {
		Model    string               `json:"model"`
		Messages []map[string]string `json:"messages"`
	}

	type responseChoice struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	type responseBody struct {
		Choices []responseChoice `json:"choices"`
	}

	reqBody := requestBody{
		Model: "mistral-small-latest",
		Messages: []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Sprintf("🔴 Error preparing request: %v", err)
	}

	client := &http.Client{Timeout: p.timeout}

	req, err := http.NewRequest("POST", "https://api.mistral.ai/v1/chat/completions", bytes.NewBuffer(reqBytes))
	if err != nil {
		return fmt.Sprintf("🔴 Error sending request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("🔴 API call error: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("🔴 Error reading response: %v", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Sprintf("🔴 API error: %s", string(bodyBytes))
	}

	var res responseBody
	err = json.Unmarshal(bodyBytes, &res)
	if err != nil {
		return fmt.Sprintf("🔴 Error decoding response: %v", err)
	}

	if len(res.Choices) == 0 {
		return "⚠️ No response received from ChatGPT."
	}

	answer := res.Choices[0].Message.Content

	const maxLen = 400
	if len(answer) > maxLen {
		answer = answer[:maxLen] + "..."
	}

	return fmt.Sprintf("💬 : %s", answer)
}


func (p *ChatGPTPlugin) askChatGPT(prompt, channel string) {
	type requestBody struct {
		Model    string               `json:"model"`
		Messages []map[string]string `json:"messages"`
	}

	type responseChoice struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	type responseBody struct {
		Choices []responseChoice `json:"choices"`
	}

	reqBody := requestBody{
		Model: "mistral-small-latest",
		Messages: []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		p.bot.SendMessage(channel, fmt.Sprintf("🔴 Error preparing request: %v", err))
		return
	}

	client := &http.Client{Timeout: p.timeout}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqBytes))
	if err != nil {
		p.bot.SendMessage(channel, fmt.Sprintf("🔴 Error sending request: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := client.Do(req)
	if err != nil {
		p.bot.SendMessage(channel, fmt.Sprintf("🔴 API call error: %v", err))
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		p.bot.SendMessage(channel, fmt.Sprintf("🔴 Error reading response: %v", err))
		return
	}

	if resp.StatusCode != 200 {
		p.bot.SendMessage(channel, fmt.Sprintf("🔴 API error: %s", string(bodyBytes)))
		return
	}

	var res responseBody
	err = json.Unmarshal(bodyBytes, &res)
	if err != nil {
		p.bot.SendMessage(channel, fmt.Sprintf("🔴 Error decoding response: %v", err))
		return
	}

	if len(res.Choices) == 0 {
		p.bot.SendMessage(channel, "⚠️ No response received from ChatGPT.")
		return
	}

	answer := res.Choices[0].Message.Content

	const maxLen = 400
	if len(answer) > maxLen {
		answer = answer[:maxLen] + "..."
	}

	lines := strings.Split(answer, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			p.bot.SendMessage(channel, fmt.Sprintf("💬 ChatGPT says: %s", line))
			time.Sleep(300 * time.Millisecond)
		}
	}
}

func (p *ChatGPTPlugin) OnTick() []YnMIrC.Message {
	return nil
}