package middleware

import (
	"context"
	"fmt"

	"github.com/phongln/go-relay/relay"
)

// Validatable is an optional interface for [relay.Command] and [relay.Query] structs.
// When implemented, ValidationBehavior calls Validate() before the handler runs.
// Return a non-nil error to abort execution before the handler is called.
//
//	type CreateCaseCmd struct{ OrgID string }
//
//	func (c CreateCaseCmd) CommandMarker() {}
//	func (c CreateCaseCmd) Validate() error {
//	    if c.OrgID == "" { return errors.New("org_id is required") }
//	    return nil
//	}
//
// Works with any validation library — go-playground/validator, ozzo-validation,
// or plain conditional logic.
type Validatable interface {
	Validate() error
}

// ValidationBehavior calls Validate() on requests that implement [Validatable]
// before passing control to the handler. A validation failure is returned as
// an error and the handler is never called.
//
// Register this as the last (innermost) pipeline behavior so that tracing and
// logging behaviors can capture validation failures in their spans/logs.
type ValidationBehavior struct{}

// Handle implements [relay.PipelineBehavior].
func (v *ValidationBehavior) Handle(
	ctx context.Context,
	request any,
	next relay.RequestHandlerFunc,
) (any, error) {
	if val, ok := request.(Validatable); ok {
		if err := val.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed for %T: %w", request, err)
		}
	}
	return next(ctx)
}
