package iam

import (
	"fmt"
	"slices"
	"strings"
)

// Role is the fixed Pymes membership vocabulary. Provider roles are only a
// ceiling over these local roles; they never grant access on their own.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// Permission is returned by /session and used by product handlers.
type Permission string

const (
	PermissionOrganizationView   Permission = "organization:view"
	PermissionOrganizationUpdate Permission = "organization:update"
	PermissionTeamView           Permission = "team:view"
	PermissionMemberUpdate       Permission = "team:member:update"
	PermissionMemberRemove       Permission = "team:member:remove"
	PermissionInvitationCreate   Permission = "team:invitation:create"
	PermissionInvitationManage   Permission = "team:invitation:manage"
	PermissionSessionsManageSelf Permission = "sessions:manage:self"
)

var permissionsByRole = map[Role][]Permission{
	RoleMember: {
		PermissionOrganizationView,
		PermissionTeamView,
		PermissionSessionsManageSelf,
	},
	RoleAdmin: {
		PermissionOrganizationView,
		PermissionTeamView,
		PermissionMemberUpdate,
		PermissionMemberRemove,
		PermissionInvitationCreate,
		PermissionInvitationManage,
		PermissionSessionsManageSelf,
	},
	RoleOwner: {
		PermissionOrganizationView,
		PermissionOrganizationUpdate,
		PermissionTeamView,
		PermissionMemberUpdate,
		PermissionMemberRemove,
		PermissionInvitationCreate,
		PermissionInvitationManage,
		PermissionSessionsManageSelf,
	},
}

func ParseRole(value string) (Role, error) {
	role := Role(strings.ToLower(strings.TrimSpace(value)))
	if !role.Valid() {
		return "", fmt.Errorf("unsupported IAM role %q", value)
	}
	return role, nil
}

func (role Role) Valid() bool {
	return role == RoleOwner || role == RoleAdmin || role == RoleMember
}

// EffectiveRole intersects local authority with Clerk's coarse organization
// role. Clerk's org:admin is the provider ceiling for both local owner and
// admin; the local database remains the only source that can distinguish them.
func EffectiveRole(local Role, providerRole string) (Role, error) {
	if !local.Valid() {
		return "", fmt.Errorf("invalid local IAM role %q", local)
	}
	ceiling, ok := providerRoleCeiling(providerRole)
	if !ok {
		return "", fmt.Errorf("unsupported provider organization role %q", providerRole)
	}
	if roleRank(local) < roleRank(ceiling) {
		return local, nil
	}
	return ceiling, nil
}

func Permissions(role Role) []Permission {
	values := permissionsByRole[role]
	return slices.Clone(values)
}

func HasPermission(role Role, permission Permission) bool {
	return slices.Contains(permissionsByRole[role], permission)
}

// CanInvite enforces that tenant admins can invite only members while global
// owners can invite tenant admins or members.
func CanInvite(actor, invited Role) bool {
	switch actor {
	case RoleOwner:
		return invited == RoleAdmin || invited == RoleMember
	case RoleAdmin:
		return invited == RoleMember
	default:
		return false
	}
}

// CanChangeRole excludes the product-wide owner role from tenant membership
// changes.
func CanChangeRole(actor, current, desired Role) bool {
	if !actor.Valid() || !current.Valid() || !desired.Valid() {
		return false
	}
	if current == RoleOwner || desired == RoleOwner {
		return false
	}
	switch actor {
	case RoleOwner:
		return current == RoleAdmin || current == RoleMember
	case RoleAdmin:
		return current == RoleMember && desired == RoleMember
	default:
		return false
	}
}

func CanRemove(actor, target Role) bool {
	if !actor.Valid() || !target.Valid() || target == RoleOwner {
		return false
	}
	switch actor {
	case RoleOwner:
		return target == RoleAdmin || target == RoleMember
	case RoleAdmin:
		return target == RoleMember
	default:
		return false
	}
}

func providerRoleCeiling(value string) (Role, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "org:admin", "admin", "org:owner", "owner":
		return RoleOwner, true
	case "org:member", "member":
		return RoleMember, true
	default:
		return "", false
	}
}

func roleRank(role Role) int {
	switch role {
	case RoleMember:
		return 1
	case RoleAdmin:
		return 2
	case RoleOwner:
		return 3
	default:
		return 0
	}
}
