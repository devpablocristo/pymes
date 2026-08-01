export interface CSRRequestDTO {
  cuit: string;
  environment: "homologation" | "production";
  legal_name: string;
  common_name: string;
}

export interface CertificateUploadDTO {
  certificate_pem: string;
  expected_version: number;
}

export interface PointOfSaleDTO {
  enabled: boolean;
}

export interface CredentialDTO {
  id: string;
  organization_id: string;
  cuit: string;
  environment: "homologation" | "production";
  legal_name: string;
  common_name: string;
  status: "pending_certificate" | "ready" | "disabled" | "expired";
  certificate_fingerprint?: string;
  certificate_valid_from?: string;
  certificate_expires_at?: string;
  certificate_serial_number?: string;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface CSRResultDTO {
  credential: CredentialDTO;
  csr_pem: string;
}

export interface PointOfSaleResultDTO {
  organization_id: string;
  credential_id: string;
  environment: "homologation" | "production";
  number: number;
  enabled: boolean;
  validated_at?: string;
}
