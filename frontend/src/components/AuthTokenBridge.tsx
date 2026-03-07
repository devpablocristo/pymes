import { SharedAuthTokenBridge } from '../shared/frontendShell';
import { registerTokenProvider } from '@pymes/ts-pkg/http';
import { registerProfessionalsTokenProvider } from '../lib/professionalsApi';

export function AuthTokenBridge() {
  return (
    <SharedAuthTokenBridge
      registerProviders={[registerTokenProvider, registerProfessionalsTokenProvider]}
    />
  );
}
