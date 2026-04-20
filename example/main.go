// main.go demonstrates go-relay end-to-end.
//
// Run with:
//
//	cd example && go run main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/phongln/go-relay/example/bootstrap"
	"github.com/phongln/go-relay/example/case/commands"
	"github.com/phongln/go-relay/example/case/events"
	"github.com/phongln/go-relay/example/case/queries"
	"github.com/phongln/go-relay/example/case/resources"
	"github.com/phongln/go-relay/relay"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r := bootstrap.New(logger)
	ctx := context.Background()

	sep := "══════════════════════════════════════════════"
	fmt.Println("\n" + sep)
	fmt.Println("  go-relay — live demo")
	fmt.Println(sep)

	// ── 1. Command (write) ───────────────────────────────────────────────────
	fmt.Println("\n[1] relay.Dispatch — CreateCaseCmd (write)")
	fmt.Println("    pipeline: validation → transaction → handler → commit")

	result, err := relay.Dispatch[resources.CaseResource](ctx, r, commands.CreateCaseCmd{
		OrgID:     "org-acme",
		PlayerID:  "player-007",
		RiskScore: 87.5,
	})
	must(err)
	fmt.Printf("    ✓ case created  id=%-30s status=%s  risk=%.1f\n",
		result.ID, result.Status, result.RiskScore)

	// ── 2. Query (read) ──────────────────────────────────────────────────────
	fmt.Println("\n[2] relay.Ask — GetDashboardQuery (read)")
	fmt.Println("    handler reads from read collection directly, no domain model")

	list, err := relay.Ask[[]resources.CaseSummary](ctx, r, queries.GetDashboardQuery{
		OrgID: "org-acme", Page: 1, Status: "open",
	})
	must(err)
	fmt.Printf("    ✓ dashboard returned %d cases:\n", len(list))
	for _, c := range list {
		fmt.Printf("      - %-6s  status=%-6s  risk=%.1f\n", c.ID, c.Status, c.RiskScore)
	}

	// ── 3. Notification (one-to-many) ────────────────────────────────────────
	fmt.Println("\n[3] relay.Publish — CaseCreatedEvent")
	fmt.Println("    WebhookDispatcher and AuditLogger both run")

	err = relay.Publish(ctx, r, events.CaseCreatedEvent{
		CaseID:    result.ID,
		OrgID:     result.OrgID,
		PlayerID:  result.PlayerID,
		RiskScore: result.RiskScore,
		CreatedAt: time.Now(),
	})
	must(err)
	fmt.Println("    ✓ event published to all registered handlers")

	// ── 4. Validation failure ────────────────────────────────────────────────
	fmt.Println("\n[4] Validation — empty OrgID rejected before handler runs")

	_, err = relay.Dispatch[resources.CaseResource](ctx, r, commands.CreateCaseCmd{
		OrgID: "", PlayerID: "p-1", RiskScore: 50,
	})
	if err != nil {
		fmt.Printf("    ✓ caught: %v\n", err)
	}

	// ── 5. HandlerError unwrapping ───────────────────────────────────────────
	fmt.Println("\n[5] HandlerError — errors.As / errors.Is")

	synthetic := &relay.HandlerError{
		RequestType: "commands.CreateCaseCmd",
		Cause:       relay.ErrHandlerNotFound,
	}
	fmt.Printf("    errors.Is(ErrHandlerNotFound) : %v\n",
		errors.Is(synthetic, relay.ErrHandlerNotFound))

	var he *relay.HandlerError
	if errors.As(synthetic, &he) {
		fmt.Printf("    errors.As(*HandlerError)      : true  RequestType=%s\n", he.RequestType)
	}

	// ── 6. Close a case ──────────────────────────────────────────────────────
	fmt.Println("\n[6] relay.Dispatch — CloseCaseCmd")

	closed, err := relay.Dispatch[resources.CaseResource](ctx, r, commands.CloseCaseCmd{
		CaseID: result.ID, Reason: "investigation complete",
	})
	must(err)
	fmt.Printf("    ✓ case closed   id=%-30s status=%s\n", closed.ID, closed.Status)

	fmt.Printf("\n%s\n", sep)
	fmt.Println("  Done. See example/controller, service, worker for more patterns.")
	fmt.Println(sep + "\n")
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}
