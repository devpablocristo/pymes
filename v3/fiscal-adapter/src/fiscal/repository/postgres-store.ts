import type { Pool } from "pg";
import { FiscalError } from "../domain/fiscal.js";
import type { AuthorityDecision } from "../ports/authority.js";
import type { FiscalLedger, FiscalRecord } from "../ports/ledger.js";
import type { MockAuthorityStore } from "../ports/mock-authority-store.js";
import type { FiscalRuntimeMetrics, FiscalRuntimeObserver } from "../ports/runtime-observer.js";

export class PostgresFiscalStore implements FiscalLedger, MockAuthorityStore, FiscalRuntimeObserver {
  constructor(private readonly pool: Pool) {}

  async ping(): Promise<void> {
    await this.pool.query("SELECT 1");
  }

  async metrics(): Promise<FiscalRuntimeMetrics> {
    const result = await this.pool.query(`
      SELECT
        count(*) FILTER (WHERE result->>'status' = 'authorized')::int AS authorized,
        count(*) FILTER (WHERE result->>'status' = 'rejected')::int AS rejected,
        count(*) FILTER (WHERE result->>'status' = 'uncertain')::int AS uncertain,
        count(*) FILTER (WHERE result->>'status' = 'not_found')::int AS not_found
      FROM fiscal.requests`);
    return result.rows[0] as FiscalRuntimeMetrics;
  }

  async findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined> {
    const result = await this.pool.query(
      `SELECT idempotency_key,payload_hash,request,result
         FROM fiscal.requests
        WHERE organization_id=$1 AND idempotency_key=$2`,
      [organizationId, idempotencyKey],
    );
    return rowToRecord(result.rows[0]);
  }

  async findByRequest(organizationId: string, requestId: string): Promise<FiscalRecord | undefined> {
    const result = await this.pool.query(
      `SELECT idempotency_key,payload_hash,request,result
         FROM fiscal.requests
        WHERE organization_id=$1 AND request_id=$2`,
      [organizationId, requestId],
    );
    return rowToRecord(result.rows[0]);
  }

  async save(record: FiscalRecord): Promise<void> {
    try {
      const result = await this.pool.query(
        `INSERT INTO fiscal.requests
           (organization_id,request_id,idempotency_key,payload_hash,request,result,created_at,updated_at)
         VALUES ($1,$2,$3,$4,$5,$6,now(),now())
         ON CONFLICT (organization_id,request_id) DO UPDATE
           SET result=EXCLUDED.result,updated_at=now()
         WHERE fiscal.requests.payload_hash=EXCLUDED.payload_hash
         RETURNING request_id`,
        [record.request.organization_id, record.request.request_id, record.idempotencyKey, record.payloadHash, record.request, record.result],
      );
      if (result.rowCount !== 1) throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
    } catch (error) {
      if (isUniqueViolation(error)) throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
      throw error;
    }
  }

  async find(voucherKey: string): Promise<AuthorityDecision | undefined> {
    const result = await this.pool.query(`SELECT decision FROM fiscal.mock_authorizations WHERE voucher_key=$1`, [voucherKey]);
    return result.rows[0]?.decision as AuthorityDecision | undefined;
  }

  async saveDecision(voucherKey: string, decision: AuthorityDecision): Promise<void> {
    await this.pool.query(
      `INSERT INTO fiscal.mock_authorizations(voucher_key,decision,created_at,updated_at)
       VALUES($1,$2,now(),now())
       ON CONFLICT(voucher_key) DO UPDATE SET decision=EXCLUDED.decision,updated_at=now()`,
      [voucherKey, decision],
    );
  }
}

function rowToRecord(row: Record<string, unknown> | undefined): FiscalRecord | undefined {
  if (row === undefined) return undefined;
  return {
    idempotencyKey: row.idempotency_key as string,
    payloadHash: row.payload_hash as string,
    request: row.request as FiscalRecord["request"],
    result: row.result as FiscalRecord["result"],
  };
}

function isUniqueViolation(error: unknown): boolean {
  return typeof error === "object" && error !== null && "code" in error && (error as { code: unknown }).code === "23505";
}
