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
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
)

type MediaConfig struct {
	YnMMedia map[string][]string `yaml:"YnM_WebHook"` // lista minden eseményhez
}

type WebhookPlugin struct {
	bot           *YnMIrC.Client
	discordPlugin *discord.DiscordPlugin
	config        MediaConfig
	defaultChan   string
}

func NewWebhookPlugin(bot *YnMIrC.Client, discordPlugin *discord.DiscordPlugin) *WebhookPlugin {
	config := loadMediaConfig("YnMConfig/ynm.yaml")
	defChan := "#YnM"
	if chans, ok := config.YnMMedia["default"]; ok && len(chans) > 0 {
		defChan = chans[0]
	}
	return &WebhookPlugin{
		bot:           bot,
		discordPlugin: discordPlugin,
		config:        config,
		defaultChan:   defChan,
	}
}

func loadMediaConfig(path string) MediaConfig {
	var cfg MediaConfig
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[WebhookPlugin] ⚠️ Nem sikerült betölteni a config fájlt (%s): %v", path, err)
		return cfg
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Printf("[WebhookPlugin] ⚠️ Hibás YAML fájl (%s): %v", path, err)
	}
	return cfg
}

func (p *WebhookPlugin) StartHTTP(port string) {
	http.HandleFunc("/webhook", p.handleWebhook)
	log.Printf("[WebhookPlugin] Webhook szerver elindult a :%s porton", port)
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("[WebhookPlugin] HTTP szerver hiba: %v", err)
		}
	}()
}

