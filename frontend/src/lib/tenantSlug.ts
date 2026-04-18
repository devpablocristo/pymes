import { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { getTenantProfile, type TenantProfile } from './tenantProfile';

/**
 * Normaliza un string a slug URL-safe (lowercase, guiones, sin tildes).
 * Ejemplo: "Bici Max S.R.L." → "bici-max-srl"
 */
export function slugify(input: string): string {
  return input
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
}

/**
 * Slug del tenant actual derivado de `tenantProfile.businessName`. Cuando no
 * hay profile o el nombre es vacío, devuelve `null` (el caller debe decidir
 * redirigir a onboarding).
 */
export function getTenantSlug(profile: TenantProfile | null = getTenantProfile()): string | null {
  const name = profile?.businessName?.trim();
  if (!name) return null;
  const slug = slugify(name);
  return slug || null;
}

/** Hook reactivo: slug del tenant activo (null si no hay profile). */
export function useTenantSlug(): string | null {
  return useMemo(() => getTenantSlug(), []);
}

/**
 * Hook que lee el `:orgSlug` de la URL actual. Si no está, cae al slug del profile.
 * Usar este cuando se arman links internos a partir del URL actual (p. ej. tabs).
 */
export function useActiveTenantSlug(): string | null {
  const params = useParams();
  const profileSlug = useTenantSlug();
  const urlSlug = typeof params.orgSlug === 'string' ? params.orgSlug : null;
  return urlSlug || profileSlug;
}

/**
 * Prefija una path con el slug del tenant activo (sin slash inicial adicional).
 * `tenantLink('/dashboard', 'bicimax')` → `/bicimax/dashboard`
 */
export function tenantLink(path: string, slug: string | null): string {
  if (!slug) return path;
  const normalized = path.startsWith('/') ? path : `/${path}`;
  return `/${slug}${normalized}`;
}

/** Hook que devuelve un builder de URLs con el slug activo ya resuelto. */
export function useTenantLinkBuilder(): (path: string) => string {
  const slug = useActiveTenantSlug();
  return useMemo(() => (path: string) => tenantLink(path, slug), [slug]);
}
