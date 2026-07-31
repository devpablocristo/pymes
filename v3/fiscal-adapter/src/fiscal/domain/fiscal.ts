export const documentTypes = ["FA", "NDA", "NCA", "FB", "NDB", "NCB", "FC", "NDC", "NCC"] as const;

export type DocumentType = (typeof documentTypes)[number];
export type FiscalEnvironment = "homologation" | "production";
export type FiscalStatus = "authorized" | "rejected" | "uncertain" | "not_found";
export type FiscalErrorCode =
  | "UNAUTHORIZED_SERVICE"
  | "IDEMPOTENCY_KEY_REUSED"
  | "AUTHORITY_TIMEOUT"
  | "VALIDATION_ERROR"
  | "INTERNAL_ERROR";

export interface MoneyTotals {
  net: string;
  vat: string;
  exempt: string;
  total: string;
}

export interface FiscalRequest {
  request_id: string;
  organization_id: string;
  credential_ref: string;
  environment: FiscalEnvironment;
  point_of_sale: number;
  document_type: DocumentType;
  voucher_number: number;
  issue_date: string;
  currency: string;
  exchange_rate?: string;
  totals: MoneyTotals;
  recipient: {
    document_type: string;
    document_number: string;
    vat_condition: string;
  };
  associated_voucher?: {
    point_of_sale: number;
    document_type: DocumentType;
    voucher_number: number;
    issue_date: string;
  };
  lines: Array<{
    description: string;
    quantity: string;
    unit_price: string;
    vat_rate: string;
    net: string;
  }>;
  snapshot_digest: string;
}

export interface FiscalResult {
  request_id: string;
  organization_id: string;
  status: FiscalStatus;
  cae?: string;
  cae_expires_on?: string;
  authority_result_code?: string;
  authority_messages?: string[];
  artifact_ref?: string;
  snapshot_digest: string;
  observed_at: string;
  correlation_id: string;
}

export interface FiscalProblem {
  code: FiscalErrorCode;
  title: string;
  correlation_id: string;
}

export class FiscalError extends Error {
  constructor(
    readonly code: FiscalErrorCode,
    message = code,
  ) {
    super(message);
    this.name = "FiscalError";
  }
}

export function voucherKey(request: Pick<FiscalRequest, "organization_id" | "environment" | "point_of_sale" | "document_type" | "voucher_number">): string {
  return [request.organization_id, request.environment, request.point_of_sale, request.document_type, request.voucher_number].join("/");
}
