package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/events"
	"github.com/phongln/go-relay/example/case/queries"
	"github.com/phongln/go-relay/example/case/resources"
	"github.com/phongln/go-relay/example/controller"
	"github.com/phongln/go-relay/mockrelay"
	"github.com/phongln/go-relay/relay"
)

func body(s string) *strings.Reader { return strings.NewReader(s) }

// =============================================================================
// Create
// =============================================================================

func TestCreate_Success(t *testing.T) {
	mr := mockrelay.New()

	want := resources.CaseResource{
		ID: "case-123", OrgID: "org-1", PlayerID: "p-1",
		RiskScore: 75, Status: "open", CreatedAt: time.Now(),
	}
	mockrelay.OnDispatch(mr,
		commands.CreateCaseCmd{OrgID: "org-1", PlayerID: "p-1", RiskScore: 75},
		want, nil,
	)
	mockrelay.OnPublish[events.CaseCreatedEvent](mr, nil)

	ctrl := controller.New(mr)
	req := httptest.NewRequest(http.MethodPost, "/cases",
		body(`{"org_id":"org-1","player_id":"p-1","risk_score":75}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctrl.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	var got resources.CaseResource
	json.NewDecoder(w.Body).Decode(&got) //nolint:errcheck
	if got.ID != "case-123" {
		t.Errorf("want case-123, got %s", got.ID)
	}
	mr.AssertExpectations(t)
}

func TestCreate_HandlerError_Returns500(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatchError[commands.CreateCaseCmd](mr, errors.New("db down"))

	ctrl := controller.New(mr)
	req := httptest.NewRequest(http.MethodPost, "/cases",
		body(`{"org_id":"org-1","player_id":"p-1","risk_score":75}`))
	w := httptest.NewRecorder()

	ctrl.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
	mr.AssertExpectations(t)
}

func TestCreate_InvalidBody_Returns400(t *testing.T) {
	mr := mockrelay.New()
	ctrl := controller.New(mr)

	req := httptest.NewRequest(http.MethodPost, "/cases", body(`not-json`))
	w := httptest.NewRecorder()
	ctrl.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	mockrelay.AssertNotCalled[commands.CreateCaseCmd](t, mr)
}

func TestCreate_DynamicResponse(t *testing.T) {
	mr := mockrelay.New()

	mockrelay.OnDispatchFn(mr, func(_ context.Context, cmd commands.CreateCaseCmd) (resources.CaseResource, error) {
		status := "open"
		if cmd.RiskScore > 90 {
			status = "high-risk"
		}
		return resources.CaseResource{ID: "c-1", Status: status}, nil
	})
	mockrelay.OnPublish[events.CaseCreatedEvent](mr, nil)

	ctrl := controller.New(mr)
	req := httptest.NewRequest(http.MethodPost, "/cases",
		body(`{"org_id":"org-1","player_id":"p-1","risk_score":95}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctrl.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	var got resources.CaseResource
	json.NewDecoder(w.Body).Decode(&got) //nolint:errcheck
	if got.Status != "high-risk" {
		t.Errorf("want high-risk for score 95, got %s", got.Status)
	}
	mr.AssertExpectations(t)
}

// =============================================================================
// Dashboard
// =============================================================================

func TestDashboard_Success(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnAsk(mr,
		queries.GetDashboardQuery{OrgID: "org-1", Page: 1, Status: "open"},
		[]resources.CaseSummary{{ID: "c-1"}, {ID: "c-2"}},
		nil,
	)

	ctrl := controller.New(mr)
	req := httptest.NewRequest(http.MethodGet, "/cases?org_id=org-1&status=open", nil)
	w := httptest.NewRecorder()
	ctrl.Dashboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got []resources.CaseSummary
	json.NewDecoder(w.Body).Decode(&got) //nolint:errcheck
	if len(got) != 2 {
		t.Errorf("want 2 results, got %d", len(got))
	}
	mr.AssertExpectations(t)
}

func TestDashboard_QueryError_Returns500(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnAskError[queries.GetDashboardQuery](mr, errors.New("read replica down"))

	ctrl := controller.New(mr)
	req := httptest.NewRequest(http.MethodGet, "/cases?org_id=org-1", nil)
	w := httptest.NewRecorder()
	ctrl.Dashboard(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
	mr.AssertExpectations(t)
}

// =============================================================================
// Close
// =============================================================================

func TestClose_Success(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatch(mr,
		commands.CloseCaseCmd{CaseID: "case-123", Reason: "resolved"},
		resources.CaseResource{ID: "case-123", Status: "closed"},
		nil,
	)

	ctrl := controller.New(mr)
	req := httptest.NewRequest(http.MethodPost, "/cases/close",
		body(`{"case_id":"case-123","reason":"resolved"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctrl.Close(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got resources.CaseResource
	json.NewDecoder(w.Body).Decode(&got) //nolint:errcheck
	if got.Status != "closed" {
		t.Errorf("want closed, got %s", got.Status)
	}
	mr.AssertExpectations(t)
}

// =============================================================================
// Design contract: only relay.Relay is needed
// =============================================================================

func TestController_AcceptsRelayInterface(t *testing.T) {
	var r relay.Relay = mockrelay.New()
	ctrl := controller.New(r)
	_ = ctrl
}
