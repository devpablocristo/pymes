import type {
  DocumentType,
  FiscalEnvironment,
} from "../../usecases/domain/fiscal.js";

export interface FiscalRequestDTO {
  request_id: string;
  organization_id: string;
  idempotency_key: string;
  correlation_id: string;
  source_version: number;
  credential_ref: string;
  environment: FiscalEnvironment;
  point_of_sale: number;
  document_type: DocumentType;
  voucher_number: number;
  issue_date: string;
  concept: "products" | "services" | "products_and_services";
  service_period?: {
    from: string;
    to: string;
    payment_due: string;
  };
  currency: string;
  exchange_rate?: string;
  totals: {
    net: string;
    vat: string;
    exempt: string;
    total: string;
  };
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

export interface ProblemDTO {
  code: string;
  title: string;
  correlation_id: string;
}

export interface DocumentTypeDTO {
  code: DocumentType;
  letter: string | undefined;
  kind: "credit_note" | "debit_note" | "invoice";
}
