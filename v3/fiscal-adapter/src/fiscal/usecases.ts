import { createHash } from "node:crypto";
import {
  documentTypes,
  FiscalError,
  type FiscalRequest,
  type FiscalResult,
} from "./usecases/domain/fiscal.js";

export type AuthorityDecision =
  | { status: "authorized"; cae: string; cae_expires_on: string; result_code?: string; messages?: string[]; artifact_ref?: string }
  | { status: "rejected"; result_code?: string; messages: string[] }
  | { status: "uncertain"; result_code?: string; messages?: string[] }
  | { status: "not_found" };

export interface FiscalAuthority {
  authorize(request: FiscalRequest): Promise<AuthorityDecision>;
  consult(request: FiscalRequest): Promise<AuthorityDecision>;
}

export interface InternalIdentity {
  issuer: string;
  subject: string;
  organizationId: string;
  actorId?: string;
  delegatedActorId?: string;
  roles: string[];
  requestId: string;
  correlationId: string;
  tokenId: string;
}

export interface FiscalAuditMetadata {
  correlationId: string;
  actorRef?: string;
  delegatedActorRef?: string;
  workloadIssuer: string;
  workloadSubject: string;
  workloadRequestId: string;
  workloadTokenId: string;
}

export interface FiscalRecord {
  idempotencyKey: string;
  payloadHash: string;
  audit: FiscalAuditMetadata;
  request: FiscalRequest;
  result: FiscalResult;
}

export interface FiscalLedger {
  findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined>;
  findByRequest(organizationId: string, requestId: string): Promise<FiscalRecord | undefined>;
  save(record: FiscalRecord): Promise<void>;
}

export interface RequestContext {
  idempotencyKey: string;
  correlationId: string;
  identity: InternalIdentity;
}

export class FiscalService {
  constructor(
    private readonly authority: FiscalAuthority,
    private readonly ledger: FiscalLedger,
    private readonly now: () => Date = () => new Date(),
  ) {}

  async authorize(request: FiscalRequest, context: RequestContext): Promise<FiscalResult> {
    validateRequest(request);
    validateContext(context, request.organization_id);
    validateMetadata(request, context);
    const payloadHash = hashPayload(request);
    const existing = await this.findExisting(request, context.idempotencyKey, payloadHash);
    if (existing !== undefined) return structuredClone(existing.result);

    const decision = await this.authority.authorize(structuredClone(request));
    const result = this.toResult(request, decision, context);
    await this.ledger.save({
      idempotencyKey: context.idempotencyKey,
      payloadHash,
      audit: auditMetadata(context.identity),
      request: structuredClone(request),
      result,
    });
    return structuredClone(result);
  }

