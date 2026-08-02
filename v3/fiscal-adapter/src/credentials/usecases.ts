import type {
  CredentialEnvironment,
  CredentialMetadata,
  PointOfSale,
} from "./usecases/domain/credential.js";
import {
  CredentialError,
  normalizeCUIT,
  validateCredentialName,
  validateCredentialReference,
  validatePointOfSale,
} from "./usecases/domain/credential.js";

export interface SealedValue {
  format: "aes-256-gcm+kms-v1";
  ciphertext: string;
  encryptedDataKey: string;
  iv: string;
  authTag: string;
  kmsKeyName: string;
}

export interface StoredCredential extends CredentialMetadata {
  idempotencyKey: string;
  requestHash: string;
  csrPem: string;
  encryptedPrivateKey: SealedValue;
  encryptedCertificate?: SealedValue;
}

export interface CredentialActor {
  organizationId: string;
  subject: string;
  roles: string[];
  correlationId: string;
}

export interface RequestCSRInput {
  organizationId: string;
  cuit: string;
  environment: CredentialEnvironment;
  legalName: string;
  commonName: string;
  idempotencyKey: string;
}

export interface CSRResult {
  credential: CredentialMetadata;
  csrPem: string;
}

export interface UploadCertificateInput {
  organizationId: string;
  credentialId: string;
  certificatePem: string;
  expectedVersion: number;
}

export interface ConfigurePointOfSaleInput {
  organizationId: string;
  credentialId: string;
  number: number;
  enabled: boolean;
}

export interface CredentialMaterial {
  credential: CredentialMetadata;
  certificatePem: string;
  privateKeyPem: string;
}

export interface CredentialProbeInput {
  material: CredentialMaterial;
  pointOfSale: number;
}

export interface CredentialProbe {
  validatePointOfSale(input: CredentialProbeInput): Promise<void>;
}

export interface StoredAccessTicket {
  organizationId: string;
  credentialId: string;
  environment: CredentialEnvironment;
  service: string;
  encryptedTicket: SealedValue;
  expiresAt: string;
}

export interface StoredArtifact {
  organizationId: string;
  artifactId: string;
  requestId: string;
  kind: "wsfe_authorization" | "wsfe_consultation";
  encryptedPayload: SealedValue;
}

export interface CertificateInspection {
  fingerprint: string;
  validFrom: string;
  expiresAt: string;
  serialNumber: string;
}

export interface GeneratedCSR {
  privateKeyPem: string;
  csrPem: string;
}

export interface CredentialRepository {
  findByIdempotency(
    organizationId: string,
    idempotencyKey: string,
  ): Promise<StoredCredential | undefined>;
  insertPending(record: StoredCredential): Promise<StoredCredential>;
  find(
    organizationId: string,
    credentialId: string,
  ): Promise<StoredCredential | undefined>;
  activate(
    organizationId: string,
    credentialId: string,
    expectedVersion: number,
    certificate: SealedValue,
    inspection: CertificateInspection,
  ): Promise<StoredCredential>;
  hasValidatedPointOfSale(
    organizationId: string,
    environment: CredentialEnvironment,
    cuit: string,
  ): Promise<boolean>;
  upsertPointOfSale(pointOfSale: PointOfSale): Promise<PointOfSale>;
  findPointOfSale(
    organizationId: string,
    credentialId: string,
    environment: CredentialEnvironment,
    number: number,
  ): Promise<PointOfSale | undefined>;
}

export interface TicketRepository {
  findTicket(
    organizationId: string,
    credentialId: string,
    environment: CredentialEnvironment,
    service: string,
  ): Promise<StoredAccessTicket | undefined>;
  saveTicket(ticket: StoredAccessTicket): Promise<void>;
  deleteTicket(
    organizationId: string,
    credentialId: string,
    environment: CredentialEnvironment,
    service: string,
  ): Promise<void>;
}

