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
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"gopkg.in/yaml.v2"
)

type ExternalTriggerConfig struct {
	ExternalTrigger struct {
		Enabled          bool                `yaml:"enabled"`
		WebhookURL       string              `yaml:"webhook_url"`
		SourceBot        string              `yaml:"source_bot"`
		RateLimit        int                 `yaml:"rate_limit_seconds"`
		ChannelTriggers  map[string][]string `yaml:"channel_triggers"`  // ← ÚJ
		GlobalTriggers   []string            `yaml:"global_triggers"`    // ← ÚJ
	} `yaml:"ExternalTrigger"`
}

type ExternalTriggerPlugin struct {
	bot         *YnMIrC.Client
	config      ExternalTriggerConfig
	lastTrigger map[string]time.Time
}

func NewExternalTriggerPlugin(bot *YnMIrC.Client) *ExternalTriggerPlugin {
	config := loadExternalTriggerConfig("YnMConfig/ynm.yaml")
	return &ExternalTriggerPlugin{
		bot:         bot,
		config:      config,
		lastTrigger: make(map[string]time.Time),
	}
}

func loadExternalTriggerConfig(path string) ExternalTriggerConfig {
	var cfg ExternalTriggerConfig
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[ExternalTrigger] ⚠️ Nem sikerült betölteni a config fájlt (%s): %v", path, err)
		return cfg
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Printf("[ExternalTrigger] ⚠️ Hibás YAML fájl (%s): %v", path, err)
	}
	
	// Default rate limit
	if cfg.ExternalTrigger.RateLimit == 0 {
		cfg.ExternalTrigger.RateLimit = 30
	}
	
	return cfg
}

type WebhookPayload struct {
	NotificationType string `json:"NotificationType"`
	SourceBot        string `json:"SourceBot"`
	Trigger          string `json:"Trigger"`
	SourceChannel    string `json:"SourceChannel"`
	SourceUser       string `json:"SourceUser"`
	OriginalMessage  string `json:"OriginalMessage"`
	Timestamp        string `json:"Timestamp"`
}

func (p *ExternalTriggerPlugin) sendWebhook(payload WebhookPayload) error {
	if p.config.ExternalTrigger.WebhookURL == "" {
		return nil
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", p.config.ExternalTrigger.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[ExternalTrigger] ❌ Webhook HTTP hiba: %d", resp.StatusCode)
		return nil
	}

	log.Printf("[ExternalTrigger] ✅ Webhook elküldve: %s", payload.Trigger)
	return nil
}

func (p *ExternalTriggerPlugin) HandleMessage(msg YnMIrC.Message) string {
	if !p.config.ExternalTrigger.Enabled {
		return ""
	}
	
	lowerNick := strings.ToLower(msg.Nick)
    // 1. ha üres → server line
    if lowerNick == "" {
        return ""
    }

    // 2. ha domain formájú (van benne ".")
    if strings.Contains(lowerNick, ".") {
        return ""
    }
	// Rate limiting
	rateLimit := time.Duration(p.config.ExternalTrigger.RateLimit) * time.Second
	if lastTime, ok := p.lastTrigger[msg.Nick]; ok {
		if time.Since(lastTime) < rateLimit {
			remaining := int(rateLimit.Seconds() - time.Since(lastTime).Seconds())
			log.Printf("[ExternalTrigger] ⏱️ Rate limit: %s (még %d másodperc)", msg.Nick, remaining)
			return ""
		}
	}

	lowerMsg := strings.ToLower(msg.Text)
	lowerChannel := strings.ToLower(msg.Channel)
	
	// 1. Csatorna-specifikus triggerek ellenőrzése (case-insensitive)
	for configChannel, triggers := range p.config.ExternalTrigger.ChannelTriggers {
		if strings.ToLower(configChannel) == lowerChannel {
			for _, trigger := range triggers {
				if strings.Contains(lowerMsg, strings.ToLower(trigger)) {
					p.sendTriggerWebhook(msg, trigger)
					return ""
				}
			}
		}
	}
	
	// 2. Globális triggerek ellenőrzése (minden csatornán)
	for _, trigger := range p.config.ExternalTrigger.GlobalTriggers {
		if strings.Contains(lowerMsg, strings.ToLower(trigger)) {
			p.sendTriggerWebhook(msg, trigger)
			return ""
		}
	}

	return ""
}

func (p *ExternalTriggerPlugin) sendTriggerWebhook(msg YnMIrC.Message, trigger string) {
	log.Printf("[ExternalTrigger] 🔔 Trigger észlelve: '%s' | Channel: %s | User: %s", trigger, msg.Channel, msg.Nick)
	
	p.lastTrigger[msg.Nick] = time.Now()

	payload := WebhookPayload{
		NotificationType: "ExternalTrigger",
		SourceBot:        p.config.ExternalTrigger.SourceBot,
		Trigger:          trigger,
		SourceChannel:    msg.Channel,
		SourceUser:       msg.Nick,
		OriginalMessage:  msg.Text,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	go p.sendWebhook(payload)
}

func (p *ExternalTriggerPlugin) OnTick() []YnMIrC.Message {
	return nil
}