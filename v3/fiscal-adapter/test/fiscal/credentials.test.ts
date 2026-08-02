import assert from "node:assert/strict";
import test from "node:test";
import {
  CredentialService,
  type CertificateInspection,
  type CredentialRepository,
  type SealedValue,
  type StoredCredential,
} from "../../src/credentials/usecases.js";
import {
  CredentialError,
  type CredentialEnvironment,
  type PointOfSale,
} from "../../src/credentials/usecases/domain/credential.js";
import { GoogleKMSEnvelopeCipher } from "../../src/credentials/kms.js";
import { LocalKMSClient } from "../../src/credentials/local_kms.js";
import type {
  KMSBytesResponse,
  KMSClient,
} from "../../src/credentials/kms/models/client.js";
import {
  crc32c,
} from "../../src/credentials/kms/helpers/crc32c.js";

const keyName =
  "projects/local/locations/global/keyRings/test/cryptoKeys/fiscal";

test("envelope encryption authenticates tenant, credential and purpose", async () => {
  const cipher = new GoogleKMSEnvelopeCipher(
    new LocalKMSClient(Buffer.alloc(32, 17).toString("base64")),
    keyName,
  );
  const aad = Buffer.from("org_one\u0000fcred_00000001\u0000private-key");
  const otherTenant = Buffer.from(
    "org_two\u0000fcred_00000001\u0000private-key",
  );
  const sealed = await cipher.seal(Buffer.from("private material"), aad);

  assert.equal(
    Buffer.from(await cipher.open(sealed, aad)).toString("utf8"),
    "private material",
  );
  await assert.rejects(
    cipher.open(sealed, otherTenant),
    (error: unknown) =>
      error instanceof CredentialError &&
      error.code === "CREDENTIAL_NOT_READY",
  );
  await assert.rejects(
    cipher.open(
      {
        ...sealed,
        ciphertext: mutateBase64(sealed.ciphertext),
      },
      aad,
    ),
    (error: unknown) =>
      error instanceof CredentialError &&
      error.code === "CREDENTIAL_NOT_READY",
  );
});

test("CRC32C uses the Cloud KMS Castagnoli checksum", () => {
  assert.equal(crc32c(Buffer.from("123456789", "utf8")), 0xe3069283);
});

test("envelope encryption rejects missing, false and corrupt KMS integrity evidence", async (context) => {
  const aad = Buffer.from("org_one\u0000fcred_integrity\u0000private-key");
  const plaintext = Buffer.from("private material");
  const encodedKey = Buffer.alloc(32, 29).toString("base64");
  const encryptScenarios: Array<{
    name: string;
    mutate(response: KMSBytesResponse): KMSBytesResponse;
  }> = [
    {
      name: "false plaintext verification",
      mutate: (response) => ({
        ...response,
        verifiedPlaintextCrc32c: false,
      }),
    },
    {
      name: "missing AAD verification",
      mutate: (response) => ({
        ...response,
        verifiedAdditionalAuthenticatedDataCrc32c: undefined,
      }),
    },
    {
      name: "missing ciphertext CRC",
      mutate: (response) => ({
        ...response,
        ciphertextCrc32c: undefined,
      }),
    },
    {
      name: "corrupt ciphertext",
      mutate: (response) => ({
        ...response,
        ciphertext: mutateBytes(response.ciphertext),
      }),
    },
  ];
  for (const scenario of encryptScenarios) {
    await context.test(`encrypt: ${scenario.name}`, async () => {
      const delegate = new LocalKMSClient(encodedKey);
      const cipher = new GoogleKMSEnvelopeCipher(
        mutateKMS(delegate, "encrypt", scenario.mutate),
        keyName,
      );
      await assert.rejects(
        cipher.seal(plaintext, aad),
        hasCredentialMessage("fiscal KMS encrypt integrity verification failed"),
      );
    });
  }

  const sourceCipher = new GoogleKMSEnvelopeCipher(
    new LocalKMSClient(encodedKey),
    keyName,
  );
  const sealed = await sourceCipher.seal(plaintext, aad);
  const decryptScenarios: Array<{
    name: string;
    mutate(response: KMSBytesResponse): KMSBytesResponse;
  }> = [
    {
      name: "false ciphertext verification",
      mutate: (response) => ({
        ...response,
        verifiedCiphertextCrc32c: false,
      }),
    },
    {
      name: "missing AAD verification",
      mutate: (response) => ({
        ...response,
        verifiedAdditionalAuthenticatedDataCrc32c: undefined,
      }),
    },
    {
      name: "missing plaintext CRC",
      mutate: (response) => ({
        ...response,
        plaintextCrc32c: undefined,
      }),
    },
    {
      name: "corrupt plaintext",
      mutate: (response) => ({
        ...response,
        plaintext: mutateBytes(response.plaintext),
      }),
    },
  ];
  for (const scenario of decryptScenarios) {
    await context.test(`decrypt: ${scenario.name}`, async () => {
      const delegate = new LocalKMSClient(encodedKey);
      const cipher = new GoogleKMSEnvelopeCipher(
        mutateKMS(delegate, "decrypt", scenario.mutate),
        keyName,
      );
      await assert.rejects(
        cipher.open(sealed, aad),
        hasCredentialMessage("fiscal KMS decrypt integrity verification failed"),
      );
    });
  }
});

