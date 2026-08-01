package domain

type Principal struct {
	OrganizationID     string
	ActorID            string
	Role               string
	Permissions        []string
	OrganizationStatus string
	MembershipStatus   string
}

func (p Principal) Allows(organizationID, permission string) bool {
	if p.OrganizationID != organizationID || p.ActorID == "" ||
		p.OrganizationStatus != "ready" || p.MembershipStatus != "active" {
		return false
	}
	if p.Role == "owner" || p.Role == "admin" {
		return true
	}
	for _, current := range p.Permissions {
		if current == permission {
			return true
		}
	}
	return false
}
