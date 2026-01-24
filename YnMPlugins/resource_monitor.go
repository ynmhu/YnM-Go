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
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	ps "github.com/shirou/gopsutil/process"

	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMModule"
)

type YnMResourceMonitorPlugin struct {
	bot             *YnMIrC.Client
	discord         *discord.DiscordAdapter
	adminPlugin     *owner.YnmAdminPlugin // Admin plugin referencia jogosultság ellenőrzéshez
	channels        []string
	discordChannels []string

	threshold   float64
	interval    time.Duration
	enabled     bool
	mtx         sync.Mutex
	stopChan    chan struct{}
	configName  string
	
	// Új: megerősítési mechanizmus
	inWarningState bool
	warningTime    time.Time
}


func NewYnMResourceMonitorPlugin(bot *YnMIrC.Client, adminPlugin *owner.YnmAdminPlugin, cfg *YnMConfig.Config, discordAdapter *discord.DiscordAdapter) *YnMResourceMonitorPlugin {
	// Alapértékek
	threshold := 90.0
	interval := 3600 * time.Second

	// Ha van a konfigurációban beállítás, olvassuk ki (típuskényszerítés egyszerűsített példa)
	if cfg != nil && cfg.ResourceMonitor != nil {
		if cfg.ResourceMonitor.Threshold > 0 {
			threshold = float64(cfg.ResourceMonitor.Threshold)
		}
		if cfg.ResourceMonitor.IntervalSeconds > 0 {
			interval = time.Duration(cfg.ResourceMonitor.IntervalSeconds) * time.Second
		}
	}

	// Csatornák szétválogatása IRC/Discord szerint (szám-only -> Discord)
	var ircChannels []string
	var dChannels []string
	if cfg != nil && cfg.ResourceMonitor != nil {
		for _, ch := range cfg.ResourceMonitor.Channels {
			if isDiscordChannel(ch) {
				dChannels = append(dChannels, ch)
			} else {
				ircChannels = append(ircChannels, ch)
			}
		}
	}

	return &YnMResourceMonitorPlugin{
		bot:             bot,
		discord:         discordAdapter,
		adminPlugin:     adminPlugin,
		channels:        ircChannels,
		discordChannels: dChannels,
		threshold:       threshold,
		interval:        interval,
		enabled:         false,
		stopChan:        make(chan struct{}),
		configName:      "ResourceMonitor",
		inWarningState:  false,
	}
}

// Start elindítja a figyelést (nem blokkoló).
func (p *YnMResourceMonitorPlugin) Start() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if p.enabled {
		return
	}
	p.enabled = true
	p.stopChan = make(chan struct{})
	go p.monitorLoop()
}

// Stop leállítja a figyelést.
func (p *YnMResourceMonitorPlugin) Stop() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if !p.enabled {
		return
	}
	close(p.stopChan)
	p.enabled = false
	p.inWarningState = false
}

// monitorLoop a háttér goroutine, ami időzítve ellenőrzi az erőforrásokat.
func (p *YnMResourceMonitorPlugin) monitorLoop() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// első ellenőrzés azonnal
	p.checkAndAlert()

	for {
		select {
		case <-ticker.C:
			p.checkAndAlert()
		case <-p.stopChan:
			return
		}
	}
}

// TopProcessInfo egyszerű struktúra a top folyamat adataihoz.
type TopProcessInfo struct {
	Name       string
	PID        int32
	CPUPercent float64
}

