// Szerzői jog: 2024, YnM
// Szerkesztette: Markus (markus@ynm.hu)
// Minden jog fenntartva.

package plugins

import (
	"log"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/ynmhu/YnM-Go/irc"
)

type SzekelyhonPlugin struct {
	bot       *irc.Client
	channels  []string
	startHour int
	endHour   int
	interval  time.Duration
	lastCheck *time.Time
}




func NewSzekelyhonPlugin(bot *irc.Client, channels []string, interval time.Duration, startHour, endHour int) *SzekelyhonPlugin {
	return &SzekelyhonPlugin{
		bot:       bot,
		channels:  channels,
		interval:  interval,
		startHour: startHour,
		endHour:   endHour,
	}
}

func (p *SzekelyhonPlugin) Start() {
	log.Printf("ℹ️ Székelyhon plugin elindult. Időzítés: %v, időablak: %02d–%02d", p.interval, p.startHour, p.endHour)

	ticker := time.NewTicker(p.interval)
	go func() {
		for range ticker.C {
			p.checkAndSendNews()
		}
	}()
}


func (p *SzekelyhonPlugin) checkAndSendNews() {
	now := time.Now()
	log.Printf("🕒 Székelyhon ellenőrzés fut: %02d:%02d", now.Hour(), now.Minute())
	if now.Hour() < p.startHour || now.Hour() >= p.endHour {
		return
	}

	feed, err := gofeed.NewParser().ParseURL("https://szekelyhon.ro/rss/szekelyhon_hirek.xml")
	if err != nil {
		log.Printf("Székelyhon RSS olvasási hiba: %v", err)
		return
	}

	if len(feed.Items) == 0 {
		return
	}

	// Az első (legfrissebb) elem vizsgálata
	latest := feed.Items[0]
	if latest.PublishedParsed == nil {
		return
	}

	if p.lastCheck == nil || latest.PublishedParsed.After(*p.lastCheck) {
		p.lastCheck = latest.PublishedParsed

		msg := "📰: " + latest.Title + " - Link: " + latest.Link + " - Közzétéve: " + latest.Published
		for _, ch := range p.channels {
			p.bot.SendMessage(ch, msg)
		}
	}
}
