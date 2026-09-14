# Contributing

## Setup

```bash
make build
./prettylogs --help
```

## Before opening a PR

```bash
gofmt -l .        # should print nothing
go vet ./...
go test ./...
```

CI runs the same checks (`gofmt`, `go vet`, `golangci-lint`, `go test`) and must pass before merge.

## PRs

- Keep PRs focused on one change.
- `main` requires a linear history — rebase your branch on `main` rather than merging it in.
- Add or update tests for behavior changes.

## Branches & commits

Keep these consistent so history stays readable and `git log`/`git blame` stay useful.

**Branches**: `<type>/<short-kebab-description>`, e.g. `fix/wrapper-stop-deadlock`, `feat/json-array-support`, `ci/add-lint-job`. Types: `feat`, `fix`, `chore`, `docs`, `ci`, `refactor`, `test`.

**Commits**:
- Summary line: imperative present tense ("Fix", not "Fixed"/"Fixes"), no trailing period, ≤72 characters.
- One logical change per commit — don't bundle an unrelated fix into a feature commit.
- Add a body (blank line, then wrapped prose) when the *why* isn't obvious from the diff — a constraint, a bug's root cause, a tradeoff. Skip it when the summary already says everything.

## Versioning

Tags are `vMAJOR.MINOR.PATCH` ([semver](https://semver.org)). Pushing a `v*` tag is what ships a release — it triggers `update-tap.yml`, which bumps the Homebrew formula to that tag automatically.

- **PATCH** (`v0.1.0` → `v0.1.1`): bug fixes, performance/internal cleanup, docs, CI — nothing a user of the CLI would need to change anything for.
- **MINOR** (`v0.1.1` → `v0.2.0`): new features or flags that don't break existing usage (e.g. a new `--format`, a new key binding).
- **MAJOR** (`v0.x.y` → `v1.0.0`, then `v1.x.y` → `v2.0.0`): breaking changes — a flag removed/renamed, config file format changed, a key rebound in a way that changes existing muscle memory.

**While we're on `v0.x.y`** (current state): the API/CLI surface is still allowed to break in a **MINOR** bump, per semver's own carve-out for `0.x` releases — there's no user base yet to protect from breakage, so PATCH vs MINOR here is really just "fix" vs "added something new," not a compatibility promise.

**Cutting `v1.0.0`**: do this once the CLI's flags, config format, and key bindings are something you're willing to commit to *not* breaking casually. After that point, any breaking change requires a MAJOR bump, not a MINOR one.
