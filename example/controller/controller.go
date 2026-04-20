// Package controller shows how to use go-relay in an HTTP handler.
package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/events"
	"github.com/phongln/go-relay/example/case/queries"
	"github.com/phongln/go-relay/example/case/resources"
	"github.com/phongln/go-relay/relay"
)

// CaseController handles HTTP requests for the case domain.
// It has a single relay.Relay field — the only dependency needed.
type CaseController struct {
	relay relay.Relay
}

// New creates a CaseController. In production, r is created once in
// main() via bootstrap.New and injected everywhere.
func New(r relay.Relay) *CaseController {
	return &CaseController{relay: r}
}

// Create handles POST /cases
func (c *CaseController) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OrgID     string  `json:"org_id"`
		PlayerID  string  `json:"player_id"`
		RiskScore float64 `json:"risk_score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := relay.Dispatch[resources.CaseResource](r.Context(), c.relay, commands.CreateCaseCmd{
		OrgID:     body.OrgID,
		PlayerID:  body.PlayerID,
		RiskScore: body.RiskScore,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish domain event — fire-and-forget.
	_ = relay.Publish(r.Context(), c.relay, events.CaseCreatedEvent{
		CaseID:    result.ID,
		OrgID:     result.OrgID,
		PlayerID:  result.PlayerID,
		RiskScore: result.RiskScore,
		CreatedAt: time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

// Dashboard handles GET /cases?org_id=...&status=...
func (c *CaseController) Dashboard(w http.ResponseWriter, r *http.Request) {
	result, err := relay.Ask[[]resources.CaseSummary](r.Context(), c.relay, queries.GetDashboardQuery{
		OrgID:  r.URL.Query().Get("org_id"),
		Page:   1,
		Status: r.URL.Query().Get("status"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

// GetByID handles GET /cases?id=...
func (c *CaseController) GetByID(w http.ResponseWriter, r *http.Request) {
	result, err := relay.Ask[resources.CaseResource](r.Context(), c.relay, queries.GetCaseByIDQuery{
		CaseID: r.URL.Query().Get("id"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

// Close handles POST /cases/close
func (c *CaseController) Close(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CaseID string `json:"case_id"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	result, err := relay.Dispatch[resources.CaseResource](r.Context(), c.relay, commands.CloseCaseCmd{
		CaseID: body.CaseID,
		Reason: body.Reason,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}
