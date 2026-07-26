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
package YnMConfig

import (
	"io/ioutil"
	"gopkg.in/yaml.v3"
	"time"
	"os"
	"strings"
	"fmt"
)

type Config struct {
	Server               string        `yaml:"Server"`	
	Port                 string        `yaml:"Port"`
	NickName             string        `yaml:"NickName"`
	UserName             string        `yaml:"UserName"`
	RealName             string        `yaml:"RealName"`
	Channels             []string      `yaml:"Channels"`
	ConsoleChannel       string        `yaml:"Console"`
	ConsoleKey    			 string `yaml:"ConsoleKey"`
	LogDir               string        `yaml:"LogDir"`
	DataDir              string        `yaml:"data_dir"`
	ReconnectOnDisconnect time.Duration `yaml:"ReconOnDiscon"`
	PingCommandCooldown  string        `yaml:"Ping"`
	Admins               []string      `yaml:"admins"`
	Version string
	Console  ConsoleConfig  `yaml:"console"`

	// NickServ beállítások
	NickservBotnick      string `yaml:"NickservBotnick"`
	NickservNick         string `yaml:"NickservNick"`
	NickservPass         string `yaml:"NickservPass"`
	NickServLogin    bool   `yaml:"NickServLogin"`
	AutoJoinWithoutLogin bool   `yaml:"AutoJoinWithoutLogin"`
	
	
	// Undernet 
	Undernet UndernetConfig `yaml:"Undernet"`
	TopicUpdateInterval string `yaml:"shell_topic_update"`
	TopicOtherChannels   []string `yaml:"shell_topic_channels"` 


	// 🔐 SASL mezők:
	UseSASL  bool   `yaml:"SASL"`
	SASLUser string `yaml:"SASLUser"`
	SASLPass string `yaml:"SASLPass"`

	// 🔒 TLS kapcsolathoz
	UseTLS  bool   `yaml:"TLS"`
	TLSCert string `yaml:"TLSCert"`
	TLSKey  string `yaml:"TLSKey"`
	TLSPort string `yaml:"TLSPort"`

	// Névnap plugin
	NevnapChannels []string `yaml:"NevnapChannels"`
	NevnapReggel   string   `yaml:"NevnapReggel"`
	NevnapEste     string   `yaml:"NevnapEste"`

	// Székelyhon
	SzekelyhonChannels  []string `yaml:"SzekelyhonChannels"`
	SzekelyhonInterval  string   `yaml:"SzekelyhonInterval"`
	SzekelyhonStartHour int      `yaml:"SzekelyhonStartHour"`
	SzekelyhonEndHour   int      `yaml:"SzekelyhonEndHour"`

	// Viccek
	JokeChannels []string `yaml:"JokeChannels"`
	JokeSendTime string   `yaml:"JokeSendTime"`

	// Movie plugin configuration
	JellyfinDBPath       string             `yaml:"jellyfin_db_path"`
	MovieDBPath          string             `yaml:"movie_db_path"`
	MovieRequestsChannel string             `yaml:"movie_requests_channel"`
	MoviePlugin          MoviePluginConfig  `yaml:"movie_plugin"`
	MediaAjanlat         MediaAjanlatConfig `yaml:"media_ajanlat"`

	MediaUpload struct {
		Enabled         bool     `yaml:"enabled"`
		IntervalMinutes int      `yaml:"interval_minutes"`
		Channels        []string `yaml:"channels"`
		JellyfinDB      string   `yaml:"jellyfin_db"`
		SentDatesFile   string   `yaml:"sent_dates_file"`
	} `yaml:"media_upload"`

	// Ora Reminder
	OraChan      []string `yaml:"orachan"`
	OraDatesFile string   `yaml:"ora_dates_file"`
	OraDBFile    string   `yaml:"ora_db_file"`


	MediaActivity          *MediaActivityConfig `yaml:"media_activity"`
	Weather                WeatherConfig        `yaml:"weather"`
	SeenDBPath             string               `yaml:"seen_db"`
	SearchNotificationDelay time.Duration       `yaml:"search_notification_delay"`
	SmsDBPath              string               `yaml:"SmsDBPath"`

	// ChatGPT
	OpenAI  OpenAIConfig `yaml:"openai"`
	Plugins PluginConfig `yaml:"Plugins"`
	
	    // YouTube plugin
    YtApi      string   `yaml:"YtApi"`
    YtChannels []string `yaml:"YtChannels"`
	
	//GIT plugin
	GitPlugin GitPluginConfig `yaml:"GitPlugin"`
	//OnThisDay plugin
	OnThisDayPlugin OnThisDayPluginConfig `yaml:"OnThisDayPlugin"`
	 // Update notification settings
    UpdateCheck struct {
        Enabled         bool          `yaml:"enabled"`
        CheckInterval   time.Duration `yaml:"check_interval"`
        NotifyInterval  time.Duration `yaml:"notify_interval"`
		AutoUpgrade    *bool          `yaml:"auto_upgrade"` 
    } `yaml:"update_check"`
	//Hun Chat
	HunTorrentChat HunTorrentConfig `yaml:"huntorrent_chat"`
	DebugChannel string `yaml:"debug_channel"`
	//SMTP 
	 SMTP SMTPConfig `yaml:"smtp"`
	//Fail2BanConfig
	Fail2Ban Fail2BanConfig `yaml:"Fail2Ban"`
	
		// 🎮 ÚJ: Discord beállítások
	Discord struct {
		Enabled  bool     `yaml:"enabled"`
		Token    string   `yaml:"token"`
		Channels []string `yaml:"channels"`
	} `yaml:"discord"`
	
	    BruteforceAttack struct {
        LogPath  string   `yaml:"log_path"`
        Channels []string `yaml:"channels"`
    } `yaml:"bruteforce_attack_plugin"`
	
	ResourceMonitor *ResourceMonitorConfig `yaml:"resource_monitor"`

	// Join Plugin
    JoinMessageChannels      []string      `yaml:"joinmessage_channels"`
    JoinMessageDiscordChannels []string    `yaml:"joinmessage_discord_channels"`
    JoinMessageText          string        `yaml:"joinmessage_text"`
    JoinMessageDelay         time.Duration `yaml:"joinmessage_delay"`
	
	
	// LegszebbNotak
	LatestMusicChannels []string `yaml:"latest_music_channels"`
	LatestMusicInterval string   `yaml:"latest_music_interval"`
	// Telegram automatikus posztolás
	Telegram TelegramConfig `yaml:"telegram"`  
	// Facebook konfiguráció
	FacebookEnabled    bool   `yaml:"facebook_enabled"`
	FacebookScriptPath string `yaml:"facebook_script_path"`
	Mp3Scanner Mp3ScannerConfig `yaml:"Mp3Scanner"` 
	BotManager struct {
        ConfigPath string `yaml:"config_path"`
    } `yaml:"botmanager"`

	//Alexa 
	AlexaNodeRed  string       `yaml:"alexa_nodered_url"`

	

}

