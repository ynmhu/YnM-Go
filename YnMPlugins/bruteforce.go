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
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord" // Discord package import
)

type BruteforceAttackPlugin struct {
	bot          *YnMIrC.Client
	discord      *discord.DiscordAdapter // Discord adapter
	logPath      string
	channels     []string
	discordChannels []string // Külön lista Discord csatornákhoz
}

func NewBruteforceAttackPlugin(bot *YnMIrC.Client, config *YnMConfig.Config, discordAdapter *discord.DiscordAdapter) *BruteforceAttackPlugin {
	// Szűrd ki a Discord csatornákat (amik számok)
	var discordChannels []string
	var ircChannels []string
	
	for _, channel := range config.BruteforceAttack.Channels {
		// Ha a channel csak számokat tartalmaz, akkor Discord channel
		if isDiscordChannel(channel) {
			discordChannels = append(discordChannels, channel)
		} else {
			ircChannels = append(ircChannels, channel)
		}
	}
	
	return &BruteforceAttackPlugin{
		bot:             bot,
		discord:         discordAdapter,
		logPath:         config.BruteforceAttack.LogPath,
		channels:        ircChannels,
		discordChannels: discordChannels,
	}
}

// isDiscordChannel ellenőrzi, hogy a channel ID csak számokat tartalmaz-e (Discord channel ID)
func isDiscordChannel(channel string) bool {
	// Discord channel ID-k általában csak számok
	for _, char := range channel {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(channel) > 0
}

func (p *BruteforceAttackPlugin) Start() {
	go p.watchLog()
}

func (p *BruteforceAttackPlugin) watchLog() {
	failedRegex := regexp.MustCompile(`(Failed password|authentication failure|Invalid user|sudo:.*authentication failure|su: Authentication failure).*?(?:from|user)?\s*(\d+\.\d+\.\d+\.\d+)?`)
	successRegex := regexp.MustCompile(`Accepted password for (\w+) from (\d+\.\d+\.\d+\.\d+)`)

	// Brute force támadás észleléshez
	failedAttempts := make(map[string]int)
	lastReset := time.Now()

	for {
		file, err := os.Open(p.logPath)
		if err != nil {
			log.Printf("[BruteforceAttackPlugin] Nem lehet megnyitni az auth.log fájlt: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		file.Seek(0, os.SEEK_END)
		reader := bufio.NewReader(file)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			// Óránként reseteljük a számlálókat
			if time.Since(lastReset) > time.Hour {
				failedAttempts = make(map[string]int)
				lastReset = time.Now()
			}

			if matches := failedRegex.FindStringSubmatch(line); matches != nil {
				ip := matches[len(matches)-1]
				if ip != "" {
					failedAttempts[ip]++
					
					// Brute force támadás riasztás
					if failedAttempts[ip] >= 5 {
						msg := fmt.Sprintf("[🚨] BRUTE FORCE TÁMADÁS ÉSZLELVE! IP: %s (%d sikertelen próbálkozás)", ip, failedAttempts[ip])
						p.sendToAllChannels(msg)
						// Reseteljük, hogy ne küldjön túl sok üzenetet
						failedAttempts[ip] = 0
					} else {
						msg := fmt.Sprintf("[🛡️]  Sikertelen bejelentkezés: %s (%d/5)", ip, failedAttempts[ip])
						p.sendToAllChannels(msg)
					}
				}
			} else if matches := successRegex.FindStringSubmatch(line); matches != nil {
				user := matches[1]
				ip := matches[2]
				// Sikeres bejelentkezés után reseteljük a számlálót
				delete(failedAttempts, ip)
				msg := fmt.Sprintf("[✅] Sikeres bejelentkezés: %s  %s (port: 22)", user, ip)
				p.sendToAllChannels(msg)
			}
		}
	}
}

func (p *BruteforceAttackPlugin) sendToAllChannels(msg string) {
	// Küldés IRC csatornákra
	for _, ch := range p.channels {
		p.bot.SendMessage(ch, msg)
	}
	
	// Küldés Discord csatornákra
	for _, ch := range p.discordChannels {
		if p.discord != nil {
			err := p.discord.SendMessage(ch, msg)
			if err != nil {
				log.Printf("[BruteforceAttackPlugin] Hiba Discord üzenet küldéskor: %v", err)
			}
		}
	}
}

// Régi metódus kompatibilitásért
func (p *BruteforceAttackPlugin) sendToChannels(msg string) {
	p.sendToAllChannels(msg)
}