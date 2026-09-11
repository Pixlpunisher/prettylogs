# prettylogs

A CLI log TUI. Wrap a command or pipe logs in, then browse them as collapsible blocks with level filter and regex search.

## Build

```bash
make build
```

```bash
go install ./cmd/prettylogs
```

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
