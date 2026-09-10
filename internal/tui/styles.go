package tui

import (
	"image/color"
	"strings"

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
	Highlight  lipgloss.Style
	levelColor map[string]lipgloss.Style
	spineColor map[string]color.Color
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
			BorderForeground(themeColor(theme.Info)).
			PaddingLeft(1),
		Message:   lipgloss.NewStyle(),
		Detail:    lipgloss.NewStyle().Faint(true).PaddingLeft(4),
		Help:      lipgloss.NewStyle().Padding(1, 2),
		Error:     lipgloss.NewStyle().Foreground(themeColor(theme.Error)).Bold(true),
		Warn:      lipgloss.NewStyle().Foreground(themeColor(theme.Warn)).Bold(true),
		Info:      lipgloss.NewStyle().Foreground(themeColor(theme.Info)).Bold(true),
		Debug:     lipgloss.NewStyle().Foreground(themeColor(theme.Debug)),
		Dim:       lipgloss.NewStyle().Faint(true),
		SearchErr: lipgloss.NewStyle().Foreground(themeColor(theme.Error)),
		Highlight: lipgloss.NewStyle().Reverse(true).Bold(true),
	}
	if theme.Background != "" && theme.Background != "default" {
		bg := themeColor(theme.Background)
		s.Header = s.Header.Background(bg)
		s.Footer = s.Footer.Background(bg)
	}
	s.levelColor = map[string]lipgloss.Style{
		record.LevelError: s.Error,
		record.LevelWarn:  s.Warn,
		record.LevelInfo:  s.Info,
		record.LevelDebug: s.Debug,
	}
	s.spineColor = map[string]color.Color{
		record.LevelError: themeColor(theme.Error),
		record.LevelWarn:  themeColor(theme.Warn),
		record.LevelInfo:  themeColor(theme.Info),
		record.LevelDebug: themeColor(theme.Debug),
	}
	return s
}

func (s Styles) Level(level string) lipgloss.Style {
	if st, ok := s.levelColor[level]; ok {
		return st
	}
	return s.Info
}

// Spine returns the left-border wrapping style for a row, colored by its
// level so the level is scannable down the left margin of every row, not
// just the badge text. Selection is shown via a heavier border weight
// (ThickBorder vs NormalBorder) rather than a different color, so a row's
// spine color never changes as the cursor moves onto or off of it.
func (s Styles) Spine(level string, selected bool) lipgloss.Style {
	color, ok := s.spineColor[level]
	if !ok {
		color = s.spineColor[record.LevelInfo]
	}
	border := lipgloss.NormalBorder()
	if selected {
		border = lipgloss.ThickBorder()
	}
	return lipgloss.NewStyle().
		Border(border, false, false, false, true).
		BorderForeground(color).
		PaddingLeft(1)
}

func themeColor(name string) color.Color {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" || key == "default" || key == "dim" {
		return lipgloss.Color("8")
	}
	if c, ok := namedANSI[key]; ok {
		return c
	}
	return lipgloss.Color(name)
}

// lipgloss.Color only accepts hex (#rrggbb) or ANSI indexes ("1", "12").
// Named values like "red" otherwise become NoColor, so badges render bold
// but uncolored.
//
// These map to explicit hex RGB values rather than lipgloss's indexed
// ANSI constants (lipgloss.Red etc.) deliberately: those are ansi.BasicColor
// values, i.e. references to whatever the terminal's own palette defines
// for that slot — a customized terminal color scheme can remap "ANSI red"
// to something that isn't red at all (observed: rendering as orange/yellow).
// Explicit hex is deterministic regardless of the terminal's palette.
var namedANSI = map[string]color.Color{
	"black":         lipgloss.Color("#000000"),
	"red":           lipgloss.Color("#ff0000"),
	"green":         lipgloss.Color("#00ff00"),
	"yellow":        lipgloss.Color("#ffff00"),
	"blue":          lipgloss.Color("#0000ff"),
	"magenta":       lipgloss.Color("#ff00ff"),
	"cyan":          lipgloss.Color("#00ffff"),
	"white":         lipgloss.Color("#ffffff"),
	"brightblack":   lipgloss.Color("#808080"),
	"brightred":     lipgloss.Color("#ff5555"),
	"brightgreen":   lipgloss.Color("#55ff55"),
	"brightyellow":  lipgloss.Color("#ffff55"),
	"brightblue":    lipgloss.Color("#5555ff"),
	"brightmagenta": lipgloss.Color("#ff55ff"),
	"brightcyan":    lipgloss.Color("#55ffff"),
	"brightwhite":   lipgloss.Color("#ffffff"),
	"gray":          lipgloss.Color("#808080"),
	"grey":          lipgloss.Color("#808080"),
}

// levelBadge returns a fixed-width (5 char) text tag for level, colored via
// Styles.Level. Fixed width keeps columns aligned regardless of terminal
// font/emoji-width quirks.
func levelBadge(level string) string {
	switch level {
	case record.LevelError:
		return "[ERR]"
	case record.LevelWarn:
		return "[WRN]"
	case record.LevelDebug:
		return "[DBG]"
	default:
		return "[INF]"
	}
}
