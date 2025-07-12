package app

import (
	"log"
	"time"
    "os"
	"path/filepath"
	"github.com/ynmhu/YnM-Go/config"
	"github.com/ynmhu/YnM-Go/irc"
	"github.com/ynmhu/YnM-Go/plugins"
	"github.com/ynmhu/YnM-Go/plugins/media"
	"github.com/ynmhu/YnM-Go/plugins/admin"
		"github.com/ynmhu/YnM-Go/plugins/ynm"
)

// Plugin interfészek
type Plugin interface {
	HandleMessage(msg irc.Message) string
	OnTick() []irc.Message
}

type ScheduledPlugin interface {
	Start()
	Stop()
	Name() string
}

type ScheduledMessage = irc.Message

// Manager struktúra - alapvető plugin kezeléshez
type Manager struct {
	plugins []Plugin
}

func NewManager() *Manager {
	return &Manager{
		plugins: make([]Plugin, 0),
	}
}

func (m *Manager) Register(plugin Plugin) {
	m.plugins = append(m.plugins, plugin)
}

func (m *Manager) HandleMessage(msg irc.Message) string {
	for _, plugin := range m.plugins {
		if response := plugin.HandleMessage(msg); response != "" {
			return response
		}
	}
	return ""
}

func (m *Manager) GetPlugins() []Plugin {
	return m.plugins
}

// PluginManager - magasabb szintű plugin kezelés az app-ban
type PluginManager struct {
	manager          *Manager
	scheduledPlugins []ScheduledPlugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		manager:          NewManager(),
		scheduledPlugins: make([]ScheduledPlugin, 0),
	}
}

func (pm *PluginManager) RegisterAll(bot *irc.Client, cfg *config.Config) error {
	// Admin plugin (először, mert mások függnek tőle)
	adminPlugin := pm.registerAdminPlugin(bot, cfg)

	// Core pluginok
	pm.registerCorePlugins(bot, cfg, adminPlugin)

	// Movie pluginok
	pm.registerMoviePlugins(bot, cfg, adminPlugin)

	// Media pluginok
	pm.registerMediaPlugins(bot, cfg, adminPlugin)

	// Időzített pluginok
	pm.registerScheduledPlugins(bot, cfg)

	return nil
}

func (pm *PluginManager) registerAdminPlugin(bot *irc.Client, cfg *config.Config) *admin.AdminPlugin {
	adminPlugin := admin.NewAdminPlugin(cfg)
	adminPlugin.Initialize(bot)
	pm.manager.Register(adminPlugin)
	
	for _, admin := range cfg.Admins {
		adminPlugin.AddAdmin(admin)
	}
	
	log.Printf("✅ Admin plugin regisztrálva")
	return adminPlugin
}

func (pm *PluginManager) registerCorePlugins(bot *irc.Client, cfg *config.Config, adminPlugin *admin.AdminPlugin) {
	// Ping plugin
	duration, err := time.ParseDuration(cfg.PingCommandCooldown)
	if err != nil {
		log.Fatalf("Ping cooldown parsing hiba: %v", err)
	}
	pingPlugin := ynm.NewPingPlugin(bot, duration, adminPlugin)
	bot.OnPong = func(pongID string) { pingPlugin.HandlePong(pongID) }
	pm.manager.Register(pingPlugin)

	// Névnap plugin
	nameDayPlugin, err := ynm.NewNameDayPlugin(cfg.NevnapChannels, cfg.NevnapReggel, cfg.NevnapEste)
	if err != nil {
		log.Fatalf("Névnap plugin inicializálás hiba: %v", err)
	}
	pm.manager.Register(nameDayPlugin)

	// Óra plugin
	oraPlugin := ynm.NewOraPlugin(bot, adminPlugin, cfg)
	pm.manager.Register(oraPlugin)

	// Test plugin


	// Státusz plugin
	statusPlugin := ynm.NewStatusPlugin(bot)
	pm.manager.Register(statusPlugin)

	// Vicc plugin
	viccPlugin := ynm.NewViccPlugin(bot, adminPlugin)
	pm.manager.Register(viccPlugin)
	
	// Tamagotchi plugin
	dataDir := filepath.Join(cfg.DataDirectory, "data") // vagy használj egy fix útvonalat
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("❌ Nem sikerült létrehozni a Tamagotchi adatkönyvtárat: %v", err)
		return
	}

	tamagotchiPlugin := plugins.NewTamagotchiPlugin(dataDir, bot)
	if err := tamagotchiPlugin.Initialize(bot, cfg); err != nil {
		log.Printf("❌ Tamagotchi plugin inicializálás hiba: %v", err)
	} else {
		pm.manager.Register(tamagotchiPlugin)
		log.Printf("✅ Tamagotchi plugin regisztrálva")
	}
	
	

	log.Printf("✅ Core pluginok regisztrálva")
}

