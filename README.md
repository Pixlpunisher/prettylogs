# prettylogs

A CLI log TUI. Wrap a command or pipe logs in, then browse them as collapsible blocks with level filter and regex search.

Similar INFO, DEBUG, and WARN lines fold into groups. ERROR stays as its own row. Expand a row to read the full message; long stacks scroll in place.

## Install

Via Homebrew:

```bash
brew tap Pixlpunisher/prettylogs
brew install prettylogs
```

Upgrading later:

```bash
brew update
brew upgrade prettylogs
```

## Build

```bash
make build
./prettylogs --help
```

`make build` only writes `./prettylogs` in this repo. zsh will say `command not found: prettylogs` until the binary is on your `PATH`. Either use `./prettylogs`, or:

```bash
make install
hash -r
```

`make install` runs `go install` into `GOBIN` (often `~/go/bin`). `hash -r` clears zsh’s stale command cache.

## Usage

```bash
prettylogs nx serve backend-service-api
prettylogs npm run dev
cat testdata/samples/json.log | prettylogs
prettylogs --input
prettylogs --format json npm run start
prettylogs --config ~/.prettyLogs/config npm run dev
```

Format is auto-detected unless you pass `--format`: `json`, `logfmt`, `syslog`, or `plain`. A pipe with no command args is enough; `--input` forces stdin (and skips wrapping a command).

## Keys

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down (scrolls expanded detail if it doesn't fit) |
| `k` / `↑` | Move up (scrolls expanded detail if it doesn't fit) |
| `Enter` | Expand/collapse selected entry or group |
| `/` | Search (regex) |
| `n` / `N` | Next/previous match |
| `l` | Filter by log level |
| `y` | Copy selected (pretty: long stacks are collapsed) |
| `Y` | Copy selected (raw) |
| `r` | Restart the wrapped command (no-op when reading stdin) |
| `g` / `G` | First/last (or top/bottom of scrolling detail) |
| `PageDown` / `PageUp` | Page (or page through detail) |
| `t` | Choose theme (from help) |
| `?` | Help |
| `Esc` | Close dialog / help |
| `q` / `ctrl+c` | Quit |

## Config

Optional YAML at `~/.prettyLogs/config`. Built-in presets: Default, Nord, High contrast (`t` in help applies one and writes this file).

```yaml
theme:
  name: "Default"
  error: "red"
  warn: "yellow"
  info: "blue"
  debug: "gray"
  background: "default"
  border: "dim"
```
