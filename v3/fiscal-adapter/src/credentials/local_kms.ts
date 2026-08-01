import {
  createCipheriv,
  createDecipheriv,
  randomBytes,
} from "node:crypto";
import type {
  KMSBytesResponse,
  KMSClient,
} from "./kms/models/client.js";
import {
  LOCAL_KMS_IV_BYTES,
  LOCAL_KMS_TAG_BYTES,
} from "./local_kms/models/ciphertext.js";
import { decodeLocalKMSKey } from "./local_kms/helpers/key.js";
import { CredentialError } from "./usecases/domain/credential.js";

/**
 * Emulador local del límite KMS. Sólo se compone en development/test; permite
 * probar persistencia y AAD sin credenciales GCP.
 */
export class LocalKMSClient implements KMSClient {
  private readonly key: Uint8Array;

  constructor(
    encodedKey: string,
    private readonly entropy: (size: number) => Uint8Array = randomBytes,
  ) {
    this.key = decodeLocalKMSKey(encodedKey);
  }

  async encrypt(request: {
    name: string;
    plaintext: Uint8Array;
    additionalAuthenticatedData: Uint8Array;
  }): Promise<[KMSBytesResponse]> {
    const iv = Buffer.from(this.entropy(LOCAL_KMS_IV_BYTES));
    const cipher = createCipheriv("aes-256-gcm", this.key, iv);
    cipher.setAAD(request.additionalAuthenticatedData);
    const ciphertext = Buffer.concat([
      cipher.update(request.plaintext),
      cipher.final(),
    ]);
    return [{
      ciphertext: Buffer.concat([iv, cipher.getAuthTag(), ciphertext]),
    }];
  }

  async decrypt(request: {
    name: string;
    ciphertext: Uint8Array;
    additionalAuthenticatedData: Uint8Array;
  }): Promise<[KMSBytesResponse]> {
    const encrypted = Buffer.from(request.ciphertext);
    if (encrypted.byteLength <= LOCAL_KMS_IV_BYTES + LOCAL_KMS_TAG_BYTES) {
      throw new CredentialError("CREDENTIAL_NOT_READY", "invalid local KMS envelope");
    }
    try {
      const iv = encrypted.subarray(0, LOCAL_KMS_IV_BYTES);
      const tag = encrypted.subarray(
        LOCAL_KMS_IV_BYTES,
        LOCAL_KMS_IV_BYTES + LOCAL_KMS_TAG_BYTES,
      );
      const ciphertext = encrypted.subarray(
        LOCAL_KMS_IV_BYTES + LOCAL_KMS_TAG_BYTES,
      );
      const decipher = createDecipheriv("aes-256-gcm", this.key, iv);
      decipher.setAAD(request.additionalAuthenticatedData);
      decipher.setAuthTag(tag);
      return [{
        plaintext: Buffer.concat([
          decipher.update(ciphertext),
          decipher.final(),
        ]),
      }];
    } catch {
      throw new CredentialError("CREDENTIAL_NOT_READY", "local KMS authentication failed");
    }
  }
}
