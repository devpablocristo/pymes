import { SharedAuthTokenBridge, type TokenProviderRegistrar } from '../shared/frontendAuth';
import { registerTokenProvider } from '@devpablocristo/core-authn/http/fetch';
import { registerTeachersTokenProvider } from '../lib/teachersApi';

// Referencia estable: un array nuevo en cada render disparaba useEffect en ClerkAuthTokenBridge
// y re-registraba el proveedor en bucle, pudiendo dejar /v1/session y /v1/users/me colgados.
const registerProviders: TokenProviderRegistrar[] = [registerTokenProvider, registerTeachersTokenProvider];

export function AuthTokenBridge() {
  return <SharedAuthTokenBridge registerProviders={registerProviders} />;
}
