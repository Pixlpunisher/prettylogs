# prettylogs

A CLI log TUI. Wrap a command or pipe logs in, then browse them as collapsible blocks with level filter and regex search.

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
cat testdata/samples/json.log | prettylogs --input
prettylogs --format json npm run start
prettylogs --config ~/.prettyLogs/config npm run dev
```

## Keys

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Expand/collapse |
| `/` | Search (regex) |
| `n` / `N` | Next/previous match |
| `l` | Filter by log level |
| `y` / `Y` | Copy selected (pretty / raw) |
| `r` | Restart wrapped command |
| `g` / `G` | First/last |
| `PageDown` / `PageUp` | Page |
| `?` | Help |
| `q` / `ctrl+c` | Quit |

## Config

Optional YAML at `~/.prettyLogs/config`:

```yaml
theme:
  error: "red"
  warn: "yellow"
  info: "blue"
  debug: "gray"
  background: "default"
  border: "dim"
```