test("KMS readiness verifies encrypt and decrypt once and caches the result", async () => {
  const delegate = new LocalKMSClient(
    Buffer.alloc(32, 31).toString("base64"),
  );
  let encryptCalls = 0;
  let decryptCalls = 0;
  const cipher = new GoogleKMSEnvelopeCipher(
    {
      async encrypt(request) {
        encryptCalls += 1;
        return delegate.encrypt(request);
      },
      async decrypt(request) {
        decryptCalls += 1;
        return delegate.decrypt(request);
      },
    },
    keyName,
  );

  await Promise.all([cipher.ready(), cipher.ready(), cipher.ready()]);
  await cipher.ready();

  assert.equal(encryptCalls, 1);
  assert.equal(decryptCalls, 1);
});

test("KMS readiness shares an in-flight failure but permits immediate recovery", async () => {
  const delegate = new LocalKMSClient(
    Buffer.alloc(32, 37).toString("base64"),
  );
  let encryptCalls = 0;
  let decryptCalls = 0;
  let available = false;
  let now = 0;
  const cipher = new GoogleKMSEnvelopeCipher(
    {
      async encrypt(request) {
        encryptCalls += 1;
        if (!available) throw new Error("permission denied");
        return delegate.encrypt(request);
      },
      async decrypt(request) {
        decryptCalls += 1;
        return delegate.decrypt(request);
      },
    },
    keyName,
    (size) => Buffer.alloc(size, 41),
    () => now,
  );

  await Promise.all(
    [cipher.ready(), cipher.ready(), cipher.ready()].map((attempt) =>
      assert.rejects(
        attempt,
        hasCredentialCode("CREDENTIAL_NOT_READY"),
      )
    ),
  );
  assert.equal(encryptCalls, 1);
  assert.equal(decryptCalls, 0);

  available = true;
  await cipher.ready();
  await cipher.ready();
  assert.equal(encryptCalls, 2);
  assert.equal(decryptCalls, 1);

  now = 5 * 60 * 1000;
  await cipher.ready();
  assert.equal(encryptCalls, 3);
  assert.equal(decryptCalls, 2);
});

