package domain

import "testing"

func TestPrincipalAuthorizationFailsClosed(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{RoleOwner, RoleAdmin, RoleMember, RoleViewer} {
		principal := Principal{
			OrganizationID: "org_1", ActorID: "user_1", Role: role,
			OrganizationStatus: "ready", MembershipStatus: "active",
		}
		if !principal.CanRead("org_1") {
			t.Fatalf("role %q should read its organization", role)
		}
		wantMutation := role == RoleOwner || role == RoleAdmin
		if principal.CanMutateRole() != wantMutation {
			t.Fatalf("role %q mutation=%v want=%v", role, principal.CanMutateRole(), wantMutation)
		}
	}

	for name, principal := range map[string]Principal{
		"wrong organization": {OrganizationID: "org_2", ActorID: "user_1", Role: RoleOwner, MembershipStatus: "active"},
		"missing actor":      {OrganizationID: "org_1", Role: RoleOwner, MembershipStatus: "active"},
		"inactive":           {OrganizationID: "org_1", ActorID: "user_1", Role: RoleOwner, MembershipStatus: "inactive"},
		"unknown role":       {OrganizationID: "org_1", ActorID: "user_1", Role: "custom", MembershipStatus: "active"},
	} {
		if principal.CanRead("org_1") {
			t.Fatalf("%s principal should fail closed", name)
		}
	}

	principal := Principal{OrganizationID: "org_1", ActorID: "user_1", Role: RoleOwner, MembershipStatus: "active"}
	for _, status := range []string{"pending", "failed", "suspended", "", "READY"} {
		principal.OrganizationStatus = status
		if principal.OrganizationReady() {
			t.Fatalf("status %q must not be ready", status)
		}
	}
	principal.OrganizationStatus = "ready"
	if !principal.OrganizationReady() {
		t.Fatal("ready principal must report a ready organization")
	}
}
