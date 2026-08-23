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
	"io"
	"net/http"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
)

// rateLimitedError jelzi, hogy a hívás upstream 429-et kapott (kvóta/rate limit)
type rateLimitedError struct {
	raw string
}

func (e *rateLimitedError) Error() string {
	return "rate limited: " + e.raw
}

type ChatGPTPlugin struct {
	bot         *YnMIrC.Client
	adminPlugin *owner.YnmAdminPlugin
	Db          *YnMDb.AdminDB
	apiKey      string
	apiURL      string
	model       string   // elsődleges modell
	fallbacks   []string // tartalék modellek, sorrendben próbálva, ha az elsődleges rate-limitelt
	timeout     time.Duration
}

// NewChatGPTPlugin - apiURL: pl. "https://openrouter.ai/api/v1/chat/completions"
//                    model:  elsődleges modell, pl. "google/gemma-4-26b-a4b-it:free"
//
// A fallback modell-lista jelenleg egy kis, ésszerű, ingyenes/olcsó default készlet.
// Ha szeretnéd testreszabni, hívd meg a SetFallbackModels-t regisztráció után.
func NewChatGPTPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, db *YnMDb.AdminDB, apiKey string, apiURL string, model string, timeout time.Duration) *ChatGPTPlugin {
	return &ChatGPTPlugin{
		bot:         bot,
		adminPlugin: adminPlugin,
		Db:          db,
		apiKey:      apiKey,
		apiURL:      apiURL,
		model:       model,
		fallbacks: []string{
			// openrouter/free: hivatalos OpenRouter router, ami maga választ egy
			// éppen elérhető, működő ingyenes chat modellt, és kezeli a failovert.
			// Ez megbízhatóbb, mint kézzel felsorolt konkrét :free modellazonosítók,
			// mert nem kell tudnunk előre, melyik modell chat-kompatibilis és épp elérhető.
			"openrouter/free",
		},
		timeout: timeout,
	}
}

// SetFallbackModels lehetővé teszi a tartalék modell-lista felülírását regisztráció után.
func (p *ChatGPTPlugin) SetFallbackModels(models []string) {
	p.fallbacks = models
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
		return fmt.Sprintf("Usage: %sgpt <question>", prefix)
	}
	prompt := strings.Join(parts[1:], " ")

	return p.askSync(prompt)
}

// callAPIOnce - egyetlen HTTP hívás egy adott modellel, retry nélkül
func (p *ChatGPTPlugin) callAPIOnce(prompt string, model string) (string, error) {
	type requestBody struct {
		Model    string              `json:"model"`
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
		Model: model,
		Messages: []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error preparing request: %w", err)
	}

	client := &http.Client{Timeout: p.timeout}

	req, err := http.NewRequest("POST", p.apiURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("HTTP-Referer", "https://ynm.hu")
	req.Header.Set("X-Title", "YnM-Go IRC Bot")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		// 429 - upstream rate limit, ezt külön jelezzük, hogy a hívó másik modellre válthasson
		return "", &rateLimitedError{raw: string(bodyBytes)}
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res responseBody
	err = json.Unmarshal(bodyBytes, &res)
	if err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if len(res.Choices) == 0 {
		return "", fmt.Errorf("no response received from model")
	}

	return res.Choices[0].Message.Content, nil
}

// callAPI - végigpróbálja az elsődleges modellt, majd rate limit esetén a fallback listát.
// Minden modellnél egy gyors retry is történik (rövid backoff), mielőtt továbblépne a következőre.
func (p *ChatGPTPlugin) callAPI(prompt string) (string, error) {
	models := append([]string{p.model}, p.fallbacks...)

	const retriesPerModel = 1
	backoffBase := 1500 * time.Millisecond

	var lastErr error
	for _, model := range models {
		backoff := backoffBase
		for attempt := 0; attempt <= retriesPerModel; attempt++ {
			answer, err := p.callAPIOnce(prompt, model)
			if err == nil {
				return answer, nil
			}

			lastErr = err

			if _, isRateLimit := err.(*rateLimitedError); isRateLimit {
				if attempt < retriesPerModel {
					time.Sleep(backoff)
					backoff *= 2
					continue
				}
				// elfogytak a próbálkozások ennél a modellnél -> ugrás a következő modellre
				break
			}

			// nem rate-limit jellegű hiba -> a következő modell más providerhez tartozhat, úgyhogy engedjük tovább is
			break
		}
	}

	if rlErr, ok := lastErr.(*rateLimitedError); ok {
		return "", fmt.Errorf("minden elérhető ingyenes modell túlterhelt jelenleg (rate limit) [%s]", rlErr.raw)
	}

	return "", lastErr
}

// truncateRunes - UTF-8 biztos vágás (ékezetes karakterek nem törnek el)
func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

// askSync - szinkron hívás, egyetlen üzenetként adja vissza a választ
func (p *ChatGPTPlugin) askSync(prompt string) string {
	answer, err := p.callAPI(prompt)
	if err != nil {
		return fmt.Sprintf("🔴 %v", err)
	}

	answer = truncateRunes(answer, 400)

	return fmt.Sprintf("💬 : %s", answer)
}

// askAsync - aszinkron hívás, soronként küldi el a választ a csatornára
func (p *ChatGPTPlugin) askAsync(prompt, channel string) {
	answer, err := p.callAPI(prompt)
	if err != nil {
		p.bot.SendMessage(channel, fmt.Sprintf("🔴 %v", err))
		return
	}

	answer = truncateRunes(answer, 400)

	lines := strings.Split(answer, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			p.bot.SendMessage(channel, fmt.Sprintf("💬 says: %s", line))
			time.Sleep(300 * time.Millisecond)
		}
	}
}

func (p *ChatGPTPlugin) OnTick() []YnMIrC.Message {
	return nil
}