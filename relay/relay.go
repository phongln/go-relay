package relay

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// ---------------------------------------------------------------------------
// Internal function types
// ---------------------------------------------------------------------------

type handlerFn func(context.Context, any) (any, error)
type notifFn func(context.Context, any) error

// ---------------------------------------------------------------------------
// RealRelay — the production implementation
// ---------------------------------------------------------------------------

// RealRelay is the concrete Relay returned by [New].
// It is exported so that RegisterCommand, RegisterQuery,
// RegisterNotificationHandler, WithTransactor, AddPipeline, and
// AssertAllRegistered can accept it as a concrete type while callers
// inject only the Relay interface.
type RealRelay struct {
	mu            sync.RWMutex
	commands      map[reflect.Type]handlerFn
	queries       map[reflect.Type]handlerFn
	notifications map[reflect.Type][]notifFn
	pipelines     []PipelineBehavior
	transactor    Transactor
}

// New creates a new [RealRelay]. Configure it before passing to handlers:
//
//	r := relay.New()
//	r.WithTransactor(&MongoTransactor{Client: client})
//	r.AddPipeline(&middleware.RecoveryBehavior{Logger: logger})
//	r.AddPipeline(&middleware.TracingBehavior{})
//	r.AddPipeline(&middleware.LoggingBehavior{Logger: logger})
//	r.AddPipeline(&middleware.ValidationBehavior{})
//	relay.RegisterCommand(r, &CreateCaseHandler{Repo: repo})
//	relay.RegisterQuery(r, &GetDashboardHandler{ReadRepo: readRepo})
//	r.AssertAllRegistered(expectedCmds, expectedQueries)
func New() *RealRelay {
	return &RealRelay{
		commands:      make(map[reflect.Type]handlerFn),
		queries:       make(map[reflect.Type]handlerFn),
		notifications: make(map[reflect.Type][]notifFn),
	}
}

// WithTransactor sets the [Transactor] for automatic transaction wrapping.
// Returns the relay for method chaining.
func (r *RealRelay) WithTransactor(t Transactor) *RealRelay {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transactor = t
	return r
}

// AddPipeline appends a [PipelineBehavior] to the chain.
// The first behavior added is the outermost wrapper.
// Returns the relay for method chaining.
func (r *RealRelay) AddPipeline(p PipelineBehavior) *RealRelay {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pipelines = append(r.pipelines, p)
	return r
}

// AssertAllRegistered panics at startup if any expected handler is missing.
// Call this at the end of your bootstrap function after all registrations.
//
//	r.AssertAllRegistered(
//	    []relay.Command{CreateCaseCmd{}, CloseCaseCmd{}},
//	    []relay.Query{GetDashboardQuery{}, GetCaseByIDQuery{}},
//	)
func (r *RealRelay) AssertAllRegistered(cmds []Command, queries []Query) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, c := range cmds {
		if _, ok := r.commands[reflect.TypeOf(c)]; !ok {
			panic(fmt.Sprintf(
				"go-relay: no command handler registered for %T — did you forget RegisterCommand?", c,
			))
		}
	}
	for _, q := range queries {
		if _, ok := r.queries[reflect.TypeOf(q)]; !ok {
			panic(fmt.Sprintf(
				"go-relay: no query handler registered for %T — did you forget RegisterQuery?", q,
			))
		}
	}
}

// ---------------------------------------------------------------------------
// Registration — package-level generic functions
// ---------------------------------------------------------------------------

// RegisterCommand wires a [CommandHandler] to its [Command] type.
// Safe to call concurrently during bootstrap.
//
//	relay.RegisterCommand(r, &handlers.CreateCaseHandler{Repo: repo})
func RegisterCommand[C Command, R any](r *RealRelay, h CommandHandler[C, R]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero C
	t := reflect.TypeOf(zero)
	if _, exists := r.commands[t]; exists {
		panic(fmt.Sprintf("go-relay: duplicate command handler registered for %T", zero))
	}
	r.commands[t] = func(ctx context.Context, raw any) (any, error) {
		return h.Handle(ctx, raw.(C))
	}
}

