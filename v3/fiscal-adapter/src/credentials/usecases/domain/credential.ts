export const credentialEnvironments = ["homologation", "production"] as const;
export const credentialStatuses = [
  "pending_certificate",
  "ready",
  "disabled",
  "expired",
] as const;

export type CredentialEnvironment = (typeof credentialEnvironments)[number];
export type CredentialStatus = (typeof credentialStatuses)[number];

export interface CredentialMetadata {
  id: string;
  organizationId: string;
  cuit: string;
  environment: CredentialEnvironment;
  legalName: string;
  commonName: string;
  status: CredentialStatus;
  certificateFingerprint?: string;
  certificateValidFrom?: string;
  certificateExpiresAt?: string;
  certificateSerialNumber?: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface PointOfSale {
  organizationId: string;
  credentialId: string;
  environment: CredentialEnvironment;
  number: number;
  enabled: boolean;
  validatedAt?: string;
}

export type CredentialErrorCode =
  | "CREDENTIAL_NOT_FOUND"
  | "CREDENTIAL_NOT_READY"
  | "CREDENTIAL_VERSION_CONFLICT"
  | "IDEMPOTENCY_KEY_REUSED"
  | "CERTIFICATE_INVALID"
  | "CERTIFICATE_CUIT_MISMATCH"
  | "CERTIFICATE_KEY_MISMATCH"
  | "CERTIFICATE_ENVIRONMENT_MISMATCH"
  | "CERTIFICATE_EXPIRED"
  | "HOMOLOGATION_REQUIRED"
  | "POINT_OF_SALE_NOT_ENABLED"
  | "POINT_OF_SALE_NOT_VALIDATED"
  | "VALIDATION_ERROR";

export class CredentialError extends Error {
  constructor(
    readonly code: CredentialErrorCode,
    message: string = code,
  ) {
    super(message);
    this.name = "CredentialError";
  }
}

export function normalizeCUIT(value: string): string {
  const normalized = value.replace(/\D/g, "");
  if (!validCUIT(normalized)) {
    throw new CredentialError("VALIDATION_ERROR", "invalid CUIT");
  }
  return normalized;
}

export function validCUIT(value: string): boolean {
  if (!/^\d{11}$/.test(value)) return false;
  const weights = [5, 4, 3, 2, 7, 6, 5, 4, 3, 2];
  const sum = weights.reduce(
    (total, weight, index) => total + Number(value[index]) * weight,
    0,
  );
  const remainder = 11 - (sum % 11);
  const checkDigit = remainder === 11 ? 0 : remainder === 10 ? 9 : remainder;
  return checkDigit === Number(value[10]);
}

export function validateCredentialName(value: string, field: string): string {
  const normalized = value.normalize("NFKC").trim();
  if (
    normalized.length < 1 ||
    normalized.length > 120 ||
    /[\u0000-\u001f\u007f]/.test(normalized)
  ) {
    throw new CredentialError("VALIDATION_ERROR", `invalid ${field}`);
  }
  return normalized;
}

export function validateCredentialReference(value: string): string {
  if (!/^fcred_[A-Za-z0-9_-]{8,80}$/.test(value)) {
    throw new CredentialError("VALIDATION_ERROR", "invalid credential reference");
  }
  return value;
}

export function validatePointOfSale(value: number): number {
  if (!Number.isSafeInteger(value) || value < 1 || value > 99999) {
    throw new CredentialError("VALIDATION_ERROR", "invalid point of sale");
  }
  return value;
}
