package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/phongln/go-relay/relay"
)

// LoggingBehavior emits a structured slog entry for every command and query.
//
// Log entry fields:
//
//	request_type — fully qualified Go type name
//	request_kind — "command" or "query"
//	duration_ms  — wall-clock execution time in milliseconds
//	trace_id     — OTel trace ID when a span is active in ctx
//	span_id      — OTel span ID when a span is active in ctx
//	error        — error message on failure (only present on error)
//
// trace_id and span_id are populated automatically when registered
// after TracingBehavior, enabling trace/log correlation in Datadog,
// Grafana Loki, GCP Cloud Logging, etc.
//
// SlowThreshold: when non-zero, emits a Warn log when a handler exceeds
// this duration. Zero disables slow-handler warnings.
type LoggingBehavior struct {
	Logger        *slog.Logger
	SlowThreshold time.Duration
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
		"request_kind", requestKind(request),
		"duration_ms", ms,
	}

	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		attrs = append(attrs,
			"trace_id", sc.TraceID().String(),
			"span_id", sc.SpanID().String(),
		)
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
