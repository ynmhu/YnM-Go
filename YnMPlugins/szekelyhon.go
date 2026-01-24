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
	"sync"
	"time"
	"github.com/mmcdole/gofeed"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
)

type SzekelyhonPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	channels        []string
	discordChannels []string
	startHour       int
	endHour         int
	interval        time.Duration
	lastCheck       *time.Time
	mutex           sync.RWMutex
	ticker          *time.Ticker
	stopChan        chan struct{}
}

// Új konstruktor: config-ból automatikusan szétválogatja IRC és Discord csatornákat
func NewSzekelyhonPluginWithDiscord(bot *YnMIrC.Client, config *YnMConfig.Config, discordAdapter *discord.DiscordAdapter) *SzekelyhonPlugin {
	var discordChannels []string
	var ircChannels []string
	
	//log.Printf("🔍 Székelyhon csatornák feldolgozása...")
	
	// Csatornák szétválogatása
	for _, channel := range config.SzekelyhonChannels {
		if isDiscordChannelSzekelyhon(channel) {
			discordChannels = append(discordChannels, channel)
			//log.Printf("  🎮 Discord csatorna: %s", channel)
		} else {
			ircChannels = append(ircChannels, channel)
			//log.Printf("  📡 IRC csatorna: %s", channel)
		}
	}
	
	// JAVÍTÁS: 24 órával korábbi időpont, hogy az első futáskor küldje a mai híreket
	past := time.Now().Add(-24 * time.Hour)
	interval := parseDurationSzekelyhon(config.SzekelyhonInterval)
	
	//log.Printf("⏰ Székelyhon beállítások: interval=%v, óra=%d-%d", interval, config.SzekelyhonStartHour, config.SzekelyhonEndHour)
	//log.Printf("📊 Csatorna összesítő: %d IRC, %d Discord", len(ircChannels), len(discordChannels))
//	log.Printf("🕐 Utolsó ellenőrzés beállítva: %s (24 órával ezelőtt)", past.Format("2006-01-02 15:04:05"))
	
	return &SzekelyhonPlugin{
		bot:             bot,
		discord:         discordAdapter,
		channels:        ircChannels,
		discordChannels: discordChannels,
		interval:        interval,
		startHour:       config.SzekelyhonStartHour,
		endHour:         config.SzekelyhonEndHour,
		lastCheck:       &past, // 24 órával korábbi időpont
		stopChan:        make(chan struct{}),
	}
}

// Eredeti IRC-only konstruktor (backward compatibility)
func NewSzekelyhonPlugin(bot *YnMIrC.Client, channels []string, interval time.Duration, startHour, endHour int) *SzekelyhonPlugin {
	// JAVÍTÁS: 24 órával korábbi időpont, hogy az első futáskor küldje a mai híreket
	past := time.Now().Add(-24 * time.Hour)
	log.Printf("🕐 Székelyhon IRC-only: Utolsó ellenőrzés beállítva: %s", past.Format("2006-01-02 15:04:05"))
	return &SzekelyhonPlugin{
		bot:       bot,
		channels:  channels,
		interval:  interval,
		startHour: startHour,
		endHour:   endHour,
		lastCheck: &past, // 24 órával korábbi időpont
		stopChan:  make(chan struct{}),
	}
}

// isDiscordChannelSzekelyhon ellenőrzi, hogy a channel ID csak számokat tartalmaz-e (Discord channel ID)
func isDiscordChannelSzekelyhon(channel string) bool {
	if len(channel) == 0 {
		return false
	}
	for _, char := range channel {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// parseDurationSzekelyhon string időtartamot konvertál time.Duration-ra
func parseDurationSzekelyhon(durationStr string) time.Duration {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		log.Printf("⚠️ Székelyhon: Érvénytelen időtartam (%s), alapértelmezett 30 perc használata", durationStr)
		return 30 * time.Minute
	}
	//log.Printf("✅ Székelyhon interval sikeresen beállítva: %v", duration)
	return duration
}

func (p *SzekelyhonPlugin) Start() {
	//log.Printf("ℹ️ Székelyhon plugin elindult. Időzítés: %v, időablak: %02d–%02d", p.interval, p.startHour, p.endHour)
	if len(p.channels) > 0 {
	//	log.Printf("📡 IRC csatornák: %v", p.channels)
	}
	if len(p.discordChannels) > 0 {
	//	log.Printf("🎮 Discord csatornák: %v", p.discordChannels)
	}
	
	// Üres csatorna lista figyelmeztetés
	if len(p.channels) == 0 && len(p.discordChannels) == 0 {
		log.Printf("⚠️ FIGYELEM: Székelyhon plugin csatorna lista üres!")
	}
	
	p.ticker = time.NewTicker(p.interval)
	
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.checkAndSendNews()
			case <-p.stopChan:
				p.ticker.Stop()
				return
			}
		}
	}()
}