type PluginConfig struct {
	// Core plugins
	EnablePing         bool `yaml:"enable_ping"`
	EnableNameDay      bool `yaml:"enable_nameday"`
	EnableOra          bool `yaml:"enable_ora"`
	EnableHuntorrent   bool `yaml:"enable_huntorrent"`
	EnableHoroscope    bool `yaml:"enable_horoscope"`
	EnableAutoVoice    bool `yaml:"enable_autovoice"`
	EnableWeather      bool `yaml:"enable_weather"`
	EnableSeen         bool `yaml:"enable_seen"`
	EnableSms          bool `yaml:"enable_sms"`
	EnableAlexa 	   bool  `yaml:"enable_alexa"` 
	EnableStatus       bool `yaml:"enable_status"`
	EnableVicc         bool `yaml:"enable_vicc"`
	EnableXP           bool `yaml:"enable_xp"`
	EnableMonitor      bool `yaml:"enable_monitor"`
	EnableLink         bool `yaml:"enable_link"`
	EnableForum        bool `yaml:"enable_forum"`
	EnableHirek       bool `yaml:"enable_hirek"`
	EnableBruteforceAttack         bool `yaml:"enable_bruteforce"`
	EnableResourceMonitor	        bool `yaml:"enable_resource_monitor"`
	EnableWebhook      bool `yaml:"enable_webhook"`
	EnableExternalTrigger      bool `yaml:"enable_ExTrigger"`
	EnableYnMApi		bool `yaml:"enable_ynmapi"`
	EnableServiceManager		bool `yaml:"enable_service_m"`
	EnableFail2Ban bool `yaml:"enable_fail2ban"`
	EnableDiscord bool `yaml:"enable_discord"`

	// Media plugins
	EnableMovie           bool `yaml:"enable_movie"`
	EnableMovieRequest    bool `yaml:"enable_movie_request"`
	EnableMovieCompletion bool `yaml:"enable_movie_completion"`
	EnableMovieDeletion   bool `yaml:"enable_movie_deletion"`
	EnableMediaUpload     bool `yaml:"enable_media_upload"`
	EnableMediaAjanlat    bool `yaml:"enable_media_ajanlat"`
	EnableJoke            bool `yaml:"enable_joke"`
	EnableJellyfinInfo    bool `yaml:"enable_jellyfin_info"`
	EnableMediaActivity   bool `yaml:"enable_media_activity"`

	// Scheduled plugins
	EnableSzekelyhon bool `yaml:"enable_szekelyhon"`

	// YnM plugins (existing)
	EnableGit      bool `yaml:"enable_git"`
	EnableOnthisDay  bool `yaml:"enableOnthisDay"`
	EnableImdb     bool `yaml:"enable_imdb"`
	EnableMail     bool `yaml:"enable_mail"`
	EnableXes0     bool `yaml:"enable_xes0"`
	EnableSsh      bool `yaml:"enable_ssh"`
	EnableNmap     bool `yaml:"enable_nmap"`
	EnableDns      bool `yaml:"enable_dns"`
	EnableChatGPT  bool `yaml:"enable_chatgpt"`
	EnableIp       bool 		`yaml:"enable_ip"`
	EnablePingHost bool `yaml:"enable_pinghost"`
	EnableUptime bool	 `yaml:"enable_uptime"`
	EnableLearn    bool `yaml:"enable_learn"`
	EnableYouTube    bool `yaml:"enable_yt"`
	EnableDebug    bool `yaml:"enable_debug"`
	EnableControl	bool `yaml:"enable_control"`
	EnableUnifiedUpdate	bool `yaml:"enable_update"`
	EnableCTCP	bool `yaml:"enable_ctcp"`
	EnableBotManager	bool `yaml:"enable_botmanager"`
	EnableHelp	bool `yaml:"enable_help"`
	EnableTopicUpdater	bool `yaml:"enable_autotopic"`
	EnableJoinMessage        bool `yaml:"enable_joinmessage"`
	EnableJoinMessageDiscord bool `yaml:"enable_joinmessage_discord"`
	EnableLatestMusic bool `yaml:"enable_latest_music"`
	EnableTelegram bool `yaml:"enable_telegram"`  
	EnableMp3Scanner	bool `yaml:"enable_mp3scanner"`  
	
	
}

