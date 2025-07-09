// ============================================================================
//  Szerzői jog © 2024 Markus (markus@ynm.hu)
//  https://ynm.hu   – főoldal
//  https://forum.ynm.hu   – hivatalos fórum
//  https://bot.ynm.hu     – bot oldala és dokumentáció
//
//  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
//  módosítani a szerző írásos engedélye nélkül.
//
//  Ez a fájl a YnM-Go IRC-bot rendszerének része.
// ============================================================================

package plugins

import (
    "github.com/ynmhu/YnM-Go/irc"
    "strings"
    "time"
    "fmt"
    "sync"
)

type PingPlugin struct {
    pingSentAt      map[string]time.Time
    pingChannel     map[string]string
    userPingTimes   map[string][]time.Time
    userBanUntil    map[string]time.Time
    userBanNotified map[string]bool
    mu              sync.Mutex
    bot             *irc.Client
    cooldown        time.Duration
    adminPlugin     *AdminPlugin  // hozzáadva
}

// Konstruktor a PingPluginhez
func NewPingPlugin(bot *irc.Client, cooldown time.Duration, adminPlugin *AdminPlugin) *PingPlugin {
    return &PingPlugin{
        pingSentAt:      make(map[string]time.Time),
        pingChannel:     make(map[string]string),
        userPingTimes:   make(map[string][]time.Time),
        userBanUntil:    make(map[string]time.Time),
        userBanNotified: make(map[string]bool),
        bot:             bot,
        cooldown:        cooldown,
        adminPlugin:     adminPlugin,  // beállítva
    }
}


// Kezeli az üzeneteket
func (p *PingPlugin) HandleMessage(msg irc.Message) string {
    if strings.ToLower(msg.Text) != "!ping" {
        return ""
    }
	
	
	
	// Itt nézzük az admin szintet
    nick := strings.Split(msg.Sender, "!")[0]
    hostmask := msg.Sender
    level := p.adminPlugin.store.GetAdminLevel(nick, hostmask)

    if level < AdminLevelAdmin { // csak admin (2) és owner (3)
        return ""  //return "Csak admin és owner használhatja a !ping parancsot." (Ezt is válaszolhassa)
    }
	
	//vége admin ellenörzése

    p.mu.Lock()
    defer p.mu.Unlock()

    now := time.Now()
    user := msg.Sender

    // Kitöröljük az 5 percnél régebbi ping időpontokat
    times := p.userPingTimes[user]
    validTimes := []time.Time{}
    for _, t := range times {
        if now.Sub(t) <= 5*time.Minute {
            validTimes = append(validTimes, t)
        }
    }
    p.userPingTimes[user] = validTimes

    // Tiltás ellenőrzése
    banUntil, banned := p.userBanUntil[user]
    if banned {
        if now.Before(banUntil) {
            // Tiltás alatt van
            if !p.userBanNotified[user] {
                remaining := banUntil.Sub(now).Round(time.Second)
                p.userBanNotified[user] = true  // jelezzük, hogy már figyelmeztettünk
                return fmt.Sprintf("Túl sok !ping parancs, kérlek várj még %s-ig!", remaining)
            }
            return "" // már jeleztük, nem válaszolunk többször
        } else {
            // Tiltás lejárt, töröljük az adatokat és a figyelmeztető flag-et
            delete(p.userBanUntil, user)
            delete(p.userBanNotified, user)
        }
    }

    // Ha 3 vagy több hívás van 5 perc alatt, tiltjuk 24 órára
    if len(validTimes) >= 3 {
        p.userBanUntil[user] = now.Add(24 * time.Hour)
        p.userPingTimes[user] = nil // reseteljük az időpontokat
        p.userBanNotified[user] = true // azonnal jelezzük az új tiltást
        return "Túl sok !ping parancs, ezért 24 órára le vagy tiltva erről a parancsról."
    }

    // Alap cooldown ellenőrzése (pl. 30s)
    if len(validTimes) > 0 {
        last := validTimes[len(validTimes)-1]
        if now.Sub(last) < p.cooldown {
            return "" // túl gyors, nem válaszolunk
        }
    }

    // Hozzáadjuk az aktuális időpontot
    p.userPingTimes[user] = append(p.userPingTimes[user], now)

    // Ping küldése az IRC szerver felé
    id := fmt.Sprintf("%d", now.UnixNano())
    p.pingSentAt[id] = now
    p.pingChannel[id] = msg.Channel
    p.bot.SendRaw(fmt.Sprintf("PING %s", id))

    return ""
}


// Ezt a függvényt hívja az IRC kliens, amikor PONG üzenet érkezik
func (p *PingPlugin) HandlePong(id string) {
    p.mu.Lock()
    start, ok := p.pingSentAt[id]
    channel, chOk := p.pingChannel[id]
    if !ok || !chOk {
        p.mu.Unlock()
        return
    }
    delete(p.pingSentAt, id)
    delete(p.pingChannel, id)
    p.mu.Unlock()

    elapsed := time.Since(start)
    p.bot.SendMessage(channel, fmt.Sprintf("PING reply of %.3f s", elapsed.Seconds()))
}

func (p *PingPlugin) OnTick() []irc.Message {
    return nil
}
