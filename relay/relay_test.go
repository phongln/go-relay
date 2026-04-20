package relay_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/phongln/go-relay/relay"
)

// ---------------------------------------------------------------------------
// Test domain types
// ---------------------------------------------------------------------------

type createCmd struct{ OrgID string }

func (createCmd) CommandMarker() {}

type txCmd struct{ OrgID string }

func (txCmd) CommandMarker()   {}
func (txCmd) WithTransaction() {}

type failCmd struct{}

func (failCmd) CommandMarker() {}

type dashQuery struct{ OrgID string }

func (dashQuery) QueryMarker() {}

type caseEvent struct{ CaseID string }

func (caseEvent) NotificationMarker() {}

type cmdResult struct{ ID string }
type queryResult struct{ OrgID string }

// validatableCmd has a Validate() method for ValidationBehavior tests.
type validatableCmd struct{ OrgID string }

func (validatableCmd) CommandMarker() {}

func (c validatableCmd) Validate() error {
	if c.OrgID == "" {
		return errors.New("org_id is required")
	}
	return nil
}

type validatableResult struct{ ID string }

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

type createHandler struct {
	mu     sync.Mutex
	called bool
}

func (h *createHandler) Handle(_ context.Context, cmd createCmd) (cmdResult, error) {
	h.mu.Lock()
	h.called = true
	h.mu.Unlock()
	return cmdResult{ID: "case-" + cmd.OrgID}, nil
}

func (h *createHandler) wasCalled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.called
}

type failHandler struct{}

func (h *failHandler) Handle(_ context.Context, _ failCmd) (cmdResult, error) {
	return cmdResult{}, errors.New("handler error")
}

type txHandler struct{ called bool }

func (h *txHandler) Handle(_ context.Context, _ txCmd) (cmdResult, error) {
	h.called = true
	return cmdResult{ID: "tx-result"}, nil
}

type txFailHandler struct{ err error }

func (h *txFailHandler) Handle(_ context.Context, _ txCmd) (cmdResult, error) {
	return cmdResult{}, h.err
}

type dashHandler struct{}

func (h *dashHandler) Handle(_ context.Context, q dashQuery) (queryResult, error) {
	return queryResult(q), nil
}

type notifHandler struct {
	mu     sync.Mutex
	called bool
	err    error
}

func (h *notifHandler) Handle(_ context.Context, _ caseEvent) error {
	h.mu.Lock()
	h.called = true
	h.mu.Unlock()
	return h.err
}

func (h *notifHandler) wasCalled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.called
}

type validatableHandler struct{ called bool }

func (h *validatableHandler) Handle(_ context.Context, cmd validatableCmd) (validatableResult, error) {
	h.called = true
	return validatableResult{ID: cmd.OrgID}, nil
}

// ---------------------------------------------------------------------------
// Pipeline test helpers
// ---------------------------------------------------------------------------

type orderBehavior struct {
	label string
	log   *[]string
	mu    sync.Mutex
}

func (o *orderBehavior) Handle(ctx context.Context, req any, next relay.RequestHandlerFunc) (any, error) {
	o.mu.Lock()
	*o.log = append(*o.log, o.label+"-in")
	o.mu.Unlock()
	res, err := next(ctx)
	o.mu.Lock()
	*o.log = append(*o.log, o.label+"-out")
	o.mu.Unlock()
	return res, err
}

type blockBehavior struct{ err error }

func (b *blockBehavior) Handle(_ context.Context, _ any, _ relay.RequestHandlerFunc) (any, error) {
	return nil, b.err
}

type validationBehavior struct{}

func (v *validationBehavior) Handle(ctx context.Context, req any, next relay.RequestHandlerFunc) (any, error) {
	type validatable interface{ Validate() error }
	if val, ok := req.(validatable); ok {
		if err := val.Validate(); err != nil {
			return nil, err
		}
	}
	return next(ctx)
}

// ---------------------------------------------------------------------------
// Transactor mock
// ---------------------------------------------------------------------------

