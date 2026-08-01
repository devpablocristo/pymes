import type { KMSInt64Value } from "../helpers/crc32c.js";

export interface KMSBytesResponse {
  ciphertext?: Uint8Array | string | null;
  plaintext?: Uint8Array | string | null;
  ciphertextCrc32c?: KMSInt64Value | null;
  plaintextCrc32c?: KMSInt64Value | null;
  verifiedPlaintextCrc32c?: boolean | null;
  verifiedCiphertextCrc32c?: boolean | null;
  verifiedAdditionalAuthenticatedDataCrc32c?: boolean | null;
}

export interface KMSClient {
  encrypt(request: {
    name: string;
    plaintext: Uint8Array;
    additionalAuthenticatedData: Uint8Array;
    plaintextCrc32c: KMSInt64Value;
    additionalAuthenticatedDataCrc32c: KMSInt64Value;
  }): Promise<[KMSBytesResponse]>;
  decrypt(request: {
    name: string;
    ciphertext: Uint8Array;
    additionalAuthenticatedData: Uint8Array;
    ciphertextCrc32c: KMSInt64Value;
    additionalAuthenticatedDataCrc32c: KMSInt64Value;
  }): Promise<[KMSBytesResponse]>;
}
