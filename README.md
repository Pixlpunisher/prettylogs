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

## Branches & commits

Keep these consistent so history stays readable and `git log`/`git blame` stay useful.

**Branches**: `<type>/<short-kebab-description>`, e.g. `fix/wrapper-stop-deadlock`, `feat/json-array-support`, `ci/add-lint-job`. Types: `feat`, `fix`, `chore`, `docs`, `ci`, `refactor`, `test`.

**Commits**:
- Summary line: imperative present tense ("Fix", not "Fixed"/"Fixes"), no trailing period, ≤72 characters.
- One logical change per commit — don't bundle an unrelated fix into a feature commit.
- Add a body (blank line, then wrapped prose) when the *why* isn't obvious from the diff — a constraint, a bug's root cause, a tradeoff. Skip it when the summary already says everything.

**Before merging**: rebase your branch onto `main` rather than merging `main` into it — `main` requires a linear history, so a merge commit (or an unrebased branch) won't be mergeable. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full pre-PR checklist.
