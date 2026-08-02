import { CredentialError } from "../../usecases/domain/credential.js";

export function printableDN(value: string, field: string): string {
  const normalized = value
    .normalize("NFKD")
    .replace(/\p{M}/gu, "")
    .replace(/[^\x20-\x7e]/g, "")
    .trim();
  if (normalized.length < 1 || normalized.length > 120) {
    throw new CredentialError("VALIDATION_ERROR", `invalid ${field}`);
  }
  return normalized;
}
