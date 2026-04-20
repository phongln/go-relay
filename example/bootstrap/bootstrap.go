// Package bootstrap wires the relay, all handlers, pipeline behaviors,
// and the transactor. Call New once in main() and share the result.
package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/handlers"
	"github.com/phongln/go-relay/example/case/queries"
	"github.com/phongln/go-relay/middleware"
	"github.com/phongln/go-relay/relay"
)

// NoopTransactor satisfies relay.Transactor without a real database.
// Replace with MongoTransactor or PgxTransactor in production.
type NoopTransactor struct{}

func (t *NoopTransactor) WithTransaction(ctx context.Context, fn relay.TxFunc) error {
	return fn(ctx)
}

// New constructs and returns a fully configured relay.Relay.
//
// *slog.Logger satisfies relay.Logger directly — no adapter needed.
// For OTel tracing, import github.com/phongln/go-relay/relayotel and add:
//
//	r.AddPipeline(&relayotel.TracingBehavior{})
//
// For trace/log correlation, set ContextAttrs:
//
//	r.AddPipeline(&middleware.LoggingBehavior{
//	    Logger:       logger,
//	    ContextAttrs: relayotel.TraceAttrs,
//	})
func New(logger *slog.Logger) relay.Relay {
	r := relay.New()

	r.WithTransactor(&NoopTransactor{})

	r.AddPipeline(&middleware.RecoveryBehavior{Logger: logger})
	r.AddPipeline(&middleware.LoggingBehavior{Logger: logger, SlowThreshold: 500 * time.Millisecond})
	r.AddPipeline(&middleware.ValidationBehavior{})

	relay.RegisterCommand(r, &handlers.CreateCaseHandler{Logger: logger})
	relay.RegisterCommand(r, &handlers.CloseCaseHandler{Logger: logger})
	relay.RegisterQuery(r, &handlers.GetDashboardHandler{Logger: logger})
	relay.RegisterQuery(r, &handlers.GetCaseByIDHandler{Logger: logger})
	relay.RegisterNotificationHandler(r, &handlers.WebhookDispatcher{Logger: logger})
	relay.RegisterNotificationHandler(r, &handlers.AuditLogger{Logger: logger})

	r.AssertAllRegistered(
		[]relay.Command{commands.CreateCaseCmd{}, commands.CloseCaseCmd{}},
		[]relay.Query{queries.GetDashboardQuery{}, queries.GetCaseByIDQuery{}},
	)

	return r
}
