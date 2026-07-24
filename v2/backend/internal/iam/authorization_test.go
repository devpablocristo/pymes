package iam

import (
	"slices"
	"testing"
)

func TestEffectiveRoleIsIntersectionWithProviderCeiling(t *testing.T) {
	tests := []struct {
		name         string
		local        Role
		providerRole string
		want         Role
		wantError    bool
	}{
		{name: "owner through Clerk admin", local: RoleOwner, providerRole: "org:admin", want: RoleOwner},
		{name: "admin through Clerk admin", local: RoleAdmin, providerRole: "org:admin", want: RoleAdmin},
		{name: "member through Clerk admin", local: RoleMember, providerRole: "org:admin", want: RoleMember},
		{name: "owner reduced by Clerk member", local: RoleOwner, providerRole: "org:member", want: RoleMember},
		{name: "admin reduced by Clerk member", local: RoleAdmin, providerRole: "org:member", want: RoleMember},
		{name: "unknown provider role", local: RoleOwner, providerRole: "org:unknown", wantError: true},
		{name: "unknown local role", local: "superadmin", providerRole: "org:admin", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := EffectiveRole(test.local, test.providerRole)
			if test.wantError {
				if err == nil {
					t.Fatalf("EffectiveRole() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("EffectiveRole() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("EffectiveRole() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPermissionsAreFixedAndReturnedAsCopy(t *testing.T) {
	owner := Permissions(RoleOwner)
	if !slices.Contains(owner, PermissionOrganizationUpdate) {
		t.Fatal("owner lacks organization update")
	}
	if slices.Contains(Permissions(RoleAdmin), PermissionOrganizationUpdate) {
		t.Fatal("admin may update organization")
	}
	if slices.Contains(Permissions(RoleMember), PermissionInvitationCreate) {
		t.Fatal("member may create invitations")
	}

	owner[0] = "mutated"
	if HasPermission(RoleOwner, "mutated") {
		t.Fatal("Permissions exposed mutable package state")
	}
}

func TestInvitationMatrix(t *testing.T) {
	tests := []struct {
		actor   Role
		invited Role
		want    bool
	}{
		{RoleOwner, RoleOwner, false},
		{RoleOwner, RoleAdmin, true},
		{RoleOwner, RoleMember, true},
		{RoleAdmin, RoleOwner, false},
		{RoleAdmin, RoleAdmin, false},
		{RoleAdmin, RoleMember, true},
		{RoleMember, RoleOwner, false},
		{RoleMember, RoleAdmin, false},
		{RoleMember, RoleMember, false},
	}
	for _, test := range tests {
		if got := CanInvite(test.actor, test.invited); got != test.want {
			t.Errorf("CanInvite(%q, %q) = %v, want %v", test.actor, test.invited, got, test.want)
		}
	}
}

func TestMemberAdministrationMatrix(t *testing.T) {
	roles := []Role{RoleOwner, RoleAdmin, RoleMember}
	for _, actor := range roles {
		for _, target := range roles {
			remove := CanRemove(actor, target)
			wantRemove := actor == RoleOwner && target != RoleOwner ||
				actor == RoleAdmin && target == RoleMember
			if remove != wantRemove {
				t.Errorf("CanRemove(%q, %q) = %v, want %v", actor, target, remove, wantRemove)
			}

			for _, desired := range roles {
				change := CanChangeRole(actor, target, desired)
				wantChange := target != RoleOwner && desired != RoleOwner &&
					(actor == RoleOwner ||
						actor == RoleAdmin && target == RoleMember && desired == RoleMember)
				if change != wantChange {
					t.Errorf(
						"CanChangeRole(%q, %q, %q) = %v, want %v",
						actor,
						target,
						desired,
						change,
						wantChange,
					)
				}
			}
		}
	}
}
