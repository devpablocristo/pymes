import type { AuthorityDecision } from "../ports/authority.js";
import type { MockAuthorityStore } from "../ports/mock-authority-store.js";

export class InMemoryMockAuthorityStore implements MockAuthorityStore {
  private readonly issued = new Map<string, AuthorityDecision>();

  async find(voucherKey: string): Promise<AuthorityDecision | undefined> {
    const decision = this.issued.get(voucherKey);
    return decision === undefined ? undefined : structuredClone(decision);
  }

  async saveDecision(voucherKey: string, decision: AuthorityDecision): Promise<void> {
    this.issued.set(voucherKey, structuredClone(decision));
  }
}
