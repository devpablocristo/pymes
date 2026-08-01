import {
  X509Certificate,
  createPublicKey,
  timingSafeEqual,
} from "node:crypto";
import { CredentialError } from "../../usecases/domain/credential.js";

export function parseCertificate(pem: string): X509Certificate {
  if (
    !/^-----BEGIN CERTIFICATE-----[\s\S]+-----END CERTIFICATE-----\s*$/.test(
      pem,
    )
  ) {
    throw new CredentialError("CERTIFICATE_INVALID", "certificate must be PEM");
  }
  try {
    return new X509Certificate(pem);
  } catch {
    throw new CredentialError("CERTIFICATE_INVALID");
  }
}

export function assertPrivateKeyMatches(
  certificate: X509Certificate,
  privateKeyPem: string,
): void {
  try {
    const expected = Buffer.from(
      certificate.publicKey.export({ type: "spki", format: "der" }),
    );
    const actual = Buffer.from(
      createPublicKey(privateKeyPem).export({ type: "spki", format: "der" }),
    );
    if (
      expected.byteLength !== actual.byteLength ||
      !timingSafeEqual(expected, actual)
    ) {
      throw new CredentialError("CERTIFICATE_KEY_MISMATCH");
    }
  } catch (error) {
    if (error instanceof CredentialError) throw error;
    throw new CredentialError("CERTIFICATE_KEY_MISMATCH");
  }
}

export function subjectCUIT(subject: string): string | undefined {
  const match = subject.match(/(?:serialNumber\s*=\s*)?CUIT\s+(\d{11})/i);
  return match?.[1];
}
