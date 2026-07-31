import type { AuthorityDecision } from "./authority.js";

export interface MockAuthorityStore {
  find(voucherKey: string): Promise<AuthorityDecision | undefined>;
  saveDecision(voucherKey: string, decision: AuthorityDecision): Promise<void>;
}