// checkAndAlert lekéri a CPU és RAM használatot és riaszt, ha szükséges.
func (p *YnMResourceMonitorPlugin) checkAndAlert() {
	cpuPercents, err := cpu.Percent(1*time.Second, false)
	if err != nil || len(cpuPercents) == 0 {
		log.Printf("[ResourceMonitor] Nem sikerült lekérni a CPU használatot: %v", err)
		return
	}
	cpuUsage := cpuPercents[0]

	vm, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("[ResourceMonitor] Nem sikerült lekérni a memória statisztikát: %v", err)
		return
	}
	ramUsage := vm.UsedPercent

	// DEBUG LOG - mindig írjuk ki a küszöböt
	//log.Printf("[ResourceMonitor] CPU: %.2f%%, RAM: %.2f%% (küszöb: %.2f%%)", 
	//	cpuUsage, ramUsage, p.threshold)

	// Ellenőrizzük, hogy meghaladja-e a küszöböt
	overThreshold := cpuUsage >= p.threshold || ramUsage >= p.threshold

	if overThreshold {
		if !p.inWarningState {
			// Első alkalommal meghaladta - indítsuk el a megerősítést
			p.inWarningState = true
			p.warningTime = time.Now()
			log.Printf("[ResourceMonitor] Küszöb túllépve első alkalommal, várakozás 1 percig...")
			
			// Várunk 1 percet
			time.Sleep(1 * time.Minute)
			
			// Újra ellenőrizzük
			cpuPercents2, err2 := cpu.Percent(1*time.Second, false)
			if err2 != nil || len(cpuPercents2) == 0 {
				log.Printf("[ResourceMonitor] Nem sikerült újra lekérni a CPU használatot: %v", err2)
				p.inWarningState = false
				return
			}
			cpuUsage2 := cpuPercents2[0]

			vm2, err2 := mem.VirtualMemory()
			if err2 != nil {
				log.Printf("[ResourceMonitor] Nem sikerült újra lekérni a memória statisztikát: %v", err2)
				p.inWarningState = false
				return
			}
			ramUsage2 := vm2.UsedPercent

			//log.Printf("[ResourceMonitor] Megerősítő ellenőrzés - CPU: %.2f%%, RAM: %.2f%%", 
			//	cpuUsage2, ramUsage2)

			// Ha még mindig meghaladja, akkor küldjünk riasztást
			if cpuUsage2 >= p.threshold || ramUsage2 >= p.threshold {
				topProc := p.getTopCPUProcess()
				msg := fmt.Sprintf("🚨 Erőforrás riasztás (megerősítve)! CPU: %.2f%%, RAM: %.2f%%", 
					cpuUsage2, ramUsage2)
				
				if topProc != nil && topProc.CPUPercent > 0.5 {
					msg = fmt.Sprintf("%s — Process: %s (PID: %d, CPU: %.2f%%)", 
						msg, topProc.Name, topProc.PID, topProc.CPUPercent)
				} else {
					msg = fmt.Sprintf("%s — (Nincs jelentős CPU-fogyasztó folyamat)", msg)
				}

				p.sendToAllChannels(msg)
				log.Printf("[ResourceMonitor] Riasztás elküldve!")
			} else {
				log.Printf("[ResourceMonitor] Visszaállt normál értékre, nincs riasztás.")
			}
			
			p.inWarningState = false
		}
		// Ha már warning state-ben vagyunk, ne csináljunk semmit (várunk az újraellenőrzésre)
	} else {
		// Ha nem haladja meg a küszöböt, reseteljük a warning state-et
		if p.inWarningState {
			log.Printf("[ResourceMonitor] Visszaállt normál értékre a warning period alatt.")
		}
		p.inWarningState = false
	}
}

// getTopCPUProcess javított verziója pontosabb CPU méréshez
func (p *YnMResourceMonitorPlugin) getTopCPUProcess() *TopProcessInfo {
	procs, err := ps.Processes()
	if err != nil {
		log.Printf("[ResourceMonitor] Hiba a folyamatok lekérésekor: %v", err)
		return nil
	}

	type candidate struct {
		name string
		pid  int32
		cpu  float64
	}

	candidates := make([]candidate, 0)

	// ELSŐ mérés - inicializálás (ez még 0-kat adhat)
	for _, pr := range procs {
		_, _ = pr.Percent(0)
	}

	// Rövid várakozás a mintavételhez
	time.Sleep(500 * time.Millisecond)

	// MÁSODIK mérés - valós adatok
	for _, pr := range procs {
		name, nerr := pr.Name()
		if nerr != nil {
			continue
		}

		// Most már pontos CPU százalékot kapunk
		cpuPct, cerr := pr.Percent(0)
		if cerr != nil {
			continue
		}

		// Csak > 0.1% CPU-t fogyasztó folyamatokat vesszük figyelembe
		if cpuPct > 0.1 {
			candidates = append(candidates, candidate{
				name: name,
				pid:  pr.Pid,
				cpu:  cpuPct,
			})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Rendezés csökkenő CPU szerint
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].cpu > candidates[j].cpu
	})

	top := candidates[0]
	return &TopProcessInfo{
		Name:       top.name,
		PID:        top.pid,
		CPUPercent: top.cpu,
	}
}

