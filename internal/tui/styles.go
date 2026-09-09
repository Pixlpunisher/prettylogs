package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/tcontardo/prettylogs/internal/config"
	"github.com/tcontardo/prettylogs/internal/record"
)

type Styles struct {
	Header     lipgloss.Style
	Footer     lipgloss.Style
	Border     lipgloss.Style
	Selected   lipgloss.Style
	Message    lipgloss.Style
	Detail     lipgloss.Style
	Help       lipgloss.Style
	Error      lipgloss.Style
	Warn       lipgloss.Style
	Info       lipgloss.Style
	Debug      lipgloss.Style
	Dim        lipgloss.Style
	SearchErr  lipgloss.Style
	levelColor map[string]lipgloss.Style
}

func NewStyles(theme config.Theme) Styles {
	border := themeColor(theme.Border)
	s := Styles{
		Header: lipgloss.NewStyle().Bold(true).Padding(0, 1),
		Footer: lipgloss.NewStyle().Faint(true).Padding(0, 1),
		Border: lipgloss.NewStyle().Foreground(border),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color(theme.Info)).
			PaddingLeft(1),
		Message:   lipgloss.NewStyle(),
		Detail:    lipgloss.NewStyle().Faint(true).PaddingLeft(4),
		Help:      lipgloss.NewStyle().Padding(1, 2),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Error)).Bold(true),
		Warn:      lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Warn)).Bold(true),
		Info:      lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Info)).Bold(true),
		Debug:     lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Debug)),
		Dim:       lipgloss.NewStyle().Faint(true),
		SearchErr: lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Error)),
	}
	if theme.Background != "" && theme.Background != "default" {
		bg := lipgloss.Color(theme.Background)
		s.Header = s.Header.Background(bg)
		s.Footer = s.Footer.Background(bg)
	}
	s.levelColor = map[string]lipgloss.Style{
		record.LevelError: s.Error,
		record.LevelWarn:  s.Warn,
		record.LevelInfo:  s.Info,
		record.LevelDebug: s.Debug,
	}
	return s
}

func (s Styles) Level(level string) lipgloss.Style {
	if st, ok := s.levelColor[level]; ok {
		return st
	}
	return s.Info
}

func themeColor(name string) color.Color {
	if name == "" || name == "default" || name == "dim" {
		return lipgloss.Color("8")
	}
	return lipgloss.Color(name)
}

func levelEmoji(level string) string {
	switch level {
	case record.LevelError:
		return "🚨"
	case record.LevelWarn:
		return "🟡"
	case record.LevelDebug:
		return "🔽"
	default:
		return "🔵"
	}
}
