// Szerzői jog: 2024, YnM
// Szerkesztette: Markus (markus@ynm.hu)
// Minden jog fenntartva.
package plugins

import (
	"log"
	"sync"
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
	mutex     sync.RWMutex
	ticker    *time.Ticker
	stopChan  chan struct{}
}

func NewSzekelyhonPlugin(bot *irc.Client, channels []string, interval time.Duration, startHour, endHour int) *SzekelyhonPlugin {
	// Inicializáljuk a lastCheck-et az aktuális időre, hogy ne küldjön minden hírt az első futáskor
	now := time.Now()
	return &SzekelyhonPlugin{
		bot:       bot,
		channels:  channels,
		interval:  interval,
		startHour: startHour,
		endHour:   endHour,
		lastCheck: &now,
		stopChan:  make(chan struct{}),
	}
}

func (p *SzekelyhonPlugin) Start() {
	log.Printf("ℹ️ Székelyhon plugin elindult. Időzítés: %v, időablak: %02d–%02d", p.interval, p.startHour, p.endHour)
	
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
	
	// Az első (legfrissebb) elem vizsgálata
	latest := feed.Items[0]
	if latest.PublishedParsed == nil {
		log.Printf("⚠️ Székelyhon: A legfrissebb hír dátuma nem értelmezhető")
		return
	}
	
	p.mutex.RLock()
	lastCheck := p.lastCheck
	p.mutex.RUnlock()
	
	if lastCheck == nil || latest.PublishedParsed.After(*lastCheck) {
		p.mutex.Lock()
		p.lastCheck = latest.PublishedParsed
		p.mutex.Unlock()
		
		msg := "📰: " + latest.Title + " - Link: " + latest.Link + " - Közzétéve: " + latest.Published
		
		for _, ch := range p.channels {
			p.bot.SendMessage(ch, msg)
			log.Printf("✅ Székelyhon hír elküldve a %s csatornára: %s", ch, latest.Title)
		}
	}
}
func (p *SzekelyhonPlugin) Name() string {
	return "Székelyhon RSS"
}