import { Pool } from "pg";
import type { Config } from "./config.js";
import { MockFiscalAuthority } from "./fiscal/companion/mock-authority.js";
import { createFiscalHTTPServer } from "./fiscal/handler/http.js";
import { observePoolErrors } from "./fiscal/repository/pool-errors.js";
import { PostgresFiscalStore } from "./fiscal/repository/postgres-store.js";
import { FiscalService } from "./fiscal/usecases/fiscal-service.js";
import type { InternalAuthorizer } from "./fiscal/ports/internal-authorizer.js";
import { Ed25519JWTAuthorizer } from "./identity/access/ed25519-jwt-authorizer.js";
import { InsecureLocalAuthorizer } from "./identity/access/authorizer.js";

export async function initialize(config: Config) {
  const pool = new Pool({ connectionString: config.databaseURL });
  observePoolErrors(pool, (event) => {
    process.stderr.write(`${JSON.stringify(event)}\n`);
  });
  const store = new PostgresFiscalStore(pool);
  try {
    await store.ping();
  } catch (error) {
    await pool.end();
    throw error;
  }
  const authority = new MockFiscalAuthority(config.mockScenario, store);
  const application = new FiscalService(authority, store);
  const authorizer: InternalAuthorizer = config.allowInsecureLocal
    ? new InsecureLocalAuthorizer()
    : new Ed25519JWTAuthorizer(config.internalIssuer ?? "", config.internalJWKSJSON);
  const server = createFiscalHTTPServer(application, authorizer, store);
  return {
    server,
    async close(): Promise<void> {
      await new Promise<void>((resolve, reject) => server.close((error) => error === undefined ? resolve() : reject(error)));
      await pool.end();
    },
  };
}
