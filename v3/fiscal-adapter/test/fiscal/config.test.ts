import assert from "node:assert/strict";
import test from "node:test";
import { loadConfig } from "../../src/config.js";

test("startup is fail closed unless mock mode is explicit", () => {
  assert.throws(() => loadConfig({}), /FISCAL_ADAPTER_MODE=mock/);
  assert.throws(() => loadConfig({ FISCAL_ADAPTER_MODE: "mock" }), /FISCAL_DATABASE_URL/);
  const base = {
    FISCAL_ADAPTER_MODE: "mock",
    FISCAL_DATABASE_URL: "postgres://fiscal",
    PYMES_ENVIRONMENT: "test",
  };
  assert.throws(() => loadConfig(base), /PYMES_INTERNAL_ISSUER/);
  assert.equal(loadConfig({ ...base, FISCAL_ALLOW_INSECURE_LOCAL: "true" }).allowInsecureLocal, true);
  const jwks = JSON.stringify({
    keys: [{
      kty: "OKP",
      crv: "Ed25519",
      alg: "EdDSA",
      use: "sig",
      kid: "local-dev-1",
      x: "ebVWLo_mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ",
    }],
  });
  const secure = loadConfig({
    ...base,
    PYMES_INTERNAL_ISSUER: "pymes-v3",
    PYMES_INTERNAL_JWKS_JSON: jwks,
  });
  assert.equal(secure.allowInsecureLocal, false);
  assert.equal(secure.internalJWKSJSON, jwks);
});

test("legacy key and insecure bypass require an explicit local environment", () => {
  const base = {
    FISCAL_ADAPTER_MODE: "mock",
    FISCAL_DATABASE_URL: "postgres://fiscal",
    PYMES_INTERNAL_ISSUER: "pymes-v3",
  };
  for (const compatibility of [
    { FISCAL_ALLOW_INSECURE_LOCAL: "true" },
    {
      PYMES_ALLOW_LEGACY_INTERNAL_KEY_LOCAL: "true",
      PYMES_INTERNAL_PUBLIC_KEY_B64: "ebVWLo/mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ=",
      PYMES_INTERNAL_KEY_ID: "local-dev-1",
    },
  ]) {
    assert.throws(
      () => loadConfig({ ...base, ...compatibility, PYMES_ENVIRONMENT: "production" }),
      /forbidden outside development or test/,
    );
  }

  const legacy = loadConfig({
    ...base,
    PYMES_ENVIRONMENT: "development",
    PYMES_ALLOW_LEGACY_INTERNAL_KEY_LOCAL: "true",
    PYMES_INTERNAL_PUBLIC_KEY_B64: "ebVWLo/mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ=",
    PYMES_INTERNAL_KEY_ID: "local-dev-1",
  });
  assert.match(legacy.internalJWKSJSON, /"kid":"local-dev-1"/);
});