export interface ArtifactRepository {
  saveArtifact(artifact: StoredArtifact): Promise<void>;
}

export interface EnvelopeCipher {
  seal(plaintext: Uint8Array, aad: Uint8Array): Promise<SealedValue>;
  open(value: SealedValue, aad: Uint8Array): Promise<Uint8Array>;
}

export interface CSRGenerator {
  generate(input: {
    cuit: string;
    legalName: string;
    commonName: string;
  }): Promise<GeneratedCSR>;
}

export interface CertificateValidator {
  inspect(input: {
    certificatePem: string;
    privateKeyPem: string;
    expectedCUIT: string;
    environment: CredentialEnvironment;
  }): CertificateInspection;
}

export interface CredentialIDGenerator {
  next(): string;
}

export interface Clock {
  now(): Date;
}

export class CredentialService {
  constructor(
    private readonly repository: CredentialRepository,
    private readonly cipher: EnvelopeCipher,
    private readonly csr: CSRGenerator,
    private readonly certificates: CertificateValidator,
    private readonly ids: CredentialIDGenerator,
    private readonly probe: CredentialProbe,
    private readonly clock: Clock = { now: () => new Date() },
  ) {}

  async requestCSR(
    input: RequestCSRInput,
    actor: CredentialActor,
  ): Promise<CSRResult> {
    assertActor(input.organizationId, actor);
    if (
      typeof input.cuit !== "string" ||
      typeof input.legalName !== "string" ||
      typeof input.commonName !== "string" ||
      typeof input.idempotencyKey !== "string"
    ) {
      throw new CredentialError("VALIDATION_ERROR");
    }
    if (input.environment !== "homologation" && input.environment !== "production") {
      throw new CredentialError("VALIDATION_ERROR", "invalid environment");
    }
    const cuit = normalizeCUIT(input.cuit);
    const legalName = validateCredentialName(input.legalName, "legal name");
    const commonName = validateCredentialName(input.commonName, "common name");
    if (
      input.idempotencyKey.length < 8 ||
      input.idempotencyKey.length > 128
    ) {
      throw new CredentialError("VALIDATION_ERROR", "invalid idempotency key");
    }
    const requestHash = createHash("sha256")
      .update(
        JSON.stringify({
          commonName,
          cuit,
          environment: input.environment,
          legalName,
        }),
      )
      .digest("hex");
    const existing = await this.repository.findByIdempotency(
      input.organizationId,
      input.idempotencyKey,
    );
    if (existing !== undefined) {
      if (existing.requestHash !== requestHash) {
        throw new CredentialError("IDEMPOTENCY_KEY_REUSED");
      }
      return { credential: metadata(existing), csrPem: existing.csrPem };
    }
    const id = validateCredentialReference(this.ids.next());
    const generated = await this.csr.generate({ cuit, legalName, commonName });
    const now = this.clock.now().toISOString();
    const encryptedPrivateKey = await this.cipher.seal(
      Buffer.from(generated.privateKeyPem, "utf8"),
      credentialAAD(input.organizationId, id, input.environment, "private-key"),
    );
    let stored: StoredCredential;
    try {
      stored = await this.repository.insertPending({
        id,
        organizationId: input.organizationId,
        cuit,
        environment: input.environment,
        legalName,
        commonName,
        status: "pending_certificate",
        version: 1,
        createdAt: now,
        updatedAt: now,
        idempotencyKey: input.idempotencyKey,
        requestHash,
        csrPem: generated.csrPem,
        encryptedPrivateKey,
      });
    } catch (error) {
      if (
        !(error instanceof CredentialError) ||
        error.code !== "CREDENTIAL_VERSION_CONFLICT"
      ) {
        throw error;
      }
      const raced = await this.repository.findByIdempotency(
        input.organizationId,
        input.idempotencyKey,
      );
      if (raced === undefined || raced.requestHash !== requestHash) {
        throw new CredentialError("IDEMPOTENCY_KEY_REUSED");
      }
      stored = raced;
    }
    return { credential: metadata(stored), csrPem: generated.csrPem };
  }

