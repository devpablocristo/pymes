import type { FiscalRequest } from "../domain/fiscal.js";

export type AuthorityDecision =
  | { status: "authorized"; cae: string; cae_expires_on: string; result_code?: string; messages?: string[]; artifact_ref?: string }
  | { status: "rejected"; result_code?: string; messages: string[] }
  | { status: "uncertain"; result_code?: string; messages?: string[] }
  | { status: "not_found" };

// FiscalAuthority is the only seam that a future real ARCA adapter may
// implement. No SDK type crosses this port.
export interface FiscalAuthority {
  authorize(request: FiscalRequest): Promise<AuthorityDecision>;
  consult(request: FiscalRequest): Promise<AuthorityDecision>;
}
