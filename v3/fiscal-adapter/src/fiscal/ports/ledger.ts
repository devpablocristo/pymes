import type { FiscalRequest, FiscalResult } from "../domain/fiscal.js";

export interface FiscalRecord {
  idempotencyKey: string;
  payloadHash: string;
  request: FiscalRequest;
  result: FiscalResult;
}

export interface FiscalLedger {
  findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined>;
  findByRequest(organizationId: string, requestId: string): Promise<FiscalRecord | undefined>;
  save(record: FiscalRecord): Promise<void>;
}
