import type { Pool, PoolClient } from "pg";
import { FiscalError } from "./usecases/domain/fiscal.js";
import type {
  AuthorityDecision,
  FiscalLedger,
  FiscalRecord,
} from "./usecases.js";
import type {
  FiscalRuntimeMetrics,
  FiscalRuntimeObserver,
} from "./handler.js";
import type { MockAuthorityStore } from "./mock_authority.js";
import type {
  FiscalMetricsRow,
  FiscalRecordRow,
} from "./repository/models/rows.js";
import {
  isUniqueViolation,
  rowToRecord,
} from "./repository/helpers/mappers.js";
import {
  opaqueReference,
  voucherOrganization,
} from "./repository/helpers/tenancy.js";

export class PostgresFiscalStore implements FiscalLedger, MockAuthorityStore, FiscalRuntimeObserver {
  constructor(private readonly pool: Pool) {}

  async ping(): Promise<void> {
    await this.pool.query("SELECT 1");
  }

  async metrics(): Promise<FiscalRuntimeMetrics> {
    const result = await this.pool.query<FiscalMetricsRow>(
      "SELECT * FROM fiscal.request_metrics()",
    );
    return result.rows[0]!;
  }

  async findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query<FiscalRecordRow>(
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
      const result = await client.query<FiscalRecordRow>(
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
    const organizationId = voucherOrganization(voucherKey);
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query(
        `SELECT decision
           FROM fiscal.mock_authorizations
          WHERE organization_id=$1 AND voucher_key=$2`,
        [organizationId, voucherKey],
      );
      return result.rows[0]?.decision as AuthorityDecision | undefined;
    });
  }

  async saveDecision(voucherKey: string, decision: AuthorityDecision): Promise<void> {
    const organizationId = voucherOrganization(voucherKey);
    await this.withOrganization(organizationId, (client) =>
      client.query(
        `INSERT INTO fiscal.mock_authorizations
           (organization_id,voucher_key,decision,created_at,updated_at)
         VALUES($1,$2,$3,now(),now())
         ON CONFLICT(organization_id,voucher_key)
         DO UPDATE SET decision=EXCLUDED.decision,updated_at=now()`,
        [organizationId, voucherKey, decision],
      ),
    );
  }
}
