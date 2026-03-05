import { useEffect } from 'react';
import { useAuth } from '@clerk/clerk-react';
import { registerTokenProvider } from '../api/client';
import { clerkEnabled } from '../lib/auth';

export function AuthTokenBridge() {
  const { getToken } = useAuth();

  useEffect(() => {
    if (!clerkEnabled) {
      registerTokenProvider(async () => null);
      return;
    }
    registerTokenProvider(async () => (await getToken()) ?? null);
  }, [getToken]);

  return null;
}
