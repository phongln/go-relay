package mockrelay_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/phongln/go-relay/mockrelay"
	"github.com/phongln/go-relay/relay"
)

// ---------------------------------------------------------------------------
// Test domain types
// ---------------------------------------------------------------------------

type createCmd struct{ OrgID string }

func (createCmd) CommandMarker() {}

type closeCmd struct{ CaseID string }

func (closeCmd) CommandMarker() {}

type dashQuery struct {
	OrgID string
	Page  int
}

func (dashQuery) QueryMarker() {}

type caseEvent struct{ CaseID string }

func (caseEvent) NotificationMarker() {}

type caseResource struct{ ID, Status string }
type caseSummary struct{ ID string }

// fakeT captures failures without stopping test execution.
// Satisfies mockrelay.TB so we can verify assertion failure behavior.
type fakeT struct{ failed bool }

func (f *fakeT) Helper()                   {}
func (f *fakeT) Errorf(_ string, _ ...any) { f.failed = true }

// ---------------------------------------------------------------------------
// OnDispatch
// ---------------------------------------------------------------------------

func TestOnDispatch_ReturnsFixedResult(t *testing.T) {
	mr := mockrelay.New()
	want := caseResource{ID: "c-1", Status: "open"}

	mockrelay.OnDispatch(mr, createCmd{OrgID: "org-1"}, want, nil)

	got, err := relay.Dispatch[caseResource](context.Background(), mr, createCmd{OrgID: "org-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("want %+v, got %+v", want, got)
	}
	mr.AssertExpectations(t)
}

func TestOnDispatch_NoExpectation_ReturnsDescriptiveError(t *testing.T) {
	mr := mockrelay.New()
	_, err := relay.Dispatch[caseResource](context.Background(), mr, createCmd{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "createCmd") {
		t.Errorf("error should mention command type, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// OnDispatchFn
// ---------------------------------------------------------------------------

func TestOnDispatchFn_DynamicResponse(t *testing.T) {
	mr := mockrelay.New()

	mockrelay.OnDispatchFn(mr, func(_ context.Context, cmd createCmd) (caseResource, error) {
		if cmd.OrgID == "vip" {
			return caseResource{ID: "c-vip", Status: "priority"}, nil
		}
		return caseResource{ID: "c-std", Status: "open"}, nil
	})

	r1, _ := relay.Dispatch[caseResource](context.Background(), mr, createCmd{OrgID: "vip"})
	if r1.Status != "priority" {
		t.Errorf("want priority, got %s", r1.Status)
	}

	r2, _ := relay.Dispatch[caseResource](context.Background(), mr, createCmd{OrgID: "other"})
	if r2.Status != "open" {
		t.Errorf("want open, got %s", r2.Status)
	}

	mr.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// OnDispatchError
// ---------------------------------------------------------------------------

func TestOnDispatchError_ReturnsError(t *testing.T) {
	mr := mockrelay.New()
	sentinel := errors.New("db down")

	mockrelay.OnDispatchError[createCmd](mr, sentinel)

	_, err := relay.Dispatch[caseResource](context.Background(), mr, createCmd{})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
	mr.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// OnAsk
// ---------------------------------------------------------------------------

func TestOnAsk_ReturnsFixedResult(t *testing.T) {
	mr := mockrelay.New()
	want := []caseSummary{{ID: "c-1"}, {ID: "c-2"}}

	mockrelay.OnAsk(mr, dashQuery{OrgID: "org-1", Page: 1}, want, nil)

	got, err := relay.Ask[[]caseSummary](context.Background(), mr, dashQuery{OrgID: "org-1", Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Errorf("want %d results, got %d", len(want), len(got))
	}
	mr.AssertExpectations(t)
}

func TestOnAsk_NoExpectation_ReturnsDescriptiveError(t *testing.T) {
	mr := mockrelay.New()
	_, err := relay.Ask[[]caseSummary](context.Background(), mr, dashQuery{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "dashQuery") {
		t.Errorf("error should mention query type, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// OnAskFn
// ---------------------------------------------------------------------------

func TestOnAskFn_DynamicResponse(t *testing.T) {
	mr := mockrelay.New()

	mockrelay.OnAskFn(mr, func(_ context.Context, q dashQuery) ([]caseSummary, error) {
		if q.Page < 1 {
			return nil, errors.New("invalid page")
		}
		return []caseSummary{{ID: "c-1"}}, nil
	})

	_, err := relay.Ask[[]caseSummary](context.Background(), mr, dashQuery{OrgID: "o", Page: 0})
	if err == nil {
		t.Fatal("expected error for page < 1")
	}

	res, err := relay.Ask[[]caseSummary](context.Background(), mr, dashQuery{OrgID: "o", Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("want 1 result, got %d", len(res))
	}

	mr.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// OnAskError
// ---------------------------------------------------------------------------

func TestOnAskError_ReturnsError(t *testing.T) {
	mr := mockrelay.New()
	sentinel := errors.New("db timeout")

	mockrelay.OnAskError[dashQuery](mr, sentinel)

	_, err := relay.Ask[[]caseSummary](context.Background(), mr, dashQuery{})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
	mr.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// OnPublish
// ---------------------------------------------------------------------------

func TestOnPublish_Success(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnPublish[caseEvent](mr, nil)
	if err := relay.Publish(context.Background(), mr, caseEvent{CaseID: "c-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOnPublish_Error(t *testing.T) {
	mr := mockrelay.New()
	sentinel := errors.New("publish failed")
	mockrelay.OnPublish[caseEvent](mr, sentinel)
	if err := relay.Publish(context.Background(), mr, caseEvent{}); !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
}

func TestPublish_UnregisteredNotification_IsIgnored(t *testing.T) {
	mr := mockrelay.New()
	if err := relay.Publish(context.Background(), mr, caseEvent{}); err != nil {
		t.Errorf("expected nil for unregistered notification, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AssertExpectations
// ---------------------------------------------------------------------------

func TestAssertExpectations_FailsForUncalledExpectation(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatch(mr, createCmd{OrgID: "org-1"}, caseResource{}, nil)

	ft := &fakeT{}
	mr.AssertExpectations(ft)
	if !ft.failed {
		t.Error("AssertExpectations should fail when expectation is unmet")
	}
}

func TestAssertExpectations_PassesWhenAllMet(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatch(mr, createCmd{OrgID: "org-1"}, caseResource{ID: "c-1"}, nil)
	mockrelay.OnAsk(mr, dashQuery{OrgID: "org-1", Page: 1}, []caseSummary{}, nil)

	_, _ = relay.Dispatch[caseResource](context.Background(), mr, createCmd{OrgID: "org-1"})
	_, _ = relay.Ask[[]caseSummary](context.Background(), mr, dashQuery{OrgID: "org-1", Page: 1})

	mr.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// AssertNotCalled
// ---------------------------------------------------------------------------

func TestAssertNotCalled_PassesWhenNeverDispatched(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.AssertNotCalled[closeCmd](t, mr)
}

func TestAssertNotCalled_FailsWhenWasDispatched(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatch(mr, closeCmd{CaseID: "c-1"}, caseResource{}, nil)
	_, _ = relay.Dispatch[caseResource](context.Background(), mr, closeCmd{CaseID: "c-1"})

	ft := &fakeT{}
	mockrelay.AssertNotCalled[closeCmd](ft, mr)
	if !ft.failed {
		t.Error("AssertNotCalled should fail when the command was dispatched")
	}
}

func TestAssertNotCalled_FailsEvenWithNoExpectation(t *testing.T) {
	mr := mockrelay.New()
	// No expectation set — dispatch returns error but the attempt is still tracked.
	_, _ = relay.Ask[[]caseSummary](context.Background(), mr, dashQuery{OrgID: "x", Page: 1})

	ft := &fakeT{}
	mockrelay.AssertNotCalled[dashQuery](ft, mr)
	if !ft.failed {
		t.Error("AssertNotCalled should fail — query was attempted")
	}
}

// ---------------------------------------------------------------------------
// Multiple command types
// ---------------------------------------------------------------------------

func TestMockRelay_MultipleCommandTypes(t *testing.T) {
	mr := mockrelay.New()

	mockrelay.OnDispatch(mr, createCmd{OrgID: "org-1"}, caseResource{ID: "c-1"}, nil)
	mockrelay.OnDispatch(mr, closeCmd{CaseID: "c-1"}, caseResource{ID: "c-1", Status: "closed"}, nil)

	r1, err := relay.Dispatch[caseResource](context.Background(), mr, createCmd{OrgID: "org-1"})
	if err != nil || r1.ID != "c-1" {
		t.Errorf("unexpected: err=%v id=%s", err, r1.ID)
	}

	r2, err := relay.Dispatch[caseResource](context.Background(), mr, closeCmd{CaseID: "c-1"})
	if err != nil || r2.Status != "closed" {
		t.Errorf("unexpected: err=%v status=%s", err, r2.Status)
	}

	mr.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// Verify relay.Relay is the only injection needed
// ---------------------------------------------------------------------------

func TestMockRelay_SatisfiesRelayInterface(t *testing.T) {
	var r relay.Relay = mockrelay.New()
	_ = r
}
