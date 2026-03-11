import { SharedAuthTokenBridge } from '../shared/frontendShell';
import { registerTokenProvider } from '@pymes/ts-pkg/http';
import { registerTeachersTokenProvider } from '../lib/teachersApi';

export function AuthTokenBridge() {
  return (
    <SharedAuthTokenBridge
      registerProviders={[registerTokenProvider, registerTeachersTokenProvider]}
    />
  );
}
