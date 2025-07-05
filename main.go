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
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("Config betöltési hiba: %v", err)
	}
    if cfg.LogDir == "" {
    log.Fatal("Log könyvtár nincs megadva a configban!")
	
}
	// Log könyvtár létrehozása
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		log.Fatalf("Log könyvtár létrehozási hiba: %v", err)
	}

	// IRC kapcsolat létrehozása
	bot := irc.NewClient(cfg)

	// Pluginok betöltése
	pluginManager := plugins.NewManager()

	// Ping plugin létrehozása és beállítása a botban
	duration, err := time.ParseDuration(cfg.PingCommandCooldown)
	if err != nil {
		log.Fatalf("Nem sikerült beolvasni a ping parancs cooldown időt: %v", err)
	}
	pingPlugin := plugins.NewPingPlugin(bot, duration)
	bot.OnPong = func(pongID string) {
		pingPlugin.HandlePong(pongID)
	}
	pluginManager.Register(pingPlugin)

	// Névnap plugin regisztrálása
	nameDayPlugin := plugins.NewNameDayPlugin([]string{"#YnM", "#Magyar"})
	pluginManager.Register(nameDayPlugin)
	
	//admin
	adminPlugin := plugins.NewAdminPlugin()
	pluginManager.Register(adminPlugin)


	// Időzített üzenetek kezelése
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			messages := nameDayPlugin.OnTick()
			for _, msg := range messages {
				bot.SendMessage(msg.Channel, msg.Text)
			}
		}
	}()

	// Események kezelése
	bot.OnConnect = func() {
		for _, channel := range cfg.Channels {
			bot.Join(channel)
		}
	}
	
	bot.OnMessage = func(msg irc.Message) {
		// Logolás
		logMessage(msg, cfg.LogDir)
		
		// Pluginok kezelése
		response := pluginManager.HandleMessage(msg)
		if response != "" {
			bot.SendMessage(msg.Channel, response)
		}
	}

	// Bot indítása
	if err := bot.Connect(); err != nil {
		log.Fatal(err)
	}
	defer bot.Disconnect()

	// SIGINT kezelése
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}

func logMessage(msg irc.Message, logDir string) {
	date := time.Now().Format("2006-01-02")
	logFile := fmt.Sprintf("%s/%s_%s.log", logDir, msg.Channel, date)
	
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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