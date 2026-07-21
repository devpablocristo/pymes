import { render, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { LanguageProvider, toSentenceCase, useI18n } from '.';

function Probe() {
  const { t } = useI18n();

  return (
    <>
      <span data-testid="dashboard">{t('shell.nav.dashboard')}</span>
      <span data-testid="admin">{t('shell.nav.admin')}</span>
    </>
  );
}

describe('LanguageProvider', () => {
  it('traduce las etiquetas del shell en español', () => {
    const view = render(
      <LanguageProvider initialLanguage="es">
        <Probe />
      </LanguageProvider>,
    );
    const scope = within(view.container);

    expect(scope.getByTestId('dashboard')).toHaveTextContent('Inicio');
    expect(scope.getByTestId('admin')).toHaveTextContent('Administración');
  });

  it('traduce las etiquetas del shell en inglés', () => {
    const view = render(
      <LanguageProvider initialLanguage="en">
        <Probe />
      </LanguageProvider>,
    );
    const scope = within(view.container);

    expect(scope.getByTestId('dashboard')).toHaveTextContent('Home');
    expect(scope.getByTestId('admin')).toHaveTextContent('Administration');
  });

  it('normalizes UI labels to sentence case while preserving acronyms and Dashboard', () => {
    expect(toSentenceCase('Dashboard')).toBe('Dashboard');
    expect(toSentenceCase('API Keys')).toBe('API keys');
    expect(toSentenceCase('Teachers · Specialties')).toBe('Teachers · specialties');
  });
});
