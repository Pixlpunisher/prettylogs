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
