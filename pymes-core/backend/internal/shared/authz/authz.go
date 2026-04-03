package authz

import "strings"

// Roles privilegiados alineados con core/saas/go/tenant.NormalizeRole (owner, admin, secops, viewer).
// La consola usa solo dos niveles de producto: admin | user (ver ProductRole).
func IsPrivilegedRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin", "secops":
		return true
	default:
		return false
	}
}

func isConsoleScoped(scopes []string) bool {
	return HasScope(scopes, "admin:console:write") || HasScope(scopes, "admin:console:read")
}

// ProductRole reduce el rol del IdP/core a dos valores para UI y políticas de producto.
// "admin" incluye roles humanos privilegiados y credenciales con scopes explícitos de consola.
func ProductRole(role string, scopes []string) string {
	if IsPrivilegedRole(role) || isConsoleScoped(scopes) {
		return "admin"
	}
	return "user"
}

func HasScope(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target {
			return true
		}
	}
	return false
}

// IsAdmin: acceso a operaciones reservadas al panel (bootstrap admin, RBAC admin, etc.).
// Las credenciales técnicas no heredan admin por rol: requieren scopes explícitos de consola.
func IsAdmin(role string, scopes []string) bool {
	if IsPrivilegedRole(role) {
		return true
	}
	return HasScope(scopes, "admin:console:write")
}

// CanReadConsoleSettings: lectura de ajustes del tenant y endpoints admin de solo lectura.
func CanReadConsoleSettings(role string, scopes []string) bool {
	if IsPrivilegedRole(role) {
		return true
	}
	return isConsoleScoped(scopes)
}

// CanWriteConsoleSettings: mutación de ajustes del tenant.
func CanWriteConsoleSettings(role string, scopes []string) bool {
	if IsPrivilegedRole(role) {
		return true
	}
	return HasScope(scopes, "admin:console:write")
}

func CanManageAPIKeys(role string, scopes []string, authMethod string) bool {
	if !strings.EqualFold(strings.TrimSpace(authMethod), "jwt") {
		return false
	}
	if IsPrivilegedRole(role) {
		return true
	}
	return HasScope(scopes, "admin:console:write")
}
