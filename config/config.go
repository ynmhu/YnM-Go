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
package config

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"time"
)

type Config struct {
	Server               string        `yaml:"Server"`
	Port                 string        `yaml:"Port"`
	NickName             string        `yaml:"NickName"`
	UserName             string        `yaml:"UserName"`
	RealName             string        `yaml:"RealName"`
	ConsoleChannel       string        				`yaml:"Console"`
	Channels             							[]string      			`yaml:"Channels"`
	LogDir               							string        			`yaml:"LogDir"`
	ReconnectOnDisconnect				time.Duration		`yaml:"ReconOnDiscon"`
	PingCommandCooldown  string        `yaml:"Ping"`
	Admins               []string      `yaml:"admins"`

	// NickServ beállítások
	NickservBotnick      string `yaml:"NickservBotnick"`
	NickservNick         string `yaml:"NickservNick"`
	NickservPass         string `yaml:"NickservPass"`
	AutoLogin            bool   `yaml:"autologin"`
	AutoJoinWithoutLogin bool   `yaml:"AutoJoinWithoutLogin"`

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
	JellyfinDBPath       string                `yaml:"jellyfin_db_path"`
	MovieDBPath          string                `yaml:"movie_db_path"`
	MovieRequestsChannel string                `yaml:"movie_requests_channel"`
	MoviePlugin          MoviePluginConfig     `yaml:"movie_plugin"`
	MediaAjanlat         MediaAjanlatConfig    `yaml:"media_ajanlat"`
	
	    MediaUpload struct {
        Enabled         bool     `yaml:"enabled"`
        IntervalMinutes int      `yaml:"interval_minutes"`
        Channels        []string `yaml:"channels"`
        JellyfinDB      string   `yaml:"jellyfin_db"`
        SentDatesFile   string   `yaml:"sent_dates_file"`
    } `yaml:"media_upload"`

	//Ora Reminder
	OraChan     []string `yaml:"orachan"`
	OraDatesFile string  `yaml:"ora_dates_file"`
	OraDBFile    string  `yaml:"ora_db_file"`

}

type MoviePluginConfig struct {
	PostTime string `yaml:"post_time"`
	PostChan string `yaml:"post_chan"`
	PostNick string `yaml:"post_nick"`
}

type MediaAjanlatConfig struct {
	Channel string `yaml:"channel"`
	Time    string `yaml:"time"` // formátum: "HH:MM"
}

type MediaUploadConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Channels        []string `yaml:"channels"`
	IntervalMinutes int      `yaml:"interval_minutes"`
	JellyfinDB      string   `yaml:"jellyfin_db"`
	SentDatesFile   string   `yaml:"sent_dates_file"`
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