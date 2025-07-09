package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ynmhu/YnM-Go/config"
	"github.com/ynmhu/YnM-Go/irc"
	"github.com/ynmhu/YnM-Go/plugins"
)

func main() {
	// ─── Konfiguráció betöltése ───────────────────────────────────────
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("Config betöltési hiba: %v", err)
	}

	log.Printf("Csatornák a configból: %+v\n", cfg.Channels)

	if cfg.LogDir == "" {
		log.Fatal("Log könyvtár nincs megadva a configban!")
	}
	if cfg.ConsoleChannel == "" {
		log.Fatal("A 'console_channel' nincs megadva a config.yaml‑ben!")
	}

	// Log mappa létrehozása
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		log.Fatalf("Log könyvtár létrehozási hiba: %v", err)
	}
	
	channels := cfg.Channels
	if len(channels) == 0 {
		channels = []string{"#YnM"}  
	}
	
	nameDayPlugin, err := plugins.NewNameDayPlugin(cfg.NevnapChannels, cfg.NevnapReggel, cfg.NevnapEste)
	if err != nil {
		log.Fatalf("Nem sikerült a névnap plugin inicializálása: %v", err)
	}	

	// ─── IRC kliens létrehozása ───────────────────────────────────────
	bot := irc.NewClient(cfg)

	// ─── Plugin‑kezelő ────────────────────────────────────────────────
	pluginManager := plugins.NewManager()

	// Admin plugin
	adminPlugin := plugins.NewAdminPlugin(cfg)
	adminPlugin.Initialize(bot)
	pluginManager.Register(adminPlugin)
	for _, admin := range cfg.Admins {
		adminPlugin.AddAdmin(admin)
	}

	// Ping plugin (felhasználói !ping parancs)
	duration, err := time.ParseDuration(cfg.PingCommandCooldown)
	if err != nil {
		log.Fatalf("Nem sikerült beolvasni a ping parancs cooldown időt: %v", err)
	}
	pingPlugin := plugins.NewPingPlugin(bot, duration, adminPlugin)
	bot.OnPong = func(pongID string) { pingPlugin.HandlePong(pongID) }
	pluginManager.Register(pingPlugin)

	// Névnap plugin
	pluginManager.Register(nameDayPlugin)

	// Test plugin
	testPlugin := plugins.NewTestPlugin(adminPlugin)
	pluginManager.Register(testPlugin)

	// Státusz plugin
	statusPlugin := plugins.NewStatusPlugin(bot)
	pluginManager.Register(statusPlugin)

	// ─── Időzített névnap‑üzenetek ────────────────────────────────────
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			for _, msg := range nameDayPlugin.OnTick() {
				bot.SendMessage(msg.Channel, msg.Text)
			}
		}
	}()

	// ─── Eseménykezelők beállítása ────────────────────────────────────
	var loginSuccessHandled bool

	// OnConnect esemény
	bot.OnConnect = func() {
		log.Println("DEBUG: OnConnect - kapcsolat létrejött")

		go func() {
			time.Sleep(2 * time.Second)
			bot.Join(cfg.ConsoleChannel)
			time.Sleep(1 * time.Second)

			log.Printf("DEBUG: AutoLogin=%v, AutoJoinWithoutLogin=%v, UseSASL=%v",
				cfg.AutoLogin, cfg.AutoJoinWithoutLogin, cfg.UseSASL)

			if cfg.UseSASL {
				bot.SendMessage(cfg.ConsoleChannel, "🔑 SASL típusú azonosítás sikeresen létrejött.")
			} else if cfg.AutoLogin {
				bot.SendMessage(cfg.ConsoleChannel, "🔑 NickServ azonosítás folyamatban...")
				if err := bot.IdentifyNickServ(); err != nil {
					log.Printf("NickServ azonosítás sikertelen: %v", err)
				}
			} else if cfg.AutoJoinWithoutLogin {
				bot.SendMessage(cfg.ConsoleChannel, "ℹ️ Nincs authentication, de autojoin engedélyezve — csatlakozás a csatornákhoz...")
				for _, ch := range cfg.Channels {
					if ch != cfg.ConsoleChannel {
						bot.Join(ch)
					}
				}
				bot.SendMessage(cfg.ConsoleChannel, "⚠️ Nincs azonosítás, így nem garantált minden funkció működése.")
			} else {
				bot.SendMessage(cfg.ConsoleChannel, "ℹ️ Minden automatikus funkció kikapcsolva. Csak console channelben vagyok.")
			}
		}()
	}

	// OnLoginSuccess esemény
	bot.OnLoginSuccess = func() {
		if loginSuccessHandled {
			log.Println("DEBUG: OnLoginSuccess - már kezelve, kihagyás")
			return
		}
		loginSuccessHandled = true

		log.Println("DEBUG: OnLoginSuccess - sikeres authentication, belépés a csatornákba")

		if cfg.AutoJoinWithoutLogin && !cfg.AutoLogin && !cfg.UseSASL {
			log.Println("DEBUG: OnLoginSuccess - már beléptünk autojoin_without_login-nal")
			return
		}

		var authMethod string
		if cfg.UseSASL {
			authMethod = "SASL"
		} else if cfg.AutoLogin {
			authMethod = "NickServ"
		} else {
			authMethod = "sima IRC"
		}

		bot.SendMessage(cfg.ConsoleChannel, fmt.Sprintf("✅ Sikeres %s authentication, csatlakozom a csatornákhoz…", authMethod))

		go func() {
			time.Sleep(500 * time.Millisecond)
			for _, ch := range cfg.Channels {
				if ch != cfg.ConsoleChannel {
					bot.Join(ch)
				}
			}
		}()
	}

	// OnLoginFailed esemény
	bot.OnLoginFailed = func(reason string) {
		bot.SendMessage(cfg.ConsoleChannel, "❌ Authentication sikertelen: "+reason)

		if cfg.AutoJoinWithoutLogin {
			bot.SendMessage(cfg.ConsoleChannel, "ℹ️ Autojoin engedélyezve, belépés authentication nélkül...")
			go func() {
				time.Sleep(500 * time.Millisecond)
				for _, ch := range cfg.Channels {
					if ch != cfg.ConsoleChannel {
						bot.Join(ch)
					}
				}
			}()
			bot.SendMessage(cfg.ConsoleChannel, "⚠️ Nincs authentication, így nem garantált minden funkció működése.")
		}
	}

	// OnMessage esemény
	bot.OnMessage = func(msg irc.Message) {
		fmt.Printf("IRC üzenet érkezett: [%s] <%s> %s\n", msg.Channel, msg.Sender, msg.Text)

		if response := pluginManager.HandleMessage(msg); response != "" {
			bot.SendMessage(msg.Channel, response)
		}

		logMessage(msg, cfg.LogDir)
	}

	// ─── Bot indítása ────────────────────────────────────────────────
	if err := bot.Connect(); err != nil {
		log.Fatal(err)
	}
	defer bot.Disconnect()

	// SIGINT/SIGTERM kezelés (Ctrl‑C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}

// ─── Segédfüggvény: üzenetek naplózása ───────────────────────────────
func logMessage(msg irc.Message, logDir string) {
	date := time.Now().Format("2006-01-02")
	logFile := fmt.Sprintf("%s/%s_%s.log", logDir, msg.Channel, date)

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
