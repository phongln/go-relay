// Package mockrelay provides a [relay.Relay] test double with typed helpers.
//
// It requires no knowledge of relay internals — no method name strings,
// no type assertions, no testify in test files.
//
// # Basic usage
//
//	func TestCreate(t *testing.T) {
//	    mr := mockrelay.New()
//
//	    mockrelay.OnDispatch(mr,
//	        commands.CreateCaseCmd{OrgID: "org-1", PlayerID: "p-1"},
//	        resources.CaseResource{ID: "case-123", Status: "open"},
//	        nil,
//	    )
//
//	    ctrl := controller.New(mr)
//	    // ... call ctrl and assert the HTTP response
//	    mr.AssertExpectations(t)
//	}
//
// # Error path
//
//	mockrelay.OnDispatchError[commands.CreateCaseCmd](mr, errors.New("db down"))
//
// # Dynamic response
//
//	mockrelay.OnDispatchFn(mr, func(ctx context.Context, cmd commands.CreateCaseCmd) (resources.CaseResource, error) {
//	    if cmd.RiskScore > 90 {
//	        return resources.CaseResource{Status: "high-risk"}, nil
//	    }
//	    return resources.CaseResource{Status: "open"}, nil
//	})
package mockrelay

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/phongln/go-relay/relay"
)

// TB is the subset of [testing.TB] used by mockrelay assertions.
// *testing.T and *testing.B both satisfy this interface automatically.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
}

// ---------------------------------------------------------------------------
// MockRelay
// ---------------------------------------------------------------------------

// MockRelay is a test double for [relay.Relay].
// Create with [New], register expectations with the typed helpers,
// inject into the component under test, then call [MockRelay.AssertExpectations].
type MockRelay struct {
	mu           sync.Mutex
	commands     map[reflect.Type]func(context.Context, any) (any, error)
	queries      map[reflect.Type]func(context.Context, any) (any, error)
	notifs       map[reflect.Type]func(context.Context, any) error
	expectations []expectation
	actuals      []string // call attempts with no matching expectation
}

type expectation struct {
	requestType string
	called      bool
}

// New returns a ready-to-use MockRelay.
func New() *MockRelay {
	return &MockRelay{
		commands: make(map[reflect.Type]func(context.Context, any) (any, error)),
		queries:  make(map[reflect.Type]func(context.Context, any) (any, error)),
		notifs:   make(map[reflect.Type]func(context.Context, any) error),
	}
}

// ---------------------------------------------------------------------------
// relay.Relay interface
// ---------------------------------------------------------------------------

// Dispatch implements [relay.Relay].
func (m *MockRelay) Dispatch(ctx context.Context, cmd relay.Command) (any, error) {
	m.mu.Lock()
	h, ok := m.commands[reflect.TypeOf(cmd)]
	m.recordCall(fmt.Sprintf("%T", cmd))
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf(
			"mockrelay: no expectation for command %T\n"+
				"  → call mockrelay.OnDispatch, OnDispatchFn, or OnDispatchError first",
			cmd,
		)
	}
	return h(ctx, cmd)
}

// Ask implements [relay.Relay].
func (m *MockRelay) Ask(ctx context.Context, qry relay.Query) (any, error) {
	m.mu.Lock()
	h, ok := m.queries[reflect.TypeOf(qry)]
	m.recordCall(fmt.Sprintf("%T", qry))
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf(
			"mockrelay: no expectation for query %T\n"+
				"  → call mockrelay.OnAsk, OnAskFn, or OnAskError first",
			qry,
		)
	}
	return h(ctx, qry)
}

// Publish implements [relay.Relay].
func (m *MockRelay) Publish(ctx context.Context, n relay.Notification) error {
	m.mu.Lock()
	h, ok := m.notifs[reflect.TypeOf(n)]
	m.mu.Unlock()

	if !ok {
		return nil // unregistered notifications are silently ignored
	}
	return h(ctx, n)
}

func (m *MockRelay) recordCall(requestType string) {
	for i, e := range m.expectations {
		if e.requestType == requestType && !e.called {
			m.expectations[i].called = true
			return
		}
	}
	m.actuals = append(m.actuals, requestType)
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

// AssertExpectations verifies all registered expectations were fulfilled.
// Call at the end of every test, or use defer:
//
//	defer mr.AssertExpectations(t)
func (m *MockRelay) AssertExpectations(t TB) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.expectations {
		if !e.called {
			t.Errorf("mockrelay: expected %s to be dispatched, but it was not", e.requestType)
		}
	}
}

