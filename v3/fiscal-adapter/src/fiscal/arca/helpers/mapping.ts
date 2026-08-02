import type { FiscalRequest } from "../../usecases/domain/fiscal.js";
import { FiscalError } from "../../usecases/domain/fiscal.js";
import type {
  SDKInvoiceDetail,
  SDKInvoiceRequest,
} from "../models/sdk.js";
import {
  addDecimals,
  centsToNumber,
  decimalNumber,
  multiplyRateHalfUp,
} from "./decimal.js";

const voucherTypes: Record<FiscalRequest["document_type"], number> = {
  FA: 1,
  NDA: 2,
  NCA: 3,
  FB: 6,
  NDB: 7,
  NCB: 8,
  FC: 11,
  NDC: 12,
  NCC: 13,
};

export const supportedVoucherTypes: readonly number[] = Object.freeze(
  Object.values(voucherTypes),
);

const documentTypes: Record<string, number> = {
  CUIT: 80,
  CUIL: 86,
  CDI: 87,
  LE: 89,
  LC: 90,
  CI_EXTRANJERA: 91,
  EN_TRAMITE: 92,
  ACTA_NACIMIENTO: 93,
  PASAPORTE: 94,
  CI_BS_AS_RNP: 95,
  DNI: 96,
  CONSUMIDOR_FINAL: 99,
};

const vatConditions: Record<string, number> = {
  RESPONSABLE_INSCRIPTO: 1,
  REGISTERED: 1,
  EXENTO: 4,
  EXEMPT: 4,
  CONSUMIDOR_FINAL: 5,
  FINAL_CONSUMER: 5,
  MONOTRIBUTISTA: 6,
  MONOTAX: 6,
  NO_CATEGORIZADO: 7,
  PROVEEDOR_EXTERIOR: 8,
  CLIENTE_EXTERIOR: 9,
  IVA_LIBERADO: 10,
  MONOTRIBUTO_SOCIAL: 13,
  IVA_NO_ALCANZADO: 15,
  MONOTRIBUTO_INDEPENDIENTE_PROMOVIDO: 16,
};

const vatRateIds: Record<string, number> = {
  "0": 3,
  "2.5": 9,
  "5": 8,
  "10.5": 4,
  "21": 5,
  "27": 6,
};

const currencies: Record<string, string> = {
  ARS: "PES",
  USD: "DOL",
  EUR: "060",
};

const concepts = {
  products: 1,
  services: 2,
  products_and_services: 3,
} as const;

export function voucherType(request: FiscalRequest): number {
  return voucherTypes[request.document_type];
}

export function mapFiscalRequest(request: FiscalRequest): SDKInvoiceRequest {
  const concept = request.concept;
  const detail: SDKInvoiceDetail = {
    Concepto: concepts[concept],
    DocTipo: mappedCode(documentTypes, request.recipient.document_type, "document type"),
    DocNro: documentNumber(request.recipient.document_number),
    CbteDesde: request.voucher_number,
    CbteHasta: request.voucher_number,
    CbteFch: arcaDate(request.issue_date),
    ImpTotal: decimalNumber(request.totals.total),
    ImpTotConc: 0,
    ImpNeto: decimalNumber(request.totals.net),
    ImpOpEx: decimalNumber(request.totals.exempt),
    ImpTrib: 0,
    ImpIVA: decimalNumber(request.totals.vat),
    MonId: mappedCode(currencies, request.currency, "currency"),
    MonCotiz:
      request.currency === "ARS"
        ? 1
        : decimalNumber(request.exchange_rate!, 6),
    CondicionIVAReceptorId: mappedCode(
      vatConditions,
      request.recipient.vat_condition,
      "VAT condition",
    ),
    ...(request.currency === "ARS" ? {} : { CanMisMonExt: "S" as const }),
  };

  if (concept !== "products") {
    const period = request.service_period;
    if (period === undefined) throw new FiscalError("VALIDATION_ERROR");
    detail.FchServDesde = arcaDate(period.from);
    detail.FchServHasta = arcaDate(period.to);
    detail.FchVtoPago = arcaDate(period.payment_due);
  }

  if (request.document_type.endsWith("C")) {
    if (decimalNumber(request.totals.vat) !== 0) {
      throw new FiscalError("VALIDATION_ERROR", "type C cannot discriminate VAT");
    }
  } else {
    detail.Iva = mapVAT(request);
  }

  if (request.associated_voucher !== undefined) {
    detail.CbtesAsoc = [
      {
        Tipo: voucherTypes[request.associated_voucher.document_type],
        PtoVta: request.associated_voucher.point_of_sale,
        Nro: request.associated_voucher.voucher_number,
        CbteFch: arcaDate(request.associated_voucher.issue_date),
      },
    ];
  }

  return {
    PtoVta: request.point_of_sale,
    CbteTipo: voucherType(request),
    invoices: [detail],
  };
}

function mapVAT(
  request: FiscalRequest,
): Array<{ Id: number; BaseImp: number; Importe: number }> {
  const grouped = new Map<string, string[]>();
  for (const line of request.lines) {
    const rate = normalizeDecimal(line.vat_rate);
    const values = grouped.get(rate) ?? [];
    values.push(line.net);
    grouped.set(rate, values);
  }
  let calculatedVAT = 0n;
  const result = [...grouped.entries()].map(([rate, values]) => {
    const base = addDecimals(values);
    const vat = multiplyRateHalfUp(base, rate);
    calculatedVAT += vat;
    return {
      Id: mappedCode(vatRateIds, rate, "VAT rate"),
      BaseImp: centsToNumber(base),
      Importe: centsToNumber(vat),
    };
  });
  if (calculatedVAT !== addDecimals([request.totals.vat])) {
    throw new FiscalError(
      "VALIDATION_ERROR",
      "VAT total does not match deterministic ARCA rounding",
    );
  }
  return result;
}

function documentNumber(value: string): number {
  if (!/^\d{1,11}$/.test(value)) {
    throw new FiscalError("VALIDATION_ERROR", "invalid recipient document");
  }
  const number = Number(value);
  if (!Number.isSafeInteger(number)) {
    throw new FiscalError("VALIDATION_ERROR", "recipient document is unsafe");
  }
  return number;
}

function arcaDate(value: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    throw new FiscalError("VALIDATION_ERROR", "invalid date");
  }
  return value.replaceAll("-", "");
}

function mappedCode<T>(
  values: Record<string, T>,
  input: string,
  field: string,
): T {
  const value = values[input.trim().toUpperCase()];
  if (value === undefined) {
    throw new FiscalError("VALIDATION_ERROR", `unsupported ${field}`);
  }
  return value;
}

function normalizeDecimal(value: string): string {
  return value.includes(".")
    ? value.replace(/0+$/, "").replace(/\.$/, "")
    : value;
}
