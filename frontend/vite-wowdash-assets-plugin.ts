import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Plugin } from 'vite';
import serveStatic from 'serve-static';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Sirve `wowdash-assets/` en dev y lo copia a `dist/wowdash-assets` en build
 * (la carpeta `public/` del FE puede ser root-owned en algunos entornos Docker).
 */
export function wowdashAssetsPlugin(): Plugin {
  const assetsDir = path.resolve(__dirname, 'wowdash-assets');
  let outDir = 'dist';

  return {
    name: 'wowdash-assets-static',
    configResolved(config) {
      outDir = config.build.outDir;
    },
    configureServer(server) {
      server.middlewares.use('/wowdash-assets', serveStatic(assetsDir, { index: false }));
    },
    closeBundle() {
      if (!fs.existsSync(assetsDir)) {
        return;
      }
      const dest = path.join(outDir, 'wowdash-assets');
      fs.mkdirSync(path.dirname(dest), { recursive: true });
      fs.cpSync(assetsDir, dest, { recursive: true });
    },
  };
}
