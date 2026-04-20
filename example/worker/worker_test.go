package worker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/resources"
	"github.com/phongln/go-relay/example/worker"
	"github.com/phongln/go-relay/mockrelay"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

func TestHandleCreateCase_Success(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatch(mr,
		commands.CreateCaseCmd{OrgID: "org-1", PlayerID: "p-1", RiskScore: 60},
		resources.CaseResource{ID: "c-worker"},
		nil,
	)

	w := worker.New(mr, logger)
	err := w.HandleCreateCase(context.Background(), worker.Message{
		Body: `{"org_id":"org-1","player_id":"p-1","risk_score":60}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mr.AssertExpectations(t)
}

func TestHandleCreateCase_InvalidJSON(t *testing.T) {
	mr := mockrelay.New()
	w := worker.New(mr, logger)

	err := w.HandleCreateCase(context.Background(), worker.Message{Body: `not-json`})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	mockrelay.AssertNotCalled[commands.CreateCaseCmd](t, mr)
}

func TestHandleCreateCase_DispatchError(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatchError[commands.CreateCaseCmd](mr, errors.New("db write failed"))

	w := worker.New(mr, logger)
	err := w.HandleCreateCase(context.Background(), worker.Message{
		Body: `{"org_id":"org-1","player_id":"p-1","risk_score":60}`,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	mr.AssertExpectations(t)
}

func TestHandleCloseCase_Success(t *testing.T) {
	mr := mockrelay.New()
	mockrelay.OnDispatch(mr,
		commands.CloseCaseCmd{CaseID: "c-1", Reason: "resolved"},
		resources.CaseResource{ID: "c-1", Status: "closed"},
		nil,
	)

	w := worker.New(mr, logger)
	err := w.HandleCloseCase(context.Background(), worker.Message{
		Body: `{"case_id":"c-1","reason":"resolved"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mr.AssertExpectations(t)
}
