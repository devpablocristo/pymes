import { Pool } from "pg";
import { KeyManagementServiceClient } from "@google-cloud/kms";
import type { Config } from "./config.js";
import { X509CertificateValidator } from "./credentials/certificate.js";
import { RandomCredentialIDGenerator } from "./credentials/credential_id.js";
import { ForgeCSRGenerator } from "./credentials/csr.js";
import { GoogleKMSEnvelopeCipher } from "./credentials/kms.js";
import type { KMSClient } from "./credentials/kms/models/client.js";
import { LocalKMSClient } from "./credentials/local_kms.js";
import { PostgresCredentialRepository } from "./credentials/repository.js";
import { CredentialService } from "./credentials/usecases.js";
import { ArcaFiscalAuthority } from "./fiscal/arca.js";
import { MockFiscalAuthority } from "./fiscal/mock_authority.js";
import {
  createFiscalHTTPServer,
  type InternalAuthorizer,
  type FiscalRuntimeObserver,
} from "./fiscal/handler.js";
import { PostgresFiscalStore } from "./fiscal/repository.js";
import { FiscalService } from "./fiscal/usecases.js";
import { observePoolErrors } from "./fiscal/repository/helpers/pool-errors.js";
import { Ed25519JWTAuthorizer } from "./identity/internal_jwt.js";
import { InsecureLocalAuthorizer } from "./identity/insecure_local.js";

export function createFiscalRuntimeObserver(
  store: FiscalRuntimeObserver,
  cipher: Pick<GoogleKMSEnvelopeCipher, "ready">,
): FiscalRuntimeObserver {
  return {
    async ping(): Promise<void> {
      await store.ping();
      await cipher.ready();
    },
    metrics() {
      return store.metrics();
    },
  };
}

export async function initialize(config: Config) {
  const pool = new Pool({ connectionString: config.databaseURL });
  observePoolErrors(pool, (event) => {
    process.stderr.write(`${JSON.stringify(event)}\n`);
  });
  const store = new PostgresFiscalStore(pool);
  const credentialRepository = new PostgresCredentialRepository(pool);
  const cloudKMS =
    config.localKMSKeyB64 === undefined
      ? new KeyManagementServiceClient()
      : undefined;
  const kmsClient: KMSClient =
    cloudKMS === undefined
      ? new LocalKMSClient(config.localKMSKeyB64!)
      : (cloudKMS as unknown as KMSClient);
  const cipher = new GoogleKMSEnvelopeCipher(
    kmsClient,
    config.fiscalKMSKeyName,
  );
  const runtime = createFiscalRuntimeObserver(store, cipher);
  let credentials: CredentialService;
  const authority =
    config.mode === "mock"
      ? new MockFiscalAuthority(config.mockScenario, store)
      : new ArcaFiscalAuthority(
          {
            resolveMaterial: (input) => credentials.resolveMaterial(input),
          },
          credentialRepository,
          credentialRepository,
          cipher,
          { requestTimeoutMs: config.requestTimeoutMs },
        );
  credentials = new CredentialService(
    credentialRepository,
    cipher,
    new ForgeCSRGenerator(),
    new X509CertificateValidator({
      homologation: new RegExp(
        config.homologationIssuerPattern ?? ".*",
        "i",
      ),
      production: new RegExp(
        config.productionIssuerPattern ?? ".*",
        "i",
      ),
    }),
    new RandomCredentialIDGenerator(),
    authority,
  );
  try {
    await runtime.ping();
  } catch (error) {
    await cloudKMS?.close();
    await pool.end();
    throw error;
  }
  const application = new FiscalService(authority, store, undefined, {
    // ARCA authorize performs an exact consultation before FECAESolicitar.
    // Keep the durable lease beyond both bounded network operations so a live
    // owner is not mistaken for a crashed one.
    leaseDurationMs: config.requestTimeoutMs * 3,
  });
  const authorizer: InternalAuthorizer = config.allowInsecureLocal
    ? new InsecureLocalAuthorizer()
    : new Ed25519JWTAuthorizer(config.internalIssuer ?? "", config.internalJWKSJSON);
  const server = createFiscalHTTPServer(
    application,
    authorizer,
    runtime,
    credentials,
  );
  return {
    server,
    async close(): Promise<void> {
      await new Promise<void>((resolve, reject) => server.close((error) => error === undefined ? resolve() : reject(error)));
      await cloudKMS?.close();
      await pool.end();
    },
  };
}