// RegisterQuery wires a [QueryHandler] to its [Query] type.
// Safe to call concurrently during bootstrap.
//
//	relay.RegisterQuery(r, &handlers.GetDashboardHandler{ReadRepo: readRepo})
func RegisterQuery[Q Query, R any](r *RealRelay, h QueryHandler[Q, R]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero Q
	t := reflect.TypeOf(zero)
	if _, exists := r.queries[t]; exists {
		panic(fmt.Sprintf("go-relay: duplicate query handler registered for %T", zero))
	}
	r.queries[t] = func(ctx context.Context, raw any) (any, error) {
		return h.Handle(ctx, raw.(Q))
	}
}

// RegisterNotificationHandler adds a [NotificationHandler] for a [Notification] type.
// Multiple handlers may be registered for the same type; all run on [Publish].
// Safe to call concurrently during bootstrap.
//
//	relay.RegisterNotificationHandler(r, &handlers.WebhookDispatcher{})
//	relay.RegisterNotificationHandler(r, &handlers.AuditLogger{})
func RegisterNotificationHandler[N Notification](r *RealRelay, h NotificationHandler[N]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero N
	t := reflect.TypeOf(zero)
	r.notifications[t] = append(r.notifications[t], func(ctx context.Context, raw any) error {
		return h.Handle(ctx, raw.(N))
	})
}

// ---------------------------------------------------------------------------
// Factory registration — new handler instance per request
// ---------------------------------------------------------------------------

// RegisterCommandFactory registers a factory that creates a new [CommandHandler]
// for each dispatch. Use this when your handler has per-request state or scoped
// dependencies (e.g., a unit-of-work that must not be shared across requests).
//
//	relay.RegisterCommandFactory(r, func() relay.CommandHandler[CreateCaseCmd, CaseResource] {
//	    return &CreateCaseHandler{Repo: repo.NewScoped()}
//	})
//
// For stateless handlers (the common case), prefer [RegisterCommand] instead.
func RegisterCommandFactory[C Command, R any](r *RealRelay, factory func() CommandHandler[C, R]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero C
	t := reflect.TypeOf(zero)
	if _, exists := r.commands[t]; exists {
		panic(fmt.Sprintf("go-relay: duplicate command handler registered for %T", zero))
	}
	r.commands[t] = func(ctx context.Context, raw any) (any, error) {
		return factory().Handle(ctx, raw.(C))
	}
}

// RegisterQueryFactory registers a factory that creates a new [QueryHandler]
// for each ask. Use this when your handler has per-request state or scoped
// dependencies.
//
//	relay.RegisterQueryFactory(r, func() relay.QueryHandler[GetCaseQuery, CaseResource] {
//	    return &GetCaseHandler{ReadRepo: readRepo.NewScoped()}
//	})
//
// For stateless handlers (the common case), prefer [RegisterQuery] instead.
func RegisterQueryFactory[Q Query, R any](r *RealRelay, factory func() QueryHandler[Q, R]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero Q
	t := reflect.TypeOf(zero)
	if _, exists := r.queries[t]; exists {
		panic(fmt.Sprintf("go-relay: duplicate query handler registered for %T", zero))
	}
	r.queries[t] = func(ctx context.Context, raw any) (any, error) {
		return factory().Handle(ctx, raw.(Q))
	}
}

// RegisterNotificationHandlerFactory adds a factory that creates a new
// [NotificationHandler] for each publish. Multiple factories may be registered
// for the same notification type; all run on [Publish].
//
//	relay.RegisterNotificationHandlerFactory(r, func() relay.NotificationHandler[CaseCreatedEvent] {
//	    return &WebhookDispatcher{Client: http.DefaultClient}
//	})
func RegisterNotificationHandlerFactory[N Notification](r *RealRelay, factory func() NotificationHandler[N]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero N
	t := reflect.TypeOf(zero)
	r.notifications[t] = append(r.notifications[t], func(ctx context.Context, raw any) error {
		return factory().Handle(ctx, raw.(N))
	})
}

// ---------------------------------------------------------------------------
// Relay interface implementation
// ---------------------------------------------------------------------------

