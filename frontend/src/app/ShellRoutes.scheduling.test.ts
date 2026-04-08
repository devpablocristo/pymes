import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

describe('ShellRoutes scheduling paths', () => {
  it('keeps the public scheduling preview route and avoids legacy scheduling entrypoints', () => {
    const dir = path.dirname(fileURLToPath(import.meta.url));
    const src = readFileSync(path.join(dir, 'ShellRoutes.tsx'), 'utf8');

    expect(src).toMatch(/path="\/web-clientes"/);
    expect(src).not.toMatch(/path="\/modules\/appointments"/);
    expect(src).not.toMatch(/path="\/professionals\/teachers\/public"/);
  });
});