test("credential onboarding is tenant-bound, idempotent and requires homologation", async () => {
  const repository = new MemoryCredentialRepository();
  const cipher = new GoogleKMSEnvelopeCipher(
    new LocalKMSClient(Buffer.alloc(32, 23).toString("base64")),
    keyName,
  );
  let sequence = 0;
  const service = new CredentialService(
    repository,
    cipher,
    {
      async generate() {
        sequence += 1;
        return {
          privateKeyPem: `private-${sequence}`,
          csrPem: `csr-${sequence}`,
        };
      },
    },
    {
      inspect({ certificatePem, privateKeyPem, expectedCUIT }) {
        assert.match(certificatePem, /^certificate-/);
        assert.match(privateKeyPem, /^private-/);
        assert.equal(expectedCUIT, "20123456786");
        return inspection;
      },
    },
    {
      next() {
        return `fcred_${String(sequence + 1).padStart(8, "0")}`;
      },
    },
    acceptingProbe,
    { now: () => new Date("2026-08-01T12:00:00.000Z") },
  );
  const actor = serviceActor("org_one");
  const request = {
    organizationId: "org_one",
    cuit: "20-12345678-6",
    environment: "homologation" as const,
    legalName: "Cliente Uno SA",
    commonName: "pymes-homologacion",
    idempotencyKey: "csr:organization:one",
  };

  const first = await service.requestCSR(request, actor);
  const repeated = await service.requestCSR(request, actor);
  assert.deepEqual(repeated, first);
  assert.equal(first.credential.status, "pending_certificate");
  assert.equal(
    "encryptedPrivateKey" in first.credential,
    false,
    "private material must never enter the public result",
  );
  await assert.rejects(
    service.requestCSR(
      { ...request, commonName: "changed" },
      actor,
    ),
    hasCredentialCode("IDEMPOTENCY_KEY_REUSED"),
  );
  await assert.rejects(
    service.getCredential(
      "org_two",
      first.credential.id,
      serviceActor("org_two"),
    ),
    hasCredentialCode("CREDENTIAL_NOT_FOUND"),
  );

  const homologation = await service.uploadCertificate(
    {
      organizationId: "org_one",
      credentialId: first.credential.id,
      certificatePem: "certificate-homologation",
      expectedVersion: 1,
    },
    actor,
  );
  assert.equal(homologation.status, "ready");
  assert.equal(homologation.version, 2);
  await assert.rejects(
    service.uploadCertificate(
      {
        organizationId: "org_one",
        credentialId: first.credential.id,
        certificatePem: "certificate-repeat",
        expectedVersion: 1,
      },
      actor,
    ),
    hasCredentialCode("CREDENTIAL_VERSION_CONFLICT"),
  );

  const productionCSR = await service.requestCSR(
    {
      ...request,
      environment: "production",
      commonName: "pymes-produccion",
      idempotencyKey: "csr:organization:production",
    },
    actor,
  );
  await assert.rejects(
    service.uploadCertificate(
      {
        organizationId: "org_one",
        credentialId: productionCSR.credential.id,
        certificatePem: "certificate-production",
        expectedVersion: 1,
      },
      actor,
    ),
    hasCredentialCode("HOMOLOGATION_REQUIRED"),
  );
  await service.validatePointOfSale(
    {
      organizationId: "org_one",
      credentialId: first.credential.id,
      number: 6,
      enabled: true,
    },
    actor,
  );
  await service.uploadCertificate(
    {
      organizationId: "org_one",
      credentialId: productionCSR.credential.id,
      certificatePem: "certificate-production",
      expectedVersion: 1,
    },
    actor,
  );
  await assert.rejects(
    service.configurePointOfSale(
      {
        organizationId: "org_one",
        credentialId: productionCSR.credential.id,
        number: 7,
        enabled: true,
      },
      actor,
    ),
    hasCredentialCode("POINT_OF_SALE_NOT_VALIDATED"),
  );
  const validated = await service.validatePointOfSale(
    {
      organizationId: "org_one",
      credentialId: productionCSR.credential.id,
      number: 7,
      enabled: true,
    },
    actor,
  );
  assert.equal(validated.validatedAt, "2026-08-01T12:00:00.000Z");
  const disabled = await service.configurePointOfSale(
    {
      organizationId: "org_one",
      credentialId: productionCSR.credential.id,
      number: 7,
      enabled: false,
    },
    actor,
  );
  assert.equal(disabled.enabled, false);
  await assert.rejects(
    service.configurePointOfSale(
      {
        organizationId: "org_one",
        credentialId: productionCSR.credential.id,
        number: 7,
        enabled: true,
      },
      actor,
    ),
    hasCredentialCode("POINT_OF_SALE_NOT_VALIDATED"),
  );
  await service.validatePointOfSale(
    {
      organizationId: "org_one",
      credentialId: productionCSR.credential.id,
      number: 7,
      enabled: true,
    },
    actor,
  );
  const material = await service.resolveMaterial({
    organizationId: "org_one",
    credentialId: productionCSR.credential.id,
    environment: "production",
    pointOfSale: 7,
  });
  assert.equal(material.privateKeyPem, "private-2");
  assert.equal(material.certificatePem, "certificate-production");
  await assert.rejects(
    service.resolveMaterial({
      organizationId: "org_one",
      credentialId: productionCSR.credential.id,
      environment: "production",
      pointOfSale: 8,
    }),
    hasCredentialCode("POINT_OF_SALE_NOT_ENABLED"),
  );
});

