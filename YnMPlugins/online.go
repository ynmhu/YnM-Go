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
	"log"
	"strings"
	"time"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
)

type JoinMessagePlugin struct {
	bot                  *YnMIrC.Client
	discordAdapter       *discord.DiscordAdapter
	ircChannels          []string    // Több IRC csatorna
	discordChannels      []string    // Több Discord csatorna
	message              string
	delay                time.Duration
	enableDiscord        bool
	hasSent              map[string]bool // Nyomon követjük mely csatornákba küldtünk már
}

func NewJoinMessagePluginWithDiscord(bot *YnMIrC.Client, cfg *YnMConfig.Config, discordAdapter *discord.DiscordAdapter) *JoinMessagePlugin {
	// Normalizáljuk az IRC csatorna neveket (kisbetűssé)
	var ircChannels []string
	for _, channel := range cfg.JoinMessageChannels {
		ircChannels = append(ircChannels, strings.ToLower(channel))
	}
	
	// Inicializáljuk a sent tracker-t
	sentTracker := make(map[string]bool)
	for _, channel := range ircChannels {
		sentTracker[channel] = false
	}
	
	return &JoinMessagePlugin{
		bot:             bot,
		discordAdapter:  discordAdapter,
		ircChannels:     ircChannels,
		discordChannels: cfg.JoinMessageDiscordChannels,
		message:         cfg.JoinMessageText,
		delay:           cfg.JoinMessageDelay,
		enableDiscord:   cfg.Plugins.EnableJoinMessageDiscord,
		hasSent:         sentTracker,
	}
}

// HandleMessage - Plugin interfész implementációja
func (p *JoinMessagePlugin) HandleMessage(msg YnMIrC.Message) string {
	return ""
}

// OnTick - Plugin interfész implementációja  
func (p *JoinMessagePlugin) OnTick() []YnMIrC.Message {
	return nil
}

func (p *JoinMessagePlugin) Start() {
	//log.Printf("ℹ️ JoinMessage plugin elindult. IRC csatornák: %v, Discord csatornák: %v, Késleltetés: %v", 
	//	p.ircChannels, p.discordChannels, p.delay)
	
	// Használd az OnConnect callback-et - ez biztonságosabb
	originalOnConnect := p.bot.OnConnect
	p.bot.OnConnect = func() {
		// Hívd meg az eredeti callback-et ha van
		if originalOnConnect != nil {
			originalOnConnect()
		}
		
		// Join message küldése MINDEN IRC csatornába
		go func() {
			time.Sleep(p.delay)
			
			for _, ircChannel := range p.ircChannels {
				if !p.hasSent[ircChannel] {
					p.bot.SendMessage(ircChannel, p.message)
				//	log.Printf("✅ IRC csatlakozási üzenet elküldve a %s csatornába: %s", ircChannel, p.message)
					p.hasSent[ircChannel] = true
				}
			}
			
			// Discord üzenet küldése MINDEN Discord csatornába
			if p.enableDiscord && p.discordAdapter != nil && len(p.discordChannels) > 0 {
				for _, discordChannel := range p.discordChannels {
					err := p.discordAdapter.SendMessage(discordChannel, p.message)
					if err != nil {
				//		log.Printf("❌ Discord csatlakozási üzenet küldési hiba a %s csatornába: %v", discordChannel, err)
					} else {
				//		log.Printf("✅ Discord csatlakozási üzenet elküldve a %s csatornába", discordChannel)
					}
				}
			}
		}()
	}
}

func (p *JoinMessagePlugin) Stop() {
	log.Printf("🛑 JoinMessage plugin leállt")
	// Reset állapot
	for channel := range p.hasSent {
		p.hasSent[channel] = false
	}
}

func (p *JoinMessagePlugin) handleBotJoin(channel, nick, hostmask string) {
	// Ellenőrizzük, hogy a bot saját maga csatlakozott-e
	if nick != p.bot.GetNick() {
		return
	}
	
	// Ellenőrizzük, hogy a csatlakozott csatorna a mi listánkban van-e
	channelLower := strings.ToLower(channel)
	
	for _, targetChannel := range p.ircChannels {
		if channelLower == targetChannel {
			// Megakadályozzuk többszöri küldést erre a csatornára
			if p.hasSent[targetChannel] {
				log.Printf("ℹ️ JoinMessage már elküldve a %s csatornába korábban, újraküldés kihagyva", targetChannel)
				return
			}
			
			log.Printf("🤖 Bot csatlakozott a %s csatornára, üzenet küldése %v késleltetéssel", targetChannel, p.delay)
			p.hasSent[targetChannel] = true
			
			// Várakozás a megadott ideig aszinkron módon
			go p.sendJoinMessages(targetChannel)
			break
		}
	}
}

func (p *JoinMessagePlugin) sendJoinMessages(ircChannel string) {
	time.Sleep(p.delay)
	
	// Üzenet küldése IRC-re a megadott csatornába
	p.bot.SendMessage(ircChannel, p.message)
	log.Printf("✅ IRC csatlakozási üzenet elküldve a %s csatornába: %s", ircChannel, p.message)
	
	// Discord üzenet küldése MINDEN megadott Discord csatornába, ha engedélyezve van
	if p.enableDiscord && p.discordAdapter != nil && len(p.discordChannels) > 0 {
		for _, discordChannel := range p.discordChannels {
			err := p.discordAdapter.SendMessage(discordChannel, p.message)
			if err != nil {
				log.Printf("❌ Discord csatlakozási üzenet küldési hiba a %s csatornába: %v", discordChannel, err)
			} else {
				log.Printf("✅ Discord csatlakozási üzenet elküldve a %s csatornába", discordChannel)
			}
		}
	}
}

func (p *JoinMessagePlugin) Name() string {
	return "Join Message"
}