// Package service shows how to use go-relay in a service / use-case layer.
// The pattern is identical to the controller layer — inject relay.Relay, call Dispatch/Ask/Publish.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/events"
	"github.com/phongln/go-relay/example/case/queries"
	"github.com/phongln/go-relay/example/case/resources"
	"github.com/phongln/go-relay/relay"
)

// CaseService orchestrates multi-step operations across multiple relay calls.
type CaseService struct {
	relay relay.Relay
}

// New creates a CaseService.
func New(r relay.Relay) *CaseService {
	return &CaseService{relay: r}
}

// ProcessHighRisk creates a case and applies auto-close logic when
// the org already has too many open cases.
func (s *CaseService) ProcessHighRisk(ctx context.Context, orgID, playerID string, riskScore float64) (resources.CaseResource, error) {
	if riskScore < 80 {
		return resources.CaseResource{}, errors.New("risk score below high-risk threshold (80)")
	}

	result, err := relay.Dispatch[resources.CaseResource](ctx, s.relay, commands.CreateCaseCmd{
		OrgID:     orgID,
		PlayerID:  playerID,
		RiskScore: riskScore,
	})
	if err != nil {
		return resources.CaseResource{}, err
	}

	existing, err := relay.Ask[[]resources.CaseSummary](ctx, s.relay, queries.GetDashboardQuery{
		OrgID:  orgID,
		Page:   1,
		Status: "open",
	})
	if err != nil {
		return resources.CaseResource{}, err
	}

	if len(existing) > 10 {
		closed, err := relay.Dispatch[resources.CaseResource](ctx, s.relay, commands.CloseCaseCmd{
			CaseID: result.ID,
			Reason: "auto-closed: org exceeded 10 open cases",
		})
		if err != nil {
			return resources.CaseResource{}, err
		}
		return closed, nil
	}

	_ = relay.Publish(ctx, s.relay, events.CaseCreatedEvent{
		CaseID:    result.ID,
		OrgID:     result.OrgID,
		PlayerID:  result.PlayerID,
		RiskScore: result.RiskScore,
		CreatedAt: time.Now(),
	})

	return result, nil
}
