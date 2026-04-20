# Changelog

## [v1.1.0] — 2026-04-20

### Breaking Changes

- **`TracingBehavior` moved** from `middleware` to `relayotel` sub-module
  (`github.com/phongln/go-relay/relayotel`). Users who import
  `middleware.TracingBehavior` must change to `relayotel.TracingBehavior`.
- **`RecoveryBehavior.Logger`** changed from `*slog.Logger` to `relay.Logger`.
  `*slog.Logger` satisfies the interface directly — existing code compiles
  without changes.
- **`LoggingBehavior.Logger`** changed from `*slog.Logger` to `relay.Logger`.
  Same compatibility note as above.
- **`LoggingBehavior`** no longer auto-extracts OTel trace/span IDs. Set
  `ContextAttrs: relayotel.TraceAttrs` to restore trace/log correlation.

### Added

- **`relay.Logger`** — minimal logging interface (`InfoContext`, `WarnContext`,
  `ErrorContext`). `*slog.Logger` satisfies it out of the box; other loggers
  (zap, zerolog) need a thin adapter.
- **`relay.RequestKind`** — exported helper returning `"command"`, `"query"`,
  `"notification"`, or `"unknown"` for custom pipeline behaviors.
- **`relay.RegisterCommandFactory`** — creates a new `CommandHandler` per
  dispatch, for handlers with per-request state or scoped dependencies.
- **`relay.RegisterQueryFactory`** — same pattern for queries.
- **`relay.RegisterNotificationHandlerFactory`** — same pattern for
  notifications.
- **`LoggingBehavior.ContextAttrs`** — optional `func(ctx) []any` hook to
  append extra attributes (e.g., trace IDs) to every log entry without
  coupling to a specific tracing library.

### Added — `relayotel` sub-module

- **`relayotel.TracingBehavior`** — OTel span per handler (moved from
  `middleware`). Same API, new import path.
- **`relayotel.TraceAttrs`** — extracts `trace_id` and `span_id` from context
  for use with `LoggingBehavior.ContextAttrs`.

### Changed

- **Root `go.mod` has zero external dependencies.** OTel is isolated in
  `relayotel/go.mod`. Users who don't need tracing no longer pull in
  OTel's transitive dependency tree.

---

## [v1.0.0] — 2026-04-20

#### Core (`relay` package)

- `relay.Relay` — injectable interface (`Dispatch`, `Ask`, `Publish`)
- `relay.RealRelay` — exported concrete type returned by `New()`
- `relay.Dispatch[R]` — typed command dispatch with safe type assertion
- `relay.Ask[R]` — typed query dispatch with safe type assertion
- `relay.Publish` — one-to-many notification dispatch
- `relay.Command`, `relay.Query`, `relay.Notification` — marker interfaces
- `relay.CommandHandler[C,R]`, `relay.QueryHandler[Q,R]`, `relay.NotificationHandler[N]`
- `relay.Transactional` — opt-in marker for automatic transaction wrapping
- `relay.Transactor` — pluggable transaction backend interface
- `relay.TxFunc` — transaction body function type
- `relay.PipelineBehavior` — middleware interface
- `relay.RegisterCommand`, `relay.RegisterQuery`, `relay.RegisterNotificationHandler`
- `relay.RealRelay.AssertAllRegistered` — startup safety guard
- `relay.RealRelay.WithTransactor`, `relay.RealRelay.AddPipeline` — builder with chaining
- `relay.HandlerError` — typed error supporting `errors.As` / `errors.Is`
- `relay.ErrHandlerNotFound`, `relay.ErrTransactorNotSet` — sentinel errors
- Context cancellation propagation on all dispatch paths
- `sync.RWMutex` throughout — safe for concurrent use
- `RequestHandlerFunc` accepts `context.Context` — signature
  `func(ctx context.Context) (any, error)` ensures context values (OTel spans,
  deadlines) propagate through pipeline behaviors and into handlers
- Marker interfaces exported — `CommandMarker()`, `QueryMarker()`,
  `NotificationMarker()` — so types in external packages can implement them
- Duplicate handler registration panics at boot time, preventing silent overwrites

#### Middleware (`middleware` package)

- `RecoveryBehavior` — panic recovery with stack trace logging
- `LoggingBehavior` — structured logging with slow-handler warnings
- `ValidationBehavior` — calls `Validate() error` when implemented
- `Validatable` — optional interface for commands and queries

#### MockRelay (`mockrelay` package)

- `MockRelay` — implements `relay.Relay`; no testify dependency
- `mockrelay.TB` — `Helper() + Errorf()` interface; accepts `*testing.T`
- `OnDispatch[C,R]`, `OnDispatchFn[C,R]`, `OnDispatchError[C]`
- `OnAsk[Q,R]`, `OnAskFn[Q,R]`, `OnAskError[Q]`
- `OnPublish[N]`
- `MockRelay.AssertExpectations(TB)` — verifies all expectations were called
- `AssertNotCalled[T](TB, *MockRelay)` — verifies a type was never dispatched

#### Build & CI

- **Makefile** — `make fmt`, `make lint`, `make lint-fix`, `make vet`, `make test`,
  `make check` (all-in-one), and `make tools`.
- **`tools/` module** — `tools/go.mod` pins exact versions of `goimports` and
  `golangci-lint`, installed automatically on lint/fmt targets. Keeps tool
  dependencies isolated from the library's `go.mod`.
- GitHub Actions: Go 1.21, 1.22, 1.23 matrix with race detector
- golangci-lint configuration
- MIT License, Contributing guide
- Runnable example: controller, service, worker, `main.go`
