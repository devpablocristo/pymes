import { describe, expect, it } from 'vitest';
import {
  CALENDAR_BOOKING_COLOR_OPTIONS,
  DEFAULT_BOOKING_COLOR_HEX,
  PRODUCT_PALETTE,
  PRODUCT_PALETTE_LABELS_ES,
  themeHubColorSwatches,
} from './productPalette';

describe('productPalette', () => {
  it('tiene hex estable y token por entrada', () => {
    expect(PRODUCT_PALETTE.length).toBeGreaterThanOrEqual(8);
    for (const p of PRODUCT_PALETTE) {
      expect(p.id).toMatch(/^[a-z_]+$/);
      expect(p.hex).toMatch(/^#[0-9a-f]{6}$/i);
      expect(p.token).toMatch(/^var\(--color-/);
    }
  });

  it('default de citas coincide con el primer swatch', () => {
    expect(DEFAULT_BOOKING_COLOR_HEX).toBe(PRODUCT_PALETTE[0].hex);
  });

  it('opciones de calendario alinean hex y swatch', () => {
    expect(CALENDAR_BOOKING_COLOR_OPTIONS.length).toBe(PRODUCT_PALETTE.length);
    expect(CALENDAR_BOOKING_COLOR_OPTIONS[0]).toEqual({
      hex: PRODUCT_PALETTE[0].hex,
      swatch: PRODUCT_PALETTE[0].token,
    });
  });

  it('theme hub: 6 swatches con etiquetas ES', () => {
    const hub = themeHubColorSwatches();
    expect(hub).toHaveLength(6);
    expect(hub[0].id).toBe('primary');
    expect(hub[0].label).toBe(PRODUCT_PALETTE_LABELS_ES.primary);
    expect(hub[0].bg).toBe('var(--color-primary)');
  });
});
