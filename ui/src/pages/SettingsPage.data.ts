import { getMe, getSession } from '../lib/api';
import type { MeProfileResponse, SessionResponse } from '../lib/types';

/** Evita spinner infinito si Clerk/getToken o la red no resuelven. */
export const PROFILE_LOAD_TIMEOUT_MS = 45_000;

function rejectAfterMs(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) => {
    window.setTimeout(() => reject(new Error(message)), ms);
  });
}

export async function getSessionWithTimeout(): Promise<SessionResponse> {
  return Promise.race([getSession(), rejectAfterMs(PROFILE_LOAD_TIMEOUT_MS, 'profile_fetch_timeout')]);
}

export async function getMeWithTimeout(): Promise<MeProfileResponse> {
  return Promise.race([getMe(), rejectAfterMs(PROFILE_LOAD_TIMEOUT_MS, 'profile_fetch_timeout')]);
}

export function profileTenantLabel(auth: SessionResponse['auth'], clerkTenantName: string | null | undefined): string {
  const clerk = clerkTenantName?.trim() || '';
  const apiName = typeof auth.tenant_name === 'string' ? auth.tenant_name.trim() : '';
  const id = auth.org_id?.trim() || '';
  return clerk || apiName || id || '—';
}

export function accountTypeLabel(
  t: (key: string) => string,
  productRole: SessionResponse['auth']['product_role'],
): string {
  return productRole === 'admin' ? t('profile.accountTypeValue.admin') : t('profile.accountTypeValue.user');
}
