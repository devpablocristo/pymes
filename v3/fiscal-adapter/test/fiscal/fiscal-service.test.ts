import assert from "node:assert/strict";
import test from "node:test";
import { InMemoryFiscalLedger } from "../../src/fiscal/in_memory_ledger.js";
import { MockFiscalAuthority } from "../../src/fiscal/mock_authority.js";
import {
  FiscalError,
  type FiscalRequest,
} from "../../src/fiscal/usecases/domain/fiscal.js";
import {
  FiscalService,
  type AuthorityDecision,
  type FiscalClaim,
  type FiscalCompletion,
  type FiscalLease,
  type FiscalLedger,
  type FiscalRecord,
} from "../../src/fiscal/usecases.js";

const request: FiscalRequest = {
  request_id: "fiscal:sale-1:1",
  organization_id: "org_a",
  idempotency_key: "fiscal:sale-1:1",
  correlation_id: "sale-1",
  source_version: 1,
  credential_ref: "mock://credential/a",
  environment: "homologation",
  point_of_sale: 1,
  document_type: "FA",
  voucher_number: 42,
  issue_date: "2026-07-30",
  concept: "products",
  currency: "ARS",
  totals: { net: "100", vat: "21", exempt: "0", total: "121.00" },
  recipient: { document_type: "CUIT", document_number: "20123456789", vat_condition: "registered" },
  lines: [{ description: "Servicio", quantity: "1", unit_price: "100", vat_rate: "21", net: "100" }],
  snapshot_digest: "a".repeat(64),
};
const context = contextFor(request);
const now = () => new Date("2026-07-30T12:00:00.000Z");

test("authorizes the exact number supplied by Pymes", async () => {
  const authority = new MockFiscalAuthority("authorized");
  const ledger = new InMemoryFiscalLedger();
  const service = new FiscalService(authority, ledger, now);
  const result = await service.authorize(request, context);
  assert.equal(result.status, "authorized");
  assert.equal(result.cae?.length, 14);
  assert.equal(result.organization_id, request.organization_id);
  assert.equal(result.request_id, request.request_id);
  assert.equal(result.idempotency_key, request.idempotency_key);
  assert.equal(result.correlation_id, request.correlation_id);
  assert.equal(result.source_version, request.source_version);
  assert.equal(result.snapshot_digest, request.snapshot_digest);
  assert.equal(authority.received[0].voucher_number, 42);
  assert.equal(authority.received[0].point_of_sale, 1);
  assert.deepEqual((await ledger.findByRequest(request.organization_id, request.request_id))?.audit, {
    correlationId: "sale-1",
    actorRef: "user_primary",
    delegatedActorRef: "user_delegated",
    workloadIssuer: "pymes-v3",
    workloadSubject: "worker:fiscal",
    workloadRequestId: "http-request-1",
    workloadTokenId: "token-1",
  });
});

test("response loss becomes uncertain and exact consultation recovers the CAE", async () => {
  const authority = new MockFiscalAuthority("response_lost_after_processing");
  const service = new FiscalService(authority, new InMemoryFiscalLedger(), now);
  const first = await service.authorize(request, context);
  assert.equal(first.status, "uncertain");
  const reconciled = await service.consult(request.organization_id, request.request_id, request, context);
  assert.equal(reconciled.status, "authorized");
  assert.equal(reconciled.cae?.length, 14);
  assert.equal(authority.received.length, 1, "consultation must not issue a second voucher");
});

test("exact retries return the original result and changed payloads are rejected", async () => {
  const authority = new MockFiscalAuthority();
  const service = new FiscalService(authority, new InMemoryFiscalLedger(), now);
  const first = await service.authorize(request, context);
  const repeated = await service.authorize(structuredClone(request), context);
  assert.deepEqual(repeated, first);
  assert.equal(authority.received.length, 1);

  const changed = structuredClone(request);
  changed.recipient.document_number = "20999999999";
  await assert.rejects(() => service.authorize(changed, context), (error) => error instanceof FiscalError && error.code === "IDEMPOTENCY_KEY_REUSED");
});

test("concurrent exact retries share one durable claim and dispatch once", async () => {
  let authorizeCalls = 0;
  const authority = {
    async authorize(): Promise<AuthorityDecision> {
      authorizeCalls += 1;
      await new Promise((resolve) => setTimeout(resolve, 20));
      return authorizedDecision("12345678901234");
    },
    async consult(): Promise<AuthorityDecision> {
      return { status: "not_found" };
    },
  };
  const service = new FiscalService(
    authority,
    new InMemoryFiscalLedger(),
    now,
  );

  const results = await Promise.all(
    Array.from({ length: 12 }, () =>
      service.authorize(structuredClone(request), contextFor(request))
    ),
  );

  assert.equal(authorizeCalls, 1);
  assert.equal(new Set(results.map((result) => result.cae)).size, 1);
  assert.ok(results.every((result) => result.status === "authorized"));
});

