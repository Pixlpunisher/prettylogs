package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/tcontardo/prettylogs/internal/config"
	"github.com/tcontardo/prettylogs/internal/record"
)

func TestThemeColorResolvesNamedColors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want color.Color
	}{
		{name: "red", want: lipgloss.Color("#ff0000")},
		{name: "Yellow", want: lipgloss.Color("#ffff00")},
		{name: "BLUE", want: lipgloss.Color("#0000ff")},
		{name: "gray", want: lipgloss.Color("#808080")},
		{name: "magenta", want: lipgloss.Color("#ff00ff")},
		{name: "cyan", want: lipgloss.Color("#00ffff")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := themeColor(tc.name)
			if _, ok := got.(lipgloss.NoColor); ok {
				t.Fatalf("themeColor(%q) collapsed to NoColor; lipgloss.Color does not accept names", tc.name)
			}
			if !sameRGBA(got, tc.want) {
				t.Fatalf("themeColor(%q) RGBA = %v, want %v", tc.name, rgba(got), rgba(tc.want))
			}
		})
	}
}

func TestThemeColorKeepsHexAndANSI(t *testing.T) {
	t.Parallel()
	hex := themeColor("#ff0000")
	if _, ok := hex.(lipgloss.NoColor); ok {
		t.Fatal("hex color must parse")
	}
	if !sameRGBA(themeColor("1"), lipgloss.Red) {
		t.Fatalf("ANSI index 1 should be red, got %v", rgba(themeColor("1")))
	}
}

func TestDefaultThemeLevelStylesEmitColor(t *testing.T) {
	t.Parallel()
	s := NewStyles(config.Default().Theme)
	out := s.Level(record.LevelError).Render(levelBadge(record.LevelError))
	boldOnly := lipgloss.NewStyle().Bold(true).Render(levelBadge(record.LevelError))
	if out == boldOnly {
		t.Fatal("default error badge should include a color SGR, not just bold")
	}
}

func sameRGBA(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func rgba(c color.Color) [4]uint32 {
	r, g, b, a := c.RGBA()
	return [4]uint32{r, g, b, a}
}
