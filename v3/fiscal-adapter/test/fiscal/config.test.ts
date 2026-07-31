import assert from "node:assert/strict";
import test from "node:test";
import { loadConfig } from "../../src/config.js";

test("startup is fail closed unless mock mode is explicit", () => {
  assert.throws(() => loadConfig({}), /FISCAL_ADAPTER_MODE=mock/);
  assert.throws(() => loadConfig({ FISCAL_ADAPTER_MODE: "mock" }), /FISCAL_DATABASE_URL/);
  const base = { FISCAL_ADAPTER_MODE: "mock", FISCAL_DATABASE_URL: "postgres://fiscal" };
  assert.throws(() => loadConfig(base), /PYMES_INTERNAL_ISSUER/);
  assert.equal(loadConfig({ ...base, FISCAL_ALLOW_INSECURE_LOCAL: "true" }).allowInsecureLocal, true);
  assert.equal(loadConfig({ ...base, PYMES_INTERNAL_ISSUER: "pymes-v3", PYMES_INTERNAL_PUBLIC_KEY_B64: "a".repeat(44) }).allowInsecureLocal, false);
});
