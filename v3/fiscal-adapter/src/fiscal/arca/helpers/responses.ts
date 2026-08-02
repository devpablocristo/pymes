import type { AuthorityDecision } from "../../usecases.js";
import type { FiscalRequest } from "../../usecases/domain/fiscal.js";
import { FiscalError } from "../../usecases/domain/fiscal.js";
import type {
  SDKAuthorizationResponse,
  SDKConsultResponse,
  SDKError,
} from "../models/sdk.js";
import { mapFiscalRequest } from "./mapping.js";

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
  const expectedRequest = mapFiscalRequest(request);
  const expected = expectedRequest.invoices[0]!;
  if (
    (result.PtoVta !== undefined && result.PtoVta !== request.point_of_sale) ||
    (result.CbteTipo !== undefined &&
      result.CbteTipo !== expectedRequest.CbteTipo) ||
    result.Concepto !== expected.Concepto ||
    result.CbteDesde !== expected.CbteDesde ||
    result.CbteHasta !== expected.CbteHasta ||
    result.CbteFch !== expected.CbteFch ||
    result.DocTipo !== expected.DocTipo ||
    result.DocNro !== expected.DocNro ||
    result.ImpTotal !== expected.ImpTotal ||
    result.ImpTotConc !== expected.ImpTotConc ||
    result.ImpNeto !== expected.ImpNeto ||
    result.ImpOpEx !== expected.ImpOpEx ||
    result.ImpTrib !== expected.ImpTrib ||
    result.ImpIVA !== expected.ImpIVA ||
    result.FchServDesde !== (expected.FchServDesde ?? "") ||
    result.FchServHasta !== (expected.FchServHasta ?? "") ||
    result.FchVtoPago !== (expected.FchVtoPago ?? "") ||
    result.MonId !== expected.MonId ||
    result.MonCotiz !== expected.MonCotiz ||
    result.EmisionTipo !== "CAE" ||
    !sameVATBreakdown(result.Iva, expected.Iva) ||
    !sameAssociatedVouchers(result.CbtesAsoc, expected.CbtesAsoc) ||
    (result.CondicionIVAReceptorId !== undefined &&
      result.CondicionIVAReceptorId !== expected.CondicionIVAReceptorId) ||
    (result.CanMisMonExt !== undefined &&
      result.CanMisMonExt !== expected.CanMisMonExt)
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

function sameVATBreakdown(
  actual:
    | {
        AlicIva:
          | { Id: number; BaseImp: number; Importe: number }
          | Array<{ Id: number; BaseImp: number; Importe: number }>;
      }
    | undefined,
  expected:
    | Array<{ Id: number; BaseImp: number; Importe: number }>
    | undefined,
): boolean {
  const actualItems = arrayOf(actual?.AlicIva)
    .map((item) => `${item.Id}:${item.BaseImp}:${item.Importe}`)
    .sort();
  const expectedItems = (expected ?? [])
    .map((item) => `${item.Id}:${item.BaseImp}:${item.Importe}`)
    .sort();
  return sameStrings(actualItems, expectedItems);
}

function sameAssociatedVouchers(
  actual:
    | {
        CbteAsoc:
          | {
              Tipo: number;
              PtoVta: number;
              Nro: number;
              CbteFch?: string;
            }
          | Array<{
              Tipo: number;
              PtoVta: number;
              Nro: number;
              CbteFch?: string;
            }>;
      }
    | undefined,
  expected:
    | Array<{
        Tipo: number;
        PtoVta: number;
        Nro: number;
        CbteFch?: string;
      }>
    | undefined,
): boolean {
  const actualItems = arrayOf(actual?.CbteAsoc)
    .map(
      (item) =>
        `${item.Tipo}:${item.PtoVta}:${item.Nro}:${item.CbteFch ?? ""}`,
    )
    .sort();
  const expectedItems = (expected ?? [])
    .map(
      (item) =>
        `${item.Tipo}:${item.PtoVta}:${item.Nro}:${item.CbteFch ?? ""}`,
    )
    .sort();
  return sameStrings(actualItems, expectedItems);
}

function arrayOf<T>(value: T | T[] | undefined): T[] {
  if (value === undefined) return [];
  return Array.isArray(value) ? value : [value];
}

function sameStrings(left: string[], right: string[]): boolean {
  return (
    left.length === right.length &&
    left.every((value, index) => value === right[index])
  );
}