type mockTx struct {
	called bool
	fn     func(context.Context, relay.TxFunc) error
}

func (m *mockTx) WithTransaction(ctx context.Context, fn relay.TxFunc) error {
	m.called = true
	if m.fn != nil {
		return m.fn(ctx, fn)
	}
	return fn(ctx)
}

// ---------------------------------------------------------------------------
// Dispatch — success
// ---------------------------------------------------------------------------

func TestDispatch_Success(t *testing.T) {
	r := relay.New()
	h := &createHandler{}
	relay.RegisterCommand(r, h)

	res, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "org-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != "case-org-1" {
		t.Errorf("want case-org-1, got %s", res.ID)
	}
	if !h.wasCalled() {
		t.Error("handler was not called")
	}
}

func TestDispatch_HandlerError_WrappedAsHandlerError(t *testing.T) {
	r := relay.New()
	relay.RegisterCommand(r, &failHandler{})

	_, err := relay.Dispatch[cmdResult](context.Background(), r, failCmd{})
	if err == nil {
		t.Fatal("expected error")
	}

	var he *relay.HandlerError
	if !errors.As(err, &he) {
		t.Errorf("expected *relay.HandlerError, got %T: %v", err, err)
	}
	if he.RequestType != "relay_test.failCmd" {
		t.Errorf("unexpected RequestType: %s", he.RequestType)
	}
}

// ---------------------------------------------------------------------------
// Dispatch — not found
// ---------------------------------------------------------------------------

func TestDispatch_HandlerNotFound(t *testing.T) {
	r := relay.New()
	_, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{})
	if !errors.Is(err, relay.ErrHandlerNotFound) {
		t.Errorf("expected ErrHandlerNotFound, got %v", err)
	}
}

func TestDispatch_HandlerNotFound_WrappedInHandlerError(t *testing.T) {
	r := relay.New()
	_, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{})

	var he *relay.HandlerError
	if !errors.As(err, &he) {
		t.Fatalf("expected *relay.HandlerError, got %T", err)
	}
	if !errors.Is(err, relay.ErrHandlerNotFound) {
		t.Error("ErrHandlerNotFound should unwrap from HandlerError")
	}
}

// ---------------------------------------------------------------------------
// Dispatch — context cancellation
// ---------------------------------------------------------------------------

