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
	"os"
	"time"
	"log"
	"strings"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
)

// Logger struktúra és metódusok
type Logger struct {
	logDir string
}

func NewLogger(logDir string) *Logger {
	return &Logger{
		logDir: logDir,
	}
}

func (l *Logger) LogMessage(msg YnMIrC.Message) {
	date := time.Now().Format("2006-01-02")
	logFile := fmt.Sprintf("%s/%s_%s.log", l.logDir, msg.Channel, date)
	
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	
	logLine := fmt.Sprintf("[%s] <%s> %s\n",
		time.Now().Format("15:04:05"),
		msg.Sender,
		msg.Text)
		
	if _, err := file.WriteString(logLine); err != nil {
	}
}

// EventHandler struktúra és metódusok
type EventHandler struct {
	bot                   *YnMIrC.Client
	config                *YnMConfig.Config
	pluginManager         *PluginManager
	logger                *Logger
	loginSuccessHandled   bool
}

func NewEventHandler(bot *YnMIrC.Client, cfg *YnMConfig.Config, pm *PluginManager, logger *Logger) *EventHandler {
	return &EventHandler{
		bot:           bot,
		config:        cfg,
		pluginManager: pm,
		logger:        logger,
	}
}

func (h *EventHandler) Setup() {
	log.Println("🔧 EventHandler Setup() - Callbacks beállítása...")
	h.bot.OnConnect = h.handleConnect
	h.bot.OnLoginSuccess = h.handleLoginSuccess
	h.bot.OnLoginFailed = h.handleLoginFailed
	h.bot.OnMessage = h.handleMessage
	log.Println("✅ EventHandler Setup() befejezve")
}


func (h *EventHandler) handleConnect() {
	go func() {
		time.Sleep(5 * time.Second)
		//log.Println("📡 Csatlakozva az IRC szerverhez, belépés a console channelbe...")
		
		log.Printf("🔍 Console channel: '%s'\n", h.config.ConsoleChannel)
		
		h.bot.SendRaw(fmt.Sprintf("JOIN %s", h.config.ConsoleChannel))
		
		//log.Println("✅ JOIN parancs elküldve a console channelhez")
		
		time.Sleep(3 * time.Second)
		//log.Println("🔄 Második próbálkozás...")
		h.bot.SendRaw(fmt.Sprintf("JOIN %s", h.config.ConsoleChannel))
		
		// ⚠️ JAVÍTÁS: NickServLogin használata
		if !h.config.NickServLogin && !h.config.UseSASL && !h.config.Undernet.Enabled {
			time.Sleep(10 * time.Second)
			//log.Println("📤 Küldöm: Autentikáció ki van kapcsolva")
			h.bot.SendMessage(h.config.ConsoleChannel, "⚠️ Autentikáció ki van kapcsolva - csak console channelben vagyok elérhető")
			h.bot.SendMessage(h.config.ConsoleChannel, "ℹ️ Az autentikáció engedélyezéséhez állítsd be a NickServLogin, UseSASL vagy Undernet opciót")
		} else {
			//log.Println("🔍 Auth beállítva, várakozás a NickServ/SASL/Undernet válaszra...")
		}
	}()
}


func (h *EventHandler) handleAuthenticationFlow() {
	// ⚠️ EZ A FÜGGVÉNY MÁR NEM HASZNÁLT
	// Az azonosítást a YnMIrC/client.go handleWelcome() kezeli
}

func (h *EventHandler) handleAutoJoinWithoutLogin() {
	h.bot.SendMessage(h.config.ConsoleChannel, "ℹ️ Nincs authentication, de autojoin engedélyezve — csatlakozás a csatornákhoz...")
	h.bot.SendMessage(h.config.ConsoleChannel, "⚠️ Nincs azonosítás, így nem garantált minden funkció működése.")
}