test("a conflicting payload is rejected while the original claim is active", async () => {
  const started = deferred<void>();
  const release = deferred<AuthorityDecision>();
  let authorizeCalls = 0;
  const service = new FiscalService(
    {
      async authorize(): Promise<AuthorityDecision> {
        authorizeCalls += 1;
        started.resolve();
        return release.promise;
      },
      async consult(): Promise<AuthorityDecision> {
        return { status: "not_found" };
      },
    },
    new InMemoryFiscalLedger(),
    now,
  );
  const original = service.authorize(request, context);
  await started.promise;
  const changed = structuredClone(request);
  changed.recipient.document_number = "20999999999";

  await assert.rejects(
    service.authorize(changed, contextFor(changed)),
    (error: unknown) =>
      error instanceof FiscalError &&
      error.code === "IDEMPOTENCY_KEY_REUSED",
  );
  release.resolve(authorizedDecision("12345678901234"));
  assert.equal((await original).status, "authorized");
  assert.equal(authorizeCalls, 1);
});

test("an expired pre-dispatch lease is reclaimed without an unnecessary consultation", async () => {
  let clock = 0;
  let authorizeCalls = 0;
  let consultCalls = 0;
  const ledger = new MarkFaultLedger(
    new InMemoryFiscalLedger(() => clock),
    "before",
    () => {
      clock = 11;
    },
  );
  const service = serviceWithShortLease(
    {
      async authorize(): Promise<AuthorityDecision> {
        authorizeCalls += 1;
        return authorizedDecision("12345678901234");
      },
      async consult(): Promise<AuthorityDecision> {
        consultCalls += 1;
        return { status: "not_found" };
      },
    },
    ledger,
  );

  const result = await service.authorize(request, context);

  assert.equal(result.status, "authorized");
  assert.equal(authorizeCalls, 1);
  assert.equal(consultCalls, 0);
});

test("an expired post-dispatch lease consults the exact voucher before authorizing", async () => {
  let clock = 0;
  const operations: string[] = [];
  const ledger = new MarkFaultLedger(
    new InMemoryFiscalLedger(() => clock),
    "after",
    () => {
      clock = 11;
    },
  );
  const service = serviceWithShortLease(
    {
      async authorize(input): Promise<AuthorityDecision> {
        operations.push(`authorize:${input.voucher_number}`);
        return authorizedDecision("12345678901234");
      },
      async consult(input): Promise<AuthorityDecision> {
        operations.push(`consult:${input.voucher_number}`);
        return { status: "not_found" };
      },
    },
    ledger,
  );

  const result = await service.authorize(request, context);

  assert.equal(result.status, "authorized");
  assert.deepEqual(operations, ["consult:42", "authorize:42"]);
});

test("a late fenced response cannot overwrite a newer terminal CAE", async () => {
  let clock = 0;
  const firstStarted = deferred<void>();
  const releaseFirst = deferred<AuthorityDecision>();
  let authorizeCalls = 0;
  let consultCalls = 0;
  const ledger = new InMemoryFiscalLedger(() => clock);
  const service = serviceWithShortLease(
    {
      async authorize(): Promise<AuthorityDecision> {
        authorizeCalls += 1;
        firstStarted.resolve();
        return releaseFirst.promise;
      },
      async consult(): Promise<AuthorityDecision> {
        consultCalls += 1;
        return authorizedDecision("99999999999999");
      },
    },
    ledger,
  );

  const first = service.authorize(request, context);
  await firstStarted.promise;
  clock = 11;
  const recovered = await service.authorize(
    structuredClone(request),
    contextFor(request),
  );
  releaseFirst.resolve({
    status: "rejected",
    result_code: "LATE_REJECTION",
    messages: ["late"],
  });
  const late = await first;

  assert.equal(authorizeCalls, 1);
  assert.equal(consultCalls, 1);
  assert.equal(recovered.status, "authorized");
  assert.equal(recovered.cae, "99999999999999");
  assert.equal(late.status, "authorized");
  assert.equal(late.cae, recovered.cae);
  assert.equal(
    (await ledger.findByRequest(request.organization_id, request.request_id))
      ?.result.cae,
    recovered.cae,
  );
});

