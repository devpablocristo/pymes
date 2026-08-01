import type { FiscalRecord } from "../../usecases.js";

export function cloneRecord(
  record: FiscalRecord | undefined,
): FiscalRecord | undefined {
  return record === undefined ? undefined : structuredClone(record);
}