type YnMApiConfig struct {
    YnM struct {
        Port        int      `yaml:"port"`
        WebsiteURL  string   `yaml:"website_url"`  
		MasterToken  string `yaml:"master_token"`
        Routing struct {
            BotPrefix    string `yaml:"bot_prefix"`
        } `yaml:"routing"`
        
        AllowedOrigins []string `yaml:"allowed_origins"`
        
        Session struct {
            Lifetime       int `yaml:"lifetime"`
            CleanupInterval int `yaml:"cleanup_interval"`
        } `yaml:"session"`
        
        Password struct {
            ExpiryMinutes int            `yaml:"expiry_minutes"`
            ExpiryOptions map[int]string `yaml:"expiry_options"`
        } `yaml:"password"`
        
        RateLimit struct {
            Enabled           bool `yaml:"enabled"`
            RequestsPerMinute int  `yaml:"requests_per_minute"`
        } `yaml:"rate_limit"`
    } `yaml:"ynm"`
}
func LoadYnMApiConfig(path string) (*YnMApiConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var cfg YnMApiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	
	// Set defaults
	if cfg.YnM.Port == 0 {
		cfg.YnM.Port = 5252
	}
	
	// ✅ Website URL default
	if cfg.YnM.WebsiteURL == "" {
		cfg.YnM.WebsiteURL = "https://ynm-go.ynm.hu"
	}
	
	// ✅ Routing defaults
	if cfg.YnM.Routing.BotPrefix == "" {
		cfg.YnM.Routing.BotPrefix = "/api"
	}
	
	// Normalize prefixes
	cfg.YnM.Routing.BotPrefix = normalizePrefix(cfg.YnM.Routing.BotPrefix)
	
	if cfg.YnM.Session.Lifetime == 0 {
		cfg.YnM.Session.Lifetime = 3600
	}
	if cfg.YnM.Session.CleanupInterval == 0 {
		cfg.YnM.Session.CleanupInterval = 300
	}
	if cfg.YnM.Password.ExpiryMinutes == 0 {
		cfg.YnM.Password.ExpiryMinutes = 60
	}
	
	// ✅ ExpiryOptions defaults
	if cfg.YnM.Password.ExpiryOptions == nil {
		cfg.YnM.Password.ExpiryOptions = map[int]string{
			30:     "30 perc",
			60:     "1 óra",
			180:    "3 óra",
			1440:   "24 óra",
			10080:  "1 hét",
			43200:  "1 hónap",
			525600: "1 év",
			0:      "Soha ne járjon le",
		}
	}
	
	if cfg.YnM.RateLimit.RequestsPerMinute == 0 {
		cfg.YnM.RateLimit.RequestsPerMinute = 60
	}
	
	// ✅ Debug info
	fmt.Printf("[CONFIG] Website URL: %s\n", cfg.YnM.WebsiteURL)
	fmt.Printf("[CONFIG] Login URL: %s/login\n", cfg.YnM.WebsiteURL)
	fmt.Printf("[CONFIG] Max Password Uses: 10")
	
	return &cfg, nil
}

