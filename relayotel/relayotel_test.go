package relayotel_test

import (
	"context"
	"testing"

	"github.com/phongln/go-relay/relay"
	"github.com/phongln/go-relay/relayotel"
)

type testCmd struct{ OrgID string }

func (testCmd) CommandMarker() {}

type testResult struct{ ID string }

type testHandler struct{ called bool }

func (h *testHandler) Handle(_ context.Context, cmd testCmd) (testResult, error) {
	h.called = true
	return testResult{ID: "res-" + cmd.OrgID}, nil
}

func TestTracingBehavior_ImplementsPipelineBehavior(t *testing.T) {
	var _ relay.PipelineBehavior = &relayotel.TracingBehavior{}
}

func TestTracingBehavior_PassesThroughResult(t *testing.T) {
	r := relay.New()
	r.AddPipeline(&relayotel.TracingBehavior{})

	h := &testHandler{}
	relay.RegisterCommand(r, h)

	res, err := relay.Dispatch[testResult](context.Background(), r, testCmd{OrgID: "org-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != "res-org-1" {
		t.Errorf("want res-org-1, got %s", res.ID)
	}
	if !h.called {
		t.Error("handler was not called")
	}
}

func TestTraceAttrs_NilWhenNoSpan(t *testing.T) {
	attrs := relayotel.TraceAttrs(context.Background())
	if attrs != nil {
		t.Errorf("expected nil attrs without active span, got %v", attrs)
	}
}
