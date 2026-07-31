import { createHash } from "node:crypto";
import { FiscalError, voucherKey, type FiscalRequest } from "../domain/fiscal.js";
import type { AuthorityDecision, FiscalAuthority } from "../ports/authority.js";
import type { MockAuthorityStore } from "../ports/mock-authority-store.js";
import { InMemoryMockAuthorityStore } from "./in-memory-mock-authority-store.js";

export type MockScenario = "authorized" | "rejected" | "timeout_before_processing" | "response_lost_after_processing";

export class MockFiscalAuthority implements FiscalAuthority {
  readonly received: FiscalRequest[] = [];

  constructor(
    private readonly scenario: MockScenario = "authorized",
    private readonly store: MockAuthorityStore = new InMemoryMockAuthorityStore(),
  ) {}

  async authorize(request: FiscalRequest): Promise<AuthorityDecision> {
    this.received.push(structuredClone(request));
    const key = voucherKey(request);
    const previous = await this.store.find(key);
    if (previous !== undefined) return structuredClone(previous);

    if (this.scenario === "timeout_before_processing") {
      throw new FiscalError("AUTHORITY_TIMEOUT");
    }
    if (this.scenario === "rejected") {
      return { status: "rejected", result_code: "MOCK_REJECTED", messages: ["Rechazo fiscal simulado"] };
    }

    const authorized: AuthorityDecision = {
      status: "authorized",
      cae: deterministicCAE(key),
      cae_expires_on: addDays(request.issue_date, 10),
      result_code: "MOCK_AUTHORIZED",
      artifact_ref: `mock://fiscal/${createHash("sha256").update(key).digest("hex")}`,
    };
    await this.store.saveDecision(key, authorized);
    if (this.scenario === "response_lost_after_processing") {
      return { status: "uncertain", result_code: "MOCK_RESPONSE_LOST" };
    }
    return structuredClone(authorized);
  }

  async consult(request: FiscalRequest): Promise<AuthorityDecision> {
    return structuredClone((await this.store.find(voucherKey(request))) ?? { status: "not_found" });
  }
}

function deterministicCAE(value: string): string {
  const digits = BigInt(`0x${createHash("sha256").update(value).digest("hex").slice(0, 14)}`).toString().slice(0, 14);
  return digits.padStart(14, "0");
}

function addDays(isoDate: string, days: number): string {
  const date = new Date(`${isoDate}T00:00:00.000Z`);
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}
