package config

import (
	"github.com/BurntSushi/toml"
	"os"
)

type Config struct {
	Port         int
	DatabasePath string
}

func ParseConfig(path string) (Config, error) {
	cfg := Config{}

	data, _ := os.ReadFile(path)
	err := toml.Unmarshal(data, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
