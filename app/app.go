package app

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ynmhu/YnM-Go/config"
	"github.com/ynmhu/YnM-Go/irc"
)

type App struct {
	config        *config.Config
	bot           *irc.Client
	pluginManager *PluginManager
	eventHandler  *EventHandler
	logger        *Logger
}

func New(cfg *config.Config) *App {
	return &App{
		config: cfg,
	}
}

func (a *App) Run() error {
	// Előkészítés
	if err := a.initialize(); err != nil {
		return err
	}

	// Bot indítása
	if err := a.bot.Connect(); err != nil {
		return err
	}
	defer a.bot.Disconnect()

	// Graceful shutdown
	a.setupGracefulShutdown()

	// Várakozás a leállítási jelre
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	return nil
}

func (a *App) initialize() error {
	// Validáció
	if err := a.validateConfig(); err != nil {
		return err
	}

	// Log mappa létrehozása
	if err := os.MkdirAll(a.config.LogDir, 0o755); err != nil {
		return err
	}

	// Komponensek inicializálása
	a.bot = irc.NewClient(a.config)
	a.logger = NewLogger(a.config.LogDir)
	a.pluginManager = NewPluginManager()
	a.eventHandler = NewEventHandler(a.bot, a.config, a.pluginManager, a.logger)

	// Event handlerek beállítása
	a.eventHandler.Setup()

	// Pluginok regisztrálása
	if err := a.pluginManager.RegisterAll(a.bot, a.config); err != nil {
		return err
	}

	// Időzített pluginok indítása
	a.startScheduledTasks()

	return nil
}

func (a *App) validateConfig() error {
	if a.config.LogDir == "" {
		log.Fatal("Log könyvtár nincs megadva a configban!")
	}
	if a.config.ConsoleChannel == "" {
		log.Fatal("A 'console_channel' nincs megadva a config.yaml‑ben!")
	}
	return nil
}

func (a *App) startScheduledTasks() {
	// Időzített plugin ticker
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			a.pluginManager.HandleTick(a.bot)
		}
	}()
}

func (a *App) setupGracefulShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Leállítási jel érkezett...")
		
		// Pluginok leállítása
		a.pluginManager.Shutdown()
		
		// Bot leállítása
		a.bot.Disconnect()
		os.Exit(0)
	}()
}