  async uploadCertificate(
    input: UploadCertificateInput,
    actor: CredentialActor,
  ): Promise<CredentialMetadata> {
    assertActor(input.organizationId, actor);
    if (typeof input.certificatePem !== "string") {
      throw new CredentialError("VALIDATION_ERROR");
    }
    validateCredentialReference(input.credentialId);
    if (!Number.isSafeInteger(input.expectedVersion) || input.expectedVersion < 1) {
      throw new CredentialError("VALIDATION_ERROR", "invalid credential version");
    }
    const credential = await this.requireCredential(
      input.organizationId,
      input.credentialId,
    );
    if (
      credential.environment === "production" &&
      !(await this.repository.hasValidatedPointOfSale(
        input.organizationId,
        "homologation",
        credential.cuit,
      ))
    ) {
      throw new CredentialError("HOMOLOGATION_REQUIRED");
    }
    const privateKeyBytes = await this.cipher.open(
      credential.encryptedPrivateKey,
      credentialAAD(
        credential.organizationId,
        credential.id,
        credential.environment,
        "private-key",
      ),
    );
    try {
      const privateKeyPem = Buffer.from(privateKeyBytes).toString("utf8");
      const inspection = this.certificates.inspect({
        certificatePem: input.certificatePem,
        privateKeyPem,
        expectedCUIT: credential.cuit,
        environment: credential.environment,
      });
      const encryptedCertificate = await this.cipher.seal(
        Buffer.from(input.certificatePem, "utf8"),
        credentialAAD(
          credential.organizationId,
          credential.id,
          credential.environment,
          "certificate",
        ),
      );
      const activated = await this.repository.activate(
        credential.organizationId,
        credential.id,
        input.expectedVersion,
        encryptedCertificate,
        inspection,
      );
      return metadata(activated);
    } finally {
      privateKeyBytes.fill(0);
    }
  }

  async configurePointOfSale(
    input: ConfigurePointOfSaleInput,
    actor: CredentialActor,
  ): Promise<PointOfSale> {
    assertActor(input.organizationId, actor);
    if (typeof input.enabled !== "boolean") {
      throw new CredentialError("VALIDATION_ERROR");
    }
    const credential = await this.requireReadyCredential(
      input.organizationId,
      input.credentialId,
    );
    const number = validatePointOfSale(input.number);
    const current = await this.repository.findPointOfSale(
      input.organizationId,
      credential.id,
      credential.environment,
      number,
    );
    if (input.enabled && current?.enabled !== true) {
      throw new CredentialError("POINT_OF_SALE_NOT_VALIDATED");
    }
    return this.repository.upsertPointOfSale({
      organizationId: input.organizationId,
      credentialId: credential.id,
      environment: credential.environment,
      number,
      enabled: input.enabled,
      ...(current?.validatedAt === undefined
        ? {}
        : { validatedAt: current.validatedAt }),
    });
  }

  async validatePointOfSale(
    input: ConfigurePointOfSaleInput,
    actor: CredentialActor,
  ): Promise<PointOfSale> {
    assertActor(input.organizationId, actor);
    if (typeof input.enabled !== "boolean") {
      throw new CredentialError("VALIDATION_ERROR");
    }
    const credential = await this.requireReadyCredential(
      input.organizationId,
      input.credentialId,
    );
    const number = validatePointOfSale(input.number);
    const material = await this.openCredentialMaterial(credential);
    await this.probe.validatePointOfSale({
      material,
      pointOfSale: number,
    });
    return this.repository.upsertPointOfSale({
      organizationId: input.organizationId,
      credentialId: credential.id,
      environment: credential.environment,
      number,
      enabled: input.enabled,
      validatedAt: this.clock.now().toISOString(),
    });
  }

