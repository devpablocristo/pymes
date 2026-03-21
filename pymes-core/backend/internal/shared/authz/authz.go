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

// ProductRole reduce el rol del IdP/core a dos valores para UI y políticas de producto.
// "admin" incluye owner/admin/secops del token; "service" (API key) cuenta como admin si aplica consola.
func ProductRole(role string) string {
	r := strings.ToLower(strings.TrimSpace(role))
	if r == "service" {
		return "admin"
	}
	if IsPrivilegedRole(role) {
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
// Rol service (credencial API) se trata como admin de producto, alineado con ProductRole.
func IsAdmin(role string, scopes []string) bool {
	if strings.EqualFold(strings.TrimSpace(role), "service") {
		return true
	}
	if IsPrivilegedRole(role) {
		return true
	}
	return HasScope(scopes, "admin:console:write") || HasScope(scopes, "admin:console:read")
}

// CanReadConsoleSettings: lectura de ajustes del tenant y endpoints admin de solo lectura.
func CanReadConsoleSettings(role string, scopes []string) bool {
	if IsPrivilegedRole(role) {
		return true
	}
	return HasScope(scopes, "admin:console:read") || HasScope(scopes, "admin:console:write")
}

// CanWriteConsoleSettings: mutación de ajustes del tenant.
func CanWriteConsoleSettings(role string, scopes []string) bool {
	if IsPrivilegedRole(role) {
		return true
	}
	return HasScope(scopes, "admin:console:write")
}
