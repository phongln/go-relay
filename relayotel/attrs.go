package relayotel

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// TraceAttrs extracts trace_id and span_id from the current span context.
// Returns nil when no valid span is active.
//
// Pass this to [middleware.LoggingBehavior.ContextAttrs] to correlate logs
// with OTel traces:
//
//	r.AddPipeline(&middleware.LoggingBehavior{
//	    Logger:       slog.Default(),
//	    ContextAttrs: relayotel.TraceAttrs,
//	})
func TraceAttrs(ctx context.Context) []any {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return nil
	}
	return []any{
		"trace_id", sc.TraceID().String(),
		"span_id", sc.SpanID().String(),
	}
}
