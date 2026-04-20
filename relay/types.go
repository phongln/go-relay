// Package relay provides a production-grade CQRS mediator bus for Go.
//
// go-relay implements the Command Query Responsibility Segregation (CQRS)
// and Mediator patterns with explicit separation between writes (commands)
// and reads (queries), automatic transaction support, OpenTelemetry tracing,
// structured logging, and a zero-friction mock for unit testing.
//
// # Packages
//
//   - relay      — core interfaces and dispatch functions
//   - middleware — built-in pipeline behaviors (recovery, tracing, logging, validation)
//   - mockrelay  — test double with typed helpers; no testify required
//
// # Quick start
//
//	// 1. Define a command
//	type CreateCaseCmd struct{ OrgID, PlayerID string; Risk float64 }
//	func (CreateCaseCmd) CommandMarker()   {}
//	func (CreateCaseCmd) WithTransaction() {} // opt into automatic transaction
//
//	// 2. Define a query
//	type GetDashboardQuery struct{ OrgID string; Page int }
//	func (GetDashboardQuery) QueryMarker() {}
//
//	// 3. Write handlers and register on the relay (see bootstrap example)
//
//	// 4. Dispatch from anywhere — controller, service, or worker
//	result, err := relay.Dispatch[CaseResource](ctx, r, CreateCaseCmd{OrgID: "org-1"})
//	list,   err := relay.Ask[[]CaseSummary](ctx, r, GetDashboardQuery{OrgID: "org-1"})
//	err          = relay.Publish(ctx, r, CaseCreatedEvent{CaseID: result.ID})
package relay

import "context"

// ---------------------------------------------------------------------------
// Marker interfaces
// ---------------------------------------------------------------------------

// Command marks a struct as a write operation.
// Implement the CommandMarker() method to satisfy this interface.
//
//	type CreateCaseCmd struct{ OrgID string }
//	func (CreateCaseCmd) CommandMarker() {}
type Command interface{ CommandMarker() }

// Query marks a struct as a read operation.
// Queries must never change state.
// Implement the QueryMarker() method to satisfy this interface.
//
//	type GetDashboardQuery struct{ OrgID string; Page int }
//	func (GetDashboardQuery) QueryMarker() {}
type Query interface{ QueryMarker() }

// Notification marks a struct as a domain event that is published
// to multiple handlers simultaneously.
// Implement the NotificationMarker() method to satisfy this interface.
//
//	type CaseCreatedEvent struct{ CaseID, OrgID string }
//	func (CaseCreatedEvent) NotificationMarker() {}
type Notification interface{ NotificationMarker() }

// ---------------------------------------------------------------------------
// Handler interfaces
// ---------------------------------------------------------------------------

