package resources

import "time"

// CaseResource is the full case view returned by command and query handlers.
type CaseResource struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	PlayerID  string    `json:"player_id"`
	RiskScore float64   `json:"risk_score"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// CaseSummary is the lightweight list view returned by GetDashboardQuery.
type CaseSummary struct {
	ID        string  `json:"id"`
	OrgID     string  `json:"org_id"`
	Status    string  `json:"status"`
	RiskScore float64 `json:"risk_score"`
}
