// ==================================================
// Projekt: YnM-Go IRC-bot
//
// Szerző: Markus Lajos (markus@ynm.hu)
// Web: https://bot.ynm.hu
//
// Ez a fájl a YnM-Go rendszer részét képezi.
// A fájl szabadon felhasználható a projekt szabályai szerint.
//
// License: MIT License (vagy választott nyílt forrású license)
// ==================================================

package YnM

import (
	"log"
	"time"
	"fmt"
	"os"
    "gopkg.in/yaml.v3"
	"strings"
	"git.ynm.hu/markus/YnM-Go/YnMAdmin"
	"git.ynm.hu/markus/YnM-Go/YnMConfig"
	"git.ynm.hu/markus/YnM-Go/YnMDb"
	"git.ynm.hu/markus/YnM-Go/YnMCmd"
	"git.ynm.hu/markus/YnM-Go/YnMIrC"
	ynm "git.ynm.hu/markus/YnM-Go/YnMPlugins"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/media"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/discord"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/telegram"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/botmanager"
	"git.ynm.hu/markus/YnM-Go/YnMModuls/ynmapi"
	"git.ynm.hu/markus/YnM-Go/YnMDebug"
	
)

// Plugin interfészek
type Plugin interface {
    HandleMessage(msg YnMIrC.Message) string
    OnTick() []YnMIrC.Message
}

type StartStopper interface {
    Start()
    Stop()
}

type Loader interface {
    Load() error
    Unload() error
}

type ScheduledPlugin interface {
	Start()
	Stop()
	Name() string
}

type ScheduledMessage = YnMIrC.Message

// Manager struktúra - alapvető plugin kezeléshez
type Manager struct {
    plugins []Plugin
    startStoppers []StartStopper
}


func (m *Manager) Register(plugin Plugin) {
    m.plugins = append(m.plugins, plugin)
    
    // Auto-load if plugin supports loading
    if loader, ok := plugin.(Loader); ok {
        err := loader.Load()
        if err != nil {
            log.Printf("❌ Plugin load failed: %v", err)
        } else {
            log.Printf("✅ Plugins loading ...")
        }
    }
    
    if ss, ok := plugin.(StartStopper); ok {
        m.startStoppers = append(m.startStoppers, ss)
    }
}

func NewManager() *Manager {
	return &Manager{
		plugins: make([]Plugin, 0),
	}
}

func (m *Manager) HandleMessage(msg YnMIrC.Message) string {
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
	client   *YnMIrC.Client
	plugins  []Plugin
	manager          *Manager
	scheduledPlugins []ScheduledPlugin
	adminDB          *YnMDb.AdminDB // Add reference to AdminDB
}

func NewPluginManager(adminDB *YnMDb.AdminDB) *PluginManager {
	return &PluginManager{
		manager:          NewManager(),
		scheduledPlugins: make([]ScheduledPlugin, 0),
		adminDB:          adminDB,
	}
}

func isDiscordChannelLatestMusic(channel string) bool {
		if len(channel) == 0 {
			return false
		}
		for _, char := range channel {
			if char < '0' || char > '9' {
				return false
			}
		}
		return true
	}
// UpdatePluginStatesInDB updates the plugin states in database based on configuration
func (pm *PluginManager) UpdatePluginStatesInDB(cfg *YnMConfig.Config, channels []string) error {
	// Define all available plugins with their config mapping
	pluginStates := map[string]bool{
		"ping":            cfg.Plugins.EnablePing,
		"nameday":         cfg.Plugins.EnableNameDay,
		"onthisday": 	 cfg.Plugins.EnableOnthisDay,
		"ora":             cfg.Plugins.EnableOra,
		"huntorrent":      cfg.Plugins.EnableHuntorrent,
		"horoscope":       cfg.Plugins.EnableHoroscope,
		"weather":         cfg.Plugins.EnableWeather,
		"seen":           	cfg.Plugins.EnableSeen,
		"sms":             	cfg.Plugins.EnableSms,
		"alexa":			cfg.Plugins.EnableAlexa,
		"status":          cfg.Plugins.EnableStatus,
		"vicc":            cfg.Plugins.EnableVicc,
		"xp":              cfg.Plugins.EnableXP,
		"mp3":			cfg.Plugins.EnableMp3Scanner,
		"monitor":         cfg.Plugins.EnableMonitor,
		"link":            cfg.Plugins.EnableLink,
		"forum":           cfg.Plugins.EnableForum,
		"bruteforce":            cfg.Plugins.EnableBruteforceAttack,
		"resourcemonitor":	cfg.Plugins.EnableResourceMonitor,
		"webhook":         cfg.Plugins.EnableWebhook,
		"external":		cfg.Plugins.EnableExternalTrigger,
		"movie":           cfg.Plugins.EnableMovie,
		"movie_request":   cfg.Plugins.EnableMovieRequest,
		"movie_completion": cfg.Plugins.EnableMovieCompletion,
		"movie_deletion":  cfg.Plugins.EnableMovieDeletion,
		"media_upload":    cfg.Plugins.EnableMediaUpload,
		"media_ajanlat":   cfg.Plugins.EnableMediaAjanlat,
		"joke":            cfg.Plugins.EnableJoke,
		"jellyfin_info":   cfg.Plugins.EnableJellyfinInfo,
		"media_activity":  cfg.Plugins.EnableMediaActivity,
		"szekelyhon":      cfg.Plugins.EnableSzekelyhon,
		"LegszebbNotak":	cfg.Plugins.EnableLatestMusic,
		"Telegram":		cfg.Plugins.EnableTelegram,
		"git":             cfg.Plugins.EnableGit,
		"imdb":            cfg.Plugins.EnableImdb,
		"xes0":            cfg.Plugins.EnableXes0,
		"ssh":             cfg.Plugins.EnableSsh,
		"nmap":            cfg.Plugins.EnableNmap,
		"dns":             cfg.Plugins.EnableDns,
		"chatgpt":         cfg.Plugins.EnableChatGPT,
		"ip":              cfg.Plugins.EnableIp,
		"pinghost":        cfg.Plugins.EnablePingHost,
		"uptime": 		cfg.Plugins.EnableUptime,
		"learn":           cfg.Plugins.EnableLearn,
		"youtube":         cfg.Plugins.EnableYouTube,
		"debug":           cfg.Plugins.EnableDebug,
		"control":			cfg.Plugins.EnableControl,
		"update":			cfg.Plugins.EnableUnifiedUpdate,
		"ctcp":				cfg.Plugins.EnableCTCP,
		"botok":		cfg.Plugins.EnableBotManager,
		"help":			cfg.Plugins.EnableHelp,
		"shelltopic":     cfg.Plugins.EnableTopicUpdater,
		"ynmapi":		cfg.Plugins.EnableYnMApi,
		"services":	cfg.Plugins.EnableServiceManager,
		"fail2ban": 	 cfg.Plugins.EnableFail2Ban,
		"discord":		cfg.Plugins.EnableDiscord,
	}

	// First, ensure all plugins exist in plugins table
	for pluginName := range pluginStates {
		err := pm.adminDB.EnsurePlugin(pluginName, "")
		if err != nil {
			log.Printf("❌ Failed to ensure plugin %s exists: %v", pluginName, err)
			continue
		}
	}

	// Update plugin states for each channel
	for _, channel := range channels {
		for pluginName, isEnabled := range pluginStates {
			err := pm.adminDB.SetPluginState(pluginName, channel, isEnabled)
			if err != nil {
				log.Printf("❌ Failed to set plugin state for %s in %s: %v", pluginName, channel, err)
			} else {
				status := "disabled"
				if isEnabled {
					status = "enabled"
				}
				_ = status
				//log.Printf("✅ Plugin %s %s in %s", pluginName, status, channel)
			}
		}
	}

	return nil
}

