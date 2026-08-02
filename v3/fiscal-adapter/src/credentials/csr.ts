import forge from "node-forge";
import type { CSRGenerator, GeneratedCSR } from "./usecases.js";
import type { CSRSubject } from "./csr/models/subject.js";
import { printableDN } from "./csr/helpers/distinguished-name.js";
import { CredentialError } from "./usecases/domain/credential.js";

export class ForgeCSRGenerator implements CSRGenerator {
  constructor(private readonly bits = 2048) {
    if (!Number.isSafeInteger(bits) || bits < 2048) {
      throw new CredentialError("VALIDATION_ERROR", "RSA key must be at least 2048 bits");
    }
  }

  async generate(input: CSRSubject): Promise<GeneratedCSR> {
    const keyPair = await generateKeyPair(this.bits);
    const request = forge.pki.createCertificationRequest();
    request.publicKey = keyPair.publicKey;
    request.setSubject([
      { name: "countryName", value: "AR" },
      {
        name: "organizationName",
        value: printableDN(input.legalName, "legal name"),
      },
      {
        name: "commonName",
        value: printableDN(input.commonName, "common name"),
      },
      { name: "serialNumber", value: `CUIT ${input.cuit}` },
    ]);
    request.sign(keyPair.privateKey, forge.md.sha256.create());
    if (!request.verify()) {
      throw new CredentialError("CERTIFICATE_INVALID", "generated CSR did not verify");
    }
    return {
      privateKeyPem: forge.pki.privateKeyToPem(keyPair.privateKey),
      csrPem: forge.pki.certificationRequestToPem(request),
    };
  }
}

function generateKeyPair(bits: number): Promise<forge.pki.rsa.KeyPair> {
  return new Promise((resolve, reject) => {
    forge.pki.rsa.generateKeyPair(
      { bits, workers: -1 },
      (error, keyPair) => {
        if (error !== null && error !== undefined) {
          reject(error);
          return;
        }
        if (keyPair === undefined) {
          reject(new CredentialError("CERTIFICATE_INVALID", "RSA generation failed"));
          return;
        }
        resolve(keyPair);
      },
    );
  });
}
