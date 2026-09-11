package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Theme struct {
	Name       string `mapstructure:"name"`
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
			Name:       "Default",
			Error:      "red",
			Warn:       "yellow",
			Info:       "blue",
			Debug:      "gray",
			Background: "default",
			Border:     "dim",
		},
	}
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".prettyLogs", "config"), nil
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

// Save writes a theme-only YAML file. Empty path uses DefaultPath().
func Save(path string, cfg Config) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	t := applyDefaults(cfg).Theme
	if t.Name == "" {
		t.Name = Default().Theme.Name
	}
	body := "theme:\n" +
		"  name: " + yamlQuote(t.Name) + "\n" +
		"  error: " + yamlQuote(t.Error) + "\n" +
		"  warn: " + yamlQuote(t.Warn) + "\n" +
		"  info: " + yamlQuote(t.Info) + "\n" +
		"  debug: " + yamlQuote(t.Debug) + "\n" +
		"  background: " + yamlQuote(t.Background) + "\n" +
		"  border: " + yamlQuote(t.Border) + "\n"
	return os.WriteFile(path, []byte(body), 0o600)
}

func yamlQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
