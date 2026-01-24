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
package YnM

import (
	"fmt"
	"strings"
	"time"
	"sync"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
)

type UserRateLimit struct {
	requests    []time.Time
	violations  int
	bannedUntil time.Time
}

type CTCPPlugin struct {
	bot         *YnMIrC.Client
	rateLimiter map[string]*UserRateLimit
	mutex       sync.RWMutex
}

func NewCTCPPlugin(bot *YnMIrC.Client) *CTCPPlugin {
	return &CTCPPlugin{
		bot:         bot,
		rateLimiter: make(map[string]*UserRateLimit),
	}
}

// Rate limit ellenőrzés komplex szabályokkal
func (p *CTCPPlugin) isRateLimited(nick string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	now := time.Now()
	
	// Ha nincs még bejegyzés, létrehozzuk
	if _, exists := p.rateLimiter[nick]; !exists {
		p.rateLimiter[nick] = &UserRateLimit{
			requests:    make([]time.Time, 0),
			violations:  0,
			bannedUntil: time.Time{},
		}
	}
	
	user := p.rateLimiter[nick]
	
	// Ellenőrizzük hogy még bannolva van-e
	if !user.bannedUntil.IsZero() && now.Before(user.bannedUntil) {
		return true // még mindig bannolva
	}
	
	// Ha a ban lejárt, reseteljük
	if !user.bannedUntil.IsZero() && now.After(user.bannedUntil) {
		user.bannedUntil = time.Time{}
		user.violations = 0
		user.requests = make([]time.Time, 0)
	}
	
	// Eltávolítjuk a 1 percnél régebbi kéréseket
	validRequests := make([]time.Time, 0)
	for _, reqTime := range user.requests {
		if now.Sub(reqTime) <= time.Minute {
			validRequests = append(validRequests, reqTime)
		}
	}
	user.requests = validRequests
	
	// Ellenőrizzük hogy elérte-e a limitet (3 kérés/perc)
	if len(user.requests) >= 3 {
		// Rate limit túllépés - növeljük a violations számát
		user.violations++
		
		// Első túllépés: 1 perc ban
		if user.violations == 1 {
			user.bannedUntil = now.Add(time.Minute)
			fmt.Printf("CTCP: %s rate limited for 1 minute (1st violation)\n", nick)
		}
		// Második túllépés 5 percen belül: 1 nap ban
		if user.violations >= 2 {
			user.bannedUntil = now.Add(24 * time.Hour)
			fmt.Printf("CTCP: %s banned for 24 hours (repeated violation)\n", nick)
		}
		
		return true
	}
	
	// Hozzáadjuk az aktuális kérést
	user.requests = append(user.requests, now)
	
	// Ha több mint 5 perc telt el az utolsó túllépés óta, reseteljük a violations-t
	// (Ez lehetővé teszi hogy 5 perc múlva újra "tiszta lappal" indulhasson)
	if user.violations > 0 && len(user.requests) == 1 {
		// Ha ez az első kérés egy ideje, és nincs más aktív kérés
		// akkor valószínűleg 5+ perc telt el
		lastViolationTime := now.Add(-5 * time.Minute)
		if user.violations == 1 && now.After(lastViolationTime) {
			// Ne reseteljük azonnal, hanem várjunk egy kicsit
			// A tényleges reset a következő cleanup-nál fog megtörténni
		}
	}
	
	return false
}

// Cleanup régi bejegyzéseket és reset violations-t ha szükséges
func (p *CTCPPlugin) cleanup() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	now := time.Now()
	
	for nick, user := range p.rateLimiter {
		// Ha 1 órája nem volt kérés, reseteljük a violations-t
		if len(user.requests) == 0 && user.violations > 0 && user.bannedUntil.IsZero() {
			// Töröljük a bejegyzést teljesen ha régen nem volt aktív
			delete(p.rateLimiter, nick)
			continue
		}
		
		// Ha van aktív kérés, de az utolsó több mint 10 perce volt
		if len(user.requests) > 0 {
			lastRequest := user.requests[len(user.requests)-1]
			if now.Sub(lastRequest) > 10*time.Minute && user.violations > 0 && user.bannedUntil.IsZero() {
				user.violations = 0 // Reset violations 10 perc inaktivitás után
			}
		}
	}
}

func (p *CTCPPlugin) HandleMessage(msg YnMIrC.Message) string {
	if !strings.HasPrefix(msg.Text, "\001") || !strings.HasSuffix(msg.Text, "\001") {
		return ""
	}
	
	ctcpText := strings.Trim(msg.Text, "\001")
	parts := strings.SplitN(ctcpText, " ", 2)
	command := strings.ToUpper(parts[0])
	nick := strings.Split(msg.Sender, "!")[0]
	
	// Rate limit ellenőrzés
	if p.isRateLimited(nick) {
		return ""
	}
	
	var args string
	if len(parts) > 1 {
		args = parts[1]
	}
	
	switch command {
	case "VERSION":
		// owner package-ből importálva (YnMAdmin/owner_monitor.go-ból)
		versionResponse := fmt.Sprintf("(%s) - https://bot.ynm.hu", owner.YnMVersion)
		p.bot.SendRaw(fmt.Sprintf("NOTICE %s :\001VERSION %s\001", nick, versionResponse))
		
	case "PING":
		p.bot.SendRaw(fmt.Sprintf("NOTICE %s :\001PING %s\001", nick, args))
		
	case "TIME":
		timeResponse := time.Now().Format("Mon Jan 2 15:04:05 MST 2006")
		p.bot.SendRaw(fmt.Sprintf("NOTICE %s :\001TIME %s\001", nick, timeResponse))
		
	case "FINGER":
		fingerResponse := fmt.Sprintf("(%s) - https://bot.ynm.hu", owner.YnMVersion)
		p.bot.SendRaw(fmt.Sprintf("NOTICE %s :\001FINGER %s\001", nick, fingerResponse))
		
	case "CLIENTINFO":
		clientinfoResponse := "VERSION PING TIME FINGER CLIENTINFO"
		p.bot.SendRaw(fmt.Sprintf("NOTICE %s :\001CLIENTINFO %s\001", nick, clientinfoResponse))
	}
	
	return ""
}

func (p *CTCPPlugin) OnTick() []YnMIrC.Message {
	// Periodikus cleanup
	p.cleanup()
	return nil
}