package middleware

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/phongln/go-relay/relay"
)

const tracerName = "github.com/phongln/go-relay"

// TracingBehavior creates an OpenTelemetry span for every command and query
// dispatched through the relay.
//
// Span naming:
//
//	relay.command CreateCaseCmd
//	relay.query   GetDashboardQuery
//
// Span attributes:
//
//	relay.request_type — fully qualified Go type, e.g. "commands.CreateCaseCmd"
//	relay.request_kind — "command", "query", or "notification"
//
// Uses the global OTel tracer provider by default. Configure it once at
// startup via otel.SetTracerProvider(...). Falls back to a no-op tracer
// when no provider is set — safe without an OTel backend.
//
// Register AFTER RecoveryBehavior and BEFORE LoggingBehavior so that the
// span exists in ctx when LoggingBehavior reads the trace/span IDs.
type TracingBehavior struct {
	// TracerProvider overrides the global OTel provider. Leave nil to use
	// otel.GetTracerProvider() (recommended for production).
	TracerProvider trace.TracerProvider
}

func (t *TracingBehavior) tracer() trace.Tracer {
	if t.TracerProvider != nil {
		return t.TracerProvider.Tracer(tracerName)
	}
	return otel.Tracer(tracerName)
}

// Handle implements [relay.PipelineBehavior].
func (t *TracingBehavior) Handle(
	ctx context.Context,
	request any,
	next relay.RequestHandlerFunc,
) (any, error) {
	kind := requestKind(request)
	reqType := fmt.Sprintf("%T", request)
	spanName := "relay." + kind + " " + reqType

	ctx, span := t.tracer().Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	span.SetAttributes(
		attribute.String("relay.request_type", reqType),
		attribute.String("relay.request_kind", kind),
	)

	result, err := next(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "")
	return result, nil
}
