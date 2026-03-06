import { useEffect } from 'react';
import { useAuth } from '@clerk/clerk-react';
import { clerkEnabled } from '@pymes/ts-pkg/auth';
import { registerTokenProvider } from '@pymes/ts-pkg/http';
import { registerProfessionalsTokenProvider } from '../lib/api';

export function AuthTokenBridge() {
  if (!clerkEnabled) {
    return <LocalAuthTokenBridge />;
  }

  return <ClerkAuthTokenBridge />;
}

function LocalAuthTokenBridge() {
  useEffect(() => {
    registerTokenProvider(async () => null);
    registerProfessionalsTokenProvider(async () => null);
  }, []);

  return null;
}

function ClerkAuthTokenBridge() {
  const { getToken } = useAuth();

  useEffect(() => {
    const provider = async () => (await getToken()) ?? null;
    registerTokenProvider(provider);
    registerProfessionalsTokenProvider(provider);
  }, [getToken]);

  return null;
}
