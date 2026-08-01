import { randomBytes } from "node:crypto";
import type { EnvelopeCipher, SealedValue } from "./usecases.js";
import type { KMSClient } from "./kms/models/client.js";
import {
  decodeBase64,
  decryptAESGCM,
  encryptAESGCM,
  kmsBytes,
} from "./kms/helpers/aes.js";
import { CredentialError } from "./usecases/domain/credential.js";

export class GoogleKMSEnvelopeCipher implements EnvelopeCipher {
  constructor(
    private readonly client: KMSClient,
    private readonly keyName: string,
    private readonly entropy: (size: number) => Uint8Array = randomBytes,
  ) {
    if (
      !/^projects\/[^/]+\/locations\/[^/]+\/keyRings\/[^/]+\/cryptoKeys\/[^/]+$/.test(
        keyName,
      )
    ) {
      throw new CredentialError("VALIDATION_ERROR", "invalid fiscal KMS key name");
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
      });
      return {
        format: "aes-256-gcm+kms-v1",
        ciphertext: Buffer.from(encrypted.ciphertext).toString("base64"),
        encryptedDataKey: Buffer.from(
          kmsBytes(wrapped.ciphertext, "ciphertext"),
        ).toString("base64"),
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
    });
    const dataKey = Buffer.from(kmsBytes(unwrapped.plaintext, "plaintext"));
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
}
