import { createHash, randomUUID } from "node:crypto";
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

export interface FiscalLease {
  token: string;
  durationMs: number;
}

export type FiscalClaim =
  | {
      kind: "acquired";
      recovery: "authorize" | "consult_exact";
      attempt: number;
    }
  | { kind: "busy" }
  | { kind: "stable"; record: FiscalRecord };

export interface FiscalCompletion {
  stored: boolean;
  record: FiscalRecord;
}

export interface FiscalLedger {
  findByIdempotency(organizationId: string, idempotencyKey: string): Promise<FiscalRecord | undefined>;
  findByRequest(organizationId: string, requestId: string): Promise<FiscalRecord | undefined>;
  claimAuthorization(record: FiscalRecord, lease: FiscalLease): Promise<FiscalClaim>;
  markDispatched(
    organizationId: string,
    requestId: string,
    payloadHash: string,
    lease: FiscalLease,
    attempt: number,
  ): Promise<boolean>;
  completeAuthorization(
    record: FiscalRecord,
    leaseToken: string,
    attempt: number,
  ): Promise<FiscalCompletion>;
}

export interface RequestContext {
  idempotencyKey: string;
  correlationId: string;
  identity: InternalIdentity;
}

export interface FiscalServiceOptions {
  leaseDurationMs?: number;
  contentionTimeoutMs?: number;
  contentionPollMs?: number;
  leaseToken?: () => string;
  sleep?: (durationMs: number) => Promise<void>;
}

const DEFAULT_LEASE_DURATION_MS = 30_000;
const DEFAULT_CONTENTION_TIMEOUT_MS = 5_000;
const DEFAULT_CONTENTION_POLL_MS = 10;

export class FiscalService {
  private readonly leaseDurationMs: number;
  private readonly contentionTimeoutMs: number;
  private readonly contentionPollMs: number;
  private readonly leaseToken: () => string;
  private readonly sleep: (durationMs: number) => Promise<void>;

  constructor(
    private readonly authority: FiscalAuthority,
    private readonly ledger: FiscalLedger,
    private readonly now: () => Date = () => new Date(),
    options: FiscalServiceOptions = {},
  ) {
    this.leaseDurationMs = positiveInteger(
      options.leaseDurationMs,
      DEFAULT_LEASE_DURATION_MS,
    );
    this.contentionTimeoutMs = positiveInteger(
      options.contentionTimeoutMs,
      DEFAULT_CONTENTION_TIMEOUT_MS,
    );
    this.contentionPollMs = positiveInteger(
      options.contentionPollMs,
      DEFAULT_CONTENTION_POLL_MS,
    );
    this.leaseToken = options.leaseToken ?? randomUUID;
    this.sleep = options.sleep ?? delay;
  }

  async authorize(request: FiscalRequest, context: RequestContext): Promise<FiscalResult> {
    validateRequest(request);
    validateContext(context, request.organization_id);
    validateMetadata(request, context);
    const payloadHash = hashPayload(request);
    const record = this.pendingRecord(
      request,
      context.idempotencyKey,
      payloadHash,
      auditMetadata(context.identity),
      context,
    );
    return this.executeClaimed(record, "authorize", context);
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
    const record = this.pendingRecord(
      request,
      recorded?.idempotencyKey ?? context.idempotencyKey,
      payloadHash,
      recorded?.audit ?? auditMetadata(context.identity),
      context,
    );
    return this.executeClaimed(record, "consult", context);
  }

  private async executeClaimed(
    record: FiscalRecord,
    operation: "authorize" | "consult",
    context: RequestContext,
  ): Promise<FiscalResult> {
    const startedAt = Date.now();
    while (true) {
      const lease: FiscalLease = {
        token: this.leaseToken(),
        durationMs: this.leaseDurationMs,
      };
      const claim = await this.ledger.claimAuthorization(record, lease);
      if (claim.kind === "stable") {
        return structuredClone(claim.record.result);
      }
      if (claim.kind === "busy") {
        if (Date.now() - startedAt >= this.contentionTimeoutMs) {
          throw new FiscalError(
            "AUTHORITY_TIMEOUT",
            "fiscal authorization is already in progress",
          );
        }
        await this.sleep(this.contentionPollMs);
        continue;
      }

      const marked = await this.ledger.markDispatched(
        record.request.organization_id,
        record.request.request_id,
        record.payloadHash,
        lease,
        claim.attempt,
      );
      if (!marked) {
        await this.sleep(this.contentionPollMs);
        continue;
      }

      try {
        let decision: AuthorityDecision;
        if (operation === "consult" || claim.recovery === "consult_exact") {
          decision = await this.authority.consult(
            structuredClone(record.request),
          );
          if (
            operation === "authorize" &&
            claim.recovery === "consult_exact" &&
            decision.status === "not_found"
          ) {
            decision = await this.authority.authorize(
              structuredClone(record.request),
            );
          }
        } else {
          decision = await this.authority.authorize(
            structuredClone(record.request),
          );
        }
        const completed = await this.ledger.completeAuthorization(
          {
            ...record,
            result: this.toResult(record.request, decision, context),
          },
          lease.token,
          claim.attempt,
        );
        if (completed.stored || isStable(completed.record.result)) {
          return structuredClone(completed.record.result);
        }
      } catch (error) {
        const uncertain = {
          ...record,
          result: this.toResult(
            record.request,
            {
              status: "uncertain",
              result_code: "DISPATCH_OUTCOME_UNKNOWN",
              messages: ["La ejecución fiscal debe reconciliarse por consulta exacta"],
            },
            context,
          ),
        };
        const completion = await this.ledger.completeAuthorization(
          uncertain,
          lease.token,
          claim.attempt,
        ).catch(() => undefined);
        if (
          completion !== undefined &&
          isStable(completion.record.result)
        ) {
          return structuredClone(completion.record.result);
        }
        throw error;
      }

      if (Date.now() - startedAt >= this.contentionTimeoutMs) {
        throw new FiscalError(
          "AUTHORITY_TIMEOUT",
          "fiscal authorization convergence timed out",
        );
      }
      await this.sleep(this.contentionPollMs);
    }
  }

  private pendingRecord(
    request: FiscalRequest,
    idempotencyKey: string,
    payloadHash: string,
    audit: FiscalAuditMetadata,
    context: RequestContext,
  ): FiscalRecord {
    return {
      idempotencyKey,
      payloadHash,
      audit,
      request: structuredClone(request),
      result: this.toResult(
        request,
        {
          status: "uncertain",
          result_code: "CLAIMED",
          messages: ["La ejecución fiscal todavía no tiene un resultado estable"],
        },
        context,
      ),
    };
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

function isStable(result: FiscalResult): boolean {
  return result.status === "authorized" || result.status === "rejected";
}

function positiveInteger(
  value: number | undefined,
  fallback: number,
): number {
  if (value === undefined) return fallback;
  if (!Number.isSafeInteger(value) || value < 1) {
    throw new FiscalError("VALIDATION_ERROR");
  }
  return value;
}

function delay(durationMs: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, durationMs));
}