func (pm *PluginManager) registerMoviePlugins(bot *irc.Client, cfg *config.Config, adminPlugin *admin.AdminPlugin) {
	// Movie plugin
	moviePlugin := media.NewMoviePlugin(
		bot, adminPlugin, cfg.JellyfinDBPath, cfg.MovieDBPath,
		cfg.MovieRequestsChannel, cfg.MoviePlugin.PostTime,
		cfg.MoviePlugin.PostChan, cfg.MoviePlugin.PostNick,
	)
	pm.manager.Register(moviePlugin)

	// Movie request plugin
	movieRequestPlugin := media.NewMovieRequestPlugin(bot, adminPlugin, cfg.MovieDBPath)
	if movieRequestPlugin != nil {
		pm.manager.Register(movieRequestPlugin)
	}

	// Movie completion plugin
	movieCompletionPlugin := media.NewMovieCompletionPlugin(bot, adminPlugin, cfg.MovieDBPath)
	if movieCompletionPlugin != nil {
		pm.manager.Register(movieCompletionPlugin)
	}

	// Movie deletion plugin
	movieDeletionPlugin := media.NewMovieDeletionPlugin(bot, adminPlugin, cfg.MovieDBPath)
	if movieDeletionPlugin != nil {
		pm.manager.Register(movieDeletionPlugin)
	}

	log.Printf("✅ Movie pluginok regisztrálva")
}

func (pm *PluginManager) registerMediaPlugins(bot *irc.Client, cfg *config.Config, adminPlugin *admin.AdminPlugin) {
	// Media upload plugin
	mediaUploadPlugin := media.NewMediaUploadPlugin(bot, cfg)
	pm.manager.Register(mediaUploadPlugin)
	if err := mediaUploadPlugin.Start(); err != nil {
		log.Printf("❌ Media upload plugin indítási hiba: %v", err)
	}

	// Media ajánló plugin
	mediaPlugin := media.NewMediaAjanlatPlugin(
		bot, cfg.JellyfinDBPath, cfg.MediaAjanlat.Channel, cfg.MediaAjanlat.Time,
	)
	pm.manager.Register(mediaPlugin)

	// Viccek plugin
	jokePlugin := ynm.NewJokePlugin(bot, cfg.JokeChannels, cfg.JokeSendTime)
	pm.manager.Register(jokePlugin)
	jokePlugin.Start()

	log.Printf("✅ Media pluginok regisztrálva")
}

func (pm *PluginManager) registerScheduledPlugins(bot *irc.Client, cfg *config.Config) {
	// Székelyhon plugin
	if cfg.SzekelyhonInterval != "" && len(cfg.SzekelyhonChannels) > 0 {
		interval, err := time.ParseDuration(cfg.SzekelyhonInterval)
		if err != nil {
			log.Fatalf("❌ Hibás Székelyhon időzítés: %v", err)
		}

		if err := pm.validateSzekelyhonConfig(cfg); err != nil {
			log.Fatalf("❌ Székelyhon config hiba: %v", err)
		}

		szekelyhonPlugin := ynm.NewSzekelyhonPlugin(
			bot, cfg.SzekelyhonChannels, interval,
			cfg.SzekelyhonStartHour, cfg.SzekelyhonEndHour,
		)
		pm.scheduledPlugins = append(pm.scheduledPlugins, szekelyhonPlugin)
		szekelyhonPlugin.Start()
		log.Printf("✅ Székelyhon plugin regisztrálva")
	}
}

func (pm *PluginManager) validateSzekelyhonConfig(cfg *config.Config) error {
	if cfg.SzekelyhonStartHour < 0 || cfg.SzekelyhonStartHour > 23 {
		log.Fatalf("❌ Hibás Székelyhon kezdő óra: %d", cfg.SzekelyhonStartHour)
	}
	if cfg.SzekelyhonEndHour < 0 || cfg.SzekelyhonEndHour > 23 {
		log.Fatalf("❌ Hibás Székelyhon befejező óra: %d", cfg.SzekelyhonEndHour)
	}
	if cfg.SzekelyhonStartHour >= cfg.SzekelyhonEndHour {
		log.Fatalf("❌ Székelyhon kezdő óra nem lehet >= befejező óránál")
	}
	return nil
}

func (pm *PluginManager) HandleMessage(msg irc.Message) string {
	return pm.manager.HandleMessage(msg)
}

func (pm *PluginManager) HandleTick(bot *irc.Client) {
	// Név nap és egyéb tick pluginok
	for _, plugin := range pm.manager.GetPlugins() {
		if tickablePlugin, ok := plugin.(interface{ OnTick() []ScheduledMessage }); ok {
			for _, msg := range tickablePlugin.OnTick() {
				bot.SendMessage(msg.Channel, msg.Text)
			}
		}
	}
}

func (pm *PluginManager) Shutdown() {
	// Időzített pluginok leállítása
	for _, plugin := range pm.scheduledPlugins {
		plugin.Stop()
		log.Printf("🛑 %s leállítva", plugin.Name())
	}

	// Normál pluginok cleanup
	for _, plugin := range pm.manager.GetPlugins() {
		pm.shutdownPlugin(plugin)
	}
}

func (pm *PluginManager) shutdownPlugin(plugin interface{}) {
	switch p := plugin.(type) {
	case *media.MoviePlugin:
		p.Close()
		log.Printf("🛑 Movie plugin leállítva")
	case *media.MovieCompletionPlugin:
		p.Close()
		log.Printf("🛑 Movie completion plugin leállítva")
	case *media.MovieDeletionPlugin:
		p.Close()
		log.Printf("🛑 Movie deletion plugin leállítva")
	case *media.MovieRequestPlugin:
		p.Close()
		log.Printf("🛑 Movie request plugin leállítva")
	case *media.MediaUploadPlugin:
		p.Stop()
		log.Printf("🛑 Media upload plugin leállítva")
	case *plugins.TamagotchiPlugin:
		p.Shutdown()
		log.Printf("🛑 Tamagotchi plugin leállítva")
	}
}