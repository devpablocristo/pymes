import type {
  FiscalRequest,
  FiscalResult,
} from "../../usecases/domain/fiscal.js";

export interface FiscalRecordRow {
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

export interface FiscalMetricsRow {
  authorized: number;
  rejected: number;
  uncertain: number;
  not_found: number;
}
