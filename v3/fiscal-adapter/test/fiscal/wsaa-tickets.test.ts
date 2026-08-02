import assert from "node:assert/strict";
import test from "node:test";
import type {
  AccessTicket,
  AccessTicketProvider,
} from "@devpablocristo/arca-facturacion/explicit";
import { DurableWSAATicketProvider } from "../../src/fiscal/arca/helpers/tickets.js";
import { GoogleKMSEnvelopeCipher } from "../../src/credentials/kms.js";
import { LocalKMSClient } from "../../src/credentials/local_kms.js";
import type {
  StoredAccessTicket,
  TicketRepository,
} from "../../src/credentials/usecases.js";

test("WSAA ticket cache is durable, encrypted, tenant-bound and deduplicates concurrency", async () => {
  const repository = new MemoryTicketRepository();
  const cipher = new GoogleKMSEnvelopeCipher(
    new LocalKMSClient(Buffer.alloc(32, 37).toString("base64")),
    "projects/local/locations/global/keyRings/test/cryptoKeys/fiscal",
  );
  const upstream = new CountingProvider();
  const context = {
    organizationId: "org_ticket",
    credentialId: "fcred_00000001",
    environment: "homologation" as const,
  };
  const now = () => new Date("2026-08-01T12:00:00.000Z");
  const provider = new DurableWSAATicketProvider(
    upstream,
    repository,
    cipher,
    context,
    now,
  );

  const [left, right] = await Promise.all([
    provider.getAccessTicket("wsfe"),
    provider.getAccessTicket("wsfe"),
  ]);
  assert.deepEqual(left, right);
  assert.equal(upstream.calls, 1);
  const encrypted = await repository.findTicket(
    context.organizationId,
    context.credentialId,
    context.environment,
    "wsfe",
  );
  assert.ok(encrypted);
  assert.doesNotMatch(
    JSON.stringify(encrypted),
    /secret-token|secret-sign/,
  );

  const reconstructed = new DurableWSAATicketProvider(
    new CountingProvider(),
    repository,
    cipher,
    context,
    now,
  );
  assert.equal(
    (await reconstructed.getAccessTicket("wsfe")).token,
    "secret-token-1",
  );

  const wrongTenant = new DurableWSAATicketProvider(
    new CountingProvider(),
    repository,
    cipher,
    { ...context, organizationId: "org_other" },
    now,
  );
  assert.equal(
    (await wrongTenant.getAccessTicket("wsfe")).token,
    "secret-token-1",
    "the other tenant receives its own upstream ticket",
  );
  assert.equal(repository.values.size, 2);
});

test("clearing a ticket invalidates durable state and obtains a new ticket", async () => {
  const repository = new MemoryTicketRepository();
  const cipher = new GoogleKMSEnvelopeCipher(
    new LocalKMSClient(Buffer.alloc(32, 41).toString("base64")),
    "projects/local/locations/global/keyRings/test/cryptoKeys/fiscal",
  );
  const upstream = new CountingProvider();
  const provider = new DurableWSAATicketProvider(
    upstream,
    repository,
    cipher,
    {
      organizationId: "org_clear",
      credentialId: "fcred_00000002",
      environment: "production",
    },
    () => new Date("2026-08-01T12:00:00.000Z"),
  );
  await provider.getAccessTicket("wsfe");
  provider.clearTicket("wsfe");
  await eventually(() => repository.values.size === 0);
  assert.equal((await provider.getAccessTicket("wsfe")).token, "secret-token-2");
  assert.equal(upstream.calls, 2);
  assert.equal(upstream.clears, 1);
});

class CountingProvider implements AccessTicketProvider {
  calls = 0;
  clears = 0;

  async getAccessTicket(): Promise<AccessTicket> {
    this.calls += 1;
    return {
      token: `secret-token-${this.calls}`,
      sign: `secret-sign-${this.calls}`,
      expirationTime: new Date("2026-08-01T13:00:00.000Z"),
    };
  }

  clearTicket(): void {
    this.clears += 1;
  }
}

class MemoryTicketRepository implements TicketRepository {
  readonly values = new Map<string, StoredAccessTicket>();

  async findTicket(
    organizationId: string,
    credentialId: string,
    environment: StoredAccessTicket["environment"],
    service: string,
  ): Promise<StoredAccessTicket | undefined> {
    return this.values.get(
      key(organizationId, credentialId, environment, service),
    );
  }

  async saveTicket(ticket: StoredAccessTicket): Promise<void> {
    this.values.set(
      key(
        ticket.organizationId,
        ticket.credentialId,
        ticket.environment,
        ticket.service,
      ),
      ticket,
    );
  }

  async deleteTicket(
    organizationId: string,
    credentialId: string,
    environment: StoredAccessTicket["environment"],
    service: string,
  ): Promise<void> {
    this.values.delete(key(organizationId, credentialId, environment, service));
  }
}

function key(
  organizationId: string,
  credentialId: string,
  environment: string,
  service: string,
): string {
  return `${organizationId}/${credentialId}/${environment}/${service}`;
}

async function eventually(predicate: () => boolean): Promise<void> {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  assert.fail("condition did not become true");
}
