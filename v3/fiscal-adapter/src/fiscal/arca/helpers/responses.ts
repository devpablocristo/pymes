import type { AuthorityDecision } from "../../usecases.js";
import type { FiscalRequest } from "../../usecases/domain/fiscal.js";
import { FiscalError } from "../../usecases/domain/fiscal.js";
import type {
  SDKAuthorizationResponse,
  SDKConsultResponse,
  SDKError,
} from "../models/sdk.js";
import { decimalNumber } from "./decimal.js";

export function authorizationDecision(
  response: SDKAuthorizationResponse,
): AuthorityDecision {
  const details = response.FeDetResp?.FECAEDetResponse;
  const detail = Array.isArray(details) ? details[0] : details;
  if (detail === undefined) {
    throw new FiscalError("INTERNAL_ERROR", "ARCA omitted authorization detail");
  }
  const messages = authorityMessages(
    response.Errors?.Err,
    response.Events?.Evt,
    detail.Observaciones?.Obs,
  );
  if (detail.Resultado === "A") {
    if (
      typeof detail.CAE !== "string" ||
      !/^\d{14}$/.test(detail.CAE) ||
      typeof detail.CAEFchVto !== "string"
    ) {
      throw new FiscalError("INTERNAL_ERROR", "ARCA returned an invalid CAE");
    }
    return {
      status: "authorized",
      cae: detail.CAE,
      cae_expires_on: isoARCADate(detail.CAEFchVto),
      result_code: "ARCA_A",
      ...(messages.length === 0 ? {} : { messages }),
    };
  }
  return {
    status: "rejected",
    result_code: "ARCA_R",
    messages: messages.length === 0 ? ["ARCA rejected the voucher"] : messages,
  };
}

export function consultationDecision(
  request: FiscalRequest,
  response: SDKConsultResponse,
): AuthorityDecision {
  const result = response.ResultGet;
  if (result === undefined) return { status: "not_found" };
  if (
    (result.PtoVta !== undefined && result.PtoVta !== request.point_of_sale) ||
    (result.CbteTipo !== undefined &&
      result.CbteTipo !== voucherTypeCode(request.document_type)) ||
    result.CbteDesde !== request.voucher_number ||
    result.CbteHasta !== request.voucher_number ||
    result.DocTipo !== recipientDocumentCode(request.recipient.document_type) ||
    result.DocNro !== Number(request.recipient.document_number) ||
    result.ImpTotal !== decimalNumber(request.totals.total) ||
    result.ImpNeto !== decimalNumber(request.totals.net) ||
    result.ImpOpEx !== decimalNumber(request.totals.exempt) ||
    result.ImpIVA !== decimalNumber(request.totals.vat) ||
    result.MonId !== currencyCode(request.currency) ||
    result.MonCotiz !==
      (request.currency === "ARS"
        ? 1
        : decimalNumber(request.exchange_rate!, 6))
  ) {
    return {
      status: "rejected",
      result_code: "VOUCHER_MISMATCH",
      messages: ["The existing ARCA voucher does not match the frozen snapshot"],
    };
  }
  const messages = authorityMessages(
    response.Errors?.Err,
    response.Events?.Evt,
    result.Observaciones?.Obs,
  );
  if (result.Resultado === "A") {
    if (
      typeof result.CodAutorizacion !== "string" ||
      !/^\d{14}$/.test(result.CodAutorizacion) ||
      typeof result.FchVto !== "string"
    ) {
      throw new FiscalError("INTERNAL_ERROR", "ARCA consultation omitted CAE");
    }
    return {
      status: "authorized",
      cae: result.CodAutorizacion,
      cae_expires_on: isoARCADate(result.FchVto),
      result_code: "ARCA_A",
      ...(messages.length === 0 ? {} : { messages }),
    };
  }
  return {
    status: "rejected",
    result_code: "ARCA_R",
    messages: messages.length === 0 ? ["ARCA rejected the voucher"] : messages,
  };
}

