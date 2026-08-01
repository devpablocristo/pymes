import assert from "node:assert/strict";
import test from "node:test";
import forge from "node-forge";
import { ForgeCSRGenerator } from "../../src/credentials/csr.js";
import { X509CertificateValidator } from "../../src/credentials/certificate.js";
import { CredentialError } from "../../src/credentials/usecases/domain/credential.js";

test("CSR uses an RSA-2048 SHA-256 request with ARCA-compatible CUIT subject", async () => {
  const generated = await new ForgeCSRGenerator().generate({
    cuit: "20123456786",
    legalName: "Cliente Uno SA",
    commonName: "pymes-homologacion",
  });
  const request = forge.pki.certificationRequestFromPem(generated.csrPem);
  const attributes = Object.fromEntries(
    request.subject.attributes.map((attribute) => [
      attribute.name,
      attribute.value,
    ]),
  );
  const privateKey = forge.pki.privateKeyFromPem(generated.privateKeyPem);

  assert.equal(request.verify(), true);
  assert.equal(attributes.countryName, "AR");
  assert.equal(attributes.organizationName, "Cliente Uno SA");
  assert.equal(attributes.commonName, "pymes-homologacion");
  assert.equal(attributes.serialNumber, "CUIT 20123456786");
  assert.ok(privateKey.n.bitLength() >= 2048);
  assert.equal(request.signatureOid, "1.2.840.113549.1.1.11");
});

test("certificate validation binds private key, CUIT, environment and validity", async () => {
  const generated = await new ForgeCSRGenerator().generate({
    cuit: "20123456786",
    legalName: "Cliente Uno SA",
    commonName: "pymes-homologacion",
  });
  const certificatePem = certificateFor(
    generated.privateKeyPem,
    "20123456786",
    "ARCA Homologacion",
  );
  const validator = new X509CertificateValidator(
    {
      homologation: /ARCA Homologacion/i,
      production: /ARCA Produccion/i,
    },
    () => new Date("2026-08-01T12:00:00.000Z"),
  );

  const inspection = validator.inspect({
    certificatePem,
    privateKeyPem: generated.privateKeyPem,
    expectedCUIT: "20123456786",
    environment: "homologation",
  });
  assert.match(inspection.fingerprint, /^[a-f0-9]{64}$/);
  assert.equal(inspection.expiresAt, "2027-08-01T00:00:00.000Z");

  assert.throws(
    () =>
      validator.inspect({
        certificatePem,
        privateKeyPem: generated.privateKeyPem,
        expectedCUIT: "30710158202",
        environment: "homologation",
      }),
    hasCode("CERTIFICATE_CUIT_MISMATCH"),
  );
  assert.throws(
    () =>
      validator.inspect({
        certificatePem,
        privateKeyPem: generated.privateKeyPem,
        expectedCUIT: "20123456786",
        environment: "production",
      }),
    hasCode("CERTIFICATE_ENVIRONMENT_MISMATCH"),
  );

  const other = await new ForgeCSRGenerator().generate({
    cuit: "20123456786",
    legalName: "Otro",
    commonName: "otro",
  });
  assert.throws(
    () =>
      validator.inspect({
        certificatePem,
        privateKeyPem: other.privateKeyPem,
        expectedCUIT: "20123456786",
        environment: "homologation",
      }),
    hasCode("CERTIFICATE_KEY_MISMATCH"),
  );
});

function certificateFor(
  privateKeyPem: string,
  cuit: string,
  issuerName: string,
): string {
  const privateKey = forge.pki.privateKeyFromPem(privateKeyPem);
  const certificate = forge.pki.createCertificate();
  certificate.publicKey = forge.pki.setRsaPublicKey(privateKey.n, privateKey.e);
  certificate.serialNumber = "01";
  certificate.validity.notBefore = new Date("2026-07-01T00:00:00.000Z");
  certificate.validity.notAfter = new Date("2027-08-01T00:00:00.000Z");
  certificate.setSubject([
    { name: "countryName", value: "AR" },
    { name: "organizationName", value: "Cliente Uno SA" },
    { name: "commonName", value: "pymes" },
    { name: "serialNumber", value: `CUIT ${cuit}` },
  ]);
  certificate.setIssuer([
    { name: "countryName", value: "AR" },
    { name: "organizationName", value: "ARCA" },
    { name: "commonName", value: issuerName },
  ]);
  certificate.sign(privateKey, forge.md.sha256.create());
  return forge.pki.certificateToPem(certificate);
}

function hasCode(code: string) {
  return (error: unknown) =>
    error instanceof CredentialError && error.code === code;
}
