import type {
  CertificateInspection,
  CertificateValidator,
} from "./usecases.js";
import type { TrustedIssuerPatterns } from "./certificate/models/config.js";
import {
  assertPrivateKeyMatches,
  parseCertificate,
  subjectCUIT,
} from "./certificate/helpers/x509.js";
import { CredentialError } from "./usecases/domain/credential.js";

export class X509CertificateValidator implements CertificateValidator {
  constructor(
    private readonly trustedIssuers: TrustedIssuerPatterns,
    private readonly now: () => Date = () => new Date(),
  ) {}

  inspect(
    input: Parameters<CertificateValidator["inspect"]>[0],
  ): CertificateInspection {
    const certificate = parseCertificate(input.certificatePem);
    assertPrivateKeyMatches(certificate, input.privateKeyPem);
    if (subjectCUIT(certificate.subject) !== input.expectedCUIT) {
      throw new CredentialError("CERTIFICATE_CUIT_MISMATCH");
    }
    const issuerPattern = this.trustedIssuers[input.environment];
    issuerPattern.lastIndex = 0;
    if (!issuerPattern.test(certificate.issuer)) {
      throw new CredentialError("CERTIFICATE_ENVIRONMENT_MISMATCH");
    }
    const validFrom = new Date(certificate.validFrom);
    const expiresAt = new Date(certificate.validTo);
    const now = this.now();
    if (
      Number.isNaN(validFrom.getTime()) ||
      Number.isNaN(expiresAt.getTime()) ||
      validFrom.getTime() > now.getTime() + 5 * 60_000
    ) {
      throw new CredentialError("CERTIFICATE_INVALID");
    }
    if (expiresAt.getTime() <= now.getTime()) {
      throw new CredentialError("CERTIFICATE_EXPIRED");
    }
    return {
      fingerprint: certificate.fingerprint256.replaceAll(":", "").toLowerCase(),
      validFrom: validFrom.toISOString(),
      expiresAt: expiresAt.toISOString(),
      serialNumber: certificate.serialNumber,
    };
  }
}
