package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Theme struct {
	Error      string `mapstructure:"error"`
	Warn       string `mapstructure:"warn"`
	Info       string `mapstructure:"info"`
	Debug      string `mapstructure:"debug"`
	Background string `mapstructure:"background"`
	Border     string `mapstructure:"border"`
}

type Config struct {
	Theme Theme `mapstructure:"theme"`
}

func Default() Config {
	return Config{
		Theme: Theme{
			Error:      "red",
			Warn:       "yellow",
			Info:       "blue",
			Debug:      "gray",
			Background: "default",
			Border:     "dim",
		},
	}
}

func Load(explicit string) Config {
	cfg := Default()
	v := viper.New()
	v.SetConfigType("yaml")
	if explicit != "" {
		v.SetConfigFile(explicit)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg
		}
		v.AddConfigPath(filepath.Join(home, ".prettyLogs"))
		v.SetConfigName("config")
	}
	if err := v.ReadInConfig(); err != nil {
		return cfg
	}
	_ = v.Unmarshal(&cfg)
	cfg = applyDefaults(cfg)
	return cfg
}

func applyDefaults(cfg Config) Config {
	def := Default().Theme
	if cfg.Theme.Error == "" {
		cfg.Theme.Error = def.Error
	}
	if cfg.Theme.Warn == "" {
		cfg.Theme.Warn = def.Warn
	}
	if cfg.Theme.Info == "" {
		cfg.Theme.Info = def.Info
	}
	if cfg.Theme.Debug == "" {
		cfg.Theme.Debug = def.Debug
	}
	if cfg.Theme.Background == "" {
		cfg.Theme.Background = def.Background
	}
	if cfg.Theme.Border == "" {
		cfg.Theme.Border = def.Border
	}
	return cfg
}
