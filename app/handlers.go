package app

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ynmhu/YnM-Go/config"
	"github.com/ynmhu/YnM-Go/irc"
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

func (l *Logger) LogMessage(msg irc.Message) {
	date := time.Now().Format("2006-01-02")
	logFile := fmt.Sprintf("%s/%s_%s.log", l.logDir, msg.Channel, date)
	
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("Log fájl hiba: %v", err)
		return
	}
	defer file.Close()
	
	logLine := fmt.Sprintf("[%s] <%s> %s\n",
		time.Now().Format("15:04:05"),
		msg.Sender,
		msg.Text)
		
	if _, err := file.WriteString(logLine); err != nil {
		log.Printf("Log írási hiba: %v", err)
	}
}

// EventHandler struktúra és metódusok
type EventHandler struct {
	bot                   *irc.Client
	config                *config.Config
	pluginManager         *PluginManager
	logger                *Logger
	loginSuccessHandled   bool
}

func NewEventHandler(bot *irc.Client, cfg *config.Config, pm *PluginManager, logger *Logger) *EventHandler {
	return &EventHandler{
		bot:           bot,
		config:        cfg,
		pluginManager: pm,
		logger:        logger,
	}
}

func (h *EventHandler) Setup() {
	h.bot.OnConnect = h.handleConnect
	h.bot.OnLoginSuccess = h.handleLoginSuccess
	h.bot.OnLoginFailed = h.handleLoginFailed
	h.bot.OnMessage = h.handleMessage
}

func (h *EventHandler) handleConnect() {
	log.Println("DEBUG: OnConnect - kapcsolat létrejött")

	go func() {
		time.Sleep(2 * time.Second)
		h.bot.Join(h.config.ConsoleChannel)
		time.Sleep(1 * time.Second)

		log.Printf("DEBUG: AutoLogin=%v, AutoJoinWithoutLogin=%v, UseSASL=%v",
			h.config.AutoLogin, h.config.AutoJoinWithoutLogin, h.config.UseSASL)

		h.handleAuthenticationFlow()
	}()
}

func (h *EventHandler) handleAuthenticationFlow() {
	if h.config.UseSASL {
		h.bot.SendMessage(h.config.ConsoleChannel, "🔑 SASL típusú azonosítás sikeresen létrejött.")
	} else if h.config.AutoLogin {
		h.bot.SendMessage(h.config.ConsoleChannel, "🔑 NickServ azonosítás folyamatban...")
		if err := h.bot.IdentifyNickServ(); err != nil {
			log.Printf("NickServ azonosítás sikertelen: %v", err)
		}
	} else if h.config.AutoJoinWithoutLogin {
		h.handleAutoJoinWithoutLogin()
	} else {
		h.bot.SendMessage(h.config.ConsoleChannel, "ℹ️ Minden automatikus funkció kikapcsolva. Csak console channelben vagyok.")
	}
}

func (h *EventHandler) handleAutoJoinWithoutLogin() {
	h.bot.SendMessage(h.config.ConsoleChannel, "ℹ️ Nincs authentication, de autojoin engedélyezve — csatlakozás a csatornákhoz...")
	for _, ch := range h.config.Channels {
		if ch != h.config.ConsoleChannel {
			h.bot.Join(ch)
		}
	}
	h.bot.SendMessage(h.config.ConsoleChannel, "⚠️ Nincs azonosítás, így nem garantált minden funkció működése.")
}

func (h *EventHandler) handleLoginSuccess() {
	if h.loginSuccessHandled {
		log.Println("DEBUG: OnLoginSuccess - már kezelve, kihagyás")
		return
	}
	h.loginSuccessHandled = true

	log.Println("DEBUG: OnLoginSuccess - sikeres authentication, belépés a csatornákba")

	// Ha már beléptünk autojoin_without_login-nal, ne csináljunk semmit
	if h.config.AutoJoinWithoutLogin && !h.config.AutoLogin && !h.config.UseSASL {
		log.Println("DEBUG: OnLoginSuccess - már beléptünk autojoin_without_login-nal")
		return
	}

	authMethod := h.getAuthMethod()
	h.bot.SendMessage(h.config.ConsoleChannel, fmt.Sprintf("✅ Sikeres %s authentication, csatlakozom a csatornákhoz…", authMethod))

	h.joinChannels()
}

func (h *EventHandler) handleLoginFailed(reason string) {
	h.bot.SendMessage(h.config.ConsoleChannel, "❌ Authentication sikertelen: "+reason)

	if h.config.AutoJoinWithoutLogin {
		h.bot.SendMessage(h.config.ConsoleChannel, "ℹ️ Autojoin engedélyezve, belépés authentication nélkül...")
		h.joinChannels()
		h.bot.SendMessage(h.config.ConsoleChannel, "⚠️ Nincs authentication, így nem garantált minden funkció működése.")
	}
}

func (h *EventHandler) handleMessage(msg irc.Message) {
	fmt.Printf("IRC üzenet érkezett: [%s] <%s> %s\n", msg.Channel, msg.Sender, msg.Text)

	// Plugin kezelés
	if response := h.pluginManager.HandleMessage(msg); response != "" {
		h.bot.SendMessage(msg.Channel, response)
	}

	// Üzenet naplózása
	h.logger.LogMessage(msg)
}

func (h *EventHandler) getAuthMethod() string {
	if h.config.UseSASL {
		return "SASL"
	} else if h.config.AutoLogin {
		return "NickServ"
	}
	return "sima IRC"
}

func (h *EventHandler) joinChannels() {
	go func() {
		time.Sleep(500 * time.Millisecond)
		for _, ch := range h.config.Channels {
			if ch != h.config.ConsoleChannel {
				h.bot.Join(ch)
			}
		}
	}()
}