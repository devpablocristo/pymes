export interface KMSBytesResponse {
  ciphertext?: Uint8Array | string | null;
  plaintext?: Uint8Array | string | null;
}

export interface KMSClient {
  encrypt(request: {
    name: string;
    plaintext: Uint8Array;
    additionalAuthenticatedData: Uint8Array;
  }): Promise<[KMSBytesResponse]>;
  decrypt(request: {
    name: string;
    ciphertext: Uint8Array;
    additionalAuthenticatedData: Uint8Array;
  }): Promise<[KMSBytesResponse]>;
}
