import type { Pool, PoolClient } from "pg";
import { FiscalError } from "./usecases/domain/fiscal.js";
import type {
  AuthorityDecision,
  FiscalClaim,
  FiscalCompletion,
  FiscalLease,
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
  FiscalExecutionRow,
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

  async claimAuthorization(
    record: FiscalRecord,
    lease: FiscalLease,
  ): Promise<FiscalClaim> {
    try {
      return await this.withOrganization(
        record.request.organization_id,
        async (client) => {
          const inserted = await client.query(
          `INSERT INTO fiscal.requests
             (organization_id,request_id,idempotency_key,payload_hash,
              correlation_id,actor_ref,delegated_actor_ref,workload_issuer,workload_subject,
              workload_request_id,workload_token_id,request,result,
              execution_state,execution_attempt,lease_token,lease_expires_at,
              dispatch_may_have_occurred,created_at,updated_at)
           VALUES (
             $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,
             'claimed',1,$14,
             clock_timestamp() + ($15::double precision * interval '1 millisecond'),
             false,now(),now()
           )
           ON CONFLICT DO NOTHING
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
            lease.token,
            lease.durationMs,
          ],
          );
          if (inserted.rowCount === 1) {
            return {
              kind: "acquired",
              recovery: "authorize",
              attempt: 1,
            };
          }

          const existing = await client.query<FiscalExecutionRow>(
            `SELECT organization_id,request_id,idempotency_key,payload_hash,
                    correlation_id,actor_ref,delegated_actor_ref,
                    workload_issuer,workload_subject,workload_request_id,
                    workload_token_id,request,result,execution_state,
                    execution_attempt,lease_token,lease_expires_at,
                    lease_expires_at > clock_timestamp() AS lease_active,
                    dispatch_may_have_occurred
               FROM fiscal.requests
              WHERE organization_id=$1
                AND (request_id=$2 OR idempotency_key=$3)
              FOR UPDATE`,
            [
              record.request.organization_id,
              record.request.request_id,
              record.idempotencyKey,
            ],
          );
          if (existing.rows.length !== 1) {
            throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
          }
          const current = existing.rows[0]!;
          if (
            current.payload_hash !== record.payloadHash ||
            current.request_id !== record.request.request_id ||
            current.idempotency_key !== record.idempotencyKey
          ) {
            throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
          }
          if (current.execution_state === "terminal") {
            return {
              kind: "stable",
              record: rowToRecord(current)!,
            };
          }
          if (current.lease_active === true) {
            return { kind: "busy" };
          }

          const claimed = await client.query<FiscalExecutionRow>(
            `UPDATE fiscal.requests
                SET execution_state='claimed',
                    execution_attempt=execution_attempt+1,
                    lease_token=$4,
                    lease_expires_at=
                      clock_timestamp() +
                      ($5::double precision * interval '1 millisecond'),
                    updated_at=now()
              WHERE organization_id=$1
                AND request_id=$2
                AND payload_hash=$3
                AND execution_state <> 'terminal'
                AND (
                  lease_expires_at IS NULL OR
                  lease_expires_at <= clock_timestamp()
                )
            RETURNING organization_id,request_id,idempotency_key,payload_hash,
                      correlation_id,actor_ref,delegated_actor_ref,
                      workload_issuer,workload_subject,workload_request_id,
                      workload_token_id,request,result,execution_state,
                      execution_attempt,lease_token,lease_expires_at,
                      dispatch_may_have_occurred`,
            [
              record.request.organization_id,
              record.request.request_id,
              record.payloadHash,
              lease.token,
              lease.durationMs,
            ],
          );
          const row = claimed.rows[0];
          if (row === undefined) return { kind: "busy" };
          return {
            kind: "acquired",
            recovery: row.dispatch_may_have_occurred
              ? "consult_exact"
              : "authorize",
            attempt: Number(row.execution_attempt),
          };
        },
      );
    } catch (error) {
      if (isUniqueViolation(error)) throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
      throw error;
    }
  }

  async markDispatched(
    organizationId: string,
    requestId: string,
    payloadHash: string,
    lease: FiscalLease,
    attempt: number,
  ): Promise<boolean> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query(
        `UPDATE fiscal.requests
            SET execution_state='in_progress',
                dispatch_may_have_occurred=true,
                lease_expires_at=
                  clock_timestamp() +
                  ($5::double precision * interval '1 millisecond'),
                updated_at=now()
          WHERE organization_id=$1
            AND request_id=$2
            AND payload_hash=$3
            AND lease_token=$4
            AND execution_attempt=$6
            AND lease_expires_at > clock_timestamp()
            AND execution_state='claimed'
        RETURNING request_id`,
        [
          organizationId,
          requestId,
          payloadHash,
          lease.token,
          lease.durationMs,
          attempt,
        ],
      );
      return result.rowCount === 1;
    });
  }

  async completeAuthorization(
    record: FiscalRecord,
    leaseToken: string,
    attempt: number,
  ): Promise<FiscalCompletion> {
    return this.withOrganization(
      record.request.organization_id,
      async (client) => {
        const updated = await client.query<FiscalRecordRow>(
          `UPDATE fiscal.requests
              SET result=$4,
                  execution_state=CASE
                    WHEN $5 IN ('authorized','rejected') THEN 'terminal'
                    ELSE 'uncertain'
                  END,
                  lease_token=NULL,
                  lease_expires_at=NULL,
                  updated_at=now()
            WHERE organization_id=$1
              AND request_id=$2
              AND payload_hash=$3
              AND lease_token=$6
              AND execution_attempt=$7
              AND execution_state IN ('claimed','in_progress')
          RETURNING idempotency_key,payload_hash,correlation_id,actor_ref,
                    delegated_actor_ref,workload_issuer,workload_subject,
                    workload_request_id,workload_token_id,request,result`,
          [
            record.request.organization_id,
            record.request.request_id,
            record.payloadHash,
            record.result,
            record.result.status,
            leaseToken,
            attempt,
          ],
        );
        const stored = rowToRecord(updated.rows[0]);
        if (stored !== undefined) return { stored: true, record: stored };

        const current = await client.query<FiscalRecordRow>(
          `SELECT idempotency_key,payload_hash,correlation_id,actor_ref,
                  delegated_actor_ref,workload_issuer,workload_subject,
                  workload_request_id,workload_token_id,request,result
             FROM fiscal.requests
            WHERE organization_id=$1 AND request_id=$2
            FOR UPDATE`,
          [record.request.organization_id, record.request.request_id],
        );
        const currentRecord = rowToRecord(current.rows[0]);
        if (currentRecord === undefined) {
          throw new FiscalError("INTERNAL_ERROR");
        }
        if (currentRecord.payloadHash !== record.payloadHash) {
          throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
        }
        return { stored: false, record: currentRecord };
      },
    );
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
