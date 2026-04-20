package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/events"
	"github.com/phongln/go-relay/example/case/queries"
	"github.com/phongln/go-relay/example/case/resources"
	"github.com/phongln/go-relay/example/service"
	"github.com/phongln/go-relay/mockrelay"
)

func TestProcessHighRisk_Success(t *testing.T) {
	mr := mockrelay.New()

	mockrelay.OnDispatch(mr,
		commands.CreateCaseCmd{OrgID: "org-1", PlayerID: "p-1", RiskScore: 90},
		resources.CaseResource{ID: "c-1", OrgID: "org-1", Status: "open"},
		nil,
	)
	mockrelay.OnAsk(mr,
		queries.GetDashboardQuery{OrgID: "org-1", Page: 1, Status: "open"},
		[]resources.CaseSummary{{ID: "x"}, {ID: "y"}},
		nil,
	)
	mockrelay.OnPublish[events.CaseCreatedEvent](mr, nil)

	svc := service.New(mr)
	result, err := svc.ProcessHighRisk(context.Background(), "org-1", "p-1", 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "c-1" {
		t.Errorf("want c-1, got %s", result.ID)
	}
	mr.AssertExpectations(t)
}

func TestProcessHighRisk_AutoClose_WhenTooManyOpen(t *testing.T) {
	mr := mockrelay.New()

	mockrelay.OnDispatch(mr,
		commands.CreateCaseCmd{OrgID: "org-1", PlayerID: "p-1", RiskScore: 85},
		resources.CaseResource{ID: "c-new", OrgID: "org-1", Status: "open"},
		nil,
	)
	mockrelay.OnAsk(mr,
		queries.GetDashboardQuery{OrgID: "org-1", Page: 1, Status: "open"},
		make([]resources.CaseSummary, 11),
		nil,
	)
	mockrelay.OnDispatch(mr,
		commands.CloseCaseCmd{CaseID: "c-new", Reason: "auto-closed: org exceeded 10 open cases"},
		resources.CaseResource{ID: "c-new", Status: "closed"},
		nil,
	)

	svc := service.New(mr)
	result, err := svc.ProcessHighRisk(context.Background(), "org-1", "p-1", 85)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "closed" {
		t.Errorf("want closed, got %s", result.Status)
	}
	mr.AssertExpectations(t)
}

func TestProcessHighRisk_BelowThreshold_NoCommandDispatched(t *testing.T) {
	mr := mockrelay.New()
	svc := service.New(mr)

	_, err := svc.ProcessHighRisk(context.Background(), "org-1", "p-1", 50)
	if err == nil {
		t.Fatal("expected error for low risk score")
	}
	mockrelay.AssertNotCalled[commands.CreateCaseCmd](t, mr)
}

func TestProcessHighRisk_CreateError_QueryNotCalled(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatchError[commands.CreateCaseCmd](mr, errors.New("db down"))

	svc := service.New(mr)
	_, err := svc.ProcessHighRisk(context.Background(), "org-1", "p-1", 90)
	if err == nil {
		t.Fatal("expected error")
	}

	mockrelay.AssertNotCalled[queries.GetDashboardQuery](t, mr)
	mr.AssertExpectations(t)
}
