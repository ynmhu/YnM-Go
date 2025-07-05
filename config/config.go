package config

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

type Config struct {
    Server              string
    Nick                string
    User                string
    Channels            []string
    LogDir              string `yaml:"log_dir"`
    PingCommandCooldown  string `yaml:"ping_command_cooldown"`
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
