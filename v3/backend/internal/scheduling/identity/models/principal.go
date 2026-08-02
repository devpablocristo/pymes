package models

type Principal struct {
	OrganizationID     string
	ActorID            string
	Role               string
	Permissions        []string
	OrganizationStatus string
	MembershipStatus   string
}
