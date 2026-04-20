package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/phongln/go-relay/relay"
)

// LoggingBehavior emits a structured log entry for every command and query.
//
// Log entry fields:
//
//	request_type — fully qualified Go type name
//	request_kind — "command" or "query"
//	duration_ms  — wall-clock execution time in milliseconds
//	error        — error message on failure (only present on error)
//
// ContextAttrs: when set, the function is called with the current context
// and its return value is appended to every log entry. Use this for
// trace/log correlation without coupling to a specific tracing library:
//
//	// with the relayotel sub-module:
//	r.AddPipeline(&middleware.LoggingBehavior{
//	    Logger:       slog.Default(),
//	    ContextAttrs: relayotel.TraceAttrs,
//	})
//
// SlowThreshold: when non-zero, emits a Warn log when a handler exceeds
// this duration. Zero disables slow-handler warnings.
//
// Logger accepts any [relay.Logger] implementation. *slog.Logger satisfies
// this interface directly.
type LoggingBehavior struct {
	Logger        relay.Logger
	SlowThreshold time.Duration
	ContextAttrs  func(ctx context.Context) []any
}

// Handle implements [relay.PipelineBehavior].
func (l *LoggingBehavior) Handle(
	ctx context.Context,
	request any,
	next relay.RequestHandlerFunc,
) (any, error) {
	start := time.Now()
	result, err := next(ctx)
	elapsed := time.Since(start)
	ms := float64(elapsed.Microseconds()) / 1000.0

	attrs := []any{
		"request_type", fmt.Sprintf("%T", request),
		"request_kind", relay.RequestKind(request),
		"duration_ms", ms,
	}

	if l.ContextAttrs != nil {
		attrs = append(attrs, l.ContextAttrs(ctx)...)
	}

	if err != nil {
		attrs = append(attrs, "error", err.Error())
		l.Logger.ErrorContext(ctx, "go-relay: handler failed", attrs...)
		return nil, err
	}

	if l.SlowThreshold > 0 && elapsed > l.SlowThreshold {
		l.Logger.WarnContext(ctx, "go-relay: slow handler", attrs...)
	} else {
		l.Logger.InfoContext(ctx, "go-relay: handler ok", attrs...)
	}
	return result, nil
}
