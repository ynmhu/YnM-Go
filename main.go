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

	// ─── IRC kliens létrehozása ───────────────────────────────────────
	bot := irc.NewClient(cfg)

	// ─── Plugin‑kezelő ────────────────────────────────────────────────
	pluginManager := plugins.NewManager()

	// Ping plugin (felhasználói !ping parancs)
	duration, err := time.ParseDuration(cfg.PingCommandCooldown)
	if err != nil {
		log.Fatalf("Nem sikerült beolvasni a ping parancs cooldown időt: %v", err)
	}
	pingPlugin := plugins.NewPingPlugin(bot, duration)
	bot.OnPong = func(pongID string) { pingPlugin.HandlePong(pongID) }
	pluginManager.Register(pingPlugin)

	// Névnap plugin
	nameDayPlugin := plugins.NewNameDayPlugin([]string{"#YnM", "#Magyar"})
	pluginManager.Register(nameDayPlugin)

	// Admin plugin
	adminPlugin := plugins.NewAdminPlugin(cfg)
	adminPlugin.Initialize(bot)
	pluginManager.Register(adminPlugin)

	// Optionally add admins from config:
	for _, admin := range cfg.Admins {
		adminPlugin.AddAdmin(admin)
	}

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

	// 1) Csatlakozáskor: csak console‑csatorna + NickServ azonosítás
    bot.OnConnect = func() {
	bot.Join(cfg.ConsoleChannel)

	go func() {
		// Várjunk 10 másodpercet, hogy stabil legyen a kapcsolat
		time.Sleep(0 * time.Second)

		if cfg.AutoLogin {
			// Ha autologin engedélyezett, indítsuk el a NickServ azonosítást
			if err := bot.IdentifyNickServ(); err != nil {
				log.Printf("NickServ azonosítás sikertelen: %v", err)
			}
		} else {
			// Ha nincs autologin, de autojoin engedélyezett, lépjünk be a csatornákra
			if cfg.AutoJoinWithoutLogin {
				bot.SendMessage(cfg.ConsoleChannel, "ℹ️ Autologin kikapcsolva, de autojoin engedélyezve — csatlakozás a csatornákhoz...")
				for _, ch := range cfg.Channels {
					if ch != cfg.ConsoleChannel {
						bot.Join(ch)
					}
				}
				// Jelzés a login hiányáról is lehet itt, pl:
				bot.SendMessage(cfg.ConsoleChannel, "⚠️ Nincs NickServ-login, így nem garantált minden funkció működése.")
			}
		}
	}()
}

	// 2) Sikeres NickServ‑login → lépjünk be a többi csatornába
bot.OnLoginSuccess = func() {
    log.Println("DEBUG: OnLoginSuccess called")
    bot.SendMessage(cfg.ConsoleChannel, "✅ Sikeres NickServ‑login, csatlakozom a csatornákhoz…")

    for _, ch := range cfg.Channels {
        if ch != cfg.ConsoleChannel {
            bot.Join(ch)
        }
    }
}

	// 3) Login‑hiba → jelzés a console‑csatornába
	bot.OnLoginFailed = func(reason string) {
		bot.SendMessage(cfg.ConsoleChannel,
			"❌ A bot nem tudott bejelentkezni NickServ‑hez: "+reason)
	}

	// 4) Bejövő üzenetek plugin‑kezelése + logolás
	bot.OnMessage = func(msg irc.Message) {
		logMessage(msg, cfg.LogDir)

		if response := pluginManager.HandleMessage(msg); response != "" {
			bot.SendMessage(msg.Channel, response)
		}
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
