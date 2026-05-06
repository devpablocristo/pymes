import { useAuth, useOrganizationList } from '@clerk/react';
import { useEffect, useRef } from 'react';
import { clerkEnabled } from '../lib/auth';

const defaultClerkOrgID = (import.meta.env.VITE_CLERK_DEFAULT_ORG_ID as string | undefined)?.trim() ?? '';

/**
 * Mantiene una organización activa en Clerk para que el JWT incluya el claim `o` / org.
 * Si VITE_CLERK_DEFAULT_ORG_ID está configurado, fuerza esa org; si no, activa la única
 * membresía disponible sin pedir un selector.
 */
export function ClerkSessionOrgSync() {
  const { isLoaded: authLoaded, isSignedIn, orgId } = useAuth();
  const {
    isLoaded: listLoaded,
    setActive,
    userMemberships,
  } = useOrganizationList({
    userMemberships: { pageSize: 50 },
  });
  const attemptedRef = useRef(false);

  useEffect(() => {
    if (!isSignedIn) {
      attemptedRef.current = false;
    }
  }, [isSignedIn]);

  useEffect(() => {
    if (!clerkEnabled || !authLoaded || !listLoaded || !isSignedIn) {
      return;
    }
    if (userMemberships.isLoading) {
      return;
    }
    const data = userMemberships.data ?? [];
    const targetOrgID = defaultClerkOrgID || (data.length === 1 ? (data[0]?.organization?.id ?? '') : '');
    if (!targetOrgID || orgId === targetOrgID || attemptedRef.current) {
      return;
    }
    attemptedRef.current = true;
    void setActive({ organization: targetOrgID }).catch(() => {
      attemptedRef.current = false;
    });
  }, [authLoaded, listLoaded, isSignedIn, orgId, setActive, userMemberships.data, userMemberships.isLoading]);

  return null;
}
