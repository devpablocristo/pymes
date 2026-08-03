package models

// ReportPageRequest records normalized pagination received by the accounting
// fake so contract tests can assert transport without changing response shapes.
type ReportPageRequest struct {
	OrganizationID string
	Report         string
	Limit          int
	Cursor         string
}

// ReportCursor is the private payload encoded into an opaque fake cursor.
type ReportCursor struct {
	Version        int    `json:"v"`
	OrganizationID string `json:"o"`
	AsOf           string `json:"a"`
	EntryID        string `json:"i"`
}