test("a late fenced CAE cannot overwrite a newer terminal rejection", async () => {
  let clock = 0;
  const firstStarted = deferred<void>();
  const releaseFirst = deferred<AuthorityDecision>();
  const ledger = new InMemoryFiscalLedger(() => clock);
  const service = serviceWithShortLease(
    {
      async authorize(): Promise<AuthorityDecision> {
        firstStarted.resolve();
        return releaseFirst.promise;
      },
      async consult(): Promise<AuthorityDecision> {
        return {
          status: "rejected",
          result_code: "RECOVERED_REJECTION",
          messages: ["rejected"],
        };
      },
    },
    ledger,
  );

  const first = service.authorize(request, context);
  await firstStarted.promise;
  clock = 11;
  const recovered = await service.authorize(
    structuredClone(request),
    contextFor(request),
  );
  releaseFirst.resolve(authorizedDecision("11111111111111"));
  const late = await first;

  assert.equal(recovered.status, "rejected");
  assert.equal(late.status, "rejected");
  assert.equal(
    (await ledger.findByRequest(request.organization_id, request.request_id))
      ?.result.status,
    "rejected",
  );
});

test("organizations are isolated even when request and idempotency IDs coincide", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  const rightRequest = { ...structuredClone(request), organization_id: "org_b", credential_ref: "mock://credential/b" };
  const [left, right] = await Promise.all([
    service.authorize(request, context),
    service.authorize(rightRequest, contextFor(rightRequest)),
  ]);
  assert.equal(left.status, "authorized");
  assert.equal(right.status, "authorized");
  assert.notEqual(left.cae, right.cae);
});

test("timeout before processing is distinguishable from an uncertain authorization", async () => {
  const service = new FiscalService(new MockFiscalAuthority("timeout_before_processing"), new InMemoryFiscalLedger(), now);
  await assert.rejects(() => service.authorize(request, context), (error) => error instanceof FiscalError && error.code === "AUTHORITY_TIMEOUT");
});

test("mock accepts A/B/C and their credit/debit notes with explicit association", async () => {
  const authority = new MockFiscalAuthority();
  const service = new FiscalService(authority, new InMemoryFiscalLedger(), now);
  const types = ["FA", "FB", "FC", "NCA", "NCB", "NCC", "NDA", "NDB", "NDC"] as const;
  for (const [index, documentType] of types.entries()) {
    const current = structuredClone(request);
    current.request_id = `fiscal:document-${index}:1`;
    current.idempotency_key = current.request_id;
    current.correlation_id = current.request_id;
    current.document_type = documentType;
    current.voucher_number = index + 1;
    if (documentType.startsWith("NC") || documentType.startsWith("ND")) {
      current.associated_voucher = { point_of_sale: 1, document_type: documentType.endsWith("A") ? "FA" : documentType.endsWith("B") ? "FB" : "FC", voucher_number: 1, issue_date: current.issue_date };
    }
    const result = await service.authorize(current, contextFor(current));
    assert.equal(result.status, "authorized", documentType);
  }
});

test("foreign currency requires an explicit positive exchange rate", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  const usd = { ...structuredClone(request), request_id: "fiscal:usd:1", idempotency_key: "fiscal:usd:1", correlation_id: "fiscal:usd:1", currency: "USD" };
  await assert.rejects(() => service.authorize(usd, contextFor(usd)), (error) => error instanceof FiscalError && error.code === "VALIDATION_ERROR");
  usd.exchange_rate = "1250.50";
  assert.equal((await service.authorize(usd, contextFor(usd))).status, "authorized");

  const eur = { ...structuredClone(request), request_id: "fiscal:eur:1", idempotency_key: "fiscal:eur:1", correlation_id: "fiscal:eur:1", currency: "EUR", exchange_rate: "1399.20" };
  assert.equal((await service.authorize(eur, contextFor(eur))).status, "authorized");
});

test("accepts only the supported VAT rates and rejects unknown currencies", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  for (const [index, vatRate] of ["0", "2.5", "5", "10.5", "21.0", "27"].entries()) {
    const current = structuredClone(request);
    current.request_id = `fiscal:vat-${index}:1`;
    current.idempotency_key = current.request_id;
    current.correlation_id = current.request_id;
    current.voucher_number = 100 + index;
    current.lines[0].vat_rate = vatRate;
    assert.equal((await service.authorize(current, contextFor(current))).status, "authorized");
  }
  const invalidVAT = structuredClone(request);
  invalidVAT.request_id = "fiscal:vat-invalid:1";
  invalidVAT.idempotency_key = invalidVAT.request_id;
  invalidVAT.correlation_id = invalidVAT.request_id;
  invalidVAT.lines[0].vat_rate = "19";
  await assert.rejects(() => service.authorize(invalidVAT, contextFor(invalidVAT)), FiscalError);

  const invalidCurrency = { ...structuredClone(request), request_id: "fiscal:gbp:1", idempotency_key: "fiscal:gbp:1", correlation_id: "fiscal:gbp:1", currency: "GBP", exchange_rate: "1" };
  await assert.rejects(() => service.authorize(invalidCurrency, contextFor(invalidCurrency)), FiscalError);
});

