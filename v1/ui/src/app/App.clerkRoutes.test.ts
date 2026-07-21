import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Clerk usa subrutas bajo /login y /signup (p. ej. /login/tasks/choose-organization).
 * Rutas exactas /login dejan la pantalla en blanco en esas URLs.
 */
describe('App routes (Clerk path routing)', () => {
  it('declares /login/* and /signup/*', () => {
    const dir = path.dirname(fileURLToPath(import.meta.url));
    const src = readFileSync(path.join(dir, 'App.tsx'), 'utf8');
    expect(src).toMatch(/path="\/login\/\*"/);
    expect(src).toMatch(/path="\/signup\/\*"/);
  });
});