func (h *EventHandler) handleLoginSuccess() {
	//log.Println("🎉🎉🎉 handleLoginSuccess() MEGHÍVVA!")
	if h.loginSuccessHandled {
		//log.Println("⚠️ handleLoginSuccess már lefutott egyszer, kihagyom")
		return
	}
	h.loginSuccessHandled = true
	//log.Println("✅ loginSuccessHandled = true beállítva")
	
	// ⚠️ ÚJ: Sikeres auth után küldünk üzenetet
	go func() {
		time.Sleep(2 * time.Second)
		h.bot.SendMessage(h.config.ConsoleChannel, "✅ Autentikáció sikeres!")
		log.Println("📤 Sikeres auth üzenet elküldve")
	}()
}

func (h *EventHandler) handleLoginFailed(reason string) {
	log.Println("❌❌❌ handleLoginFailed() MEGHÍVVA! Reason:", reason)
	
	h.bot.SendMessage(h.config.ConsoleChannel, "❌ Authentication sikertelen: "+reason)
	h.bot.SendMessage(h.config.ConsoleChannel, "ℹ️ Ellenőrizd a jelszavakat és az IRC szerver beállításait")

	if h.config.AutoJoinWithoutLogin {
		h.bot.SendMessage(h.config.ConsoleChannel, "ℹ️ Autojoin engedélyezve, belépés authentication nélkül...")
		h.bot.SendMessage(h.config.ConsoleChannel, "⚠️ Nincs authentication, így nem garantált minden funkció működése.")
	}
}

func (h *EventHandler) handleMessage(msg YnMIrC.Message) {
	// Plugin kezelés
	if response := h.pluginManager.HandleMessage(msg); response != "" {
		// ✅ TÖBBSOROS VÁLASZ KEZELÉSE (~~~ vagy \n szeparátorral)
		var lines []string
		
		if strings.Contains(response, "~~~") {
			lines = strings.Split(response, "~~~")
		} else if strings.Contains(response, "\n") {
			lines = strings.Split(response, "\n")
		} else {
			lines = []string{response}
		}
		
		// Minden sort külön küldünk
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				if i > 0 {
					time.Sleep(200 * time.Millisecond)
				}
				h.bot.SendMessage(msg.Channel, line)
			}
		}
	}
	// Üzenet naplózása
	h.logger.LogMessage(msg)
}

func (h *EventHandler) getAuthMethod() string {
	if h.config.UseSASL {
		return "SASL"
	} else if h.config.NickServLogin {  // ← JAVÍTÁS
		return "NickServ"
	}
	return "sima IRC"
}

func (h *EventHandler) retryJoinChannels() {
	// ⚠️ EZ A FÜGGVÉNY MOST MÁR NEM HASZNÁLT
	// Az azonosítást és csatornákba lépést a handleRawMessage() kezeli
	// NE HÍVD MEG SEHONNAN!
	
	log.Println("⚠️ retryJoinChannels() meghívva - ez már nem használt függvény!")
}

func (h *EventHandler) executeJoinChannels() {
    // ⚠️ VÉDELEM: Ellenőrizzük, hogy van-e aktív azonosítás
    if h.config.Undernet.Enabled && !h.bot.IsUndernetAuthenticated() {
        // Ha Undernet engedélyezve van, de még nincs azonosítva
        log.Println("🔒 executeJoinChannels: Undernet azonosítás még folyamatban, kihagyom")
        return
    }
    
    if h.config.UseSASL && !h.bot.IsLoggedIn() {
        // Ha SASL engedélyezve van, de még nincs bejelentkezve
        log.Println("🔒 executeJoinChannels: SASL azonosítás még folyamatban, kihagyom")
        return
    }
    
    if h.config.NickServLogin && !h.bot.IsLoggedIn() {  
        log.Println("🔒 executeJoinChannels: NickServ azonosítás még folyamatban, kihagyom")
        return
    }
    
    log.Println("🚀 executeJoinChannels: Belépés a csatornákba...")
    time.Sleep(500 * time.Millisecond)
    
    // Csatornákba lépés
    for _, channel := range h.config.Channels {
        // Console channel kihagyása, mert már benne vagyunk
        if channel == h.config.ConsoleChannel {
            continue
        }
        
        log.Printf("📍 Joining: %s\n", channel)
        h.bot.Join(channel)
        time.Sleep(200 * time.Millisecond) // Rate limiting
    }
    
    h.bot.SendMessage(h.config.ConsoleChannel, "✅ Belépés az összes csatornába.")
}