test("rejected and not-found mock outcomes remain explicit", async () => {
  const rejected = new FiscalService(new MockFiscalAuthority("rejected"), new InMemoryFiscalLedger(), now);
  assert.equal((await rejected.authorize(request, context)).status, "rejected");

  const missing = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  const result = await missing.consult(request.organization_id, request.request_id, request, context);
  assert.equal(result.status, "not_found");
});

test("rejects metadata that diverges from the authenticated HTTP envelope", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  await assert.rejects(
    () => service.authorize(request, { ...context, correlationId: "different-correlation" }),
    (error) => error instanceof FiscalError && error.code === "VALIDATION_ERROR",
  );
});

test("rejects forged operational identity and never derives actors from the fiscal body", async () => {
  const service = new FiscalService(new MockFiscalAuthority(), new InMemoryFiscalLedger(), now);
  await assert.rejects(
    () => service.authorize(request, {
      ...context,
      identity: { ...context.identity, organizationId: "org_b" },
    }),
    (error) => error instanceof FiscalError && error.code === "VALIDATION_ERROR",
  );
  await assert.rejects(
    () => service.authorize(request, {
      ...context,
      identity: { ...context.identity, delegatedActorId: "delegated without spaces", actorId: undefined },
    }),
    (error) => error instanceof FiscalError && error.code === "VALIDATION_ERROR",
  );
});

function contextFor(value: Pick<FiscalRequest, "organization_id" | "idempotency_key" | "correlation_id">) {
  return {
    idempotencyKey: value.idempotency_key,
    correlationId: value.correlation_id,
    identity: {
      issuer: "pymes-v3",
      subject: "worker:fiscal",
      organizationId: value.organization_id,
      actorId: "user_primary",
      delegatedActorId: "user_delegated",
      roles: ["service"],
      requestId: "http-request-1",
      correlationId: value.correlation_id,
      tokenId: "token-1",
    },
  };
}

class MarkFaultLedger implements FiscalLedger {
  private failed = false;

  constructor(
    private readonly delegate: FiscalLedger,
    private readonly fault: "before" | "after",
    private readonly onFault: () => void,
  ) {}

  findByIdempotency(
    organizationId: string,
    idempotencyKey: string,
  ): Promise<FiscalRecord | undefined> {
    return this.delegate.findByIdempotency(organizationId, idempotencyKey);
  }

  findByRequest(
    organizationId: string,
    requestId: string,
  ): Promise<FiscalRecord | undefined> {
    return this.delegate.findByRequest(organizationId, requestId);
  }

  claimAuthorization(
    record: FiscalRecord,
    lease: FiscalLease,
  ): Promise<FiscalClaim> {
    return this.delegate.claimAuthorization(record, lease);
  }

  async markDispatched(
    organizationId: string,
    requestId: string,
    payloadHash: string,
    lease: FiscalLease,
    attempt: number,
  ): Promise<boolean> {
    if (this.failed) {
      return this.delegate.markDispatched(
        organizationId,
        requestId,
        payloadHash,
        lease,
        attempt,
      );
    }
    this.failed = true;
    if (this.fault === "before") {
      this.onFault();
      return false;
    }
    const marked = await this.delegate.markDispatched(
      organizationId,
      requestId,
      payloadHash,
      lease,
      attempt,
    );
    this.onFault();
    return marked ? false : marked;
  }

  completeAuthorization(
    record: FiscalRecord,
    leaseToken: string,
    attempt: number,
  ): Promise<FiscalCompletion> {
    return this.delegate.completeAuthorization(record, leaseToken, attempt);
  }
}

function serviceWithShortLease(
  authority: {
    authorize(request: FiscalRequest): Promise<AuthorityDecision>;
    consult(request: FiscalRequest): Promise<AuthorityDecision>;
  },
  ledger: FiscalLedger,
): FiscalService {
  return new FiscalService(authority, ledger, now, {
    leaseDurationMs: 10,
    contentionTimeoutMs: 1_000,
    contentionPollMs: 1,
    // Reusing the token deliberately proves that the durable attempt is the
    // fencing generation; correctness does not rely only on token entropy.
    leaseToken: () => "lease-fixed",
    sleep: async () => undefined,
  });
}

function authorizedDecision(cae: string): AuthorityDecision {
  return {
    status: "authorized",
    cae,
    cae_expires_on: "2026-08-09",
    result_code: "AUTHORIZED",
  };
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}
