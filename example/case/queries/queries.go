package queries

// GetDashboardQuery returns a paginated list of case summaries for an org.
type GetDashboardQuery struct {
	OrgID  string
	Page   int
	Status string // optional: "open", "closed", or "" for all
}

func (GetDashboardQuery) QueryMarker() {}

// GetCaseByIDQuery returns a single case by ID.
type GetCaseByIDQuery struct{ CaseID string }

func (GetCaseByIDQuery) QueryMarker() {}