test("production certificate cannot be activated before tenant homologation", async () => {
  const repository = new MemoryCredentialRepository();
  const cipher = new GoogleKMSEnvelopeCipher(
    new LocalKMSClient(Buffer.alloc(32, 29).toString("base64")),
    keyName,
  );
  const service = new CredentialService(
    repository,
    cipher,
    {
      async generate() {
        return { privateKeyPem: "private", csrPem: "csr" };
      },
    },
    { inspect: () => inspection },
    { next: () => "fcred_00000009" },
    acceptingProbe,
    { now: () => new Date("2026-08-01T12:00:00.000Z") },
  );
  const actor = serviceActor("org_fresh");
  const pending = await service.requestCSR(
    {
      organizationId: "org_fresh",
      cuit: "20123456786",
      environment: "production",
      legalName: "Fresh SA",
      commonName: "fresh-production",
      idempotencyKey: "production:fresh:1",
    },
    actor,
  );
  await assert.rejects(
    service.uploadCertificate(
      {
        organizationId: "org_fresh",
        credentialId: pending.credential.id,
        certificatePem: "certificate",
        expectedVersion: 1,
      },
      actor,
    ),
    hasCredentialCode("HOMOLOGATION_REQUIRED"),
  );
});

const inspection: CertificateInspection = {
  fingerprint: "a".repeat(64),
  validFrom: "2026-07-01T00:00:00.000Z",
  expiresAt: "2027-07-01T00:00:00.000Z",
  serialNumber: "01",
};

class MemoryCredentialRepository implements CredentialRepository {
  private readonly values = new Map<string, StoredCredential>();
  private readonly points = new Map<string, PointOfSale>();

  async findByIdempotency(
    organizationId: string,
    idempotencyKey: string,
  ): Promise<StoredCredential | undefined> {
    return [...this.values.values()].find(
      (value) =>
        value.organizationId === organizationId &&
        value.idempotencyKey === idempotencyKey,
    );
  }

  async insertPending(record: StoredCredential): Promise<StoredCredential> {
    const duplicate = await this.findByIdempotency(
      record.organizationId,
      record.idempotencyKey,
    );
    if (duplicate !== undefined) {
      throw new CredentialError("CREDENTIAL_VERSION_CONFLICT");
    }
    this.values.set(key(record.organizationId, record.id), record);
    return record;
  }

  async find(
    organizationId: string,
    credentialId: string,
  ): Promise<StoredCredential | undefined> {
    return this.values.get(key(organizationId, credentialId));
  }