// AssertNotCalled verifies that a command or query was never dispatched.
//
//	mockrelay.AssertNotCalled[commands.CreateCaseCmd](t, mr)
func AssertNotCalled[T any](t TB, m *MockRelay) {
	t.Helper()
	var zero T
	want := fmt.Sprintf("%T", zero)

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, actual := range m.actuals {
		if actual == want {
			t.Errorf("mockrelay: %s was dispatched but was expected NOT to be", want)
			return
		}
	}
	for _, e := range m.expectations {
		if e.requestType == want && e.called {
			t.Errorf("mockrelay: %s was dispatched but was expected NOT to be", want)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Command helpers
// ---------------------------------------------------------------------------

// OnDispatch sets up an expectation: when cmd is dispatched, return returns and err.
//
//	mockrelay.OnDispatch(mr,
//	    commands.CreateCaseCmd{OrgID: "org-1", PlayerID: "p-1"},
//	    resources.CaseResource{ID: "case-123"},
//	    nil,
//	)
func OnDispatch[C relay.Command, R any](m *MockRelay, cmd C, returns R, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands[reflect.TypeOf(cmd)] = func(_ context.Context, _ any) (any, error) {
		return returns, err
	}
	m.expectations = append(m.expectations, expectation{requestType: fmt.Sprintf("%T", cmd)})
}

// OnDispatchFn sets up a dynamic expectation for command type C.
//
//	mockrelay.OnDispatchFn(mr, func(ctx context.Context, cmd commands.CreateCaseCmd) (resources.CaseResource, error) {
//	    if cmd.RiskScore > 90 { return resources.CaseResource{Status: "high-risk"}, nil }
//	    return resources.CaseResource{Status: "open"}, nil
//	})
func OnDispatchFn[C relay.Command, R any](m *MockRelay, fn func(context.Context, C) (R, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var zero C
	m.commands[reflect.TypeOf(zero)] = func(ctx context.Context, raw any) (any, error) {
		return fn(ctx, raw.(C))
	}
	m.expectations = append(m.expectations, expectation{requestType: fmt.Sprintf("%T", zero)})
}

// OnDispatchError sets up an error expectation for command type C.
//
//	mockrelay.OnDispatchError[commands.CreateCaseCmd](mr, errors.New("db unavailable"))
func OnDispatchError[C relay.Command](m *MockRelay, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var zero C
	m.commands[reflect.TypeOf(zero)] = func(_ context.Context, _ any) (any, error) {
		return nil, err
	}
	m.expectations = append(m.expectations, expectation{requestType: fmt.Sprintf("%T", zero)})
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// OnAsk sets up an expectation: when qry is asked, return returns and err.
//
//	mockrelay.OnAsk(mr,
//	    queries.GetDashboardQuery{OrgID: "org-1", Page: 1},
//	    []resources.CaseSummary{{ID: "c-1"}},
//	    nil,
//	)
func OnAsk[Q relay.Query, R any](m *MockRelay, qry Q, returns R, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries[reflect.TypeOf(qry)] = func(_ context.Context, _ any) (any, error) {
		return returns, err
	}
	m.expectations = append(m.expectations, expectation{requestType: fmt.Sprintf("%T", qry)})
}

// OnAskFn sets up a dynamic expectation for query type Q.
//
//	mockrelay.OnAskFn(mr, func(ctx context.Context, q queries.GetDashboardQuery) ([]resources.CaseSummary, error) {
//	    if q.Page < 1 { return nil, errors.New("invalid page") }
//	    return []resources.CaseSummary{{ID: "c-1"}}, nil
//	})
func OnAskFn[Q relay.Query, R any](m *MockRelay, fn func(context.Context, Q) (R, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var zero Q
	m.queries[reflect.TypeOf(zero)] = func(ctx context.Context, raw any) (any, error) {
		return fn(ctx, raw.(Q))
	}
	m.expectations = append(m.expectations, expectation{requestType: fmt.Sprintf("%T", zero)})
}

// OnAskError sets up an error expectation for query type Q.
//
//	mockrelay.OnAskError[queries.GetDashboardQuery](mr, errors.New("db timeout"))
func OnAskError[Q relay.Query](m *MockRelay, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var zero Q
	m.queries[reflect.TypeOf(zero)] = func(_ context.Context, _ any) (any, error) {
		return nil, err
	}
	m.expectations = append(m.expectations, expectation{requestType: fmt.Sprintf("%T", zero)})
}

// ---------------------------------------------------------------------------
// Notification helpers
// ---------------------------------------------------------------------------

// OnPublish sets up a response for notification type N.
// Unregistered notifications are silently ignored in tests.
//
//	mockrelay.OnPublish[events.CaseCreatedEvent](mr, nil)
func OnPublish[N relay.Notification](m *MockRelay, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var zero N
	m.notifs[reflect.TypeOf(zero)] = func(_ context.Context, _ any) error {
		return err
	}
}
