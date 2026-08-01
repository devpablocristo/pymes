import { createHash } from "node:crypto";
import type { AuthorityDecision } from "../../usecases.js";

export class InMemoryMockAuthorityStore {
  private readonly issued = new Map<string, AuthorityDecision>();

  async find(voucherKey: string): Promise<AuthorityDecision | undefined> {
    const decision = this.issued.get(voucherKey);
    return decision === undefined ? undefined : structuredClone(decision);
  }

  async saveDecision(
    voucherKey: string,
    decision: AuthorityDecision,
  ): Promise<void> {
    this.issued.set(voucherKey, structuredClone(decision));
  }
}

export function deterministicCAE(value: string): string {
  const digits = BigInt(
    `0x${createHash("sha256").update(value).digest("hex").slice(0, 14)}`,
  )
    .toString()
    .slice(0, 14);
  return digits.padStart(14, "0");
}

export function artifactReference(value: string): string {
  return `mock://fiscal/${createHash("sha256").update(value).digest("hex")}`;
}

export function addDays(isoDate: string, days: number): string {
  const date = new Date(`${isoDate}T00:00:00.000Z`);
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}
