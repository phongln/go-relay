package relay

import (
	"errors"
	"fmt"
)

// Sentinel errors — check with errors.Is.
var (
	// ErrHandlerNotFound is returned when no handler is registered for the
	// given command or query type.
	ErrHandlerNotFound = errors.New("go-relay: no handler registered")

	// ErrTransactorNotSet is returned when a [Transactional] command is
	// dispatched but no [Transactor] has been configured on the relay.
	ErrTransactorNotSet = errors.New("go-relay: transactor not configured — call WithTransactor during setup")
)

// HandlerError wraps a handler failure with the originating request type.
// Use errors.As to extract it for structured logging or metrics.
//
//	var he *relay.HandlerError
//	if errors.As(err, &he) {
//	    logger.Error("handler failed", "type", he.RequestType, "cause", he.Cause)
//	}
type HandlerError struct {
	// RequestType is the fully qualified Go type name of the failed request.
	RequestType string
	// Cause is the underlying error returned by the handler or transactor.
	Cause error
}

func (e *HandlerError) Error() string {
	return fmt.Sprintf("go-relay: handler failed for %s: %v", e.RequestType, e.Cause)
}

// Unwrap allows errors.Is and errors.As to traverse the error chain.
func (e *HandlerError) Unwrap() error { return e.Cause }

func wrapErr(request any, cause error) *HandlerError {
	return &HandlerError{
		RequestType: fmt.Sprintf("%T", request),
		Cause:       cause,
	}
}
