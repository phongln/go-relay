# go-relay

[![Go Reference](https://pkg.go.dev/badge/github.com/phongln/go-relay.svg)](https://pkg.go.dev/github.com/phongln/go-relay)
[![Go Report Card](https://goreportcard.com/badge/github.com/phongln/go-relay)](https://goreportcard.com/report/github.com/phongln/go-relay)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go 1.21+](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://go.dev)

**go-relay** is a production-grade CQRS mediator bus for Go.

It implements the [CQRS](https://martinfowler.com/bliki/CQRS.html) and [Mediator](https://refactoring.guru/design-patterns/mediator) patterns with explicit separation between writes (commands) and reads (queries), automatic transaction support, OpenTelemetry tracing, structured logging, and a zero-friction test double.

---

## Why go-relay?

| Problem | Without go-relay | With go-relay |
|---|---|---|
| Controller injection | One handler per operation | Single `relay.Relay` injection |
| Typed returns | Manual type assertions | `relay.Dispatch[CaseResource](...)` |
| Unit testing | Mock each handler separately | One `MockRelay`, typed helpers |
| Transactions | Manual session plumbing per handler | `WithTransaction()` marker on command |
| Cross-cutting concerns | Repeated in every handler | Pipeline behaviors, registered once |
| Missing handlers | Runtime panic in production | `AssertAllRegistered` panics at boot |

---

## Install

```bash
go get github.com/phongln/go-relay
```

Requires **Go 1.21+**. No CGo. Only depends on OpenTelemetry.

---

## Packages

| Package | Purpose |
|---|---|
| `relay` | Core — `Dispatch`, `Ask`, `Publish`, all interfaces |
| `middleware` | Built-in pipeline behaviors: recovery, tracing, logging, validation |
| `mockrelay` | Test double with typed helpers; no testify required |

---

## Quick Start

### 1. Define a Command (write)

```go
type CreateCaseCmd struct {
    OrgID     string
    PlayerID  string
    RiskScore float64
}

func (CreateCaseCmd) CommandMarker()   {} // required
func (CreateCaseCmd) WithTransaction() {} // opt into automatic transaction

func (c CreateCaseCmd) Validate() error {
    if c.OrgID == "" { return errors.New("org_id is required") }
    return nil
}
```

### 2. Define a Query (read)

```go
type GetDashboardQuery struct {
    OrgID  string
    Page   int
    Status string
}

func (GetDashboardQuery) QueryMarker() {} // required
```

### 3. Write Handlers

```go
type CreateCaseHandler struct{ Repo CaseRepository }

// ctx already carries the active transaction — pass it to every repo call.
func (h *CreateCaseHandler) Handle(ctx context.Context, cmd CreateCaseCmd) (CaseResource, error) {
    return h.Repo.Insert(ctx, cmd.OrgID, cmd.PlayerID, cmd.RiskScore)
}

type GetDashboardHandler struct{ ReadRepo CaseReadRepository }

func (h *GetDashboardHandler) Handle(ctx context.Context, q GetDashboardQuery) ([]CaseSummary, error) {
    return h.ReadRepo.FindDashboard(ctx, q.OrgID, q.Page, q.Status)
}
```

### 4. Bootstrap Once

```go
func NewRelay(logger *slog.Logger) relay.Relay {
    r := relay.New()

    r.WithTransactor(&MongoTransactor{Client: mongoClient})

    r.AddPipeline(&middleware.RecoveryBehavior{Logger: logger})  // 1. outermost
    r.AddPipeline(&middleware.TracingBehavior{})                  // 2.
    r.AddPipeline(&middleware.LoggingBehavior{                    // 3.
        Logger:        logger,
        SlowThreshold: 500 * time.Millisecond,
    })
    r.AddPipeline(&middleware.ValidationBehavior{})               // 4. innermost

    relay.RegisterCommand(r, &CreateCaseHandler{Repo: caseRepo})
    relay.RegisterQuery(r, &GetDashboardHandler{ReadRepo: readRepo})

    r.AssertAllRegistered(
        []relay.Command{CreateCaseCmd{}},
        []relay.Query{GetDashboardQuery{}},
    )

    return r
}
```

### 5. Use Everywhere — Same Pattern in Controller, Service, and Worker

```go
// HTTP controller
type CaseController struct{ relay relay.Relay }

func (c *CaseController) Create(w http.ResponseWriter, r *http.Request) {
    result, err := relay.Dispatch[CaseResource](r.Context(), c.relay, CreateCaseCmd{
        OrgID: dto.OrgID, PlayerID: dto.PlayerID,
    })
}

// Service layer — identical pattern
type CaseService struct{ relay relay.Relay }

func (s *CaseService) Process(ctx context.Context, orgID string) error {
    result, err := relay.Dispatch[CaseResource](ctx, s.relay, CreateCaseCmd{OrgID: orgID})
    list, err   := relay.Ask[[]CaseSummary](ctx, s.relay, GetDashboardQuery{OrgID: orgID})
    _ = relay.Publish(ctx, s.relay, CaseCreatedEvent{CaseID: result.ID})
    return err
}

// SQS / Kafka worker — identical pattern
type CaseWorker struct{ relay relay.Relay }

func (w *CaseWorker) Handle(ctx context.Context, msg string) error {
    _, err := relay.Dispatch[CaseResource](ctx, w.relay, CreateCaseCmd{OrgID: parsed.OrgID})
    return err
}
```

---

## Unit Testing

```go
func TestCaseController_Create(t *testing.T) {
    mr := mockrelay.New()

    // Fixed response
    mockrelay.OnDispatch(mr,
        CreateCaseCmd{OrgID: "org-1", PlayerID: "p-1"},
        CaseResource{ID: "case-123", Status: "open"},
        nil,
    )
    mockrelay.OnPublish[CaseCreatedEvent](mr, nil)

    ctrl := controller.New(mr)
    // ... call ctrl, assert HTTP response
    mr.AssertExpectations(t)
}

// Error path — one line
mockrelay.OnDispatchError[CreateCaseCmd](mr, errors.New("db unavailable"))

// Dynamic response — inspect command fields
mockrelay.OnDispatchFn(mr, func(ctx context.Context, cmd CreateCaseCmd) (CaseResource, error) {
    if cmd.RiskScore > 90 { return CaseResource{Status: "high-risk"}, nil }
    return CaseResource{Status: "open"}, nil
})

// Assert a command was never called
mockrelay.AssertNotCalled[CreateCaseCmd](t, mr)
```

---

## Transactions

Add `WithTransaction()` to a command struct — the relay handles the rest automatically.

```go
func (CreateCaseCmd) WithTransaction() {} // ← one-line opt-in
```

The `ctx` passed to your handler already carries the active transaction:

```go
func (h *CreateCaseHandler) Handle(ctx context.Context, cmd CreateCaseCmd) (CaseResource, error) {
    if err := h.caseRepo.Insert(ctx, ...); err != nil {   // ctx carries tx
        return CaseResource{}, err                          // → auto rollback
    }
    if err := h.summaryRepo.Upsert(ctx, ...); err != nil {
        return CaseResource{}, err                          // → auto rollback
    }
    return result, nil                                      // → auto commit
}
```

### MongoDB Transactor

```go
type MongoTransactor struct{ Client *mongo.Client }

func (t *MongoTransactor) WithTransaction(ctx context.Context, fn relay.TxFunc) error {
    session, err := t.Client.StartSession()
    if err != nil { return err }
    defer session.EndSession(ctx)
    _, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
        return nil, fn(sc)
    })
    return err
}
```

### PostgreSQL (pgx) Transactor

```go
type PgxTransactor struct{ Pool *pgxpool.Pool }

func (t *PgxTransactor) WithTransaction(ctx context.Context, fn relay.TxFunc) error {
    tx, err := t.Pool.Begin(ctx)
    if err != nil { return err }
    if err := fn(pgx.WithTx(ctx, tx)); err != nil {
        _ = tx.Rollback(ctx)
        return err
    }
    return tx.Commit(ctx)
}
```

---

## Domain Events

```go
type CaseCreatedEvent struct{ CaseID, OrgID string }
func (CaseCreatedEvent) NotificationMarker() {}

// Register multiple handlers — all run on Publish
relay.RegisterNotificationHandler(r, &WebhookDispatcher{})
relay.RegisterNotificationHandler(r, &AuditLogger{})

// Publish after a successful command
err := relay.Publish(ctx, r, CaseCreatedEvent{CaseID: result.ID, OrgID: result.OrgID})
```

---

## Pipeline Behaviors

| Order | Behavior | Purpose |
|---|---|---|
| 1 (outermost) | `RecoveryBehavior` | Catches panics, logs stack trace, returns error |
| 2 | `TracingBehavior` | OTel span per handler |
| 3 | `LoggingBehavior` | Structured `slog` with trace/span ID correlation |
| 4 (innermost) | `ValidationBehavior` | Calls `Validate()` before the handler |

### Custom Behavior

```go
type MetricsBehavior struct{ histogram metric.Float64Histogram }

func (m *MetricsBehavior) Handle(ctx context.Context, req any, next relay.RequestHandlerFunc) (any, error) {
    start := time.Now()
    result, err := next(ctx)
    m.histogram.Record(ctx, time.Since(start).Seconds(),
        metric.WithAttributes(attribute.String("type", fmt.Sprintf("%T", req))),
    )
    return result, err
}
```

---

## Observability

### Tracing

```
Span:        relay.command CreateCaseCmd
Attributes:  relay.request_type = "commands.CreateCaseCmd"
             relay.request_kind = "command"
```

### Logging

```json
{
  "level":        "INFO",
  "msg":          "go-relay: handler ok",
  "request_type": "commands.CreateCaseCmd",
  "request_kind": "command",
  "duration_ms":  12.4,
  "trace_id":     "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id":      "00f067aa0ba902b7"
}
```

---

## Error Handling

```go
var he *relay.HandlerError
if errors.As(err, &he) {
    fmt.Println(he.RequestType) // "commands.CreateCaseCmd"
    fmt.Println(he.Cause)       // underlying error
}

errors.Is(err, relay.ErrHandlerNotFound)  // no handler registered
errors.Is(err, relay.ErrTransactorNotSet) // Transactional cmd without transactor
```

---

## Run the Example

```bash
git clone https://github.com/phongln/go-relay
cd go-relay/example
go run main.go
```

## Development

A `Makefile` is provided to streamline formatting, linting, and testing.
Tool versions (`goimports`, `golangci-lint`) are pinned in `tools/go.mod` and
installed automatically — new developers just run `make` targets, no manual setup.

```bash
make tools      # install pinned goimports + golangci-lint
make fmt        # gofmt + goimports (autofix)
make lint       # golangci-lint (report only)
make lint-fix   # golangci-lint --fix (autofix)
make vet        # go vet
make test       # go test -race ./...
make check      # fmt + lint-fix + vet + test (all-in-one)
```

## Run Tests

```bash
go test -race ./...
```

---

## License

MIT — see [LICENSE](LICENSE).