// CommandHandler handles a single [Command] type C and returns response R.
// Register with [RegisterCommand]. One handler per command type.
//
//	type CreateCaseHandler struct{ repo CaseRepository }
//
//	func (h *CreateCaseHandler) Handle(ctx context.Context, cmd CreateCaseCmd) (CaseResource, error) {
//	    // ctx carries the active transaction when cmd implements [Transactional]
//	    return h.repo.Insert(ctx, cmd.OrgID, cmd.PlayerID, cmd.Risk)
//	}
type CommandHandler[C Command, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// QueryHandler handles a single [Query] type Q and returns response R.
// Register with [RegisterQuery]. One handler per query type.
//
//	type GetDashboardHandler struct{ readRepo CaseReadRepository }
//
//	func (h *GetDashboardHandler) Handle(ctx context.Context, q GetDashboardQuery) ([]CaseSummary, error) {
//	    return h.readRepo.FindDashboard(ctx, q.OrgID, q.Page)
//	}
type QueryHandler[Q Query, R any] interface {
	Handle(ctx context.Context, qry Q) (R, error)
}

// NotificationHandler handles a [Notification] type N.
// Multiple handlers can be registered for the same notification type;
// all run when [Publish] is called.
// Register with [RegisterNotificationHandler].
type NotificationHandler[N Notification] interface {
	Handle(ctx context.Context, n N) error
}

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

// RequestHandlerFunc is the continuation passed to each [PipelineBehavior].
// Calling it invokes the next behavior in the chain, or the actual handler
// when no more behaviors remain.
//
// The ctx parameter propagates context changes (e.g. OTel spans, deadlines)
// through the pipeline chain. Always pass your current ctx so downstream
// behaviors and the handler see enriched context values.
type RequestHandlerFunc func(ctx context.Context) (any, error)

// PipelineBehavior is middleware executed around every command and query
// dispatch. Use it for cross-cutting concerns: recovery, tracing, logging,
// validation, metrics, rate limiting, etc.
//
// Register behaviors with [Relay.AddPipeline] in this recommended order:
//
//	r.AddPipeline(&middleware.RecoveryBehavior{...})   // 1. outermost
//	r.AddPipeline(&middleware.TracingBehavior{...})    // 2.
//	r.AddPipeline(&middleware.LoggingBehavior{...})    // 3.
//	r.AddPipeline(&middleware.ValidationBehavior{})    // 4. innermost
type PipelineBehavior interface {
	Handle(ctx context.Context, request any, next RequestHandlerFunc) (any, error)
}

// ---------------------------------------------------------------------------
// Transaction
// ---------------------------------------------------------------------------

// TxFunc is the unit of work executed inside a transaction.
// The ctx it receives already carries the active transaction handle —
// pass it to every repository call so they automatically join the same transaction.
type TxFunc func(ctx context.Context) error

// Transactor abstracts database transaction lifecycle from the relay.
// Implement it once for your database driver and register it with
// [Relay.WithTransactor].
//
// The relay calls WithTransaction automatically for any [Command] that
// implements the [Transactional] marker interface.
//
// MongoDB implementation:
//
//	type MongoTransactor struct{ Client *mongo.Client }
//
//	func (t *MongoTransactor) WithTransaction(ctx context.Context, fn relay.TxFunc) error {
//	    session, err := t.Client.StartSession()
//	    if err != nil { return err }
//	    defer session.EndSession(ctx)
//	    _, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
//	        return nil, fn(sc) // sc is a context.Context carrying the session
//	    })
//	    return err
//	}
//
// PostgreSQL (pgx) implementation:
//
//	type PgxTransactor struct{ Pool *pgxpool.Pool }
//
//	func (t *PgxTransactor) WithTransaction(ctx context.Context, fn relay.TxFunc) error {
//	    tx, err := t.Pool.Begin(ctx)
//	    if err != nil { return err }
//	    if err := fn(pgx.WithTx(ctx, tx)); err != nil {
//	        _ = tx.Rollback(ctx)
//	        return err
//	    }
//	    return tx.Commit(ctx)
//	}
type Transactor interface {
	WithTransaction(ctx context.Context, fn TxFunc) error
}

// Transactional is an opt-in marker for [Command] structs.
// Commands that implement this interface are automatically wrapped in a
// transaction by the relay using the registered [Transactor].
//
// The ctx passed to the handler already carries the active transaction.
// Pass it to every repository call inside Handle so they join it.
//
//	type CreateCaseCmd struct{ OrgID string }
//	func (CreateCaseCmd) CommandMarker()   {}
//	func (CreateCaseCmd) WithTransaction() {} // one-line opt-in
type Transactional interface {
	WithTransaction()
}

// ---------------------------------------------------------------------------
// Relay
// ---------------------------------------------------------------------------

// Relay is the single injectable interface used by controllers, services,
// gRPC handlers, workers, and any other entry point.
//
// Prefer the typed generic functions [Dispatch], [Ask], and [Publish]
// over calling these methods directly — they provide compile-time type
// safety and keep type assertions isolated in one place.
//
// Create with [New] for production. Use mockrelay.New() in tests.
type Relay interface {
	Dispatch(ctx context.Context, cmd Command) (any, error)
	Ask(ctx context.Context, qry Query) (any, error)
	Publish(ctx context.Context, n Notification) error
}
