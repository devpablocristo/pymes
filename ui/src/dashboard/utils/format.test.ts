import { describe, expect, it } from 'vitest';
import {
  formatDashboardDateTime,
  formatDashboardMoney,
  formatDashboardShortDate,
  localeForLanguage,
} from './format';

describe('dashboard format helpers', () => {
  it('maps ui language to dashboard locale', () => {
    expect(localeForLanguage('es')).toBe('es-AR');
    expect(localeForLanguage('en')).toBe('en-US');
  });

  it('formats money according to the active locale', () => {
    expect(formatDashboardMoney(1_250_000, 'es')).toBe('$1,3M');
    expect(formatDashboardMoney(1_250_000, 'en')).toBe('$1.3M');
    expect(formatDashboardMoney(12.3, 'es')).toBe('$12,3');
    expect(formatDashboardMoney(12.3, 'en')).toBe('$12.3');
    expect(formatDashboardMoney(undefined, 'es')).toBe('—');
  });

  it('formats short dates and date times according to locale', () => {
    expect(formatDashboardShortDate('2026-04-02T13:45:00Z', 'es')).toContain('abr');
    expect(formatDashboardShortDate('2026-04-02T13:45:00Z', 'en')).toContain('Apr');
    expect(formatDashboardDateTime('2026-04-02T13:45:00Z', 'en')).toContain('Apr');
    expect(formatDashboardShortDate('not-a-date', 'en')).toBe('—');
    expect(formatDashboardDateTime('not-a-date', 'es')).toBe('—');
    expect(formatDashboardShortDate(undefined, 'en')).toBe('—');
  });
});
