import type { MeProfileResponse, MeProfileUser } from './types';

/**
 * Objeto usuario de Clerk con los campos que usamos para perfil / saludo.
 * Evita acoplar el módulo a tipos internos de @clerk/types.
 */
export type ClerkUserProfileSource = {
  id: string;
  firstName: string | null;
  lastName: string | null;
  fullName: string | null;
  username: string | null;
  primaryEmailAddress: { emailAddress: string } | null | undefined;
  primaryPhoneNumber: { phoneNumber: string } | null | undefined;
  imageUrl: string | null;
  externalAccounts?: { provider: string }[] | null;
};

export function splitDisplayName(full: string): { given: string; family: string } {
  const s = full.trim();
  const i = s.indexOf(' ');
  if (i < 0) {
    return { given: s, family: '' };
  }
  return { given: s.slice(0, i).trim(), family: s.slice(i + 1).trim() };
}

export function displayGivenFromUser(u: MeProfileUser): string {
  const g = (u.given_name ?? '').trim();
  if (g) {
    return g;
  }
  return splitDisplayName(u.name ?? '').given;
}

export function displayFamilyFromUser(u: MeProfileUser): string {
  const f = (u.family_name ?? '').trim();
  if (f) {
    return f;
  }
  return splitDisplayName(u.name ?? '').family;
}

/** Enriquece datos del API con lo que Clerk tiene en el browser (nombre, email, avatar). */
export function mergeClerkSessionWithApiUser(
  clerkUser: ClerkUserProfileSource,
  apiUser: MeProfileUser | null | undefined,
): MeProfileUser {
  const email =
    clerkUser.primaryEmailAddress?.emailAddress?.trim() ||
    apiUser?.email?.trim() ||
    '';
  const givenFromClerk = (typeof clerkUser.firstName === 'string' ? clerkUser.firstName.trim() : '') || '';
  const familyFromClerk = (typeof clerkUser.lastName === 'string' ? clerkUser.lastName.trim() : '') || '';
  const nameFromClerk =
    (typeof clerkUser.fullName === 'string' ? clerkUser.fullName.trim() : '') ||
    [givenFromClerk, familyFromClerk].filter(Boolean).join(' ').trim() ||
    clerkUser.username?.trim() ||
    '';
  const given = givenFromClerk || (apiUser ? displayGivenFromUser(apiUser) : '');
  const family = familyFromClerk || (apiUser ? displayFamilyFromUser(apiUser) : '');
  const name =
    nameFromClerk ||
    [given, family].filter(Boolean).join(' ').trim() ||
    apiUser?.name?.trim() ||
    '';
  const phoneFromClerk = clerkUser.primaryPhoneNumber?.phoneNumber?.trim() || '';
  const phone = (apiUser?.phone ?? '').trim() || phoneFromClerk || '';
  return {
    id: apiUser?.id || clerkUser.id,
    external_id: apiUser?.external_id || clerkUser.id,
    email,
    name,
    given_name: given || undefined,
    family_name: family || undefined,
    phone: phone || undefined,
    avatar_url: clerkUser.imageUrl || apiUser?.avatar_url || null,
  };
}

const PLACEHOLDER_NAME = 'user';

/** Primer nombre o parte amigable para “Hola, …”. */
export function greetingDisplayName(
  me: MeProfileResponse | null | undefined,
  clerkUser: ClerkUserProfileSource | null | undefined,
): string {
  const apiUser = me?.user ?? undefined;
  const merged =
    clerkUser != null ? mergeClerkSessionWithApiUser(clerkUser, apiUser) : apiUser ?? null;

  if (!merged) {
    return (me?.external_id ?? '').trim();
  }

  const given = (merged.given_name ?? '').trim();
  if (given) {
    return given;
  }

  const name = (merged.name ?? '').trim();
  if (name && name.toLowerCase() !== PLACEHOLDER_NAME) {
    return name.split(/\s+/)[0] ?? name;
  }

  const email = (merged.email ?? '').trim();
  if (email) {
    return email.split('@')[0] ?? email;
  }

  return (me?.external_id ?? '').trim();
}

export function clerkUserHasGoogleProvider(user: ClerkUserProfileSource | null | undefined): boolean {
  if (!user?.externalAccounts?.length) {
    return false;
  }
  return user.externalAccounts.some((a) => a.provider.toLowerCase().includes('google'));
}
