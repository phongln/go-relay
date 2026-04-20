# Contributing to go-relay

## Setup

```bash
git clone https://github.com/phongln/go-relay
cd go-relay
go mod tidy
```

## Running Tests

```bash
# Core library with race detector (required before any PR)
go test -race ./relay/... ./middleware/... ./mockrelay/...

# Full suite including examples
go test -race ./...

# Coverage
go test -race -coverprofile=coverage.out ./relay/... ./middleware/... ./mockrelay/...
go tool cover -html=coverage.out
```

## Code Standards

- `go fmt ./...` before committing
- `go vet ./...` before committing
- All exported symbols must have godoc comments
- Tests must pass with `-race`
- New pipeline behaviors go in `middleware/` and must implement `relay.PipelineBehavior`

## Pull Request Checklist

- [ ] Tests added or updated
- [ ] `go test -race ./...` passes
- [ ] `go vet ./...` passes
- [ ] Godoc on all exported symbols
- [ ] `CHANGELOG.md` updated under `## Unreleased`

## Reporting Issues

Please include:
- Go version (`go version`)
- go-relay version
- Minimal reproduction