  async activate(
    organizationId: string,
    credentialId: string,
    expectedVersion: number,
    certificate: SealedValue,
    certificateInspection: CertificateInspection,
  ): Promise<StoredCredential> {
    const current = this.values.get(key(organizationId, credentialId));
    if (current === undefined || current.version !== expectedVersion) {
      throw new CredentialError("CREDENTIAL_VERSION_CONFLICT");
    }
    const updated: StoredCredential = {
      ...current,
      status: "ready",
      version: current.version + 1,
      encryptedCertificate: certificate,
      certificateFingerprint: certificateInspection.fingerprint,
      certificateValidFrom: certificateInspection.validFrom,
      certificateExpiresAt: certificateInspection.expiresAt,
      certificateSerialNumber: certificateInspection.serialNumber,
      updatedAt: "2026-08-01T12:00:00.000Z",
    };
    this.values.set(key(organizationId, credentialId), updated);
    return updated;
  }

  async hasValidatedPointOfSale(
    organizationId: string,
    environment: CredentialEnvironment,
    cuit: string,
  ): Promise<boolean> {
    return [...this.values.values()].some(
      (credential) =>
        credential.organizationId === organizationId &&
        credential.environment === environment &&
        credential.cuit === cuit &&
        credential.status === "ready" &&
        [...this.points.values()].some(
          (point) =>
            point.organizationId === organizationId &&
            point.credentialId === credential.id &&
            point.environment === environment &&
            point.enabled &&
            point.validatedAt !== undefined,
        ),
    );
  }

  async upsertPointOfSale(value: PointOfSale): Promise<PointOfSale> {
    this.points.set(
      `${value.organizationId}/${value.credentialId}/${value.environment}/${value.number}`,
      value,
    );
    return value;
  }

  async findPointOfSale(
    organizationId: string,
    credentialId: string,
    environment: CredentialEnvironment,
    number: number,
  ): Promise<PointOfSale | undefined> {
    return this.points.get(
      `${organizationId}/${credentialId}/${environment}/${number}`,
    );
  }
}

function key(organizationId: string, credentialId: string): string {
  return `${organizationId}/${credentialId}`;
}

function serviceActor(organizationId: string) {
  return {
    organizationId,
    subject: "worker:fiscal",
    roles: ["service"],
    correlationId: "corr:credential:test",
  };
}

function hasCredentialCode(code: string) {
  return (error: unknown) =>
    error instanceof CredentialError && error.code === code;
}

function hasCredentialMessage(message: string) {
  return (error: unknown) =>
    error instanceof CredentialError &&
    error.code === "CREDENTIAL_NOT_READY" &&
    error.message === message;
}

function mutateKMS(
  delegate: KMSClient,
  operation: "encrypt" | "decrypt",
  mutate: (response: KMSBytesResponse) => KMSBytesResponse,
): KMSClient {
  return {
    async encrypt(request) {
      const [response] = await delegate.encrypt(request);
      return [operation === "encrypt" ? mutate(response) : response];
    },
    async decrypt(request) {
      const [response] = await delegate.decrypt(request);
      return [operation === "decrypt" ? mutate(response) : response];
    },
  };
}

function mutateBytes(
  value: Uint8Array | string | null | undefined,
): Uint8Array {
  if (value === null || value === undefined) {
    throw new Error("cannot mutate an absent KMS response");
  }
  const bytes =
    typeof value === "string"
      ? Buffer.from(value, "base64")
      : Buffer.from(value);
  if (bytes.length === 0) {
    throw new Error("cannot mutate an empty KMS response");
  }
  bytes[0] = bytes[0]! ^ 1;
  return bytes;
}

function mutateBase64(value: string): string {
  const bytes = Buffer.from(value, "base64");
  bytes[0] = bytes[0]! ^ 1;
  return bytes.toString("base64");
}

const acceptingProbe = {
  async validatePointOfSale(): Promise<void> {},
};
