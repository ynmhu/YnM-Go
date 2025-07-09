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
	Server									 string			`yaml:"Server"`
	Port										 string 			`yaml:"Port"`   
	NickName                  			string			`yaml:"NickName"`
	UserName                  			string			`yaml:"UserName"`
	RealName       						string			`yaml:"RealName"`
	ConsoleChannel         			string			`yaml:"Console"`
	Channels          			     []string      		`yaml:"Channels"`
	LogDir               					string			`yaml:"LogDir"`
	ReconnectOnDisconnect  time.Duration 	`yaml:"ReconOnDiscon"`
	PingCommandCooldown    string       	 `yaml:"Ping"`
    // ... your existing fields ...
    Admins							 []string 			`yaml:"admins"` 

	// NickServ beállítások
	NickservBotnick				string        `yaml:"NickservBotnick"`
	NickservNick					string        `yaml:"NickservNick"`
	NickservPass					string        `yaml:"NickservPass"`
	AutoLogin						bool          `yaml:"autologin"`
	AutoJoinWithoutLogin   	bool          `yaml:"AutoJoinWithoutLogin"`
	
	
    // 🔐 ÚJ SASL mezők:
    UseSASL						bool				`yaml:"SASL"`
    SASLUser						string			`yaml:"SASLUser"`
    SASLPass						string			`yaml:"SASLPass"`

    // 🔒 TLS kapcsolathoz (ha még nincs benne)
    UseTLS							bool				`yaml:"TLS"`
    TLSCert						string			`yaml:"TLSCert"`
    TLSKey							string			`yaml:"TLSKey"`
	TLSPort						string			`yaml:"TLSPort"`
	
    NevnapChannels			[]string		`yaml:"NevnapChannels"`
    NevnapReggel				string			`yaml:"NevnapReggel"`
    NevnapEste					string			`yaml:"NevnapEste"`
	
	// Székelyhon
	SzekelyhonChannels   []string      `yaml:"SzekelyhonChannels"`
	SzekelyhonInterval   string        `yaml:"SzekelyhonInterval"`
	SzekelyhonStartHour  int           `yaml:"SzekelyhonStartHour"`
	SzekelyhonEndHour    int           `yaml:"SzekelyhonEndHour"`


	
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
