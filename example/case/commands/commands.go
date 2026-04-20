package commands

import "errors"

// CreateCaseCmd creates a new fraud detection case.
// Implements relay.Transactional — the relay wraps the handler in a transaction.
type CreateCaseCmd struct {
	OrgID     string
	PlayerID  string
	RiskScore float64
}

func (CreateCaseCmd) CommandMarker()   {}
func (CreateCaseCmd) WithTransaction() {}

func (c CreateCaseCmd) Validate() error {
	if c.OrgID == "" {
		return errors.New("org_id is required")
	}
	if c.PlayerID == "" {
		return errors.New("player_id is required")
	}
	if c.RiskScore < 0 || c.RiskScore > 100 {
		return errors.New("risk_score must be between 0 and 100")
	}
	return nil
}

// CloseCaseCmd closes an existing case.
type CloseCaseCmd struct {
	CaseID string
	Reason string
}

func (CloseCaseCmd) CommandMarker() {}

func (c CloseCaseCmd) Validate() error {
	if c.CaseID == "" {
		return errors.New("case_id is required")
	}
	return nil
}
