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
	
	// Külön lista az időzített pluginoknak (amelyek nem illeszkednek a meglévő rendszerhez)
	var scheduledPlugins []ScheduledPlugin

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

	// Viccek Plugin
	jokePlugin := plugins.NewJokePlugin(bot, cfg.JokeChannels, cfg.JokeSendTime)
	pluginManager.Register(jokePlugin)
	jokePlugin.Start()

	// Vicc plugin regisztrálása
	viccPlugin := plugins.NewViccPlugin(bot, adminPlugin)
	pluginManager.Register(viccPlugin)

	// ─── Movie Plugin ─────────────────────────────────────────────────

	moviePlugin := plugins.NewMoviePlugin(
		bot,
		adminPlugin,
		cfg.JellyfinDBPath,
		cfg.MovieDBPath,
		cfg.MovieRequestsChannel,
		cfg.MoviePlugin.PostTime,
		cfg.MoviePlugin.PostChan,
		cfg.MoviePlugin.PostNick,
	)
	pluginManager.Register(moviePlugin)
	
	movieRequestPlugin := plugins.NewMovieRequestPlugin(bot, adminPlugin, cfg.MovieDBPath)
	if movieRequestPlugin != nil {
		pluginManager.Register(movieRequestPlugin)
		log.Printf("✅ Movie request plugin sikeresen regisztrálva")
	}

	movieCompletionPlugin := plugins.NewMovieCompletionPlugin(bot, adminPlugin, cfg.MovieDBPath)
	if movieCompletionPlugin != nil {
		pluginManager.Register(movieCompletionPlugin)
		log.Printf("✅ Movie completion plugin regisztrálva (PIN teljesítés)")
	}
	
	
	
	// ─── Movie Deletion Plugin ─────────────────────────────────────────
	movieDeletionPlugin := plugins.NewMovieDeletionPlugin(bot, adminPlugin, cfg.MovieDBPath)
	if movieDeletionPlugin != nil {
		pluginManager.Register(movieDeletionPlugin)
		log.Printf("✅ Movie deletion plugin sikeresen regisztrálva")
	}
	
	// ─── Media Upload Plugin ─────────────────────────────────────────
	mediaUploadPlugin := plugins.NewMediaUploadPlugin(bot, cfg)
	pluginManager.Register(mediaUploadPlugin)
	if err := mediaUploadPlugin.Start(); err != nil {
		log.Printf("❌ Media upload plugin indítási hiba: %v", err)
	} else {
		log.Printf("✅ Media upload plugin sikeresen regisztrálva és elindítva")
	}
	
	// ─── Media Ajánló Plugin ──────────	
	mediaPlugin := plugins.NewMediaAjanlatPlugin(
		bot,
		cfg.JellyfinDBPath,
		cfg.MediaAjanlat.Channel,
		cfg.MediaAjanlat.Time, // Például "19:25"
	)
	pluginManager.Register(mediaPlugin)

	// ─── Székelyhon Plugin ─────────────────────────────────────────────
	if cfg.SzekelyhonInterval != "" && len(cfg.SzekelyhonChannels) > 0 {
		interval, err := time.ParseDuration(cfg.SzekelyhonInterval)
		if err != nil {
			log.Fatalf("❌ Hibás Székelyhon időzítés: %v", err)
		}
		
		// Validáció
		if cfg.SzekelyhonStartHour < 0 || cfg.SzekelyhonStartHour > 23 {
			log.Fatalf("❌ Hibás Székelyhon kezdő óra: %d (0-23 között kell lennie)", cfg.SzekelyhonStartHour)
		}
		if cfg.SzekelyhonEndHour < 0 || cfg.SzekelyhonEndHour > 23 {
			log.Fatalf("❌ Hibás Székelyhon befejező óra: %d (0-23 között kell lennie)", cfg.SzekelyhonEndHour)
		}
		if cfg.SzekelyhonStartHour >= cfg.SzekelyhonEndHour {
			log.Fatalf("❌ Székelyhon kezdő óra (%d) nem lehet nagyobb vagy egyenlő a befejező óránál (%d)", 
				cfg.SzekelyhonStartHour, cfg.SzekelyhonEndHour)
		}
		
		log.Printf("🔧 Székelyhon plugin inicializálás...")
		szekelyhonPlugin := plugins.NewSzekelyhonPlugin(
			bot, 
			cfg.SzekelyhonChannels, 
			interval, 
			cfg.SzekelyhonStartHour, 
			cfg.SzekelyhonEndHour,
		)
		scheduledPlugins = append(scheduledPlugins, szekelyhonPlugin)
		log.Printf("✅ Székelyhon plugin sikeresen regisztrálva")
	} else {
		log.Println("ℹ️ Székelyhon plugin ki van kapcsolva (nincs konfigurálva)")
	}

	// ─── Időzített pluginok indítása ─────────────────────────────────────
	for _, plugin := range scheduledPlugins {
		plugin.Start()
		log.Printf("🚀 Időzített plugin elindítva: %s", plugin.Name())
	}

	// ─── Időzített plugin ticker (névnap + egyéb OnTick pluginok) ─────────
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			// Névnap plugin OnTick
			for _, msg := range nameDayPlugin.OnTick() {
				bot.SendMessage(msg.Channel, msg.Text)
			}
			
			// Egyéb pluginok OnTick metódusai (ha vannak)
			for _, plugin := range pluginManager.GetPlugins() {
				if tickablePlugin, ok := plugin.(interface{ OnTick() []plugins.ScheduledMessage }); ok {
					for _, msg := range tickablePlugin.OnTick() {
						bot.SendMessage(msg.Channel, msg.Text)
					}
				}
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

	// ─── Graceful shutdown ─────────────────────────────────────────────
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("🛑 Leállítási jel érkezett...")
		
		// Időzített pluginok leállítása
		for _, plugin := range scheduledPlugins {
			plugin.Stop()
			log.Printf("🛑 Időzített plugin leállítva: %s", plugin.Name())
		}
		
		// Plugin cleanup
		for _, plugin := range pluginManager.GetPlugins() {
			if moviePlugin, ok := plugin.(*plugins.MoviePlugin); ok {
				moviePlugin.Close()
				log.Printf("🛑 Movie plugin leállítva")
			}
			if movieCompletionPlugin, ok := plugin.(*plugins.MovieCompletionPlugin); ok {
				movieCompletionPlugin.Close()
				log.Printf("🛑 Movie completion plugin leállítva")
			}
			if movieDeletionPlugin, ok := plugin.(*plugins.MovieDeletionPlugin); ok {
				movieDeletionPlugin.Close()
				log.Printf("🛑 Movie deletion plugin leállítva")
			}
			if movieRequestPlugin, ok := plugin.(*plugins.MovieRequestPlugin); ok {
				movieRequestPlugin.Close()
				log.Printf("🛑 Movie request plugin leállítva")
			}
			// Media upload plugin cleanup
			if mediaUploadPlugin, ok := plugin.(*plugins.MediaUploadPlugin); ok {
				mediaUploadPlugin.Stop()
				log.Printf("🛑 Media upload plugin leállítva")
			}
		}
		
		// Bot leállítása
		bot.Disconnect()
		os.Exit(0)
	}()
	
	// Várakozás a leállítási jelre
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

// ─── Segéd interface az időzített pluginoknak ─────────────────────────
type ScheduledPlugin interface {
	Start()
	Stop()
	Name() string
}