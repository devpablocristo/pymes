import type { FiscalRecord } from "../../usecases.js";
import type { FiscalRecordRow } from "../models/rows.js";

export function rowToRecord(
  row: FiscalRecordRow | undefined,
): FiscalRecord | undefined {
  if (row === undefined) return undefined;
  return {
    idempotencyKey: row.idempotency_key,
    payloadHash: row.payload_hash,
    audit: {
      correlationId: row.correlation_id,
      ...(row.actor_ref === null ? {} : { actorRef: row.actor_ref }),
      ...(row.delegated_actor_ref === null
        ? {}
        : { delegatedActorRef: row.delegated_actor_ref }),
      workloadIssuer: row.workload_issuer,
      workloadSubject: row.workload_subject,
      workloadRequestId: row.workload_request_id,
      workloadTokenId: row.workload_token_id,
    },
    request: row.request,
    result: row.result,
  };
}

export function isUniqueViolation(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    (error as { code: unknown }).code === "23505"
  );
}
