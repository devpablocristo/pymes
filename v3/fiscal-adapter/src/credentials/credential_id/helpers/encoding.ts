export function credentialID(bytes: Uint8Array): string {
  return `fcred_${Buffer.from(bytes).toString("base64url")}`;
}
