// Package domain owns the local identity and authorization model used after
// Clerk has verified a session. HTTP adapters never authorize directly from
// provider claims: they authorize this projected principal instead.
package domain

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

type Principal struct {
	OrganizationID     string
	ActorID            string
	Role               Role
	Permissions        []string
	OrganizationStatus string
	MembershipStatus   string
}

func (p Principal) CanRead(organizationID string) bool {
	if p.OrganizationID == "" || p.OrganizationID != organizationID || p.ActorID == "" ||
		p.MembershipStatus != "active" {
		return false
	}
	switch p.Role {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

func (p Principal) CanMutateRole() bool {
	return p.Role == RoleOwner || p.Role == RoleAdmin
}

func (p Principal) OrganizationReady() bool { return p.OrganizationStatus == "ready" }