func TestDispatch_ContextAlreadyCancelled(t *testing.T) {
	r := relay.New()
	relay.RegisterCommand(r, &createHandler{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := relay.Dispatch[cmdResult](ctx, r, createCmd{OrgID: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Transactional commands
// ---------------------------------------------------------------------------

func TestDispatch_Transactional_NoTransactorConfigured(t *testing.T) {
	r := relay.New()
	relay.RegisterCommand(r, &txHandler{})

	_, err := relay.Dispatch[cmdResult](context.Background(), r, txCmd{OrgID: "x"})
	if !errors.Is(err, relay.ErrTransactorNotSet) {
		t.Errorf("expected ErrTransactorNotSet, got %v", err)
	}
}

func TestDispatch_Transactional_TransactorAndHandlerBothCalled(t *testing.T) {
	r := relay.New()
	tx := &mockTx{}
	r.WithTransactor(tx)

	h := &txHandler{}
	relay.RegisterCommand(r, h)

	res, err := relay.Dispatch[cmdResult](context.Background(), r, txCmd{OrgID: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.called {
		t.Error("transactor was not called")
	}
	if !h.called {
		t.Error("handler was not called")
	}
	if res.ID != "tx-result" {
		t.Errorf("unexpected result ID: %s", res.ID)
	}
}

func TestDispatch_Transactional_HandlerErrorPropagated(t *testing.T) {
	r := relay.New()
	sentinel := errors.New("handler failed inside tx")

	var receivedErr error
	r.WithTransactor(&mockTx{
		fn: func(ctx context.Context, fn relay.TxFunc) error {
			receivedErr = fn(ctx)
			return receivedErr
		},
	})
	relay.RegisterCommand(r, &txFailHandler{err: sentinel})

	_, err := relay.Dispatch[cmdResult](context.Background(), r, txCmd{OrgID: "x"})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	if !errors.Is(receivedErr, sentinel) {
		t.Error("transactor did not receive handler error")
	}
}

// ---------------------------------------------------------------------------
// Ask
// ---------------------------------------------------------------------------

func TestAsk_Success(t *testing.T) {
	r := relay.New()
	relay.RegisterQuery(r, &dashHandler{})

	res, err := relay.Ask[queryResult](context.Background(), r, dashQuery{OrgID: "org-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrgID != "org-1" {
		t.Errorf("want org-1, got %s", res.OrgID)
	}
}

func TestAsk_HandlerNotFound(t *testing.T) {
	r := relay.New()
	_, err := relay.Ask[queryResult](context.Background(), r, dashQuery{})
	if !errors.Is(err, relay.ErrHandlerNotFound) {
		t.Errorf("expected ErrHandlerNotFound, got %v", err)
	}
}

func TestAsk_ContextAlreadyCancelled(t *testing.T) {
	r := relay.New()
	relay.RegisterQuery(r, &dashHandler{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := relay.Ask[queryResult](ctx, r, dashQuery{OrgID: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Publish
// ---------------------------------------------------------------------------

func TestPublish_SingleHandler(t *testing.T) {
	r := relay.New()
	h := &notifHandler{}
	relay.RegisterNotificationHandler(r, h)

	if err := relay.Publish(context.Background(), r, caseEvent{CaseID: "c-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h.wasCalled() {
		t.Error("handler was not called")
	}
}

func TestPublish_MultipleHandlers_AllCalled(t *testing.T) {
	r := relay.New()
	handlers := []*notifHandler{{}, {}, {}}
	for _, h := range handlers {
		relay.RegisterNotificationHandler(r, h)
	}

	if err := relay.Publish(context.Background(), r, caseEvent{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, h := range handlers {
		if !h.wasCalled() {
			t.Errorf("handler %d was not called", i+1)
		}
	}
}

func TestPublish_NoHandlers_ReturnsNil(t *testing.T) {
	r := relay.New()
	if err := relay.Publish(context.Background(), r, caseEvent{}); err != nil {
		t.Errorf("expected nil for unregistered notification, got %v", err)
	}
}

func TestPublish_OneFailHandler_OthersStillRun(t *testing.T) {
	r := relay.New()
	fail := &notifHandler{err: errors.New("handler-1 failed")}
	ok := &notifHandler{}
	relay.RegisterNotificationHandler(r, fail)
	relay.RegisterNotificationHandler(r, ok)

	err := relay.Publish(context.Background(), r, caseEvent{})
	if err == nil {
		t.Error("expected joined error")
	}
	if !ok.wasCalled() {
		t.Error("second handler should still run when first fails")
	}
}

func TestPublish_ContextAlreadyCancelled(t *testing.T) {
	r := relay.New()
	relay.RegisterNotificationHandler(r, &notifHandler{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := relay.Publish(ctx, r, caseEvent{}); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

func TestPipeline_ExecutionOrder_OutermostFirst(t *testing.T) {
	r := relay.New()
	var log []string
	r.AddPipeline(&orderBehavior{label: "A", log: &log})
	r.AddPipeline(&orderBehavior{label: "B", log: &log})
	r.AddPipeline(&orderBehavior{label: "C", log: &log})
	relay.RegisterCommand(r, &createHandler{})

	if _, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "x"}); err != nil {
		t.Fatal(err)
	}

	want := []string{"A-in", "B-in", "C-in", "C-out", "B-out", "A-out"}
	if len(log) != len(want) {
		t.Fatalf("want %d entries, got %d: %v", len(want), len(log), log)
	}
	for i, w := range want {
		if log[i] != w {
			t.Errorf("[%d] want %q, got %q", i, w, log[i])
		}
	}
}

func TestPipeline_BlockBehavior_PreventsHandler(t *testing.T) {
	r := relay.New()
	sentinel := errors.New("blocked")
	r.AddPipeline(&blockBehavior{err: sentinel})
	h := &createHandler{}
	relay.RegisterCommand(r, h)

	_, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
	if h.wasCalled() {
		t.Error("handler should not have been called")
	}
}

func TestPipeline_ValidationBehavior_BlocksInvalidRequest(t *testing.T) {
	r := relay.New()
	r.AddPipeline(&validationBehavior{})
	h := &validatableHandler{}
	relay.RegisterCommand(r, h)

	_, err := relay.Dispatch[validatableResult](context.Background(), r, validatableCmd{OrgID: ""})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if h.called {
		t.Error("handler should not be called when validation fails")
	}
}

func TestPipeline_ValidationBehavior_PassesValidRequest(t *testing.T) {
	r := relay.New()
	r.AddPipeline(&validationBehavior{})
	h := &validatableHandler{}
	relay.RegisterCommand(r, h)

	res, err := relay.Dispatch[validatableResult](context.Background(), r, validatableCmd{OrgID: "org-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != "org-1" {
		t.Errorf("want org-1, got %s", res.ID)
	}
	if !h.called {
		t.Error("handler should have been called")
	}
}

func TestPipeline_ValidationBehavior_SkipsNonValidatable(t *testing.T) {
	r := relay.New()
	r.AddPipeline(&validationBehavior{})
	h := &createHandler{}
	relay.RegisterCommand(r, h)

	res, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != "case-x" {
		t.Errorf("want case-x, got %s", res.ID)
	}
}

// ---------------------------------------------------------------------------
// AssertAllRegistered
// ---------------------------------------------------------------------------

func TestAssertAllRegistered_PanicsOnMissingCommand(t *testing.T) {
	r := relay.New()
	defer func() {
		if recover() == nil {
			t.Error("expected panic for missing command handler")
		}
	}()
	r.AssertAllRegistered([]relay.Command{createCmd{}}, nil)
}

func TestAssertAllRegistered_PanicsOnMissingQuery(t *testing.T) {
	r := relay.New()
	defer func() {
		if recover() == nil {
			t.Error("expected panic for missing query handler")
		}
	}()
	r.AssertAllRegistered(nil, []relay.Query{dashQuery{}})
}

func TestAssertAllRegistered_NoPanicWhenAllPresent(t *testing.T) {
	r := relay.New()
	relay.RegisterCommand(r, &createHandler{})
	relay.RegisterQuery(r, &dashHandler{})
	r.AssertAllRegistered([]relay.Command{createCmd{}}, []relay.Query{dashQuery{}})
}

// ---------------------------------------------------------------------------
// Method chaining
// ---------------------------------------------------------------------------

func TestRealRelay_MethodChaining(t *testing.T) {
	tx := &mockTx{}
	r := relay.New().
		WithTransactor(tx).
		AddPipeline(&orderBehavior{label: "x", log: new([]string)})

	relay.RegisterCommand(r, &createHandler{})
	res, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "chain"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != "case-chain" {
		t.Errorf("unexpected ID: %s", res.ID)
	}
}

// ---------------------------------------------------------------------------
// Pipeline — context propagation
// ---------------------------------------------------------------------------

type ctxKey string

type ctxEnrichBehavior struct {
	key ctxKey
	val string
}

func (b *ctxEnrichBehavior) Handle(ctx context.Context, req any, next relay.RequestHandlerFunc) (any, error) {
	return next(context.WithValue(ctx, b.key, b.val))
}

type ctxReadHandler struct {
	mu       sync.Mutex
	captured string
	key      ctxKey
}

func (h *ctxReadHandler) Handle(ctx context.Context, _ createCmd) (cmdResult, error) {
	h.mu.Lock()
	if v, ok := ctx.Value(h.key).(string); ok {
		h.captured = v
	}
	h.mu.Unlock()
	return cmdResult{ID: "ctx-test"}, nil
}

func TestPipeline_ContextPropagation_BehaviorToHandler(t *testing.T) {
	r := relay.New()
	key := ctxKey("trace-id")
	r.AddPipeline(&ctxEnrichBehavior{key: key, val: "abc-123"})
	h := &ctxReadHandler{key: key}
	relay.RegisterCommand(r, h)

	_, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.captured != "abc-123" {
		t.Errorf("handler did not receive enriched context: got %q", h.captured)
	}
}

func TestPipeline_ContextPropagation_BehaviorToBehavior(t *testing.T) {
	r := relay.New()
	key := ctxKey("span")
	r.AddPipeline(&ctxEnrichBehavior{key: key, val: "span-456"})

	var captured string
	r.AddPipeline(&ctxCaptureBehavior{key: key, captured: &captured})
	relay.RegisterCommand(r, &createHandler{})

	_, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != "span-456" {
		t.Errorf("downstream behavior did not receive enriched context: got %q", captured)
	}
}

type ctxCaptureBehavior struct {
	key      ctxKey
	captured *string
}

func (b *ctxCaptureBehavior) Handle(ctx context.Context, req any, next relay.RequestHandlerFunc) (any, error) {
	if v, ok := ctx.Value(b.key).(string); ok {
		*b.captured = v
	}
	return next(ctx)
}

// ---------------------------------------------------------------------------
// Duplicate registration
// ---------------------------------------------------------------------------

func TestRegisterCommand_PanicsOnDuplicate(t *testing.T) {
	r := relay.New()
	relay.RegisterCommand(r, &createHandler{})
	defer func() {
		if recover() == nil {
			t.Error("expected panic for duplicate command registration")
		}
	}()
	relay.RegisterCommand(r, &createHandler{})
}

func TestRegisterQuery_PanicsOnDuplicate(t *testing.T) {
	r := relay.New()
	relay.RegisterQuery(r, &dashHandler{})
	defer func() {
		if recover() == nil {
			t.Error("expected panic for duplicate query registration")
		}
	}()
	relay.RegisterQuery(r, &dashHandler{})
}

// ---------------------------------------------------------------------------
// Concurrency — run with go test -race
// ---------------------------------------------------------------------------

func TestConcurrentDispatchAndAsk_NoRace(t *testing.T) {
	r := relay.New()
	relay.RegisterCommand(r, &createHandler{})
	relay.RegisterQuery(r, &dashHandler{})

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "org"})
		}()
		go func() {
			defer wg.Done()
			_, _ = relay.Ask[queryResult](context.Background(), r, dashQuery{OrgID: "org"})
		}()
	}
	wg.Wait()
}

func TestConcurrentPublish_NoRace(t *testing.T) {
	r := relay.New()
	relay.RegisterNotificationHandler(r, &notifHandler{})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = relay.Publish(context.Background(), r, caseEvent{})
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Factory registration
// ---------------------------------------------------------------------------

type factoryCmdHandler struct {
	instanceID int
}

var factoryCallCount int
var factoryMu sync.Mutex

func (h *factoryCmdHandler) Handle(_ context.Context, cmd createCmd) (cmdResult, error) {
	return cmdResult{ID: fmt.Sprintf("case-%s-%d", cmd.OrgID, h.instanceID)}, nil
}

func TestRegisterCommandFactory_CreatesNewHandlerPerRequest(t *testing.T) {
	r := relay.New()
	factoryMu.Lock()
	factoryCallCount = 0
	factoryMu.Unlock()

	relay.RegisterCommandFactory(r, func() relay.CommandHandler[createCmd, cmdResult] {
		factoryMu.Lock()
		factoryCallCount++
		id := factoryCallCount
		factoryMu.Unlock()
		return &factoryCmdHandler{instanceID: id}
	})

	res1, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "org-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res1.ID != "case-org-1-1" {
		t.Errorf("want case-org-1-1, got %s", res1.ID)
	}

	res2, err := relay.Dispatch[cmdResult](context.Background(), r, createCmd{OrgID: "org-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.ID != "case-org-2-2" {
		t.Errorf("want case-org-2-2, got %s", res2.ID)
	}

	factoryMu.Lock()
	count := factoryCallCount
	factoryMu.Unlock()
	if count != 2 {
		t.Errorf("factory should have been called 2 times, got %d", count)
	}
}

func TestRegisterCommandFactory_PanicsOnDuplicate(t *testing.T) {
	r := relay.New()
	relay.RegisterCommandFactory(r, func() relay.CommandHandler[createCmd, cmdResult] {
		return &createHandler{}
	})
	defer func() {
		if recover() == nil {
			t.Error("expected panic for duplicate factory registration")
		}
	}()
	relay.RegisterCommandFactory(r, func() relay.CommandHandler[createCmd, cmdResult] {
		return &createHandler{}
	})
}

func TestRegisterCommandFactory_ConflictsWithRegisterCommand(t *testing.T) {
	r := relay.New()
	relay.RegisterCommand(r, &createHandler{})
	defer func() {
		if recover() == nil {
			t.Error("expected panic: factory and instance registration conflict")
		}
	}()
	relay.RegisterCommandFactory(r, func() relay.CommandHandler[createCmd, cmdResult] {
		return &createHandler{}
	})
}

type factoryQueryHandler struct{ instanceID int }

func (h *factoryQueryHandler) Handle(_ context.Context, q dashQuery) (queryResult, error) {
	return queryResult{OrgID: fmt.Sprintf("%s-%d", q.OrgID, h.instanceID)}, nil
}

func TestRegisterQueryFactory_CreatesNewHandlerPerRequest(t *testing.T) {
	r := relay.New()
	var count int
	relay.RegisterQueryFactory(r, func() relay.QueryHandler[dashQuery, queryResult] {
		count++
		return &factoryQueryHandler{instanceID: count}
	})

	res1, err := relay.Ask[queryResult](context.Background(), r, dashQuery{OrgID: "org"})
	if err != nil {
		t.Fatal(err)
	}
	if res1.OrgID != "org-1" {
		t.Errorf("want org-1, got %s", res1.OrgID)
	}

	res2, err := relay.Ask[queryResult](context.Background(), r, dashQuery{OrgID: "org"})
	if err != nil {
		t.Fatal(err)
	}
	if res2.OrgID != "org-2" {
		t.Errorf("want org-2, got %s", res2.OrgID)
	}
}

type factoryNotifHandler struct {
	mu     sync.Mutex
	called bool
}

func (h *factoryNotifHandler) Handle(_ context.Context, _ caseEvent) error {
	h.mu.Lock()
	h.called = true
	h.mu.Unlock()
	return nil
}

func TestRegisterNotificationHandlerFactory_CreatesNewHandlerPerPublish(t *testing.T) {
	r := relay.New()
	var instances []*factoryNotifHandler
	var mu sync.Mutex

	relay.RegisterNotificationHandlerFactory(r, func() relay.NotificationHandler[caseEvent] {
		h := &factoryNotifHandler{}
		mu.Lock()
		instances = append(instances, h)
		mu.Unlock()
		return h
	})

	_ = relay.Publish(context.Background(), r, caseEvent{CaseID: "c-1"})
	_ = relay.Publish(context.Background(), r, caseEvent{CaseID: "c-2"})

	mu.Lock()
	defer mu.Unlock()
	if len(instances) != 2 {
		t.Fatalf("expected 2 handler instances, got %d", len(instances))
	}
	for i, h := range instances {
		h.mu.Lock()
		called := h.called
		h.mu.Unlock()
		if !called {
			t.Errorf("handler instance %d was not called", i)
		}
	}
}

// ---------------------------------------------------------------------------
// RequestKind
// ---------------------------------------------------------------------------

func TestRequestKind(t *testing.T) {
	tests := []struct {
		name string
		req  any
		want string
	}{
		{"command", createCmd{}, "command"},
		{"query", dashQuery{}, "query"},
		{"notification", caseEvent{}, "notification"},
		{"unknown", "not a relay type", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relay.RequestKind(tt.req)
			if got != tt.want {
				t.Errorf("RequestKind(%T) = %q, want %q", tt.req, got, tt.want)
			}
		})
	}
}
