import type { FiscalLedger, FiscalRecord } from "../ports/ledger.js";

export class InMemoryFiscalLedger implements FiscalLedger {
  private readonly byIdempotency = new Map<string, FiscalRecord>();
  private readonly byRequest = new Map<string, FiscalRecord>();

  async findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined> {
    return clone(this.byIdempotency.get(`${organizationId}|${idempotencyKey}`));
  }

  async findByRequest(organizationId: string, requestId: string): Promise<FiscalRecord | undefined> {
    return clone(this.byRequest.get(`${organizationId}|${requestId}`));
  }

  async save(record: FiscalRecord): Promise<void> {
    const existing = this.byRequest.get(`${record.request.organization_id}|${record.request.request_id}`);
    const copy = clone(existing === undefined ? record : { ...record, audit: existing.audit })!;
    this.byIdempotency.set(`${record.request.organization_id}|${record.idempotencyKey}`, copy);
    this.byRequest.set(`${record.request.organization_id}|${record.request.request_id}`, copy);
  }
}

function clone(record: FiscalRecord | undefined): FiscalRecord | undefined {
  return record === undefined ? undefined : structuredClone(record);
}
