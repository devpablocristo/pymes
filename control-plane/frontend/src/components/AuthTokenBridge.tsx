import { SharedAuthTokenBridge } from '../shared/frontendShell';
import { registerTokenProvider } from '@pymes/ts-pkg/http';

export function AuthTokenBridge() {
  return <SharedAuthTokenBridge registerProviders={[registerTokenProvider]} />;
}