// sendToAllChannels küldi az üzenetet IRC és (ha elérhető) Discord csatornákra is.
func (p *YnMResourceMonitorPlugin) sendToAllChannels(msg string) {
	// IRC csatornákra
	for _, ch := range p.channels {
		// Ha ch üres, kihagyjuk
		if strings.TrimSpace(ch) == "" {
			continue
		}
		if p.bot != nil {
			p.bot.SendMessage(ch, msg)
		}
	}

	// Discord csatornákra (numerikus ID-ként kezelve)
	for _, ch := range p.discordChannels {
		if p.discord != nil {
			if err := p.discord.SendMessage(ch, msg); err != nil {
				log.Printf("[ResourceMonitor] Discord üzenet küldési hiba: %v", err)
			}
		}
	}
}

// HandleMessage kezeli a parancsokat (!monitor, !stopmonitor, !resourcestatus)
func (p *YnMResourceMonitorPlugin) HandleMessage(msg YnMIrC.Message) string {
	var nick, hostmask string
	
	if msg.Sender != "" {
		// IRC user
		nick = strings.SplitN(msg.Sender, "!", 2)[0]
		hostmask = YnMModule.SimplifyHostmask(msg.Sender)
	} else if msg.Nick != "" {
		// Discord user - lookup by Discord ID
		userInfo, err := p.adminPlugin.Db.GetUserByDiscordID(msg.Nick)
		if err != nil {
			return "❌ You need to link your Discord account first. Use !register"
		}
		nick = userInfo.Nick
		hostmask = userInfo.Hostmask
	} else {
		return ""
	}
	
	// Csak azokból a csatornákból fogadunk parancsokat, amelyek a config-ban vannak
	validChannel := false
	// Ellenőrizzük IRC csatornákat
	for _, ch := range p.channels {
		if msg.Channel == ch {
			validChannel = true
			break
		}
	}
	// Ellenőrizzük Discord csatornákat is
	if !validChannel {
		for _, ch := range p.discordChannels {
			if msg.Channel == ch {
				validChannel = true
				break
			}
		}
	}
	if !validChannel {
		return ""
	}

	parts := strings.Fields(msg.Text)
	if len(parts) == 0 {
		return ""
	}


	command := strings.ToLower(parts[0])

	switch command {
	case "!monitor":
		// Admin szint (2) vagy magasabb
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, 2) {
			return "❌ You need Admin or higher permissions."
		}
		p.Start()
		return "Erőforrás-figyelés elindítva (1 perces megerősítéssel)."

	case "!stopmonitor":
		// Admin szint (2) vagy magasabb
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, 2) {
			return "❌ You need Admin or higher permissions."
		}
		p.Stop()
		return "Erőforrás-figyelés leállítva."

	case "!resourcestatus":
		// VIP szint (3) vagy magasabb
		if !YnMModule.HasMinAdminLevelWithDB(p.adminPlugin.Db, nick, hostmask, msg.Channel, 3) {
			return "" //return "❌ You need VIP or higher permissions."
		}
		// Azonnali állapot lekérése
		cpuPercents, err := cpu.Percent(1*time.Second, false)
		if err != nil || len(cpuPercents) == 0 {
			return "Nem sikerült lekérni a CPU használatot."
		}
		vm, err := mem.VirtualMemory()
		if err != nil {
			return "Nem sikerült lekérni a memória használatot."
		}
		top := p.getTopCPUProcess()
		msg := fmt.Sprintf("Állapot — CPU: %.2f%%, RAM: %.2f%% (küszöb: %.0f%%)",
			cpuPercents[0], vm.UsedPercent, p.threshold)
		if top != nil {
			msg = fmt.Sprintf("%s — Top process: %s (PID: %d, CPU: %.2f%%)",
				msg, top.Name, top.PID, top.CPUPercent)
		}
		if p.inWarningState {
			msg = fmt.Sprintf("%s — ⚠️ Megerősítési periódusban", msg)
		}
		return msg
	}

	return ""
}

func (p *YnMResourceMonitorPlugin) OnTick() []YnMIrC.Message {
	return nil
}