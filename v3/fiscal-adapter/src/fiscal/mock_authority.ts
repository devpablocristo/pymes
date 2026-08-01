import {
  FiscalError,
  voucherKey,
  type FiscalRequest,
} from "./usecases/domain/fiscal.js";
import type { AuthorityDecision, FiscalAuthority } from "./usecases.js";
import type {
  CredentialProbe,
  CredentialProbeInput,
} from "../credentials/usecases.js";
import type { MockScenario } from "./mock_authority/models/scenario.js";
import {
  addDays,
  artifactReference,
  deterministicCAE,
  InMemoryMockAuthorityStore,
} from "./mock_authority/helpers/decisions.js";

export type { MockScenario } from "./mock_authority/models/scenario.js";

export interface MockAuthorityStore {
  find(voucherKey: string): Promise<AuthorityDecision | undefined>;
  saveDecision(
    voucherKey: string,
    decision: AuthorityDecision,
  ): Promise<void>;
}

export class MockFiscalAuthority
  implements FiscalAuthority, CredentialProbe
{
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
      artifact_ref: artifactReference(key),
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

  async validatePointOfSale(
    _input: CredentialProbeInput,
  ): Promise<void> {}
}
