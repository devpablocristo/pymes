import { createHash } from "node:crypto";
import {
  documentTypes,
  FiscalError,
  type FiscalRequest,
  type FiscalResult,
} from "../domain/fiscal.js";
import type { AuthorityDecision, FiscalAuthority } from "../ports/authority.js";
import type { FiscalLedger, FiscalRecord } from "../ports/ledger.js";

export interface RequestContext {
  idempotencyKey: string;
  correlationId: string;
}

export class FiscalService {
  constructor(
    private readonly authority: FiscalAuthority,
    private readonly ledger: FiscalLedger,
    private readonly now: () => Date = () => new Date(),
  ) {}

  async authorize(request: FiscalRequest, context: RequestContext): Promise<FiscalResult> {
    validateRequest(request);
    validateContext(context);
    const payloadHash = hashPayload(request);
    const existing = await this.findExisting(request, context.idempotencyKey, payloadHash);
    if (existing !== undefined) return structuredClone(existing.result);

    const decision = await this.authority.authorize(structuredClone(request));
    const result = this.toResult(request, decision, context.correlationId);
    await this.ledger.save({ idempotencyKey: context.idempotencyKey, payloadHash, request: structuredClone(request), result });
    return structuredClone(result);
  }

  async consult(
    organizationId: string,
    requestId: string,
    suppliedRequest: FiscalRequest | undefined,
    context: RequestContext,
  ): Promise<FiscalResult> {
    validateContext(context);
    const recorded = await this.ledger.findByRequest(organizationId, requestId);
    const request = suppliedRequest ?? recorded?.request;
    if (request === undefined || request.organization_id !== organizationId || request.request_id !== requestId) {
      throw new FiscalError("VALIDATION_ERROR");
    }
    validateRequest(request);
    const payloadHash = hashPayload(request);
    if (recorded !== undefined && recorded.payloadHash !== payloadHash) {
      throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
    }

    const decision = await this.authority.consult(structuredClone(request));
    const result = this.toResult(request, decision, context.correlationId);
    await this.ledger.save({
      idempotencyKey: recorded?.idempotencyKey ?? context.idempotencyKey,
      payloadHash,
      request: structuredClone(request),
      result,
    });
    return structuredClone(result);
  }

  private async findExisting(request: FiscalRequest, idempotencyKey: string, payloadHash: string): Promise<FiscalRecord | undefined> {
    const byKey = await this.ledger.findByIdempotency(request.organization_id, idempotencyKey);
    const byRequest = await this.ledger.findByRequest(request.organization_id, request.request_id);
    for (const record of [byKey, byRequest]) {
      if (record !== undefined && record.payloadHash !== payloadHash) {
        throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
      }
    }
    return byKey ?? byRequest;
  }

  private toResult(request: FiscalRequest, decision: AuthorityDecision, correlationId: string): FiscalResult {
    return {
      request_id: request.request_id,
      organization_id: request.organization_id,
      status: decision.status,
      ...(decision.status === "authorized" ? { cae: decision.cae, cae_expires_on: decision.cae_expires_on, artifact_ref: decision.artifact_ref } : {}),
      ...(decision.status === "rejected" || decision.status === "uncertain" ? { authority_messages: decision.messages } : {}),
      ...(decision.status !== "not_found" ? { authority_result_code: decision.result_code } : {}),
      snapshot_digest: request.snapshot_digest,
      observed_at: this.now().toISOString(),
      correlation_id: correlationId,
    };
  }
}

function validateContext(context: RequestContext): void {
  if (context.idempotencyKey.length < 8 || context.idempotencyKey.length > 128 || context.correlationId.length < 1) {
    throw new FiscalError("VALIDATION_ERROR");
  }
}

