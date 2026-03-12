import { useI18n, type LanguageCode } from '../lib/i18n';

export function LanguageSelector() {
  const { language, setLanguage, options, t } = useI18n();

  return (
    <label className="language-selector">
      <span className="language-selector-label">{t('common.language.label')}</span>
      <select
        className="language-selector-input"
        value={language}
        onChange={(event) => setLanguage(event.target.value as LanguageCode)}
        aria-label={t('common.language.label')}
      >
        {options.map((option) => (
          <option key={option.code} value={option.code}>
            {t(option.labelKey)}
          </option>
        ))}
      </select>
    </label>
  );
}
