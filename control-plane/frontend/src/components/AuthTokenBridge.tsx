import { useEffect } from 'react';
import { useAuth } from '@clerk/clerk-react';
import { clerkEnabled } from '@pymes/ts-pkg/auth';
import { registerTokenProvider } from '@pymes/ts-pkg/http';

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
