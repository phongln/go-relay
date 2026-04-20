package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/events"
	"github.com/phongln/go-relay/example/case/queries"
	"github.com/phongln/go-relay/example/case/resources"
)

// CreateCaseHandler handles CreateCaseCmd.
// ctx already carries the active transaction — pass it to all repo calls.
type CreateCaseHandler struct{ Logger *slog.Logger }

func (h *CreateCaseHandler) Handle(ctx context.Context, cmd commands.CreateCaseCmd) (resources.CaseResource, error) {
	h.Logger.InfoContext(ctx, "creating case", "org_id", cmd.OrgID, "risk_score", cmd.RiskScore)
	now := time.Now()
	return resources.CaseResource{
		ID:        fmt.Sprintf("case-%s-%d", cmd.OrgID, now.UnixNano()),
		OrgID:     cmd.OrgID,
		PlayerID:  cmd.PlayerID,
		RiskScore: cmd.RiskScore,
		Status:    "open",
		CreatedAt: now,
	}, nil
}

// CloseCaseHandler handles CloseCaseCmd.
type CloseCaseHandler struct{ Logger *slog.Logger }

func (h *CloseCaseHandler) Handle(ctx context.Context, cmd commands.CloseCaseCmd) (resources.CaseResource, error) {
	h.Logger.InfoContext(ctx, "closing case", "case_id", cmd.CaseID)
	return resources.CaseResource{ID: cmd.CaseID, Status: "closed"}, nil
}

// GetDashboardHandler handles GetDashboardQuery.
// Reads directly from the read collection — no domain model loaded.
type GetDashboardHandler struct{ Logger *slog.Logger }

func (h *GetDashboardHandler) Handle(ctx context.Context, qry queries.GetDashboardQuery) ([]resources.CaseSummary, error) {
	h.Logger.InfoContext(ctx, "fetching dashboard", "org_id", qry.OrgID, "page", qry.Page)
	return []resources.CaseSummary{
		{ID: "c-001", OrgID: qry.OrgID, Status: "open", RiskScore: 82.5},
		{ID: "c-002", OrgID: qry.OrgID, Status: "open", RiskScore: 67.0},
		{ID: "c-003", OrgID: qry.OrgID, Status: "closed", RiskScore: 45.0},
	}, nil
}

// GetCaseByIDHandler handles GetCaseByIDQuery.
type GetCaseByIDHandler struct{ Logger *slog.Logger }

func (h *GetCaseByIDHandler) Handle(ctx context.Context, qry queries.GetCaseByIDQuery) (resources.CaseResource, error) {
	if qry.CaseID == "" {
		return resources.CaseResource{}, fmt.Errorf("case_id is required")
	}
	return resources.CaseResource{ID: qry.CaseID, Status: "open", RiskScore: 55.0, CreatedAt: time.Now()}, nil
}

// WebhookDispatcher enqueues a webhook when a case is created.
type WebhookDispatcher struct{ Logger *slog.Logger }

func (h *WebhookDispatcher) Handle(ctx context.Context, e events.CaseCreatedEvent) error {
	h.Logger.InfoContext(ctx, "dispatching webhook", "case_id", e.CaseID, "org_id", e.OrgID)
	return nil
}

// AuditLogger writes an audit entry when a case is created.
type AuditLogger struct{ Logger *slog.Logger }

func (h *AuditLogger) Handle(ctx context.Context, e events.CaseCreatedEvent) error {
	h.Logger.InfoContext(ctx, "audit: case created", "case_id", e.CaseID, "risk_score", e.RiskScore)
	return nil
}
