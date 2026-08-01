export interface CredentialRow {
  organization_id: string;
  credential_id: string;
  cuit: string;
  environment: "homologation" | "production";
  legal_name: string;
  common_name: string;
  status: "pending_certificate" | "ready" | "disabled" | "expired";
  idempotency_key: string;
  request_hash: string;
  csr_pem: string;
  encrypted_private_key: unknown;
  encrypted_certificate: unknown | null;
  certificate_fingerprint: string | null;
  certificate_valid_from: Date | string | null;
  certificate_expires_at: Date | string | null;
  certificate_serial_number: string | null;
  version: number;
  created_at: Date | string;
  updated_at: Date | string;
}

export interface PointOfSaleRow {
  organization_id: string;
  credential_id: string;
  environment: "homologation" | "production";
  point_of_sale: number;
  enabled: boolean;
  validated_at: Date | string | null;
}

export interface AccessTicketRow {
  organization_id: string;
  credential_id: string;
  environment: "homologation" | "production";
  service: string;
  encrypted_ticket: unknown;
  expires_at: Date | string;
}