  async getCredential(
    organizationId: string,
    credentialId: string,
    actor: CredentialActor,
  ): Promise<CredentialMetadata> {
    assertActor(organizationId, actor);
    validateCredentialReference(credentialId);
    return metadata(await this.requireCredential(organizationId, credentialId));
  }

  async resolveMaterial(input: {
    organizationId: string;
    credentialId: string;
    environment: CredentialEnvironment;
    pointOfSale: number;
  }): Promise<CredentialMaterial> {
    const credential = await this.requireReadyCredential(
      input.organizationId,
      input.credentialId,
    );
    if (credential.environment !== input.environment) {
      throw new CredentialError("CREDENTIAL_NOT_READY");
    }
    const pointOfSale = await this.repository.findPointOfSale(
      input.organizationId,
      input.credentialId,
      input.environment,
      validatePointOfSale(input.pointOfSale),
    );
    if (pointOfSale?.enabled !== true) {
      throw new CredentialError("POINT_OF_SALE_NOT_ENABLED");
    }
    return this.openCredentialMaterial(credential);
  }

  private async openCredentialMaterial(
    credential: StoredCredential,
  ): Promise<CredentialMaterial> {
    const [privateKeyBytes, certificateBytes] = await Promise.all([
      this.cipher.open(
        credential.encryptedPrivateKey,
        credentialAAD(
          credential.organizationId,
          credential.id,
          credential.environment,
          "private-key",
        ),
      ),
      this.cipher.open(
        credential.encryptedCertificate!,
        credentialAAD(
          credential.organizationId,
          credential.id,
          credential.environment,
          "certificate",
        ),
      ),
    ]);
    try {
      return {
        credential: metadata(credential),
        privateKeyPem: Buffer.from(privateKeyBytes).toString("utf8"),
        certificatePem: Buffer.from(certificateBytes).toString("utf8"),
      };
    } finally {
      privateKeyBytes.fill(0);
      certificateBytes.fill(0);
    }
  }

  private async requireCredential(
    organizationId: string,
    credentialId: string,
  ): Promise<StoredCredential> {
    const credential = await this.repository.find(organizationId, credentialId);
    if (credential === undefined) {
      throw new CredentialError("CREDENTIAL_NOT_FOUND");
    }
    return credential;
  }

  private async requireReadyCredential(
    organizationId: string,
    credentialId: string,
  ): Promise<StoredCredential> {
    const credential = await this.requireCredential(organizationId, credentialId);
    if (
      credential.status !== "ready" ||
      credential.encryptedCertificate === undefined ||
      credential.certificateExpiresAt === undefined ||
      new Date(credential.certificateExpiresAt).getTime() <= this.clock.now().getTime()
    ) {
      throw new CredentialError("CREDENTIAL_NOT_READY");
    }
    return credential;
  }
}

function assertActor(organizationId: string, actor: CredentialActor): void {
  if (
    actor.organizationId !== organizationId ||
    !actor.roles.includes("service") ||
    !/^[A-Za-z0-9:_./-]{1,255}$/.test(actor.subject) ||
    !/^[A-Za-z0-9:_./-]{1,255}$/.test(actor.correlationId)
  ) {
    throw new CredentialError("VALIDATION_ERROR", "invalid actor");
  }
}

function credentialAAD(
  organizationId: string,
  credentialId: string,
  environment: CredentialEnvironment,
  purpose: "private-key" | "certificate",
): Uint8Array {
  return Buffer.from(
    `pymes-fiscal-v1\u0000${organizationId}\u0000${credentialId}\u0000${environment}\u0000${purpose}`,
    "utf8",
  );
}

function metadata(credential: StoredCredential): CredentialMetadata {
  const {
    idempotencyKey: _idempotencyKey,
    requestHash: _requestHash,
    csrPem: _csr,
    encryptedPrivateKey: _privateKey,
    encryptedCertificate: _certificate,
    ...publicMetadata
  } = credential;
  return structuredClone(publicMetadata);
}
import { createHash } from "node:crypto";
