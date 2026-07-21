// Mutable refs registered from RootLayoutNav (inside ClerkProvider).
// The API layer calls these to get/refresh the JWT and to sign out.
// Clerk's getToken() auto-refreshes before expiry.

let _getToken: (() => Promise<string | null>) | null = null;
let _signOut: (() => Promise<void>) | null = null;
let _orgSlug: string | null = null;

export function registerTokenGetter(fn: () => Promise<string | null>): void {
  _getToken = fn;
}

export function registerSignOut(fn: () => Promise<void>): void {
  _signOut = fn;
}

export function registerOrgSlug(slug: string | null | undefined): void {
  _orgSlug = slug ?? null;
}

export function getOrgSlug(): string | null {
  return _orgSlug;
}

export async function getAuthToken(): Promise<string | null> {
  if (!_getToken) return null;
  return _getToken();
}

export async function clerkSignOut(): Promise<void> {
  if (_signOut) await _signOut();
}
