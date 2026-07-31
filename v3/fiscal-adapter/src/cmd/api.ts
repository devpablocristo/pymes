import { loadConfig } from "../config.js";
import { initialize } from "../wire.js";

const config = loadConfig();
const app = await initialize(config);

app.server.listen(config.port, "0.0.0.0", () => {
  process.stdout.write(`fiscal mock listening on :${config.port}\n`);
});

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, () => {
    app.close().then(() => process.exit(0), (error: unknown) => {
      process.stderr.write(`${String(error)}\n`);
      process.exit(1);
    });
  });
}
