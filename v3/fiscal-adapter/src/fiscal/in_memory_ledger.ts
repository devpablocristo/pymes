import { FiscalError } from "./usecases/domain/fiscal.js";
import type {
  FiscalClaim,
  FiscalCompletion,
  FiscalLease,
  FiscalLedger,
  FiscalRecord,
} from "./usecases.js";
import type {
  InMemoryFiscalExecution,
  InMemoryFiscalState,
} from "./in_memory_ledger/models/state.js";
import { cloneRecord } from "./in_memory_ledger/helpers/records.js";

export class InMemoryFiscalLedger implements FiscalLedger {
  private readonly state: InMemoryFiscalState = {
    byIdempotency: new Map(),
    byRequest: new Map(),
  };

  constructor(private readonly clock: () => number = Date.now) {}

  async findByIdempotency(
    organizationId: string,
    idempotencyKey: string,
  ): Promise<FiscalRecord | undefined> {
    return cloneRecord(
      this.state.byIdempotency.get(idempotencyMapKey(
        organizationId,
        idempotencyKey,
      ))?.record,
    );
  }

  async findByRequest(
    organizationId: string,
    requestId: string,
  ): Promise<FiscalRecord | undefined> {
    return cloneRecord(
      this.state.byRequest.get(requestMapKey(organizationId, requestId))
        ?.record,
    );
  }

  async claimAuthorization(
    record: FiscalRecord,
    lease: FiscalLease,
  ): Promise<FiscalClaim> {
    const organizationId = record.request.organization_id;
    const requestKey = requestMapKey(
      organizationId,
      record.request.request_id,
    );
    const idempotencyKey = idempotencyMapKey(
      organizationId,
      record.idempotencyKey,
    );
    const byRequest = this.state.byRequest.get(requestKey);
    const byIdempotency = this.state.byIdempotency.get(idempotencyKey);
    if (
      byRequest !== undefined &&
      byIdempotency !== undefined &&
      byRequest !== byIdempotency
    ) {
      throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
    }

    const existing = byRequest ?? byIdempotency;
    if (existing === undefined) {
      const execution: InMemoryFiscalExecution = {
        record: structuredClone(record),
        state: "claimed",
        attempt: 1,
        leaseToken: lease.token,
        leaseExpiresAt: this.clock() + lease.durationMs,
        dispatchMayHaveOccurred: false,
      };
      this.register(execution);
      return { kind: "acquired", recovery: "authorize", attempt: 1 };
    }

    assertSameCommand(existing, record);
    if (existing.state === "terminal") {
      return { kind: "stable", record: cloneRecord(existing.record)! };
    }
    if (
      existing.leaseToken !== undefined &&
      existing.leaseExpiresAt !== undefined &&
      existing.leaseExpiresAt > this.clock()
    ) {
      return { kind: "busy" };
    }

    const recovery = existing.dispatchMayHaveOccurred
      ? "consult_exact"
      : "authorize";
    existing.state = "claimed";
    existing.attempt += 1;
    existing.leaseToken = lease.token;
    existing.leaseExpiresAt = this.clock() + lease.durationMs;
    this.register(existing);
    return { kind: "acquired", recovery, attempt: existing.attempt };
  }

  async markDispatched(
    organizationId: string,
    requestId: string,
    payloadHash: string,
    lease: FiscalLease,
    attempt: number,
  ): Promise<boolean> {
    const execution = this.state.byRequest.get(
      requestMapKey(organizationId, requestId),
    );
    if (
      execution === undefined ||
      execution.record.payloadHash !== payloadHash ||
      execution.state !== "claimed" ||
      execution.leaseToken !== lease.token ||
      execution.attempt !== attempt ||
      execution.leaseExpiresAt === undefined ||
      execution.leaseExpiresAt <= this.clock()
    ) {
      return false;
    }
    execution.state = "in_progress";
    execution.dispatchMayHaveOccurred = true;
    execution.leaseExpiresAt = this.clock() + lease.durationMs;
    return true;
  }

  async completeAuthorization(
    record: FiscalRecord,
    leaseToken: string,
    attempt: number,
  ): Promise<FiscalCompletion> {
    const execution = this.state.byRequest.get(
      requestMapKey(
        record.request.organization_id,
        record.request.request_id,
      ),
    );
    if (execution === undefined) {
      throw new FiscalError("INTERNAL_ERROR");
    }
    assertSameCommand(execution, record);
    if (execution.state === "terminal") {
      return {
        stored: false,
        record: cloneRecord(execution.record)!,
      };
    }
    if (
      execution.leaseToken !== leaseToken ||
      execution.attempt !== attempt ||
      (execution.state !== "claimed" && execution.state !== "in_progress")
    ) {
      return {
        stored: false,
        record: cloneRecord(execution.record)!,
      };
    }

    execution.record = {
      ...structuredClone(record),
      audit: structuredClone(execution.record.audit),
    };
    execution.state = isTerminal(record) ? "terminal" : "uncertain";
    execution.leaseToken = undefined;
    execution.leaseExpiresAt = undefined;
    this.register(execution);
    return {
      stored: true,
      record: cloneRecord(execution.record)!,
    };
  }

  private register(execution: InMemoryFiscalExecution): void {
    this.state.byRequest.set(
      requestMapKey(
        execution.record.request.organization_id,
        execution.record.request.request_id,
      ),
      execution,
    );
    this.state.byIdempotency.set(
      idempotencyMapKey(
        execution.record.request.organization_id,
        execution.record.idempotencyKey,
      ),
      execution,
    );
  }
}

function assertSameCommand(
  execution: InMemoryFiscalExecution,
  record: FiscalRecord,
): void {
  if (
    execution.record.payloadHash !== record.payloadHash ||
    execution.record.request.request_id !== record.request.request_id ||
    execution.record.idempotencyKey !== record.idempotencyKey
  ) {
    throw new FiscalError("IDEMPOTENCY_KEY_REUSED");
  }
}

function isTerminal(record: FiscalRecord): boolean {
  return (
    record.result.status === "authorized" ||
    record.result.status === "rejected"
  );
}

function requestMapKey(
  organizationId: string,
  requestId: string,
): string {
  return `${organizationId}|${requestId}`;
}

function idempotencyMapKey(
  organizationId: string,
  idempotencyKey: string,
): string {
  return `${organizationId}|${idempotencyKey}`;
}