func normalizePrefix(prefix string) string {
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimSuffix(prefix, "/")
}

type Mp3ScannerConfig struct {
	ScanDir        string   `yaml:"scan_dir"`
	LogFile        string   `yaml:"log_file"`
	TestLogFile    string   `yaml:"test_log_file"`
	ReportChan     []string `yaml:"report_chan"`
	FpcalcPath     string   `yaml:"fpcalc_path"`
	ScoreThreshold float64  `yaml:"score_threshold"`
	SupportedExt   []string `yaml:"supported_ext"`
	AcoustIDAPIKey string   `yaml:"acoustid_api_key"`
	// ÚJ: Automatikus szkennelés beállításai
	AutoScanEnabled bool   `yaml:"auto_scan_enabled"`
	ScanInterval    string `yaml:"scan_interval"`  // pl: "24h", "168h" (1 hét), "12h"
	ScanTime        string `yaml:"scan_time"`      // pl: "02:00", "14:30"
	QuietMode       bool   `yaml:"quiet_mode"`     // ÚJ: Csak védett tartalom esetén jelzés
}

type TelegramConfig struct {
	Enabled     bool   `yaml:"enabled"`
	MinInterval string `yaml:"min_interval"`
	BotToken    string `yaml:"bot_token"`
	ChatID      string `yaml:"chat_id"`
	ChannelID   string `yaml:"channel_id"`
}

type ConsoleConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	BindAddr string `yaml:"bind_addr"`
}


type Fail2BanConfig struct {
    Log     string   `yaml:"log"`
    Channel []string `yaml:"channel"`
}

type ResourceMonitorConfig struct {
    Channels        []string `yaml:"channels"`
    Threshold       int      `yaml:"threshold"`
    IntervalSeconds int      `yaml:"interval_seconds"`
    AutoStart       bool     `yaml:"auto_start"`
}

type SMTPConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    User     string `yaml:"user"`
    Password string `yaml:"password"`
}
// RSS feed config struktúra
type TorrentConfig struct {
    Name          string   `yaml:"name"`           
    Channels      []string `yaml:"channels"`       
    RSSUrl        string   `yaml:"rss_url"`        
    MaxItems      int      `yaml:"max_items"`      
    StartHour     int      `yaml:"start_hour"`     
    EndHour       int      `yaml:"end_hour"`       
    CheckInterval int      `yaml:"check_interval"` 
    Enabled       bool     `yaml:"enabled"`        
    
    // Régi mezők kompatibilitásért
    URL       string `yaml:"url"`
    Cookie    string `yaml:"cookie"`
    UserAgent string `yaml:"user_agent"`
}
//hun CHAT
type HunTorrentConfig struct {
	Channels  []string `yaml:"channels"`
	URL       string   `yaml:"url"`
	Cookie    string   `yaml:"cookie"`
	UserAgent string   `yaml:"user_agent"`
}

type MoviePluginConfig struct {
	PostTime string `yaml:"post_time"`
	PostChan []string `yaml:"post_chan"`
	PostNick string `yaml:"post_nick"`
}

type MediaAjanlatConfig struct {
	Channel []string `yaml:"channel"`
	Time    string `yaml:"time"` // formátum: "HH:MM"
}
type MediaConfig struct {
    YnMMedia map[string]string `yaml:"YnM_WebHook"`
}

type MediaUploadConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Channels        []string `yaml:"channels"`
	IntervalMinutes int      `yaml:"interval_minutes"`
	JellyfinDB      string   `yaml:"jellyfin_db"`
	SentDatesFile   string   `yaml:"sent_dates_file"`
}

type MediaItem struct {
	Title          string      `json:"title"`
	Genres         string      `json:"genres"`
	Overview       string      `json:"overview"`
	RuntimeTicks   interface{} `json:"runtime_ticks"`
	ProductionYear int         `json:"production_year"`
	DateCreated    string      `json:"date_created"`
	Path           string      `json:"path"`
	MediaType      string      `json:"media_type"`
}

type RobotConfig struct {
	EhsegCsokkenesPerOra     float64 `yaml:"Ehseg"`
	BoldogsagCsokkenesPerOra float64 `yaml:"Boldogsag"`
	TisztasagCsokkenesPerOra float64 `yaml:"Tisztasag"`
	PercenkentSzamolasiAlap  float64 `yaml:"Szamolas"` // pl. 60 perc
	AlertChannel             string  `yaml:"TChan"`
}


type MediaActivityConfig struct {
	Enabled          bool   `yaml:"enabled"`
	JellyfinDBPath   string `yaml:"jellyfin_db_path"`
	JellyfinURL      string `yaml:"jellyfin_url"`
	JellyfinToken    string `yaml:"jellyfin_token"`
	CheckInterval    int    `yaml:"check_interval"`
	IRCChannel       string `yaml:"irc_channel"`
	SecondaryChannel string `yaml:"secondary_channel"`
	ReportChannel    string `yaml:"report_channel"`
	OnlineCooldown   int    `yaml:"online_cooldown"`
	BaseDataDir      string `yaml:"base_data_dir"`
	NotificationURL  string `yaml:"notification_url"`
}

type WeatherConfig struct {
	APIKey          string `yaml:"weatherAPIKey"`
	DefaultLocation string `yaml:"defaultLocation"`
	Units           string `yaml:"units"`
	Language        string `yaml:"language"`
}

type OpenAIConfig struct {
	APIKey string `yaml:"api_key"`
}

type GitPluginConfig struct {
    Channel []string `yaml:"channel"`
    ApiURL  string   `yaml:"apiURL"`
}

type OnThisDayPluginConfig struct {
    Channel  []string `yaml:"channel"`
    PostTime string   `yaml:"postTime"`
}

type UndernetConfig struct {
    Enabled  bool   `yaml:"Enabled"`
    Username string `yaml:"XUser"`    
    Password string `yaml:"XPass"`    
    Modes    string `yaml:"Modes"`
}

func Load(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *ConsoleConfig) SetDefaults() {
	if c.Port == 0 {
		c.Port = 3333
	}
	if c.BindAddr == "" {
		c.BindAddr = "127.0.0.1"
	}
	if c.Password == "" {
		c.Password = "changeme"
	}
}