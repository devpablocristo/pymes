/**
 * Encapsula las hojas del template Wowdash bajo #wowdash-template-root
 * para que el reset/agresividad de Bootstrap no rompa el shell Pymes fuera del laboratorio.
 *
 * Ejecutar tras actualizar wowdash-assets: `node scripts/prefix-wowdash-css.mjs`
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import postcss from 'postcss';
import prefixwrap from 'postcss-prefixwrap';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const cssDir = path.join(frontendRoot, 'wowdash-assets', 'css');

/** Orden alineado al template original (Bootstrap → libs → editores → tema). */
const INPUTS = [
  'lib/bootstrap.min.css',
  'remixicon.css',
  'lib/apexcharts.css',
  'lib/dataTables.min.css',
  'lib/flatpickr.min.css',
  'lib/full-calendar.css',
  'lib/jquery-jvectormap-2.0.5.css',
  'lib/magnific-popup.css',
  'lib/slick.css',
  'lib/prism.css',
  'lib/editor.quill.snow.css',
  'lib/editor-katex.min.css',
  'lib/editor.atom-one-dark.min.css',
  'lib/file-upload.css',
  'lib/audioplayer.css',
  'lib/animate.min.css',
  'style.css',
  'extra.css',
];

async function main() {
  const parts = [];
  for (const rel of INPUTS) {
    const abs = path.join(cssDir, rel);
    if (!fs.existsSync(abs)) {
      console.warn('[wowdash-css] skip missing', rel);
      continue;
    }
    parts.push(`/* === ${rel} === */\n`, fs.readFileSync(abs, 'utf8'), '\n');
  }
  const combined = parts.join('');

  const result = await postcss([prefixwrap('#wowdash-template-root')]).process(combined, { from: undefined });
  const out = path.join(cssDir, 'pymes-scoped.css');
  fs.writeFileSync(out, result.css);
  console.log('[wowdash-css] wrote', out, `(${(result.css.length / 1024).toFixed(0)} KiB)`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
