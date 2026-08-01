import { CredentialError } from "../../usecases/domain/credential.js";
import type { PointOfSale } from "../../usecases/domain/credential.js";
import type {
  SealedValue,
  StoredAccessTicket,
  StoredCredential,
} from "../../usecases.js";
import type {
  AccessTicketRow,
  CredentialRow,
  PointOfSaleRow,
} from "../models/rows.js";

export function credentialFromRow(row: CredentialRow): StoredCredential {
  return {
    id: row.credential_id,
    organizationId: row.organization_id,
    cuit: row.cuit,
    environment: row.environment,
    legalName: row.legal_name,
    commonName: row.common_name,
    status: row.status,
    idempotencyKey: row.idempotency_key,
    requestHash: row.request_hash,
    csrPem: row.csr_pem,
    encryptedPrivateKey: sealedValue(row.encrypted_private_key),
    ...(row.encrypted_certificate === null
      ? {}
      : { encryptedCertificate: sealedValue(row.encrypted_certificate) }),
    ...(row.certificate_fingerprint === null
      ? {}
      : { certificateFingerprint: row.certificate_fingerprint }),
    ...(row.certificate_valid_from === null
      ? {}
      : { certificateValidFrom: iso(row.certificate_valid_from) }),
    ...(row.certificate_expires_at === null
      ? {}
      : { certificateExpiresAt: iso(row.certificate_expires_at) }),
    ...(row.certificate_serial_number === null
      ? {}
      : { certificateSerialNumber: row.certificate_serial_number }),
    version: row.version,
    createdAt: iso(row.created_at),
    updatedAt: iso(row.updated_at),
  };
}

export function pointOfSaleFromRow(row: PointOfSaleRow): PointOfSale {
  return {
    organizationId: row.organization_id,
    credentialId: row.credential_id,
    environment: row.environment,
    number: row.point_of_sale,
    enabled: row.enabled,
    ...(row.validated_at === null
      ? {}
      : { validatedAt: iso(row.validated_at) }),
  };
}

export function accessTicketFromRow(row: AccessTicketRow): StoredAccessTicket {
  return {
    organizationId: row.organization_id,
    credentialId: row.credential_id,
    environment: row.environment,
    service: row.service,
    encryptedTicket: sealedValue(row.encrypted_ticket),
    expiresAt: iso(row.expires_at),
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

function sealedValue(value: unknown): SealedValue {
  if (
    typeof value !== "object" ||
    value === null ||
    (value as { format?: unknown }).format !== "aes-256-gcm+kms-v1"
  ) {
    throw new CredentialError("CREDENTIAL_NOT_READY", "invalid encrypted material");
  }
  const candidate = value as Partial<SealedValue>;
  for (const field of [
    "ciphertext",
    "encryptedDataKey",
    "iv",
    "authTag",
    "kmsKeyName",
  ] as const) {
    if (typeof candidate[field] !== "string" || candidate[field]!.length < 1) {
      throw new CredentialError("CREDENTIAL_NOT_READY", "invalid encrypted material");
    }
  }
  return candidate as SealedValue;
}

function iso(value: Date | string): string {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    throw new CredentialError("CREDENTIAL_NOT_READY", "invalid persisted timestamp");
  }
  return date.toISOString();
}
