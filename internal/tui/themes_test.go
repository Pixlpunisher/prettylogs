package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/tcontardo/prettylogs/internal/config"
)

func TestThemePresets(t *testing.T) {
	t.Parallel()
	presets := themePresets()
	if len(presets) != 3 {
		t.Fatalf("presets: got %d want 3", len(presets))
	}
	wantNames := []string{"Default", "Nord", "High contrast"}
	def := config.Default().Theme
	for i, name := range wantNames {
		if presets[i].Name != name {
			t.Fatalf("preset %d name: got %q want %q", i, presets[i].Name, name)
		}
	}
	if presets[0].Theme.Error != def.Error || presets[0].Theme.Warn != def.Warn ||
		presets[0].Theme.Info != def.Info || presets[0].Theme.Debug != def.Debug {
		t.Fatalf("Default preset %+v want %+v", presets[0].Theme, def)
	}
}

func TestThemePickerFromHelp(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	updated, _ := m.handleKey(keyMsg("?"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("t"))
	m = updated.(Model)
	if !m.theming || !m.help {
		t.Fatalf("expected theming from help, theming=%v help=%v", m.theming, m.help)
	}
	content := ansi.Strip(m.View().Content)
	for _, want := range []string{"Default", "Nord", "High contrast", "boom", "slow", "ready", "cache"} {
		if !strings.Contains(content, want) {
			t.Fatalf("theme picker missing %q:\n%s", want, content)
		}
	}
}

func TestThemePickerJMovesIndex(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	updated, _ := m.handleKey(keyMsg("?"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("t"))
	m = updated.(Model)
	if m.themeIdx != 0 {
		t.Fatalf("start idx %d", m.themeIdx)
	}
	updated, _ = m.handleKey(keyMsg("j"))
	m = updated.(Model)
	if m.themeIdx != 1 {
		t.Fatalf("after j idx %d want 1", m.themeIdx)
	}
}

func TestThemePickerEnterAppliesAndPersists(t *testing.T) {
	orig := persistTheme
	t.Cleanup(func() { persistTheme = orig })
	var saved config.Config
	persistTheme = func(cfg config.Config) error {
		saved = cfg
		return nil
	}

	m := newTestModel()
	before := m.styles.Error.Render("[ERR]")
	updated, _ := m.handleKey(keyMsg("?"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("t"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("j"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("j"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("enter"))
	m = updated.(Model)

	if m.theming || m.help {
		t.Fatalf("picker should close, theming=%v help=%v", m.theming, m.help)
	}
	if m.themeName != "High contrast" {
		t.Fatalf("themeName %q", m.themeName)
	}
	after := m.styles.Error.Render("[ERR]")
	if before == after {
		t.Fatal("applying High contrast should change error style")
	}
	if saved.Theme.Name != "High contrast" || saved.Theme.Error != "brightred" {
		t.Fatalf("persisted %+v", saved.Theme)
	}
}

func TestThemePickerEscKeepsStyles(t *testing.T) {
	orig := persistTheme
	t.Cleanup(func() { persistTheme = orig })
	called := false
	persistTheme = func(config.Config) error {
		called = true
		return nil
	}

	m := newTestModel()
	before := m.styles.Error.Render("[ERR]")
	updated, _ := m.handleKey(keyMsg("?"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("t"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("j"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("esc"))
	m = updated.(Model)

	if m.theming {
		t.Fatal("esc should close picker")
	}
	if !m.help {
		t.Fatal("esc should return to help")
	}
	if called {
		t.Fatal("esc should not persist")
	}
	if m.styles.Error.Render("[ERR]") != before {
		t.Fatal("esc should not change applied styles")
	}
}

func TestHelpMentionsThemeKey(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	updated, _ := m.handleKey(keyMsg("?"))
	m = updated.(Model)
	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "Choose theme") {
		t.Fatalf("help missing theme key:\n%s", content)
	}
}
