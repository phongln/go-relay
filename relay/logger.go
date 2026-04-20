package relay

import "context"

// Logger is the minimal logging interface used by pipeline behaviors.
//
// *slog.Logger satisfies this interface out of the box — just pass it
// directly. For other loggers (zap, zerolog, logrus, etc.), write a
// thin adapter that delegates to the underlying logger.
//
//	// slog — works directly, no adapter needed:
//	r.AddPipeline(&middleware.RecoveryBehavior{Logger: slog.Default()})
//
//	// zap adapter example:
//	type ZapLogger struct{ L *zap.SugaredLogger }
//	func (z *ZapLogger) InfoContext(_ context.Context, msg string, args ...any)  { z.L.Infow(msg, args...) }
//	func (z *ZapLogger) WarnContext(_ context.Context, msg string, args ...any)  { z.L.Warnw(msg, args...) }
//	func (z *ZapLogger) ErrorContext(_ context.Context, msg string, args ...any) { z.L.Errorw(msg, args...) }
type Logger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}
