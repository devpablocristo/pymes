import type {
  AccessTicket,
  AccessTicketProvider,
} from "@devpablocristo/arca-facturacion/explicit";
import type {
  EnvelopeCipher,
  TicketRepository,
} from "../../../credentials/usecases.js";
import type { CredentialEnvironment } from "../../../credentials/usecases/domain/credential.js";
import { CredentialError } from "../../../credentials/usecases/domain/credential.js";
import type { SerializedAccessTicket } from "../models/ticket.js";

const EXPIRY_MARGIN_MS = 2 * 60_000;

export class DurableWSAATicketProvider implements AccessTicketProvider {
  private readonly pending = new Map<string, Promise<AccessTicket>>();
  private readonly invalidated = new Set<string>();

  constructor(
    private readonly upstream: AccessTicketProvider,
    private readonly repository: TicketRepository,
    private readonly cipher: EnvelopeCipher,
    private readonly context: {
      organizationId: string;
      credentialId: string;
      environment: CredentialEnvironment;
    },
    private readonly now: () => Date = () => new Date(),
  ) {}

  async getAccessTicket(service: string): Promise<AccessTicket> {
    validateService(service);
    const active = this.pending.get(service);
    if (active !== undefined) return active;
    const operation = this.loadOrCreate(service).finally(() => {
      this.pending.delete(service);
    });
    this.pending.set(service, operation);
    return operation;
  }

  clearTicket(service: string): void {
    validateService(service);
    this.invalidated.add(service);
    this.upstream.clearTicket(service);
    void this.repository
      .deleteTicket(
        this.context.organizationId,
        this.context.credentialId,
        this.context.environment,
        service,
      )
      .catch(() => undefined);
  }

  private async loadOrCreate(service: string): Promise<AccessTicket> {
    if (!this.invalidated.has(service)) {
      const stored = await this.repository.findTicket(
        this.context.organizationId,
        this.context.credentialId,
        this.context.environment,
        service,
      );
      if (
        stored !== undefined &&
        new Date(stored.expiresAt).getTime() - this.now().getTime() >
          EXPIRY_MARGIN_MS
      ) {
        const plaintext = await this.cipher.open(
          stored.encryptedTicket,
          ticketAAD(this.context, service),
        );
        try {
          return parseTicket(Buffer.from(plaintext).toString("utf8"));
        } finally {
          plaintext.fill(0);
        }
      }
    }

    const ticket = await this.upstream.getAccessTicket(service);
    const serialized: SerializedAccessTicket = {
      token: ticket.token,
      sign: ticket.sign,
      expirationTime: ticket.expirationTime.toISOString(),
    };
    const encryptedTicket = await this.cipher.seal(
      Buffer.from(JSON.stringify(serialized), "utf8"),
      ticketAAD(this.context, service),
    );
    await this.repository.saveTicket({
      ...this.context,
      service,
      encryptedTicket,
      expiresAt: serialized.expirationTime,
    });
    this.invalidated.delete(service);
    return ticket;
  }
}

function parseTicket(value: string): AccessTicket {
  let parsed: Partial<SerializedAccessTicket>;
  try {
    parsed = JSON.parse(value) as Partial<SerializedAccessTicket>;
  } catch {
    throw new CredentialError("CREDENTIAL_NOT_READY", "invalid WSAA ticket");
  }
  if (
    typeof parsed.token !== "string" ||
    parsed.token.length < 1 ||
    typeof parsed.sign !== "string" ||
    parsed.sign.length < 1 ||
    typeof parsed.expirationTime !== "string"
  ) {
    throw new CredentialError("CREDENTIAL_NOT_READY", "invalid WSAA ticket");
  }
  const expirationTime = new Date(parsed.expirationTime);
  if (Number.isNaN(expirationTime.getTime())) {
    throw new CredentialError("CREDENTIAL_NOT_READY", "invalid WSAA expiration");
  }
  return { token: parsed.token, sign: parsed.sign, expirationTime };
}

function ticketAAD(
  context: {
    organizationId: string;
    credentialId: string;
    environment: CredentialEnvironment;
  },
  service: string,
): Uint8Array {
  return Buffer.from(
    `pymes-fiscal-v1\u0000${context.organizationId}\u0000${context.credentialId}\u0000${context.environment}\u0000wsaa-ticket\u0000${service}`,
    "utf8",
  );
}

function validateService(value: string): void {
  if (!/^[a-z0-9_]{2,64}$/.test(value)) {
    throw new CredentialError("VALIDATION_ERROR", "invalid WSAA service");
  }
}
