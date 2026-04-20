// Package worker shows how to use go-relay in a message queue consumer.
// The relay is transport-agnostic — the same relay.Relay works identically
// in HTTP controllers, service layers, and workers.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/resources"
	"github.com/phongln/go-relay/relay"
)

// Message represents an incoming queue message.
type Message struct{ Body string }

// CaseWorker processes queue messages using the relay.
type CaseWorker struct {
	relay  relay.Relay
	logger *slog.Logger
}

// New creates a CaseWorker.
func New(r relay.Relay, logger *slog.Logger) *CaseWorker {
	return &CaseWorker{relay: r, logger: logger}
}

// HandleCreateCase processes a "create case" message.
func (w *CaseWorker) HandleCreateCase(ctx context.Context, msg Message) error {
	var p struct {
		OrgID     string  `json:"org_id"`
		PlayerID  string  `json:"player_id"`
		RiskScore float64 `json:"risk_score"`
	}
	if err := json.Unmarshal([]byte(msg.Body), &p); err != nil {
		return fmt.Errorf("worker: invalid payload: %w", err)
	}

	result, err := relay.Dispatch[resources.CaseResource](ctx, w.relay, commands.CreateCaseCmd{
		OrgID:     p.OrgID,
		PlayerID:  p.PlayerID,
		RiskScore: p.RiskScore,
	})
	if err != nil {
		return fmt.Errorf("worker: dispatch failed: %w", err)
	}

	w.logger.InfoContext(ctx, "case created from queue", "case_id", result.ID, "org_id", result.OrgID)
	return nil
}

// HandleCloseCase processes a "close case" message.
func (w *CaseWorker) HandleCloseCase(ctx context.Context, msg Message) error {
	var p struct {
		CaseID string `json:"case_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(msg.Body), &p); err != nil {
		return fmt.Errorf("worker: invalid payload: %w", err)
	}

	_, err := relay.Dispatch[resources.CaseResource](ctx, w.relay, commands.CloseCaseCmd{
		CaseID: p.CaseID,
		Reason: p.Reason,
	})
	return err
}
