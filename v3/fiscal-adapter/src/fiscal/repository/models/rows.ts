import type {
  FiscalRequest,
  FiscalResult,
} from "../../usecases/domain/fiscal.js";

export interface FiscalRecordRow {
  organization_id?: string;
  request_id?: string;
  idempotency_key: string;
  payload_hash: string;
  correlation_id: string;
  actor_ref: string | null;
  delegated_actor_ref: string | null;
  workload_issuer: string;
  workload_subject: string;
  workload_request_id: string;
  workload_token_id: string;
  request: FiscalRequest;
  result: FiscalResult;
}

export interface FiscalExecutionRow extends FiscalRecordRow {
  organization_id: string;
  request_id: string;
  execution_state: "claimed" | "in_progress" | "uncertain" | "terminal";
  execution_attempt: string | number;
  lease_token: string | null;
  lease_expires_at: Date | string | null;
  lease_active?: boolean;
  dispatch_may_have_occurred: boolean;
}

export interface FiscalMetricsRow {
  authorized: number;
  rejected: number;
  uncertain: number;
  not_found: number;
}
