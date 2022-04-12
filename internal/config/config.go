package config

import (
	"github.com/BurntSushi/toml"
	"os"
)

const (
	SQLiteBackendType = "sqlite"
)

type Config struct {
	Address     string              `toml:"address"`
	Port        int                 `toml:"port"`
	BackendType string              `toml:"backend_type"`
	Domain      string              `toml:"domain"`
	SQLite      SQLiteBackendConfig `toml:"sqlite"`
	UploadPath  string              `toml:"upload_path"`
}

type SQLiteBackendConfig struct {
	Path string `toml:"path"`
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
