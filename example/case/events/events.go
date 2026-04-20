package events

import "time"

// CaseCreatedEvent is published after a case is successfully created.
type CaseCreatedEvent struct {
	CaseID    string
	OrgID     string
	PlayerID  string
	RiskScore float64
	CreatedAt time.Time
}

func (CaseCreatedEvent) NotificationMarker() {}

// CaseClosedEvent is published after a case is closed.
type CaseClosedEvent struct {
	CaseID   string
	OrgID    string
	Reason   string
	ClosedAt time.Time
}

func (CaseClosedEvent) NotificationMarker() {}
