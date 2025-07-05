package config

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server  string   `yaml:"server"`
	Nick    string   `yaml:"nick"`
	User    string   `yaml:"user"`
	Channels []string `yaml:"channels"`
	LogDir  string   `yaml:"log_dir"`
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
