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
	Server                 string        `yaml:"server"`
	Nick                   string        `yaml:"nick"`
	User                   string        `yaml:"user"`
	ConsoleChannel         string        `yaml:"console_channel"`
	Channels               []string      `yaml:"channels"`
	LogDir                 string        `yaml:"log_dir"`
	ReconnectOnDisconnect  time.Duration `yaml:"reconnect_on_disconnect"`
	PingCommandCooldown    string        `yaml:"ping_command_cooldown"`
    // ... your existing fields ...
    Admins []string `yaml:"admins"` 

	// NickServ beállítások
	NickservBotnick        string        `yaml:"nickserv_botnick"`
	NickservNick           string        `yaml:"nickserv_nick"`
	NickservPass           string        `yaml:"nickserv_pass"`
	AutoLogin              bool          `yaml:"autologin"`
	AutoJoinWithoutLogin   bool          `yaml:"autojoin_without_login"`
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
