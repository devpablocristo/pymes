import type { FiscalLedger, FiscalRecord } from "./usecases.js";
import type { InMemoryFiscalState } from "./in_memory_ledger/models/state.js";
import { cloneRecord } from "./in_memory_ledger/helpers/records.js";

export class InMemoryFiscalLedger implements FiscalLedger {
  private readonly state: InMemoryFiscalState = {
    byIdempotency: new Map(),
    byRequest: new Map(),
  };

  async findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined> {
    return cloneRecord(
      this.state.byIdempotency.get(`${organizationId}|${idempotencyKey}`),
    );
  }

  async findByRequest(organizationId: string, requestId: string): Promise<FiscalRecord | undefined> {
    return cloneRecord(this.state.byRequest.get(`${organizationId}|${requestId}`));
  }

  async save(record: FiscalRecord): Promise<void> {
    const existing = this.state.byRequest.get(
      `${record.request.organization_id}|${record.request.request_id}`,
    );
    const copy = cloneRecord(
      existing === undefined ? record : { ...record, audit: existing.audit },
    )!;
    this.state.byIdempotency.set(
      `${record.request.organization_id}|${record.idempotencyKey}`,
      copy,
    );
    this.state.byRequest.set(
      `${record.request.organization_id}|${record.request.request_id}`,
      copy,
    );
  }
}
