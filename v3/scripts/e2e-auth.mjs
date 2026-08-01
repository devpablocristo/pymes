#!/usr/bin/env node

import {
  createHmac,
  createSign,
  randomUUID,
} from "node:crypto";
import { readFileSync } from "node:fs";

function fail(message) {
  process.stderr.write(`${message}\n`);
  process.exit(2);
}

function base64url(value) {
  return Buffer.from(value).toString("base64url");
}

function mintSession(args) {
  if (args.length !== 6) {
    fail("usage: e2e-auth.mjs session PRIVATE_KEY ISSUER AUDIENCE AZP SUBJECT ORGANIZATION_ID");
  }
  const [privateKeyPath, issuer, audience, authorizedParty, subject, organizationID] = args;
  const now = Math.floor(Date.now() / 1000);
  const header = base64url(JSON.stringify({ alg: "RS256", typ: "JWT" }));
  const payload = base64url(JSON.stringify({
    iss: issuer,
    aud: [audience],
    azp: authorizedParty,
    sub: subject,
    sid: `sess_${randomUUID()}`,
    org_id: organizationID,
    org_role: "org:admin",
    org_permissions: [
      "org:members:read",
      "org:members:manage",
    ],
    iat: now - 5,
    nbf: now - 5,
    exp: now + 1800,
  }));
  const unsigned = `${header}.${payload}`;
  const signer = createSign("RSA-SHA256");
  signer.update(unsigned);
  signer.end();
  const signature = signer.sign(readFileSync(privateKeyPath)).toString("base64url");
  process.stdout.write(`${unsigned}.${signature}\n`);
}

function signWebhook(args) {
  if (args.length !== 4) {
    fail("usage: e2e-auth.mjs webhook WEBHOOK_SECRET MESSAGE_ID TIMESTAMP PAYLOAD_FILE");
  }
  const [rawSecret, messageID, timestamp, payloadPath] = args;
  const encodedSecret = rawSecret.startsWith("whsec_")
    ? rawSecret.slice("whsec_".length)
    : rawSecret;
  const secret = Buffer.from(encodedSecret, "base64");
  if (secret.length === 0) {
    fail("webhook secret must contain base64 key material");
  }
  const payload = readFileSync(payloadPath === "-" ? 0 : payloadPath);
  const signed = Buffer.concat([
    Buffer.from(`${messageID}.${timestamp}.`, "utf8"),
    payload,
  ]);
  const signature = createHmac("sha256", secret).update(signed).digest("base64");
  process.stdout.write(`v1,${signature}\n`);
}

const [mode, ...args] = process.argv.slice(2);
switch (mode) {
  case "session":
    mintSession(args);
    break;
  case "webhook":
    signWebhook(args);
    break;
  default:
    fail("usage: e2e-auth.mjs session|webhook ...");
}
