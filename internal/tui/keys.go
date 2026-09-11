package tui

const helpText = `prettylogs - Keyboard Shortcuts

Navigation
  j / ↓     Move down (scrolls expanded detail if it doesn't fit)
  k / ↑     Move up (scrolls expanded detail if it doesn't fit)
  g         Go to first line (top of detail, if scrolling one)
  G         Go to last line (bottom of detail, if scrolling one)
  PageDown  Page down (pages through detail, if scrolling one)
  PageUp    Page up (pages through detail, if scrolling one)

Interaction
  Enter     Expand/collapse selected entry or group
  /         Search (regex)
  n         Next match
  N         Previous match
  l         Filter by log level
  y         Copy selected (pretty)
  Y         Copy selected (raw)
  r         Restart wrapped command
  ?         Show this help

General
  t          Choose theme (from help)
  q / ctrl+c Quit
  Esc        Close dialog / help
`

func formatHelp(colorProfile string) string {
	if colorProfile == "" {
		colorProfile = "unknown"
	}
	return helpText + "Color profile: " + colorProfile + "\n\nPress ? or Esc to close"
}

var levelOptions = []string{"All", "ERROR", "WARN", "INFO", "DEBUG"}