func (p *WebhookPlugin) handleWebhook(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// 1. Teljes body kiolvasása naplózáshoz
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[WebhookPlugin] ❌ Hiba a body olvasásánál: %v", err)
		http.Error(w, "Hiba a body olvasásánál", http.StatusBadRequest)
		return
	}
	
	bodyString := string(bodyBytes)
	
	// 2. Body visszaállítása a JSON dekódoláshoz
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	
	// 3. JSON dekódolás
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("[WebhookPlugin] ❌ Hibás JSON: %v", err)
		log.Printf("[WebhookPlugin] ❌ RAW Body ami hibát okozott: %s", bodyString)
		http.Error(w, "Hibás JSON", http.StatusBadRequest)
		return
	}

	// 4. Bejövő adatok naplózása
	//log.Printf("")
	//log.Printf("═══════════════════════════════════════════════════════════")
	//log.Printf("[WebhookPlugin] 📥 ÚJ WEBBOOK ÉRKEZETT")
	//log.Printf("[WebhookPlugin] 📄 RAW JSON body:")
	//log.Printf("%s", bodyString)
	//log.Printf("[WebhookPlugin] 📊 Dekódolt JSON mezők:")
	//for key, value := range data {
		//log.Printf("[WebhookPlugin]   %s: %v (típus: %T)", key, value, value)
	//}

	getString := func(key string) string {
		if val, ok := data[key]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
		return ""
	}

	notificationType := getString("NotificationType")
	if notificationType == "" {
		// Automatikus detektálás: ha van msg mező és nincs NotificationType, de van heartbeat vagy monitor
		if _, hasMsg := data["msg"]; hasMsg {
			if heartbeat, ok := data["heartbeat"].(map[string]interface{}); ok && heartbeat != nil {
				if monitor, ok := data["monitor"].(map[string]interface{}); ok && monitor != nil {
					notificationType = "Kuma"
					//log.Printf("[WebhookPlugin] 🐻 Uptime Kuma üzenet automatikusan detektálva")
				}
			}
		}
		
		if notificationType == "" {
			notificationType = "Notify"
		}
	}

	username := getString("NotificationUsername")
	device := getString("DeviceName")
	client := getString("Client")
	ip := getString("RemoteEndPoint")
	name := getString("Name")
	playbackPos := getString("PlaybackPosition")

	ircMessage := ""
	channels := p.config.YnMMedia[notificationType]
	if len(channels) == 0 {
		channels = []string{p.defaultChan}
	}

	sendToAll := func(msg string) {
		for _, ch := range channels {
			if strings.HasPrefix(ch, "#") {
				// IRC csatorna
				p.bot.SendMessage(ch, msg)
				//log.Printf("[IRC] ✅ Üzenet elküldve: %s", ch)
			} else if len(ch) >= 17 && len(ch) <= 20 {
				// Discord channel ID (általában 18-19 számjegy)
				if p.discordPlugin != nil && p.discordPlugin.Adapter != nil {
					err := p.discordPlugin.Adapter.SendMessage(ch, msg)
					if err != nil {
						log.Printf("[Discord] ❌ Hiba üzenetküldéskor (%s): %v", ch, err)
					} else {
						log.Printf("[Discord] ✅ Üzenet elküldve: %s", ch)
					}
				} else {
					log.Printf("[Discord] ⚠️ Discord plugin nincs inicializálva!")
				}
			} else {
				log.Printf("[sendToAll] ⚠️ Ismeretlen csatorna formátum: '%s'", ch)
			}
		}
	}

	switch notificationType {
case "Notify":
    msg := getString("Message")
    
    // Ha nincs Message mező, de van msg (kisbetűs) mező, akkor azt használjuk
    if msg == "" {
        if rawMsg, ok := data["msg"].(string); ok && rawMsg != "" {
            msg = rawMsg
          //  log.Printf("[WebhookPlugin] 📡 'msg' mezőből kinyert üzenet: %s", msg)
        }
    }
    
  //  log.Printf("[WebhookPlugin] 🔔 Végleges üzenet: '%s'", msg)
    
    if msg == "" {
        msg = "Üzenet érkezett, de üres volt."
    }
    
    ircMessage = fmt.Sprintf("[🤖] 🔔 %s", msg)
	 sendToAll(ircMessage)

	
	case "Kuma":
    //log.Printf("[WebhookPlugin] 🐻 Uptime Kuma webhook érkezett")
    
    var msg string
    monitorName := "Ismeretlen"
    statusText := "❓ Ismeretlen"
    
    if monitorData, ok := data["monitor"].(map[string]interface{}); ok && monitorData != nil {
        if name, ok := monitorData["name"].(string); ok {
            monitorName = name
        }
    }
    
    if heartbeatData, ok := data["heartbeat"].(map[string]interface{}); ok && heartbeatData != nil {
        if status, ok := heartbeatData["status"].(float64); ok {
            switch status {
            case 1:
                statusText = "✅ UP"
            case 0:
                statusText = "🔴 DOWN"
            case 2:
                statusText = "⚠️ PENDING"
            }
        }
        
        if hbMsg, ok := heartbeatData["msg"].(string); ok {
            msg = fmt.Sprintf("[YnM Status] %s - %s: %s", monitorName, statusText, hbMsg)
        } else {
            msg = fmt.Sprintf("[YnM Status] %s - %s", monitorName, statusText)
        }
    }
    
    if msg == "" {
        // Ha csak sima msg mező van
        if rawMsg, ok := data["msg"].(string); ok {
            msg = fmt.Sprintf("[YnM Status] %s", rawMsg)
        } else {
            msg = "[YnM Status] Állapotváltozás"
        }
    }
    
    //log.Printf("[WebhookPlugin] 🐻 Kuma üzenet: %s", msg)
	ircMessage = fmt.Sprintf("[🤖] 📊 %s - https://up.ynm.hu", msg)
    sendToAll(ircMessage)
	
	
	case "AuthenticationSuccess":
		lastLoginRaw := getString("LastLoginDate")
		lastActivityRaw := getString("LastActivityDate")
		lastLoginTime, err := time.Parse(time.RFC3339Nano, lastLoginRaw)
		if err != nil {
			lastLoginTime = time.Now()
		}
		lastActivityTime, err := time.Parse(time.RFC3339Nano, lastActivityRaw)
		if err != nil {
			lastActivityTime = time.Now()
		}
		lastLoginLocal := lastLoginTime.Local().Format("2006-01-02 15:04:05")
		lastActivityLocal := lastActivityTime.Local().Format("2006-01-02 15:04:05")
		ircMessage = fmt.Sprintf(
			"[YnM Media] 🔐 %s | 🖥️ %s | 💻 %s | 🌐 %s | ⏰ Bejelentkezés: %s | 🕓 Aktivitás: %s",
			username, device, client, ip, lastLoginLocal, lastActivityLocal)
		sendToAll(ircMessage)

	case "UserCreated":
		ircMessage = fmt.Sprintf("[YnM Media] 👤 Új felhasználó lett hozzáadva: %s", username)
		sendToAll(ircMessage)

	case "UserDeleted":
		ircMessage = fmt.Sprintf("[YnM Media] ❌ Felhasználó törölve: %s", username)
		sendToAll(ircMessage)

	case "UserPasswordChanged":
		ircMessage = fmt.Sprintf("[YnM Media] 🔑 %s jelszava megváltozott!", username)
		sendToAll(ircMessage)

	case "PlaybackStart":
		itemType, _ := data["ItemType"].(string)
		if strings.ToLower(itemType) == "audio" {
			//log.Printf("[WebhookPlugin] 🔊 Audio lejátszás, ignorálva")
			return
		}
		year := getString("Year")
		clientName := getString("ClientName")
		ircMessage = fmt.Sprintf("[YnM Media] ▶ Lejátszás indult: %s (%s) | Felhasználó: %s | Eszköz: %s | IP: %s | Kliens: %s",
			name, year, username, device, ip, clientName)
		sendToAll(ircMessage)

	case "PlaybackStop":
		itemType, _ := data["ItemType"].(string)
		if strings.ToLower(itemType) == "audio" {
			//log.Printf("[WebhookPlugin] 🔊 Audio lejátszás leállt, ignorálva")
			return
		}
		ircMessage = fmt.Sprintf("[YnM Media] ⏹️ Lejátszás leállt: %s | Felhasználó: %s | Eszköz: %s | IP: %s | Pozíció: %s",
			name, username, device, ip, playbackPos)
		sendToAll(ircMessage)

	case "ItemAdded":
		title := cleanTitle(getString("Name"))
		if title == "" {
			title = "Ismeretlen cím"
		}
		genre := getString("Genre")
		if genre == "" {
			genre = "Ismeretlen"
		}
		timestamp := getString("Timestamp")
		if timestamp == "" {
			timestamp = time.Now().Format(time.RFC3339)
		}
		category := getString("Category")
		if category == "" {
			category = "✅ Film"
		}
		runtime := getString("RunTime")
		if runtime == "" {
			runtime = "n.a."
		}
		year := getString("Year")
		if year == "" {
			year = "Ismeretlen év"
		}
		overview := getString("Overview")
		if len(overview) > 300 {
			overview = overview[:297] + "..."
		}

		sendToAll(fmt.Sprintf("「 ✦ %s ✦ 」 | 🎭: %s", title, genre))
		sendToAll(fmt.Sprintf("👆: %s | 📂: %s", timestamp, category))
		sendToAll(fmt.Sprintf("⏰: %s | 📅: %s 🎥", runtime, year))
		if overview != "" {
			sendToAll("📝: " + overview)
		}

	default:
		ircMessage = fmt.Sprintf("[YnM Media] ❓ Ismeretlen esemény: %s | Felhasználó: %s | Cím: %s | Eszköz: %s | Kliense: %s | IP: %s | Pozíció: %s",
			notificationType, username, name, device, client, ip, playbackPos)
		sendToAll(ircMessage)
	}

	//log.Printf("[WebhookPlugin] ✅ Webhook feldolgozva: %s", notificationType)
	//log.Printf("═══════════════════════════════════════════════════════════")
	//log.Printf("")
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook feldolgozva"))
}

func (p *WebhookPlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

func (p *WebhookPlugin) OnTick() []YnMIrC.Message {
	return nil
}

func cleanTitle(rawTitle string) string {
	unwanted := []string{
		"xxx", "dvdrip", "xvid", "x264", "h264", "hdrip", "webrip", "bluray", "brrip", "hunsub", "hundub", "vhsrip",
		"pornoloverblog", "1080p", "720p", "576p", "480p", "hdtv", "-", "_",
	}
	lower := strings.ToLower(rawTitle)
	for _, word := range unwanted {
		lower = strings.ReplaceAll(lower, word, "")
	}
	clean := regexp.MustCompile(`[^a-zA-Z0-9áéíóöőúüűÁÉÍÓÖŐÚÜŰ\s]`).ReplaceAllString(lower, " ")
	clean = strings.Join(strings.Fields(clean), " ")
	return strings.Title(clean)
}