  async consult(
    organizationId: string,
    requestId: string,
    suppliedRequest: FiscalRequest | undefined,
    context: RequestContext,
  ): Promise<FiscalResult> {
    validateContext(context, organizationId);
    const recorded = await this.ledger.findByRequest(organizationId, requestId);
    const request = suppliedRequest ?? recorded?.request;
    if (request === undefined || request.organization_id !== organizationId || request.request_id !== requestId) {
      throw new FiscalError("VALIDATION_ERROR");
    }
    validateRequest(request);
    validateMetadata(request, context);
    const payloadHash = hashPayload(request);
    if (recorded !== undefined && recorded.payloadHash !== payloadHash) {
      throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
    }

    const decision = await this.authority.consult(structuredClone(request));
    const result = this.toResult(request, decision, context);
    await this.ledger.save({
      idempotencyKey: recorded?.idempotencyKey ?? context.idempotencyKey,
      payloadHash,
      audit: recorded?.audit ?? auditMetadata(context.identity),
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

  private toResult(request: FiscalRequest, decision: AuthorityDecision, context: RequestContext): FiscalResult {
    return {
      request_id: request.request_id,
      organization_id: request.organization_id,
      idempotency_key: context.idempotencyKey,
      correlation_id: context.correlationId,
      source_version: request.source_version,
      status: decision.status,
      ...(decision.status === "authorized" ? { cae: decision.cae, cae_expires_on: decision.cae_expires_on, artifact_ref: decision.artifact_ref } : {}),
      ...(decision.status === "rejected" || decision.status === "uncertain" ? { authority_messages: decision.messages } : {}),
      ...(decision.status !== "not_found" ? { authority_result_code: decision.result_code } : {}),
      snapshot_digest: request.snapshot_digest,
      observed_at: this.now().toISOString(),
    };
  }
}

function validateContext(context: RequestContext, organizationId: string): void {
  const identity = context.identity;
  if (
    context.idempotencyKey.length < 8 ||
    context.idempotencyKey.length > 128 ||
    !opaqueReference(context.correlationId) ||
    identity.organizationId !== organizationId ||
    identity.correlationId !== context.correlationId ||
    !opaqueReference(identity.issuer) ||
    !opaqueReference(identity.subject) ||
    !opaqueReference(identity.requestId) ||
    !opaqueReference(identity.tokenId) ||
    !optionalOpaqueReference(identity.actorId) ||
    !optionalOpaqueReference(identity.delegatedActorId) ||
    (identity.delegatedActorId !== undefined && identity.actorId === undefined) ||
    !Array.isArray(identity.roles) ||
    !identity.roles.includes("service")
  ) {
    throw new FiscalError("VALIDATION_ERROR");
  }
}

function validateMetadata(request: FiscalRequest, context: RequestContext): void {
  if (request.idempotency_key !== context.idempotencyKey || request.correlation_id !== context.correlationId) {
    throw new FiscalError("VALIDATION_ERROR");
  }
}

function auditMetadata(identity: InternalIdentity): FiscalAuditMetadata {
  return {
    correlationId: identity.correlationId,
    ...(identity.actorId === undefined ? {} : { actorRef: identity.actorId }),
    ...(identity.delegatedActorId === undefined ? {} : { delegatedActorRef: identity.delegatedActorId }),
    workloadIssuer: identity.issuer,
    workloadSubject: identity.subject,
    workloadRequestId: identity.requestId,
    workloadTokenId: identity.tokenId,
  };
}

export function validateRequest(request: FiscalRequest): void {
  const currencies = new Set(["ARS", "USD", "EUR"]);
  const vatRates = new Set(["0", "2.5", "5", "10.5", "21", "27"]);
  const raw = request as unknown as Record<string, unknown>;
  const bodyIdentityKeys = [
    "actor_id",
    "delegated_actor_id",
    "workload_subject",
    "workload_request_id",
    "workload_token_id",
    "jti",
  ];
  if (
    bodyIdentityKeys.some((key) => Object.hasOwn(raw, key)) ||
    request.request_id.length < 1 ||
    request.organization_id.length < 1 ||
    typeof request.idempotency_key !== "string" ||
    request.idempotency_key.length < 8 ||
    request.idempotency_key.length > 128 ||
    typeof request.correlation_id !== "string" ||
    request.correlation_id.length < 1 ||
    !Number.isSafeInteger(request.source_version) ||
    request.source_version < 1 ||
    request.credential_ref.length < 1 ||
    (request.environment !== "homologation" && request.environment !== "production") ||
    !Number.isSafeInteger(request.point_of_sale) ||
    request.point_of_sale < 1 ||
    !Number.isSafeInteger(request.voucher_number) ||
    request.voucher_number < 1 ||
    !documentTypes.includes(request.document_type) ||
    !/^\d{4}-\d{2}-\d{2}$/.test(request.issue_date) ||
    (request.concept !== "products" &&
      request.concept !== "services" &&
      request.concept !== "products_and_services") ||
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
  const serviceConcept =
    request.concept === "services" ||
    request.concept === "products_and_services";
  const servicePeriod = request.service_period;
  if (
    !decimalSumEquals(request.totals.total, request.totals.net, request.totals.vat, request.totals.exempt) ||
    !decimalSumEquals(request.totals.net, ...request.lines.map((line) => line.net)) ||
    (request.currency !== "ARS" && !hasValidExchangeRate) ||
    (request.currency === "ARS" && request.exchange_rate !== undefined && !hasValidExchangeRate) ||
    (serviceConcept &&
      (servicePeriod === undefined ||
        !validISODate(servicePeriod.from) ||
        !validISODate(servicePeriod.to) ||
        !validISODate(servicePeriod.payment_due) ||
        servicePeriod.to < servicePeriod.from)) ||
    (!serviceConcept && servicePeriod !== undefined) ||
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

function opaqueReference(value: unknown): value is string {
  return typeof value === "string" && /^[A-Za-z0-9:_./-]{1,255}$/.test(value);
}

function optionalOpaqueReference(value: unknown): value is string | undefined {
  return value === undefined || opaqueReference(value);
}

function canonical(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonical);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, child]) => [key, canonical(child)]));
  }
  return value;
}
