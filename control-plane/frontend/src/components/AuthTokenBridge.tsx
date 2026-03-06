import { useEffect } from 'react';
import { useAuth } from '@clerk/clerk-react';
import { registerTokenProvider } from '../api/client';
import { clerkEnabled } from '../lib/auth';

export function AuthTokenBridge() {
  if (!clerkEnabled) {
    return <LocalAuthTokenBridge />;
  }

  return <ClerkAuthTokenBridge />;
}

function LocalAuthTokenBridge() {
  useEffect(() => {
    registerTokenProvider(async () => null);
  }, []);

  return null;
}

function ClerkAuthTokenBridge() {
  const { getToken } = useAuth();

  useEffect(() => {
    registerTokenProvider(async () => (await getToken()) ?? null);
  }, [getToken]);

  return null;
}
