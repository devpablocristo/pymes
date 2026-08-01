import { randomBytes, timingSafeEqual } from "node:crypto";
import type { EnvelopeCipher, SealedValue } from "./usecases.js";
import type { KMSClient } from "./kms/models/client.js";
import {
  decodeBase64,
  decryptAESGCM,
  encryptAESGCM,
  kmsBytes,
} from "./kms/helpers/aes.js";
import {
  crc32c,
  validCRC32C,
} from "./kms/helpers/crc32c.js";
import { CredentialError } from "./usecases/domain/credential.js";

const DEFAULT_READINESS_TTL_MS = 5 * 60 * 1000;

export class GoogleKMSEnvelopeCipher implements EnvelopeCipher {
  private readiness?: {
    expiresAt: number;
    promise: Promise<void>;
  };

  constructor(
    private readonly client: KMSClient,
    private readonly keyName: string,
    private readonly entropy: (size: number) => Uint8Array = randomBytes,
    private readonly now: () => number = Date.now,
    private readonly readinessTTLms: number = DEFAULT_READINESS_TTL_MS,
  ) {
    if (
      !/^projects\/[^/]+\/locations\/[^/]+\/keyRings\/[^/]+\/cryptoKeys\/[^/]+$/.test(
        keyName,
      )
    ) {
      throw new CredentialError("VALIDATION_ERROR", "invalid fiscal KMS key name");
    }
    if (
      !Number.isSafeInteger(readinessTTLms) ||
      readinessTTLms < 1
    ) {
      throw new CredentialError(
        "VALIDATION_ERROR",
        "invalid fiscal KMS readiness TTL",
      );
    }
  }

  async seal(plaintext: Uint8Array, aad: Uint8Array): Promise<SealedValue> {
    const dataKey = Buffer.from(this.entropy(32));
    const iv = Buffer.from(this.entropy(12));
    if (dataKey.byteLength !== 32 || iv.byteLength !== 12) {
      throw new CredentialError("CREDENTIAL_NOT_READY", "invalid entropy source");
    }
    try {
      const encrypted = encryptAESGCM(plaintext, dataKey, iv, aad);
      const [wrapped] = await this.client.encrypt({
        name: this.keyName,
        plaintext: dataKey,
        additionalAuthenticatedData: aad,
        plaintextCrc32c: { value: crc32c(dataKey) },
        additionalAuthenticatedDataCrc32c: { value: crc32c(aad) },
      });
      const wrappedDataKey = Buffer.from(
        kmsBytes(wrapped.ciphertext, "ciphertext"),
      );
      if (
        wrapped.verifiedPlaintextCrc32c !== true ||
        wrapped.verifiedAdditionalAuthenticatedDataCrc32c !== true ||
        !validCRC32C(wrapped.ciphertextCrc32c, wrappedDataKey)
      ) {
        throw new CredentialError(
          "CREDENTIAL_NOT_READY",
          "fiscal KMS encrypt integrity verification failed",
        );
      }
      return {
        format: "aes-256-gcm+kms-v1",
        ciphertext: Buffer.from(encrypted.ciphertext).toString("base64"),
        encryptedDataKey: wrappedDataKey.toString("base64"),
        iv: iv.toString("base64"),
        authTag: Buffer.from(encrypted.authTag).toString("base64"),
        kmsKeyName: this.keyName,
      };
    } finally {
      dataKey.fill(0);
    }
  }

  async open(value: SealedValue, aad: Uint8Array): Promise<Uint8Array> {
    if (
      value.format !== "aes-256-gcm+kms-v1" ||
      value.kmsKeyName !== this.keyName
    ) {
      throw new CredentialError("CREDENTIAL_NOT_READY", "unexpected KMS key");
    }
    const encryptedDataKey = decodeBase64(
      value.encryptedDataKey,
      "encrypted data key",
    );
    const [unwrapped] = await this.client.decrypt({
      name: value.kmsKeyName,
      ciphertext: encryptedDataKey,
      additionalAuthenticatedData: aad,
      ciphertextCrc32c: { value: crc32c(encryptedDataKey) },
      additionalAuthenticatedDataCrc32c: { value: crc32c(aad) },
    });
    const dataKey = Buffer.from(kmsBytes(unwrapped.plaintext, "plaintext"));
    if (
      unwrapped.verifiedCiphertextCrc32c !== true ||
      unwrapped.verifiedAdditionalAuthenticatedDataCrc32c !== true ||
      !validCRC32C(unwrapped.plaintextCrc32c, dataKey)
    ) {
      dataKey.fill(0);
      throw new CredentialError(
        "CREDENTIAL_NOT_READY",
        "fiscal KMS decrypt integrity verification failed",
      );
    }
    try {
      return decryptAESGCM(
        {
          ciphertext: decodeBase64(value.ciphertext, "ciphertext"),
          iv: decodeBase64(value.iv, "iv"),
          authTag: decodeBase64(value.authTag, "authentication tag"),
        },
        dataKey,
        aad,
      );
    } finally {
      dataKey.fill(0);
    }
  }

  /**
   * Comprueba que la identidad del workload puede cifrar y descifrar con la
   * misma clave. La promesa se comparte entre callers y se renueva cada cinco
   * minutos para limitar costo sin ocultar una caída o recuperación de KMS.
   */
  ready(): Promise<void> {
    const checkedAt = this.now();
    if (
      this.readiness === undefined ||
      checkedAt >= this.readiness.expiresAt
    ) {
      const readiness = {
        expiresAt: checkedAt + this.readinessTTLms,
        promise: this.verifyReadiness(),
      };
      this.readiness = readiness;
      void readiness.promise.catch(() => {
        if (this.readiness === readiness) {
          this.readiness = undefined;
        }
      });
    }
    return this.readiness.promise;
  }

  private async verifyReadiness(): Promise<void> {
    const plaintext = Buffer.from("pymes-fiscal-kms-readiness-v1", "utf8");
    const aad = Buffer.from(
      `pymes-fiscal-kms-readiness-v1\u0000${this.keyName}`,
      "utf8",
    );
    let decrypted: Uint8Array | undefined;
    let candidate: Buffer | undefined;
    try {
      const sealed = await this.seal(plaintext, aad);
      decrypted = await this.open(sealed, aad);
      candidate = Buffer.from(decrypted);
      if (
        candidate.byteLength !== plaintext.byteLength ||
        !timingSafeEqual(candidate, plaintext)
      ) {
        throw new Error("KMS readiness plaintext mismatch");
      }
    } catch {
      throw new CredentialError(
        "CREDENTIAL_NOT_READY",
        "fiscal KMS encrypt/decrypt readiness failed",
      );
    } finally {
      plaintext.fill(0);
      candidate?.fill(0);
      decrypted?.fill(0);
    }
  }
}
