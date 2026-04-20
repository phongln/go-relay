// Package middleware provides built-in [relay.PipelineBehavior] implementations.
//
// Register behaviors on the relay in this order for correct operation:
//
//	r.AddPipeline(&middleware.RecoveryBehavior{Logger: logger})  // 1. outermost
//	r.AddPipeline(&middleware.TracingBehavior{})                 // 2.
//	r.AddPipeline(&middleware.LoggingBehavior{Logger: logger})   // 3.
//	r.AddPipeline(&middleware.ValidationBehavior{})              // 4. innermost
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/phongln/go-relay/relay"
)

// RecoveryBehavior catches panics from any downstream handler or behavior
// and converts them to errors, preventing a single bad handler from
// crashing the entire process.
//
// Register this as the first (outermost) pipeline behavior so it wraps
// everything else — including tracing and logging.
type RecoveryBehavior struct {
	Logger *slog.Logger
}

// Handle implements [relay.PipelineBehavior].
func (r *RecoveryBehavior) Handle(
	ctx context.Context,
	request any,
	next relay.RequestHandlerFunc,
) (res any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			stack := string(debug.Stack())
			r.Logger.ErrorContext(ctx, "go-relay: panic recovered in handler",
				"request_type", fmt.Sprintf("%T", request),
				"panic", fmt.Sprintf("%v", rec),
				"stack", stack,
			)
			err = fmt.Errorf("go-relay: panic in handler for %T: %v", request, rec)
		}
	}()
	return next(ctx)
}