// Dispatch implements [Relay].
func (r *RealRelay) Dispatch(ctx context.Context, cmd Command) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	h, ok := r.commands[reflect.TypeOf(cmd)]
	tx := r.transactor
	r.mu.RUnlock()

	if !ok {
		return nil, wrapErr(cmd, fmt.Errorf("%w for %T", ErrHandlerNotFound, cmd))
	}

	if _, wantsTx := cmd.(Transactional); wantsTx {
		if tx == nil {
			return nil, wrapErr(cmd, ErrTransactorNotSet)
		}
		var result any
		err := tx.WithTransaction(ctx, func(txCtx context.Context) error {
			var handlerErr error
			result, handlerErr = r.runPipeline(txCtx, cmd, h)
			return handlerErr
		})
		if err != nil {
			return nil, wrapErr(cmd, err)
		}
		return result, nil
	}

	out, err := r.runPipeline(ctx, cmd, h)
	if err != nil {
		return nil, wrapErr(cmd, err)
	}
	return out, nil
}

// Ask implements [Relay].
func (r *RealRelay) Ask(ctx context.Context, qry Query) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	h, ok := r.queries[reflect.TypeOf(qry)]
	r.mu.RUnlock()

	if !ok {
		return nil, wrapErr(qry, fmt.Errorf("%w for %T", ErrHandlerNotFound, qry))
	}

	out, err := r.runPipeline(ctx, qry, h)
	if err != nil {
		return nil, wrapErr(qry, err)
	}
	return out, nil
}

// Publish implements [Relay].
func (r *RealRelay) Publish(ctx context.Context, n Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.RLock()
	handlers, ok := r.notifications[reflect.TypeOf(n)]
	r.mu.RUnlock()

	if !ok {
		return nil // no handlers registered is an intentional no-op
	}

	var errs []error
	for _, h := range handlers {
		if err := h(ctx, n); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// runPipeline chains all registered behaviors and the handler.
func (r *RealRelay) runPipeline(ctx context.Context, req any, handler handlerFn) (any, error) {
	r.mu.RLock()
	pipelines := r.pipelines
	r.mu.RUnlock()

	if len(pipelines) == 0 {
		return handler(ctx, req)
	}

	var build func(i int) RequestHandlerFunc
	build = func(i int) RequestHandlerFunc {
		return func(ctx context.Context) (any, error) {
			if i >= len(pipelines) {
				return handler(ctx, req)
			}
			return pipelines[i].Handle(ctx, req, build(i+1))
		}
	}
	return build(0)(ctx)
}

// ---------------------------------------------------------------------------
// Public typed API — the only three functions callers ever need
// ---------------------------------------------------------------------------

// Dispatch executes a [Command] and returns a typed result R.
//
// If the command implements [Transactional], the handler is automatically
// wrapped in a transaction using the configured [Transactor].
// The ctx received by the handler already carries the active transaction;
// pass it to all repository calls so they join it.
//
//	result, err := relay.Dispatch[CaseResource](ctx, r, CreateCaseCmd{
//	    OrgID:    "org-1",
//	    PlayerID: "player-1",
//	})
func Dispatch[R any](ctx context.Context, r Relay, cmd Command) (R, error) {
	var zero R
	out, err := r.Dispatch(ctx, cmd)
	if err != nil {
		return zero, err
	}
	typed, ok := out.(R)
	if !ok {
		return zero, fmt.Errorf("go-relay: type assertion failed: expected %T, handler returned %T", zero, out)
	}
	return typed, nil
}

// Ask executes a [Query] and returns a typed result R.
// Queries must never change state.
//
//	list, err := relay.Ask[[]CaseSummary](ctx, r, GetDashboardQuery{
//	    OrgID: "org-1",
//	    Page:  1,
//	})
func Ask[R any](ctx context.Context, r Relay, qry Query) (R, error) {
	var zero R
	out, err := r.Ask(ctx, qry)
	if err != nil {
		return zero, err
	}
	typed, ok := out.(R)
	if !ok {
		return zero, fmt.Errorf("go-relay: type assertion failed: expected %T, handler returned %T", zero, out)
	}
	return typed, nil
}

// Publish dispatches a [Notification] to all registered handlers.
// Execution continues even if individual handlers fail.
// Returns a joined error if any handlers fail; nil if none are registered.
//
//	err := relay.Publish(ctx, r, CaseCreatedEvent{
//	    CaseID: result.ID,
//	    OrgID:  result.OrgID,
//	})
func Publish(ctx context.Context, r Relay, n Notification) error {
	return r.Publish(ctx, n)
}
