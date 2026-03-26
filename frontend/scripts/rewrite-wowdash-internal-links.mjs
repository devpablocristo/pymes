/**
 * Reemplaza to="/labs/wowdash/..." por to={w('...')} e inyecta useWowdashNav en el primer componente del archivo.
 * Ejecutar desde frontend/: node scripts/rewrite-wowdash-internal-links.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const portRoot = path.resolve(__dirname, '../src/wowdash-port');

function walk(dir, acc = []) {
  for (const name of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, name.name);
    if (name.isDirectory()) walk(p, acc);
    else if (/\.(jsx|js)$/.test(name.name)) acc.push(p);
  }
  return acc;
}

function hookImportLine(fromFile) {
  const rel = path.relative(path.dirname(fromFile), path.join(portRoot, 'hook/useWowdashNav.jsx'));
  let norm = rel.split(path.sep).join('/').replace(/\.jsx$/i, '');
  const prefix = norm.startsWith('.') ? '' : './';
  return `import { useWowdashNav } from '${prefix}${norm}';\n`;
}

function replaceToLabsWowdash(content) {
  return content.replace(/to=(['"])\/labs\/wowdash(\/[^'"]*)?\1/g, (_, q, rest) => {
    const suf = !rest || rest === '/' ? '/' : rest;
    return `to={w(${q}${suf}${q})}`;
  });
}

function ensureImport(content, fromFile) {
  if (content.includes('useWowdashNav')) {
    return content;
  }
  const line = hookImportLine(fromFile);
  const lines = content.split('\n');
  let i = 0;
  while (i < lines.length && /^\s*import\s/.test(lines[i])) {
    i += 1;
  }
  lines.splice(i, 0, line.trimEnd());
  return lines.join('\n');
}

function injectHook(content) {
  if (!content.includes('to={w(')) {
    return content;
  }
  if (content.includes('useWowdashNav()')) {
    return content;
  }
  const tryPatterns = [
    /(const\s+\w+\s*=\s*\([^)]*\)\s*=>\s*\{)/,
    /(const\s+\w+\s*=\s*\(\)\s*=>\s*\{)/,
    /(function\s+\w+\s*\([^)]*\)\s*\{)/,
  ];
  for (const re of tryPatterns) {
    const m = re.exec(content);
    if (m) {
      const idx = m.index + m[0].length;
      return content.slice(0, idx) + '\n  const { w } = useWowdashNav();' + content.slice(idx);
    }
  }
  return content;
}

function processFile(absPath) {
  let t = fs.readFileSync(absPath, 'utf8');
  if (!t.includes('/labs/wowdash')) {
    return false;
  }
  const next = replaceToLabsWowdash(t);
  if (next === t) {
    return false;
  }
  let out = ensureImport(next, absPath);
  out = injectHook(out);
  fs.writeFileSync(absPath, out, 'utf8');
  return true;
}

let n = 0;
for (const f of walk(portRoot)) {
  if (processFile(f)) {
    n += 1;
    console.log('updated', path.relative(portRoot, f));
  }
}
console.log('files updated:', n);