func isDiscordChannelSzekelyhon(channel string) bool {
	if len(channel) == 0 {
		return false
	}
	for _, char := range channel {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func isDiscordChannelNameDay(channel string) bool {
    if len(channel) == 0 {
        return false
    }
    for _, char := range channel {
        if char < '0' || char > '9' {
            return false
        }
    }
    return true
}

func (pm *PluginManager) RegisterAll(bot *YnMIrC.Client, cfg *YnMConfig.Config) error {
	stopChan := make(chan struct{}) 
	var channels []string
	if cfg.ConsoleChannel != "" {
		channels = append(channels, cfg.ConsoleChannel)
	}
	// Add other channels as needed from your config
	for _, channel := range cfg.SzekelyhonChannels {
		channels = append(channels, channel)
	}
	for _, channel := range cfg.NevnapChannels {
		channels = append(channels, channel)
	}

	// If no channels found, use a default
	if len(channels) == 0 {
		channels = []string{"#YnM"}
	}

	// Update plugin states in database first
	if pm.adminDB != nil {
		if err := pm.UpdatePluginStatesInDB(cfg, channels); err != nil {
			log.Printf("❌ Failed to update plugin states in database: %v", err)
		}
	}

	// ============== YnM Admin Plugin ===============
	ynmAdminPlugin, err := owner.NewYnmAdminPlugin(cfg)
	
	if err != nil {
		log.Fatalf("Nem sikerült létrehozni az YnM admin plugint: %v", err)
	}
	if err != nil {
		log.Fatalf("Nem sikerült biztosítani a ConsoleChannel-t: %v", err)
	}

	ynmAdminPlugin.Initialize(bot)
	bot.AddPlugin(ynmAdminPlugin)

	pm.manager.Register(ynmAdminPlugin)
	//log.Printf("✅ YnM Admin regisztrálva")



	// Discord plugin betöltése
	var discordAdapter *discord.DiscordAdapter

	if cfg.Plugins.EnableDiscord {
		// Discord plugin létrehozása
		discordPlugin := discord.NewDiscordPlugin(cfg.Discord.Token, pm)
		discordInfoPlugin := discord.NewDiscordInfoPlugin(bot, discordPlugin.Adapter.Session)
		
		// Mentés az adapterre referenciának
		discordAdapter = discordPlugin.Adapter
		
		pm.Register(discordPlugin)
		pm.manager.Register(discordInfoPlugin)
		
		//log.Printf("✅ Discord plugin regisztrálva")
		//log.Printf("✅ Discord Info plugin regisztrálva")
		
		// Discord kapcsolat indítása
		if err := discordPlugin.Start(); err != nil {
			log.Printf("❌ Discord plugin indítási hiba: %v", err)
		} else {
			//log.Printf("✅ Discord plugin elindítva")
		}
	}
	// ========== YnM  PLUGINS ==========
	// Székelyhon Plugin
	if cfg.Plugins.EnableSzekelyhon && cfg.SzekelyhonInterval != "" && len(cfg.SzekelyhonChannels) > 0 {
		//log.Printf("🔍 Székelyhon plugin inicializálása...")
		
		var ircChannels []string
		var discordChannels []string
		
		for _, channel := range cfg.SzekelyhonChannels {
			if isDiscordChannelSzekelyhon(channel) {
				discordChannels = append(discordChannels, channel)
			} else {
				ircChannels = append(ircChannels, channel)
			}
		}
		
		var szekelyhonPlugin *ynm.SzekelyhonPlugin
		
		if len(discordChannels) > 0 && discordAdapter != nil {
			szekelyhonPlugin = ynm.NewSzekelyhonPluginWithDiscord(bot, cfg, discordAdapter)
			//log.Printf("✅ Székelyhon (IRC: %d, Discord: %d)", len(ircChannels), len(discordChannels))
		} else {
			interval, _ := time.ParseDuration(cfg.SzekelyhonInterval)
			szekelyhonPlugin = ynm.NewSzekelyhonPlugin(
				bot, ircChannels, interval,
				cfg.SzekelyhonStartHour, cfg.SzekelyhonEndHour,
			)
			//log.Printf("✅ Székelyhon (IRC: %d)", len(ircChannels))
		}
		
		pm.scheduledPlugins = append(pm.scheduledPlugins, szekelyhonPlugin)
		szekelyhonPlugin.Start()
	}
	
	
	// LatestMusic Plugin betöltése
	if cfg.Plugins.EnableLatestMusic && cfg.LatestMusicInterval != "" && len(cfg.LatestMusicChannels) > 0 {
	//log.Printf("🔍 LatestMusic plugin inicializálása...")
	
	var ircChannels []string
	var discordChannels []string
	
	// Csatornák szétválogatása
	for _, channel := range cfg.LatestMusicChannels {
		if isDiscordChannelLatestMusic(channel) {
			discordChannels = append(discordChannels, channel)
		} else {
			ircChannels = append(ircChannels, channel)
		}
	}
	
	// Telegram adapter inicializálása
	var telegramAdapter *telegram.TelegramAdapter
	if cfg.Plugins.EnableTelegram && cfg.Telegram.Enabled {
		telegramAdapter = telegram.NewTelegramAdapter(
			cfg.Telegram.Enabled,
			cfg.Telegram.MinInterval,
			cfg.Telegram.BotToken,
			cfg.Telegram.ChatID,
			cfg.Telegram.ChannelID,
		)
	}

	
	var latestMusicPlugin *ynm.LatestMusicPlugin
	
	// Ha van Discord vagy Telegram, használj teljes konstruktort
	if (len(discordChannels) > 0 && discordAdapter != nil) || telegramAdapter != nil {
		latestMusicPlugin = ynm.NewLatestMusicPluginWithDiscord(bot, cfg, discordAdapter, telegramAdapter)
		//log.Printf("✅ LatestMusic (IRC: %d, Discord: %d, Telegram: %v)", 
	//		len(ircChannels), 
//			len(discordChannels), 
//			telegramAdapter != nil && telegramAdapter.IsEnabled())
	} else {
		// Csak IRC - régi konstruktor
		interval, _ := time.ParseDuration(cfg.LatestMusicInterval)
		latestMusicPlugin = ynm.NewLatestMusicPlugin(bot, ircChannels, interval)
		//log.Printf("✅ LatestMusic (IRC: %d)", len(ircChannels))
	}
		
		pm.scheduledPlugins = append(pm.scheduledPlugins, latestMusicPlugin)
		latestMusicPlugin.Start()
	}
   // Mp3 Scannner
   
   	if cfg.Plugins.EnableMp3Scanner {
		mp3Plugin := ynm.NewMp3ScannerPlugin(bot, cfg, discordAdapter)
		pm.manager.Register(mp3Plugin)
		//log.Printf("✅ Mp3Scanner plugin regisztrálva")
	}

	// Nevnap Plugin
	if cfg.Plugins.EnableNameDay {
		var ircChannels []string
		var discordChannels []string
		
		for _, channel := range cfg.NevnapChannels {
			if isDiscordChannelNameDay(channel) {
				discordChannels = append(discordChannels, channel)
				//log.Printf("  🎮 Névnap Discord csatorna: %s", channel)
			} else {
				ircChannels = append(ircChannels, channel)
				//log.Printf("  📡 Névnap IRC csatorna: %s", channel)
			}
		}
		
		// Ha vannak Discord csatornák ÉS elérhető a discordAdapter, akkor használd a Discord-os konstruktort
		if len(discordChannels) > 0 && discordAdapter != nil {
			nameDayPlugin, err := ynm.NewNameDayPluginWithDiscord(ircChannels, discordChannels, cfg.NevnapReggel, cfg.NevnapEste, discordAdapter)
			if err != nil {
				log.Fatalf("Névnap plugin inicializálás hiba: %v", err)
			}
			pm.manager.Register(nameDayPlugin)
			//log.Printf("✅ Névnap plugin regisztrálva (IRC: %d, Discord: %d csatorna)", len(ircChannels), len(discordChannels))
		} else {
			// Egyébként csak az eredeti IRC konstruktor
			nameDayPlugin, err := ynm.NewNameDayPlugin(ircChannels, cfg.NevnapReggel, cfg.NevnapEste)
			if err != nil {
				log.Fatalf("Névnap plugin inicializálás hiba: %v", err)
			}
			pm.manager.Register(nameDayPlugin)
			//log.Printf("✅ Névnap plugin regisztrálva (%d IRC csatornára)", len(ircChannels))
			if len(discordChannels) > 0 {
				//log.Printf("  ⚠️ Discord csatornák figyelmen kívül maradnak (nincs Discord plugin)")
			}
		}
	}
	// On Join 	
		
	if cfg.Plugins.EnableJoinMessage && len(cfg.JoinMessageChannels) > 0 {
		joinMessagePlugin := ynm.NewJoinMessagePluginWithDiscord(
			bot, cfg, discordAdapter,
		)
		pm.scheduledPlugins = append(pm.scheduledPlugins, joinMessagePlugin)
		joinMessagePlugin.Start()
		//log.Printf("✅ JoinMessage plugin regisztrálva - %d IRC csatornára, %d Discord csatornára", 
		//	len(cfg.JoinMessageChannels), len(cfg.JoinMessageDiscordChannels))
	}


	//	Óra plugin
	if cfg.Plugins.EnableOra {
		oraPlugin := ynm.NewOraPlugin(bot, ynmAdminPlugin, cfg)
		pm.manager.Register(oraPlugin)
		//log.Printf("✅ Óra plugin regisztrálva")
	}

	// Fail2Ban plugin
	if cfg.Plugins.EnableFail2Ban {
		f2bPlugin := ynm.NewFail2BanPlugin(bot, cfg.Fail2Ban.Log, cfg.Fail2Ban.Channel)
		pm.manager.Register(f2bPlugin)
		//log.Printf("✅ Fail2Ban plugin regisztrálva és betöltve, csatornák: %v", cfg.Fail2Ban.Channel)
	}


	
	//		XP plugin
	if cfg.Plugins.EnableXP {
		xpManager, err := ynm.NewXPManagerFromConfig("YnMConfig/xp.yaml")
		if err != nil {
			log.Printf("XP Manager inicializálási hiba: %v", err)
		} else {
			xpPlugin := ynm.NewXPPlugin(xpManager)
			pm.manager.Register(xpPlugin)
			//log.Printf("✅ XP plugin regisztrálva")
		}
	}
	

	// YouTube plugin
	if cfg.Plugins.EnableYouTube {
		youtubeConfig := ynm.YouTubeConfig{
			YtApi:      cfg.YtApi,
			YtChannels: cfg.YtChannels,
		}
		youtubePlugin := ynm.NewYouTubePlugin(bot, youtubeConfig)
		pm.manager.Register(youtubePlugin)
		//log.Printf("✅ YouTube plugin regisztrálva - engedélyezett channelok: %v", youtubeConfig.YtChannels)
	}

	// Vicc plugin
	if cfg.Plugins.EnableVicc {
		viccPlugin := ynm.NewViccPlugin(bot, ynmAdminPlugin)
		pm.manager.Register(viccPlugin)
		//log.Printf("✅ Vicc plugin regisztrálva")
	}
	
	//		Seen Plugin 
	if cfg.Plugins.EnableSeen {
		seenPlugin, err := ynm.NewSeenPlugin(bot, ynmAdminPlugin, cfg.SeenDBPath, cfg.SearchNotificationDelay)
		 if err != nil {
			 log.Printf("❌ Seen plugin inicializálás hiba: %v", err)
		 } else {
			 pm.manager.Register(seenPlugin)
			 //log.Printf("✅ Seen plugin regisztrálva")
		 }
	 }
	
	
	
	//		Monitor Plugin
	if cfg.Plugins.EnableMonitor {
		monitorCfg, err := ynm.LoadServiceMonitorConfig()
		 if err != nil {
			 log.Fatalf("Nem sikerült betölteni a konfigurációt: %v", err)
		 }

		serviceMonitorPlugin, err := ynm.NewServiceMonitorPlugin(bot, monitorCfg.MChan)
		if err != nil {
			log.Fatalf("ServiceMonitorPlugin inicializálási hiba: %v", err)
		}

		pm.manager.Register(serviceMonitorPlugin)
		serviceMonitorPlugin.Start()
		//log.Printf("✅ Service Monitor plugin regisztrálva")
	}
	
		//		ChatGPT Plugin
	if cfg.Plugins.EnableChatGPT {
		chatGPTPlugin := ynm.NewChatGPTPlugin(bot, ynmAdminPlugin, ynmAdminPlugin.Db, cfg.OpenAI.APIKey, 10*time.Second)
		pm.manager.Register(chatGPTPlugin)
		//log.Printf("✅ ChatGPT plugin regisztrálva")
	}
	
		//		LinkLoger  Plugin
	if cfg.Plugins.EnableLink {
		linkPlugin, err := ynm.NewLinkPlugin(bot)
		if err != nil {
			 log.Fatalf("Nem sikerült betölteni a Link plugin-t: %v", err)
		 }
		 pm.manager.Register(linkPlugin)
		//log.Printf("✅ Link plugin regisztrálva")
	 }
	 
		//		GIT Plugin	 
	if cfg.Plugins.EnableGit {
		gitPlugin, err := ynm.NewGitPluginWithDiscord(bot, cfg.GitPlugin, discordAdapter)
		if err != nil {
			log.Printf("⚠️ Git plugin betöltési hiba: %v (kikapcsolva)", err)
		} else {
			gitPlugin.Start()
			pm.manager.Register(gitPlugin)
			//log.Printf("✅ Git plugin regisztrálva (Discord támogatással: %v)", discordAdapter != nil)
		}
	}
		
				//		OnthisDay	 
	if cfg.Plugins.EnableOnthisDay {
		onThisDayPlugin, err := ynm.NewOnThisDayPlugin(bot, cfg.OnThisDayPlugin, discordAdapter)
		if err != nil {
			log.Printf("⚠️ OnThisDay plugin betöltési hiba: %v (kikapcsolva)", err)
		} else {
			onThisDayPlugin.Start()
			pm.manager.Register(onThisDayPlugin)
			//log.Printf("✅ OnThisDay plugin regisztrálva (Discord támogatással: %v)", discordAdapter != nil)
		}
	}
	//		Horoszkóp plugin
	if cfg.Plugins.EnableHoroscope {
		 horoszkopPlugin := ynm.NewHoroszkopPlugin(bot)
		 pm.manager.Register(horoszkopPlugin)
		 //log.Printf("✅ Horoszkóp plugin regisztrálva")
	}
	
	//		Forum Plugin 

	if cfg.Plugins.EnableForum {
		forumPlugin, err := ynm.NewForumPlugin(bot, nil)
		if err != nil {
			log.Fatalf("Forum plugin hiba: %v", err)
		}
		pm.manager.Register(forumPlugin)
		forumPlugin.StartPolling() 
		//log.Printf("✅ Forum plugin regisztrálva")
	}
		
	//			Learn Plugin
	if cfg.Plugins.EnableLearn {
		 learnPlugin, err := ynm.NewLearnPlugin(bot, ynmAdminPlugin, "./data/learn.db")
		 if err != nil {
			 log.Fatalf("❌ Learn plugin init hiba: %v", err)
		 }
		 pm.manager.Register(learnPlugin)
		 //log.Printf("✅ Learn plugin regisztrálva")
	}
	
	//		SMS Plugin 
	if cfg.Plugins.EnableSms {
		smsPlugin, err := ynm.NewSmsPlugin(bot, cfg.SmsDBPath)
		if err != nil {
			 log.Printf("❌ SMS plugin inicializálási hiba: %v", err)
		} else {
			pm.manager.Register(smsPlugin)
		//	 log.Printf("✅ SMS plugin regisztrálva")
		}
	}
	
	//Alexa plugin
if cfg.Plugins.EnableAlexa {
    alexaPlugin, err := ynm.NewAlexaPlugin(bot, cfg.AlexaNodeRed)  // ← Nincs bot.Nick
    if err != nil {
        log.Printf("❌ Alexa plugin inicializálási hiba: %v", err)
    } else {
        pm.manager.Register(alexaPlugin)
        log.Printf("✅ Alexa plugin regisztrálva")
    }
}

	// Torrent RSS Plugin-ok
	if cfg.Plugins.EnableHuntorrent {
		data, err := os.ReadFile("YnMConfig/rss.yaml")
		if err != nil {
			log.Fatalf("❌ Nem található YnMConfig/rss.yaml fájl: %v", err)
		}
		
		var rssCfg struct {
			Feeds []YnMConfig.TorrentConfig `yaml:"feeds"`
		}
		if err := yaml.Unmarshal(data, &rssCfg); err != nil {
			log.Fatalf("❌ Hiba rss.yaml feldolgozásakor: %v", err)
		}
		
		for _, feed := range rssCfg.Feeds {
			if feed.Enabled && strings.TrimSpace(feed.RSSUrl) != "" {
				rssPlugin := ynm.NewTorrentRSS(bot, feed)
				pm.manager.Register(rssPlugin)
				//log.Printf("✅ Torrent RSS plugin regisztrálva: %s -> %v", feed.Name, feed.Channels)
			} else if feed.Enabled && strings.TrimSpace(feed.RSSUrl) == "" {
				log.Printf("⚠️ Torrent RSS feed '%s' engedélyezve, de nincs RSS URL megadva!", feed.Name)
			} else {
				log.Printf("ℹ️ Torrent RSS feed '%s' letiltva", feed.Name)
			}
		}
	}

	// HunTorrent Chat plugin
	if cfg.Plugins.EnableHuntorrent {
		NewHunTorrentChatPlugin := ynm.NewHunTorrentPlugin(bot, &cfg.HunTorrentChat)
		if NewHunTorrentChatPlugin == nil {
			log.Fatalf("Nem sikerült betölteni a HunTorrent Chat plugin-t")
		}
		NewHunTorrentChatPlugin.Start()
		//log.Printf("✅ HunTorrent Chat plugin regisztrálva")
	}
		
	//Server Stufs
	// BruteforceAttack 
	if cfg.Plugins.EnableBruteforceAttack {
		 NewBruteforceAttackPlugin := ynm.NewBruteforceAttackPlugin(bot, cfg, discordAdapter)
		 if NewBruteforceAttackPlugin == nil {
			 log.Fatalf("Nem sikerült betölteni a BruteforceAttack plugin-t")
		 }
		 NewBruteforceAttackPlugin.Start()
		 //log.Printf("✅ BruteforceAttack plugin regisztrálva")
	}

	if cfg.Plugins.EnableResourceMonitor {
		resourcePlugin := ynm.NewYnMResourceMonitorPlugin(bot, ynmAdminPlugin, cfg, discordAdapter)
		if resourcePlugin == nil {
			log.Fatalf("Nem sikerült betölteni a ResourceMonitor plugin-t")
		}

		// Ha automatikusan szeretnéd elindítani a figyelést a konfiguráció alapján:
		if cfg.ResourceMonitor != nil && cfg.ResourceMonitor.AutoStart {
			resourcePlugin.Start()
		}

		// Regisztráljuk a manager-ben
		pm.manager.Register(resourcePlugin)
		// log.Printf("✅ ResourceMonitor plugin regisztrálva")
	}	
	//		IMDB Plugin
	if cfg.Plugins.EnableImdb {
		 imdbCfg, err := ynm.LoadIMDBConfig("YnMConfig/ynm.yaml")
		 if err != nil {
			 log.Printf("⚠️ IMDB config hiba: %v (kikapcsolva)", err)
		 } else {
			 imdbPlugin := ynm.NewIMDBPluginFromConfig(bot, imdbCfg, ynmAdminPlugin)
			 pm.manager.Register(imdbPlugin)
			 //log.Printf("✅ IMDB plugin regisztrálva")
		 }
	 }
	 
	 //		WebHook Plugin
if cfg.Plugins.EnableWebhook {
	// Keressük meg a Discord plugint a regisztrált pluginok között
	var discordPlugin *discord.DiscordPlugin
	for _, p := range pm.manager.plugins {
		if dp, ok := p.(*discord.DiscordPlugin); ok {
			discordPlugin = dp
			break
		}
	}
	
	// Webhook plugin inicializálása Discord pluginnal
	webhookPlugin := ynm.NewWebhookPlugin(bot, discordPlugin)
	webhookPlugin.StartHTTP("2020")
	pm.manager.Register(webhookPlugin)
	
	if discordPlugin == nil {
		log.Printf("⚠️ Webhook plugin elindult, de Discord plugin nincs betöltve!")
	}
}
	
	// External Trigger Plugin
	if cfg.Plugins.EnableExternalTrigger {
		externalTriggerPlugin := ynm.NewExternalTriggerPlugin(bot)
		pm.manager.Register(externalTriggerPlugin)
		//log.Printf("✅ External Trigger plugin regisztrálva")
	}
	
	
		// ===== YnMApI Plugin =====
		if cfg.Plugins.EnableYnMApi {
			// Give admin plugin time to fully initialize its database
			time.Sleep(5 * time.Second)
			
			// Import már megtörtént, használjuk:
			ynmapIPlugin := ynmapi.NewYnMApiPlugin(bot, cfg, ynmAdminPlugin, ynmAdminPlugin.Db) 
			
			// Config reload callback beállítása
			ynmapIPlugin.SetConfigReloadCallback(func() error {
				return ynmapIPlugin.ReloadConfig()
			})
			
			pm.manager.Register(ynmapIPlugin)
			log.Printf("✅ YnMApi plugin registered with ynm-api.yaml config")
		}
	
	//		Státusz plugin
	if cfg.Plugins.EnableStatus {
		statusPlugin := ynm.NewStatusPlugin(bot, ynmAdminPlugin, ynmAdminPlugin.Db)
		pm.manager.Register(statusPlugin)
		//log.Printf("✅ Status plugin regisztrálva")
	}
	
	//		Weather plugin
	if cfg.Plugins.EnableWeather {
		weatherPlugin := ynm.NewWeatherPlugin(bot, cfg.Weather)
		pm.manager.Register(weatherPlugin)
		//log.Printf("✅ Weather plugin regisztrálva")
	}
	
	//==========IRC Stuf =================//
	
	//Ping plugin
	if cfg.Plugins.EnablePing {
		duration, err := time.ParseDuration(cfg.PingCommandCooldown)
		if err != nil {
			log.Fatalf("Ping cooldown parsing hiba: %v", err)
		}
		pingPlugin := ynm.NewPingPlugin(bot, duration, ynmAdminPlugin)
		bot.OnPong = func(pongID string) { pingPlugin.HandlePong(pongID) }
		pm.manager.Register(pingPlugin)
		//log.Printf("✅ Ping plugin regisztrálva")
	}
	
	//========= YnM Network Stuf===========//
	
	//   SSH Plugin
	if cfg.Plugins.EnableSsh {
		sshPlugin := ynm.NewSSHPlugin(bot, ynmAdminPlugin, 10*time.Second)
		pm.manager.Register(sshPlugin)
		//log.Printf("✅ SSH plugin regisztrálva")
	}
	
	
	// Nmap Plugin	
	if cfg.Plugins.EnableNmap {
		nmapPlugin := ynm.NewNmapPlugin(bot, ynmAdminPlugin)
		pm.manager.Register(nmapPlugin)
		//log.Printf("✅ Nmap plugin regisztrálva")
	}
	
	// Ip Plugin		
	if cfg.Plugins.EnableIp {
		ipPlugin := ynm.NewIPPlugin(bot, ynmAdminPlugin, 10*time.Second)
		pm.manager.Register(ipPlugin)
		//log.Printf("✅ IP plugin regisztrálva")
	}
	
	// DNS Plugin
	if cfg.Plugins.EnableDns {
		config := ynm.DNSConfig{
			Timeout:     10 * time.Second,
			MaxRecords:  8,
			CacheTTL:    3 * time.Minute,
			RateLimit:   1 * time.Second,
		}
		dnsPlugin := ynm.NewDNSPlugin(bot, ynmAdminPlugin, config)
		pm.manager.Register(dnsPlugin)
		//log.Printf("✅ DNS plugin regisztrálva")
	}
	
	if cfg.Plugins.EnablePingHost {
		pinghostPlugin := ynm.NewPingHostPlugin(bot, ynmAdminPlugin, 10*time.Second)
		pm.manager.Register(pinghostPlugin)
		//log.Printf("✅ PingHost plugin regisztrálva")
	}
	
	if cfg.Plugins.EnableUptime {
		uptimePlugin := YnMCmd.NewUptimePlugin(bot, ynmAdminPlugin, ynmAdminPlugin.Db)
		pm.manager.Register(uptimePlugin) // ✅ HELYES: manager-en keresztül regisztráljuk
		//log.Println("✅ Uptime plugin regisztrálva")
	}


	if cfg.Plugins.EnableServiceManager {
		serviceManagerPlugin := ynm.NewServiceManager(bot, ynmAdminPlugin, ynmAdminPlugin.Db)
		if serviceManagerPlugin != nil {
			pm.manager.Register(serviceManagerPlugin)
			//log.Printf("✅ Service Manager plugin regisztrálva")
		}
	}

	// YnM owner Plugins
	
	if cfg.Console.Enabled {
		consolePlugin := ynm.NewPartylineConsole(bot, ynmAdminPlugin, ynmAdminPlugin.Db, cfg.Console)
		pm.manager.Register(consolePlugin)
		//log.Printf("✅ Partyline Console plugin regisztrálva (port: %d)", cfg.Console.Port)
	}
	
	if cfg.Plugins.EnableUnifiedUpdate {
		unifiedupdatePlugin := YnMCmd.NewUnifiedUpdatePlugin(bot, ynmAdminPlugin, cfg)
		pm.manager.Register(unifiedupdatePlugin)
		//log.Printf("✅ UnifiedUpdate plugin regisztrálva")
	}
	
	if cfg.Plugins.EnableHelp {
		helpPlugin := YnMCmd.NewHelpPlugin(bot, ynmAdminPlugin)
		pm.manager.Register(helpPlugin)
		//log.Printf("✅ Help plugin regisztrálva")
	}
	
	if cfg.Plugins.EnableCTCP {
		ctcpPlugin := NewCTCPPlugin(bot)
		pm.manager.Register(ctcpPlugin)
		//log.Printf("✅ CTCP plugin regisztrálva")
	}
	// Bot Manager JAVÍTOTT verzió
	if cfg.Plugins.EnableBotManager {
		// ✅ FONTOS: Plugin managert átadjuk!
		bmPlugin, err := botmanager.NewBotManagerPlugin(bot, cfg.BotManager.ConfigPath, pm, ynmAdminPlugin)
		if err != nil {
			log.Printf("❌ BotManager inicializálási hiba: %v", err)
		} else {
			pm.manager.Register(bmPlugin)
			log.Printf("✅ BotManager plugin regisztrálva")
			
			bmPlugin.Start()
			log.Printf("✅ BotManager Start() meghívva")
		}
	}

	if true {  
		cyclePlugin := YnMCmd.NewCyclePlugin(bot, ynmAdminPlugin, stopChan)
		pm.manager.Register(cyclePlugin)		
		bot.OnJoin = func(channel, nick, hostmask string) {
			cyclePlugin.HandleJoin(nick, channel)			
			if nick != bot.GetNick() && ynmAdminPlugin != nil {
				go func() {
					time.Sleep(300 * time.Millisecond)
					ynmAdminPlugin.AutoApplyUserModes(nick, hostmask, channel)
				}()
			}
			
			if nick == bot.GetNick() {
				go func() {
					time.Sleep(5 * time.Second)
					bot.SendRaw(fmt.Sprintf("WHO %s", channel))
				}()
			}
		} 

		bot.OnWho = func(channel, nick, hostmask string) {		
			if ynmAdminPlugin != nil && nick != bot.GetNick() {
				ynmAdminPlugin.AutoApplyUserModes(nick, hostmask, channel)
			}
		}
		
		bot.OnPart = func(channel, nick, reason string) {
			cyclePlugin.HandlePart(nick, channel)
		}
		bot.OnQuit = func(nick, reason string) {
			cyclePlugin.HandleQuit(nick) 
		}
		bot.OnMode = func(channel, modes, args, prefix string) {
			if strings.Contains(modes, "+o") || strings.Contains(modes, "-o") {
				argList := strings.Fields(args)
				modeIndex := 0
				
				for i := 0; i < len(modes); i++ {
					mode := modes[i]
					if mode == '+' || mode == '-' {
						continue
					}
					
					if (mode == 'o') && modeIndex < len(argList) {
						nick := argList[modeIndex]
						modeIndex++
						
						if i > 0 {
							prefix := string(modes[i-1])
							modeStr := prefix + string(mode)
							cyclePlugin.HandleMode(channel, modeStr, nick, prefix)
						}
					}
				}
			}
		}		   
		bot.OnNames = func(channel string, names []string) {
			cyclePlugin.HandleNamesList(channel, names)
		}
	}
	if cfg.Plugins.EnableTopicUpdater {  
		topicPlugin := YnMCmd.NewTopicUpdaterPlugin(bot, cfg, ynmAdminPlugin)
		pm.manager.Register(topicPlugin)
		//log.Printf("✅ Topic Updater plugin regisztrálva")
		
		// Késleltetett inicializálás (opcionális)
		go func() {
			//log.Printf("🔄 Topic Updater inicializálás indul 60 másodperc múlva...")
			time.Sleep(60 * time.Second)
			topicPlugin.Initialize()
			//log.Printf("✅ Topic Updater inicializálva")
		}()
	}
	if cfg.Plugins.EnableControl {
		controlPlugin := YnMCmd.NewControlPlugin(bot, ynmAdminPlugin, stopChan)
		controlPlugin.SetConfigReloadCallback(func() error {
			newCfg, err := YnMConfig.Load("YnMConfig/ynm.yaml")
			if err != nil {
				return fmt.Errorf("nem sikerült betölteni a ynm.yaml fájlt: %v", err)
			}
			// IDE tedd a debug logot
			log.Printf("DEBUG: NickServLogin=%v\n", newCfg.NickServLogin)
			changes := []string{}
			if cfg.Plugins.EnablePing != newCfg.Plugins.EnablePing {
				status := "❌ KIKAPCSOLVA"
				if newCfg.Plugins.EnablePing { 
					status = "✅ BEKAPCSOLVA" 
				}
				changes = append(changes, fmt.Sprintf("Ping plugin: %s", status))
			}			
			if cfg.Plugins.EnableNameDay != newCfg.Plugins.EnableNameDay {
				status := "❌ KIKAPCSOLVA"
				if newCfg.Plugins.EnableNameDay { 
					status = "✅ BEKAPCSOLVA" 
				}
				changes = append(changes, fmt.Sprintf("NameDay plugin: %s", status))
			}			
			if cfg.Plugins.EnableXP != newCfg.Plugins.EnableXP {
				status := "❌ KIKAPCSOLVA"
				if newCfg.Plugins.EnableXP { 
					status = "✅ BEKAPCSOLVA" 
				}
				changes = append(changes, fmt.Sprintf("XP plugin: %s", status))
			}			
			if cfg.Plugins.EnableChatGPT != newCfg.Plugins.EnableChatGPT {
				status := "❌ KIKAPCSOLVA"
				if newCfg.Plugins.EnableChatGPT { 
					status = "✅ BEKAPCSOLVA" 
				}
				changes = append(changes, fmt.Sprintf("ChatGPT plugin: %s", status))
			}
			if cfg.ConsoleChannel != newCfg.ConsoleChannel {
				changes = append(changes, fmt.Sprintf("ConsoleChannel: %s → %s", cfg.ConsoleChannel, newCfg.ConsoleChannel))
			}
			log.Printf("✅ Konfiguráció újratöltve: YnMConfig/ynm.yaml")			
			if len(changes) == 0 {
				log.Printf("🔧 Nincs változás az előző config-hoz képest")
				return fmt.Errorf("📋 Config validálva - nincs változás")
			} else {
				log.Printf("🔧 Észlelt változások:")
				for _, change := range changes {
					log.Printf("   - %s", change)
				}				
				changeMsg := fmt.Sprintf("📋 %d változás észlelve: ", len(changes))
				for i, change := range changes {
					if i < 3 { // Max 3 változást mutatunk IRC-en
						if i > 0 {
							changeMsg += ", "
						}
						changeMsg += change
					}
				}
				if len(changes) > 3 {
					changeMsg += fmt.Sprintf(" (+%d további)", len(changes)-3)
				}
				changeMsg += " ⚠️ Alkalmazáshoz: !reload szükséges"
				
				return fmt.Errorf(changeMsg)
			}
		}) 		
		pm.manager.Register(controlPlugin)
		//log.Printf("✅ Control plugin regisztrálva")
	}

//================YnM Moduls =============


// ========== MEDIA Upload PLUGIN ==========
	if cfg.Plugins.EnableMediaUpload {
		var mediaUploadPlugin *media.MediaUploadPlugin
		if cfg.Plugins.EnableDiscord {
			mediaUploadPlugin = media.NewMediaUploadPluginWithDiscord(bot, cfg, discordAdapter)
		} else {
			mediaUploadPlugin = media.NewMediaUploadPlugin(bot, cfg)
		}
		
		pm.manager.Register(mediaUploadPlugin)
		if err := mediaUploadPlugin.Start(); err != nil {
			log.Printf("❌ Media upload plugin indítási hiba: %v", err)
		} else {
			//log.Printf("✅ Media upload plugin regisztrálva")
		}
	}
// ========== MEDIA AJÁNLAT PLUGIN ==========

	if cfg.Plugins.EnableMediaAjanlat {
		//log.Printf("🔍 Media Ajánlat plugin inicializálása...")
		
		var mediaPlugin *media.MediaAjanlatPlugin
		
		// Ha van Discord adapter, használjuk a Discord-támogató konstruktort
		if discordAdapter != nil {
			mediaPlugin = media.NewMediaAjanlatPluginWithDiscord(
				bot, 
				ynmAdminPlugin,
				cfg.JellyfinDBPath, 
				cfg.MediaAjanlat.Channel, 
				cfg.MediaAjanlat.Time,
				discordAdapter,
			)
			//log.Printf("✅ Media ajánlat plugin regisztrálva (Discord támogatással)")
		} else {
			// Eredeti IRC-only verzió
			mediaPlugin = media.NewMediaAjanlatPlugin(
				bot, 
				cfg.JellyfinDBPath, 
				cfg.MediaAjanlat.Channel, 
				cfg.MediaAjanlat.Time,
			)
			//log.Printf("✅ Media ajánlat plugin regisztrálva (csak IRC)")
		}
		
		pm.manager.Register(mediaPlugin)
	}

	if cfg.Plugins.EnableJellyfinInfo && cfg.JellyfinDBPath != "" {
		jellyfinInfoPlugin := media.NewJellyfinInfoPlugin(bot, cfg.JellyfinDBPath)
		pm.manager.Register(jellyfinInfoPlugin)
		//og.Printf("✅ Jellyfin info plugin regisztrálva")
	} else if cfg.Plugins.EnableJellyfinInfo {
		log.Printf("❌ Jellyfin adatbázis elérési út nincs megadva, info plugin kihagyva")
	}

	if cfg.Plugins.EnableMediaActivity && cfg.MediaActivity != nil && cfg.MediaActivity.Enabled {
		mediaActivityPlugin := media.NewMediaActivityPlugin(bot, cfg.MediaActivity)
		pm.manager.Register(mediaActivityPlugin)
		mediaActivityPlugin.Start()
		//log.Printf("✅ Media activity plugin regisztrálva (check interval: %ds)", cfg.MediaActivity.CheckInterval)
	}

	if cfg.Plugins.EnableMovie {
		moviePlugin := media.NewMoviePlugin(
			bot, ynmAdminPlugin, cfg.JellyfinDBPath, cfg.MovieDBPath, 
			cfg.MovieRequestsChannel, cfg.MoviePlugin.PostTime,
			cfg.MoviePlugin.PostChan, cfg.MoviePlugin.PostNick,
		)
		pm.manager.Register(moviePlugin)
		//log.Printf("✅ Movie plugin regisztrálva")
	}

	if cfg.Plugins.EnableMovieRequest {
		movieRequestPlugin := media.NewMovieRequestPlugin(bot, ynmAdminPlugin, cfg.MovieDBPath)
		if movieRequestPlugin != nil {
			pm.manager.Register(movieRequestPlugin)
			//log.Printf("✅ Movie request plugin regisztrálva")
		}
	}

	if cfg.Plugins.EnableMovieCompletion {
		movieCompletionPlugin := media.NewMovieCompletionPlugin(bot, ynmAdminPlugin, cfg.MovieDBPath)
		if movieCompletionPlugin != nil {
			pm.manager.Register(movieCompletionPlugin)
			//log.Printf("✅ Movie completion plugin regisztrálva")
		}
	}

	if cfg.Plugins.EnableMovieDeletion {
		movieDeletionPlugin := media.NewMovieDeletionPlugin(bot, ynmAdminPlugin, cfg.MovieDBPath)
		if movieDeletionPlugin != nil {
			pm.manager.Register(movieDeletionPlugin)
			//log.Printf("✅ Movie deletion plugin regisztrálva")
		}
	}
	


	

	

	


	

	
	

	// ============== DEBUG PLUGIN ===============
	if cfg.Plugins.EnableDebug {
		YnMDebug.Channel = cfg.DebugChannel  // a configból vesszük a csatornát
		YnMDebug.SendRawFunc = bot.SendRaw          // a bot tényleges SendRaw függvénye
		//log.Printf("✅ Debug plugin aktív, csatorna: %s", YnMDebug.Channel)
	} else {
		YnMDebug.SendRawFunc = nil
	}

		
	log.Printf("✅ All plugin register successfully")
	return nil
	}


	func (pm *PluginManager) HandleMessage(msg YnMIrC.Message) string {
		return pm.manager.HandleMessage(msg)
	}

	func (pm *PluginManager) HandleDiscordMessage(msg discord.MessageInterface) string {
		ynmMsg := YnMIrC.Message{
			Nick:    msg.GetNick(),
			Channel: msg.GetChannel(),
			Text:    msg.GetText(),
		}
		
		return pm.manager.HandleMessage(ynmMsg)
	}

	func (pm *PluginManager) Register(plugin Plugin) {
		pm.manager.Register(plugin)
	}

	func (pm *PluginManager) GetManager() *Manager {
		return pm.manager
	}

	func (pm *PluginManager) HandleTick(bot *YnMIrC.Client) {
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
		// Stop all start-stopper plugins
		for _, plugin := range pm.manager.startStoppers {
			plugin.Stop()
		}

		// Normál pluginok cleanup
		for _, plugin := range pm.manager.GetPlugins() {
			pm.shutdownPlugin(plugin)
		}
	}











	func (pm *PluginManager) shutdownPlugin(plugin interface{}) {
		switch p := plugin.(type) {
		// ============== YnM Admin Plugin ==============
		case *owner.YnmAdminPlugin:
			if p.GetDB() != nil {
				p.GetDB().Close()
			}
		log.Printf("🛑 YnM Admin plugin leállítva")

	// ============== YnM Plugins ==============
		
	case *ynm.ServiceManager:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Service plugin leállítva")

	case *ynm.PartylineConsole:
		p.Cleanup()
		log.Printf("🛑 Partyline Console plugin leállítva")

	case *ynm.NameDayPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Névnap plugin leállítva")

	case *ynm.OraPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Óra plugin leállítva")
		
	case *ynm.Fail2BanPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Fail2BanPlugin  leállítva")		

	case *ynm.XPPlugin:
		if p.Manager != nil {
			p.Manager.Shutdown()
		}
		log.Printf("🛑 XP plugin leállítva")

	case *ynm.YouTubePlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 YouTube plugin leállítva")

	case *ynm.ViccPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Vicc plugin leállítva")

	case *ynm.SeenPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Seen plugin leállítva")

	case *ynm.ServiceMonitorPlugin:
		p.Stop()
		log.Printf("🛑 Service Monitor plugin leállítva")

	case *ynm.ChatGPTPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 ChatGPT plugin leállítva")

	case *ynm.LinkPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Link plugin leállítva")

	case *ynm.GitPlugin:
		p.Stop()
		log.Printf("🛑 Git plugin leállítva")

	case *ynm.HoroszkopPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Horoszkóp plugin leállítva")

	case *ynm.ForumPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Forum plugin leállítva")

	case *ynm.LearnPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Learn plugin leállítva")

	case *ynm.SmsPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 SMS plugin leállítva")

	case *ynm.TorrentRSS:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Torrent RSS plugin leállítva")

	case *ynm.HunTorrentPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 HunTorrent Chat plugin leállítva")

	case *ynm.BruteforceAttackPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 BruteforceAttack plugin leállítva")

	case *ynm.IMDBPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 IMDB plugin leállítva")

	case *ynm.WebhookPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Webhook plugin leállítva")
		
	case *ynm.ExternalTriggerPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Webhook plugin leállítva")

	case *ynmapi.YnMApiPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 YnMApi plugin leállítva")

	case *ynm.StatusPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Status plugin leállítva")

	case *ynm.WeatherPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Weather plugin leállítva")

	case *ynm.PingPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Ping plugin leállítva")

	case *ynm.SSHPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 SSH plugin leállítva")

	case *ynm.NmapPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Nmap plugin leállítva")

	case *ynm.IPPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 IP plugin leállítva")

	case *ynm.DNSPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 DNS plugin leállítva")

	case *ynm.PingHostPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 PingHost plugin leállítva")
		

	case *ynm.SzekelyhonPlugin:
		p.Stop()
		log.Printf("🛑 Székelyhon plugin leállítva")

	// ============== YnM Command Plugins ==============
	case *YnMCmd.UnifiedUpdatePlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 UnifiedUpdate plugin leállítva")
		
	case *YnMCmd.UptimePlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Uptime plugin leállítva")	


	case *YnMCmd.TopicUpdaterPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Topic Updater plugin leállítva")
	case *YnMCmd.CyclePlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Cycle plugin leállítva")
	case *YnMCmd.ControlPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Control plugin leállítva")

	// ============== Media Plugins ==============
	case *media.MediaUploadPlugin:
		p.Stop()
		log.Printf("🛑 Media upload plugin leállítva")

	case *media.MediaAjanlatPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Media ajánlat plugin leállítva")

	case *media.JellyfinInfoPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 Jellyfin info plugin leállítva")

	case *media.MediaActivityPlugin:
		p.Stop()
		log.Printf("🛑 Media activity plugin leállítva")

	case *media.MoviePlugin:
		p.Close()
		log.Printf("🛑 Movie plugin leállítva")

	case *media.MovieRequestPlugin:
		p.Close()
		log.Printf("🛑 Movie request plugin leállítva")

	case *media.MovieCompletionPlugin:
		p.Close()
		log.Printf("🛑 Movie completion plugin leállítva")

	case *media.MovieDeletionPlugin:
		p.Close()
		log.Printf("🛑 Movie deletion plugin leállítva")
	// ==============Discord =================
	
		case *discord.DiscordPlugin:
		p.Close()
		log.Printf("🛑 Discord plugin leállítva")
		
	// ============== CTCP Plugin ==============
	case *CTCPPlugin:
		// Nincs specifikus cleanup, csak logolás
		log.Printf("🛑 CTCP plugin leállítva")

	// ============== Generic Plugin with Stopper Interface ==============
	default:
		// Ha a plugin implementálja a Stopper interface-t
		if stopper, ok := plugin.(interface{ Stop() }); ok {
			stopper.Stop()
			log.Printf("🛑 Ismeretlen plugin leállítva (Stop() hívással)")
		} else if closer, ok := plugin.(interface{ Close() }); ok {
			closer.Close()
			log.Printf("🛑 Ismeretlen plugin leállítva (Close() hívással)")
		} else {
			log.Printf("🛑 Ismeretlen plugin típus leállítva (tisztítás nélkül): %T", plugin)
		}
	}
}