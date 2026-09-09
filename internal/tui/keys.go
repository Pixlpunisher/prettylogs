package tui

const helpText = `prettylogs - Keyboard Shortcuts

Navigation
  j / ↓     Move down
  k / ↑     Move up
  g         Go to first line
  G         Go to last line
  PageDown  Page down
  PageUp    Page up

Interaction
  Enter     Expand/collapse selected entry
  /         Search (regex)
  n         Next match
  N         Previous match
  l         Filter by log level
  ?         Show this help

General
  q / ctrl+c Quit
  Esc        Close dialog / help
`

var levelOptions = []string{"All", "ERROR", "WARN", "INFO", "DEBUG"}