export function validateRequest(request: FiscalRequest): void {
  const currencies = new Set(["ARS", "USD", "EUR"]);
  const vatRates = new Set(["0", "2.5", "5", "10.5", "21", "27"]);
  if (
    request.request_id.length < 1 ||
    request.organization_id.length < 1 ||
    request.credential_ref.length < 1 ||
    (request.environment !== "homologation" && request.environment !== "production") ||
    !Number.isSafeInteger(request.point_of_sale) ||
    request.point_of_sale < 1 ||
    !Number.isSafeInteger(request.voucher_number) ||
    request.voucher_number < 1 ||
    !documentTypes.includes(request.document_type) ||
    !/^\d{4}-\d{2}-\d{2}$/.test(request.issue_date) ||
    !currencies.has(request.currency) ||
    !/^[a-f0-9]{64}$/i.test(request.snapshot_digest) ||
    request.lines.length < 1 ||
    request.recipient.document_type.length < 1 ||
    request.recipient.document_number.length < 1 ||
    request.recipient.vat_condition.length < 1
  ) {
    throw new FiscalError("VALIDATION_ERROR");
  }
  const amounts = [request.totals.net, request.totals.vat, request.totals.exempt, request.totals.total];
  if (
    amounts.some((value) => !isDecimal(value)) ||
    request.lines.some((line) =>
      !line.description ||
      !isDecimal(line.quantity) ||
      isZeroDecimal(line.quantity) ||
      !isDecimal(line.unit_price) ||
      !isDecimal(line.vat_rate) ||
      !vatRates.has(normalizeDecimal(line.vat_rate)) ||
      !isDecimal(line.net)
    )
  ) {
    throw new FiscalError("VALIDATION_ERROR");
  }
  const note = request.document_type.startsWith("NC") || request.document_type.startsWith("ND");
  const expectedAssociatedType = `F${request.document_type.at(-1)}`;
  const hasValidExchangeRate = request.exchange_rate !== undefined && isDecimal(request.exchange_rate) && !isZeroDecimal(request.exchange_rate);
  const associated = request.associated_voucher;
  if (
    !decimalSumEquals(request.totals.total, request.totals.net, request.totals.vat, request.totals.exempt) ||
    !decimalSumEquals(request.totals.net, ...request.lines.map((line) => line.net)) ||
    (request.currency !== "ARS" && !hasValidExchangeRate) ||
    (request.currency === "ARS" && request.exchange_rate !== undefined && !hasValidExchangeRate) ||
    (note && (
      associated === undefined ||
      associated.point_of_sale < 1 ||
      associated.voucher_number < 1 ||
      associated.document_type !== expectedAssociatedType ||
      !validISODate(associated.issue_date)
    )) ||
    !validISODate(request.issue_date)
  ) {
    throw new FiscalError("VALIDATION_ERROR");
  }
}

function isDecimal(value: string): boolean {
  return /^(0|[1-9]\d*)(\.\d+)?$/.test(value);
}

function isZeroDecimal(value: string): boolean {
  return /^0(?:\.0+)?$/.test(value);
}

function normalizeDecimal(value: string): string {
  if (!value.includes(".")) return value;
  return value.replace(/0+$/, "").replace(/\.$/, "");
}

function validISODate(value: string): boolean {
  const date = new Date(`${value}T00:00:00.000Z`);
  return !Number.isNaN(date.valueOf()) && date.toISOString().slice(0, 10) === value;
}

function decimalSumEquals(total: string, ...parts: string[]): boolean {
  const values = [total, ...parts];
  const scale = Math.max(...values.map((value) => value.split(".")[1]?.length ?? 0));
  const integer = (value: string): bigint => {
    const [whole, fraction = ""] = value.split(".");
    return BigInt(whole + fraction.padEnd(scale, "0"));
  };
  return integer(total) === parts.reduce((sum, value) => sum + integer(value), 0n);
}

function hashPayload(value: unknown): string {
  return createHash("sha256").update(JSON.stringify(canonical(value))).digest("hex");
}

function canonical(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonical);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, child]) => [key, canonical(child)]));
  }
  return value;
}
