import type { FiscalRecord } from "../../usecases.js";

export interface InMemoryFiscalExecution {
  record: FiscalRecord;
  state: "claimed" | "in_progress" | "uncertain" | "terminal";
  attempt: number;
  leaseToken?: string;
  leaseExpiresAt?: number;
  dispatchMayHaveOccurred: boolean;
}

export interface InMemoryFiscalState {
  byIdempotency: Map<string, InMemoryFiscalExecution>;
  byRequest: Map<string, InMemoryFiscalExecution>;
}
