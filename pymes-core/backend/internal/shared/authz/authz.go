package authz

func HasScope(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target {
			return true
		}
	}
	return false
}

func IsAdmin(role string, scopes []string) bool {
	if role == "admin" {
		return true
	}
	return HasScope(scopes, "admin:console:write") || HasScope(scopes, "admin:console:read")
}
