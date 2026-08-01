import type { Pool, PoolClient } from "pg";
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
    const result = await this.pool.query("SELECT * FROM fiscal.request_metrics()");
    return result.rows[0] as FiscalRuntimeMetrics;
  }

  async findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query(
        `SELECT idempotency_key,payload_hash,correlation_id,actor_ref,delegated_actor_ref,
                workload_issuer,workload_subject,workload_request_id,workload_token_id,request,result
           FROM fiscal.requests
          WHERE organization_id=$1 AND idempotency_key=$2`,
        [organizationId, idempotencyKey],
      );
      return rowToRecord(result.rows[0]);
    });
  }

  async findByRequest(organizationId: string, requestId: string): Promise<FiscalRecord | undefined> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query(
        `SELECT idempotency_key,payload_hash,correlation_id,actor_ref,delegated_actor_ref,
                workload_issuer,workload_subject,workload_request_id,workload_token_id,request,result
           FROM fiscal.requests
          WHERE organization_id=$1 AND request_id=$2`,
        [organizationId, requestId],
      );
      return rowToRecord(result.rows[0]);
    });
  }

  async save(record: FiscalRecord): Promise<void> {
    try {
      const result = await this.withOrganization(
        record.request.organization_id,
        (client) => client.query(
          `INSERT INTO fiscal.requests
             (organization_id,request_id,idempotency_key,payload_hash,
              correlation_id,actor_ref,delegated_actor_ref,workload_issuer,workload_subject,
              workload_request_id,workload_token_id,request,result,created_at,updated_at)
           VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,now(),now())
           ON CONFLICT (organization_id,request_id) DO UPDATE
             SET result=EXCLUDED.result,updated_at=now()
           WHERE fiscal.requests.payload_hash=EXCLUDED.payload_hash
           RETURNING request_id`,
          [
            record.request.organization_id,
            record.request.request_id,
            record.idempotencyKey,
            record.payloadHash,
            record.audit.correlationId,
            record.audit.actorRef ?? null,
            record.audit.delegatedActorRef ?? null,
            record.audit.workloadIssuer,
            record.audit.workloadSubject,
            record.audit.workloadRequestId,
            record.audit.workloadTokenId,
            record.request,
            record.result,
          ],
        ),
      );
      if (result.rowCount !== 1) throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
    } catch (error) {
      if (isUniqueViolation(error)) throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
      throw error;
    }
  }

  private async withOrganization<T>(
    organizationId: string,
    operation: (client: PoolClient) => Promise<T>,
  ): Promise<T> {
    if (!opaqueReference(organizationId)) throw new FiscalError("VALIDATION_ERROR");
    const client = await this.pool.connect();
    try {
      await client.query("BEGIN");
      await client.query("SELECT set_config('app.organization_id',$1,true)", [organizationId]);
      const result = await operation(client);
      await client.query("COMMIT");
      return result;
    } catch (error) {
      await client.query("ROLLBACK").catch(() => undefined);
      throw error;
    } finally {
      client.release();
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
    audit: {
      correlationId: row.correlation_id as string,
      ...(row.actor_ref === null || row.actor_ref === undefined ? {} : { actorRef: row.actor_ref as string }),
      ...(row.delegated_actor_ref === null || row.delegated_actor_ref === undefined
        ? {}
        : { delegatedActorRef: row.delegated_actor_ref as string }),
      workloadIssuer: row.workload_issuer as string,
      workloadSubject: row.workload_subject as string,
      workloadRequestId: row.workload_request_id as string,
      workloadTokenId: row.workload_token_id as string,
    },
    request: row.request as FiscalRecord["request"],
    result: row.result as FiscalRecord["result"],
  };
}

function isUniqueViolation(error: unknown): boolean {
  return typeof error === "object" && error !== null && "code" in error && (error as { code: unknown }).code === "23505";
}

function opaqueReference(value: unknown): value is string {
  return typeof value === "string" && /^[A-Za-z0-9:_./-]{1,255}$/.test(value);
}
