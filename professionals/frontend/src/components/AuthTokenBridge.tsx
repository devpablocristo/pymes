import { SharedAuthTokenBridge } from '@pymes/frontend-shared/frontendShell';
import { registerTokenProvider } from '@pymes/ts-pkg/http';
import { registerProfessionalsTokenProvider } from '../lib/api';

export function AuthTokenBridge() {
  return (
    <SharedAuthTokenBridge
      registerProviders={[registerTokenProvider, registerProfessionalsTokenProvider]}
    />
  );
}
