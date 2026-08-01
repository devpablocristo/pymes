import type { FiscalRecord } from "../../usecases.js";

export interface InMemoryFiscalState {
  byIdempotency: Map<string, FiscalRecord>;
  byRequest: Map<string, FiscalRecord>;
}
