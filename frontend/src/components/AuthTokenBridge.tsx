import { SharedAuthTokenBridge } from '../shared/frontendShell';
import { registerTokenProvider } from '@devpablocristo/core-authn/http/fetch';
import { registerTeachersTokenProvider } from '../lib/teachersApi';

export function AuthTokenBridge() {
  return (
    <SharedAuthTokenBridge
      registerProviders={[registerTokenProvider, registerTeachersTokenProvider]}
    />
  );
}