export function authorizationFailure(
  error: unknown,
  dispatched: boolean,
): AuthorityDecision {
  const providerErrors = errorList(error);
  if (providerErrors.some((item) => item.code === 10016)) {
    return {
      status: "uncertain",
      result_code: "ARCA_10016",
      messages: providerErrors.map((item) => sanitize(item.message)),
    };
  }
  if (errorName(error) === "ArcaWSFEError") {
    return {
      status: "rejected",
      result_code: providerErrors[0]?.code
        ? `ARCA_${providerErrors[0].code}`
        : "ARCA_REJECTED",
      messages:
        providerErrors.length === 0
          ? ["ARCA rejected the voucher"]
          : providerErrors.map((item) => sanitize(item.message)),
    };
  }
  if (dispatched) {
    return {
      status: "uncertain",
      result_code: "ARCA_RESPONSE_UNCERTAIN",
      messages: ["ARCA may have processed the exact voucher"],
    };
  }
  throw new FiscalError("AUTHORITY_TIMEOUT", sanitize(errorMessage(error)));
}

export function consultationFailure(error: unknown): AuthorityDecision {
  const providerErrors = errorList(error);
  if (errorName(error) === "ArcaWSFEError" && providerErrors.length > 0) {
    return {
      status: "rejected",
      result_code: `ARCA_${providerErrors[0]!.code}`,
      messages: providerErrors.map((item) => sanitize(item.message)),
    };
  }
  throw new FiscalError("AUTHORITY_TIMEOUT", sanitize(errorMessage(error)));
}

function authorityMessages(
  ...groups: Array<SDKError | SDKError[] | undefined>
): string[] {
  return groups.flatMap((group) =>
    group === undefined
      ? []
      : (Array.isArray(group) ? group : [group]).map(
          (item) => `${item.Code}: ${sanitize(item.Msg)}`,
        ),
  );
}

function errorList(error: unknown): Array<{ code: number; message: string }> {
  if (typeof error !== "object" || error === null || !("errors" in error)) {
    return [];
  }
  const errors = (error as { errors?: unknown }).errors;
  if (!Array.isArray(errors)) return [];
  return errors.flatMap((entry) => {
    if (
      typeof entry !== "object" ||
      entry === null ||
      typeof (entry as { code?: unknown }).code !== "number" ||
      typeof (entry as { msg?: unknown }).msg !== "string"
    ) {
      return [];
    }
    return [{
      code: (entry as { code: number }).code,
      message: (entry as { msg: string }).msg,
    }];
  });
}

function errorName(error: unknown): string {
  return typeof error === "object" &&
    error !== null &&
    "name" in error &&
    typeof (error as { name: unknown }).name === "string"
    ? (error as { name: string }).name
    : "";
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "ARCA unavailable";
}

function sanitize(value: string): string {
  return value
    .replace(/[\u0000-\u001f\u007f]/g, " ")
    .replace(/\b\d{8,14}\b/g, "[redacted]")
    .slice(0, 500);
}

function isoARCADate(value: string): string {
  if (!/^\d{8}$/.test(value)) throw new FiscalError("INTERNAL_ERROR");
  const iso = `${value.slice(0, 4)}-${value.slice(4, 6)}-${value.slice(6, 8)}`;
  const date = new Date(`${iso}T00:00:00.000Z`);
  if (Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== iso) {
    throw new FiscalError("INTERNAL_ERROR");
  }
  return iso;
}

function currencyCode(value: string): string {
  const values: Record<string, string> = { ARS: "PES", USD: "DOL", EUR: "060" };
  return values[value] ?? "";
}

function recipientDocumentCode(value: string): number {
  const values: Record<string, number> = {
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
  return values[value.trim().toUpperCase()] ?? -1;
}

function voucherTypeCode(value: FiscalRequest["document_type"]): number {
  return {
    FA: 1,
    NDA: 2,
    NCA: 3,
    FB: 6,
    NDB: 7,
    NCB: 8,
    FC: 11,
    NDC: 12,
    NCC: 13,
  }[value];
}