func (p *SzekelyhonPlugin) Stop() {
	close(p.stopChan)
	if p.ticker != nil {
		p.ticker.Stop()
	}
}

func (p *SzekelyhonPlugin) checkAndSendNews() {
	now := time.Now()
	log.Printf("🕒 Székelyhon ellenőrzés fut: %02d:%02d", now.Hour(), now.Minute())
	
	if now.Hour() < p.startHour || now.Hour() >= p.endHour {
		log.Printf("⏰ Székelyhon: Az aktuális idő (%02d:%02d) kívül esik az aktív időablakon (%02d–%02d)", 
			now.Hour(), now.Minute(), p.startHour, p.endHour)
		return
	}
	
	feed, err := gofeed.NewParser().ParseURL("https://szekelyhon.ro/rss/szekelyhon_hirek.xml")
	if err != nil {
		log.Printf("❌ Székelyhon RSS olvasási hiba: %v", err)
		return
	}
	
	if len(feed.Items) == 0 {
		log.Printf("📰 Székelyhon: Nincsenek elérhető hírek")
		return
	}
	
	log.Printf("📊 Székelyhon: %d hír elérhető az RSS-ben", len(feed.Items))
	
	latest := feed.Items[0]
	
	log.Printf("📰 Legfrissebb hír: %s", latest.Title)
	
	if latest.PublishedParsed == nil {
		log.Printf("⚠️ Székelyhon: A legfrissebb hír dátuma nem értelmezhető")
		return
	}
	
	log.Printf("📅 Hír dátuma: %s", latest.PublishedParsed.Format("2006-01-02 15:04:05"))
	
	p.mutex.RLock()
	lastCheck := p.lastCheck
	p.mutex.RUnlock()
	
	if lastCheck != nil {
		log.Printf("🕐 Utolsó ellenőrzés: %s", lastCheck.Format("2006-01-02 15:04:05"))
		log.Printf("🔍 Összehasonlítás: hír újabb? %v", latest.PublishedParsed.After(*lastCheck))
	}
	
	if lastCheck == nil || latest.PublishedParsed.After(*lastCheck) {
		p.mutex.Lock()
		p.lastCheck = latest.PublishedParsed
		p.mutex.Unlock()
		
		msg := "📰: " + latest.Title + " - Link: " + latest.Link + " - Közzétéve: " + latest.Published
		
		// IRC csatornák
		for _, ch := range p.channels {
			p.bot.SendMessage(ch, msg)
			log.Printf("✅ Székelyhon hír elküldve IRC %s csatornára: %s", ch, latest.Title)
		}
		
		// Discord csatornák
		if p.discord != nil {
			for _, ch := range p.discordChannels {
				err := p.discord.SendMessage(ch, msg)
				if err != nil {
					log.Printf("❌ Székelyhon Discord hiba (%s): %v", ch, err)
				} else {
					log.Printf("✅ Székelyhon hír elküldve Discord %s csatornára: %s", ch, latest.Title)
				}
			}
		} else {
			log.Printf("⚠️ Discord adapter nincs inicializálva, Discord csatornák kihagyása")
		}
	} else {
		log.Printf("ℹ️ Székelyhon: Nincs újabb hír a legutóbbi ellenőrzés óta")
	}
}

func (p *SzekelyhonPlugin) Name() string {
	return "Székelyhon RSS"
}