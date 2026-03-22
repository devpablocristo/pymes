import { useAuth, useOrganizationList } from '@clerk/clerk-react';
import { useEffect, useRef } from 'react';
import { clerkEnabled } from '../lib/auth';

/**
 * Si el usuario tiene una sola membresía de organización y Clerk no tiene org activa en la sesión,
 * el JWT no incluye el claim `o` / org → los backends verticales ven org vacío.
 * Activamos esa única org sin pedir un selector en la barra.
 */
export function ClerkSessionOrgSync() {
  const { isLoaded: authLoaded, isSignedIn, orgId } = useAuth();
  const { isLoaded: listLoaded, setActive, userMemberships } = useOrganizationList({
    userMemberships: { pageSize: 50 },
  });
  const attemptedRef = useRef(false);

  useEffect(() => {
    if (!isSignedIn) {
      attemptedRef.current = false;
    }
  }, [isSignedIn]);

  useEffect(() => {
    if (!clerkEnabled || !authLoaded || !listLoaded || !isSignedIn || orgId != null) {
      return;
    }
    if (userMemberships.isLoading) {
      return;
    }
    const data = userMemberships.data ?? [];
    if (data.length !== 1) {
      return;
    }
    const orgID = data[0]?.organization?.id;
    if (!orgID || attemptedRef.current) {
      return;
    }
    attemptedRef.current = true;
    void setActive({ organization: orgID }).catch(() => {
      attemptedRef.current = false;
    });
  }, [
    authLoaded,
    clerkEnabled,
    listLoaded,
    isSignedIn,
    orgId,
    setActive,
    userMemberships.data,
    userMemberships.isLoading,
  ]);

  return null;
}
