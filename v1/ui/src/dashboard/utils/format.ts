import type { LanguageCode } from '../../lib/i18n';

export function localeForLanguage(language: LanguageCode): string {
  return language === 'en' ? 'en-US' : 'es-AR';
}

function isValidDate(date: Date): boolean {
  return !Number.isNaN(date.getTime());
}

function formatCompactNumber(value: number, locale: string, maximumFractionDigits: number): string {
  return value.toLocaleString(locale, {
    minimumFractionDigits: 0,
    maximumFractionDigits,
  });
}

export function formatDashboardMoney(value: number | null | undefined, language: LanguageCode): string {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '—';
  }

  const locale = localeForLanguage(language);
  const abs = Math.abs(value);

  if (abs >= 1_000_000) {
    return `$${formatCompactNumber(value / 1_000_000, locale, 1)}M`;
  }

  if (abs >= 1_000) {
    return `$${formatCompactNumber(value / 1_000, locale, 0)}K`;
  }

  return `$${value.toLocaleString(locale)}`;
}

export function formatDashboardShortDate(value: string | undefined, language: LanguageCode): string {
  if (!value) {
    return '—';
  }

  const date = new Date(value);
  if (!isValidDate(date)) {
    return '—';
  }

  return date.toLocaleDateString(localeForLanguage(language), {
    day: '2-digit',
    month: 'short',
  });
}

export function formatDashboardDateTime(value: string | undefined, language: LanguageCode): string {
  if (!value) {
    return '—';
  }

  const date = new Date(value);
  if (!isValidDate(date)) {
    return '—';
  }

  return date.toLocaleDateString(localeForLanguage(language), {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  });
}
