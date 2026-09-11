package tui

import (
	"strings"

	"github.com/tcontardo/prettylogs/internal/config"
	"github.com/tcontardo/prettylogs/internal/record"
)

type themePreset struct {
	Name  string
	Theme config.Theme
}

func themePresets() []themePreset {
	def := config.Default().Theme
	return []themePreset{
		{Name: def.Name, Theme: def},
		{
			Name: "Nord",
			Theme: config.Theme{
				Name:       "Nord",
				Error:      "#bf616a",
				Warn:       "#ebcb8b",
				Info:       "#88c0d0",
				Debug:      "#4c566a",
				Background: "default",
				Border:     "dim",
			},
		},
		{
			Name: "High contrast",
			Theme: config.Theme{
				Name:       "High contrast",
				Error:      "brightred",
				Warn:       "brightyellow",
				Info:       "brightcyan",
				Debug:      "brightwhite",
				Background: "default",
				Border:     "dim",
			},
		},
	}
}

func themeIndexByName(name string) int {
	presets := themePresets()
	for i, p := range presets {
		if strings.EqualFold(p.Name, name) {
			return i
		}
	}
	return 0
}

func themePreviewLine(theme config.Theme) string {
	s := NewStyles(theme)
	parts := []string{
		s.Error.Render(levelBadge(record.LevelError)) + " boom",
		s.Warn.Render(levelBadge(record.LevelWarn)) + " slow",
		s.Info.Render(levelBadge(record.LevelInfo)) + " ready",
		s.Debug.Render(levelBadge(record.LevelDebug)) + " cache",
	}
	return strings.Join(parts, "   ")
}

// persistTheme writes the applied theme. Tests replace this so the home
// config file is not touched.
var persistTheme = func(cfg config.Config) error {
	return config.Save("", cfg)
}
