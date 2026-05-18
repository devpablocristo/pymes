import { useI18n, type LanguageCode } from '../lib/i18n';

type LanguageSelectorProps = {
  /** Clases extra para el contenedor (p. ej. en Perfil). */
  className?: string;
};

export function LanguageSelector(props?: LanguageSelectorProps) {
  const { className } = props ?? {};
  const { language, setLanguage, options, t } = useI18n();
  const rootClass = ['language-selector', className].filter(Boolean).join(' ');

  return (
    <div className={rootClass}>
      <select
        className="language-selector-input"
        value={language}
        onChange={(event) => setLanguage(event.target.value as LanguageCode)}
        aria-label={t('common.language.selectAria')}
      >
        {options.map((option) => (
          <option key={option.code} value={option.code}>
            {t(option.labelKey)}
          </option>
        ))}
      </select>
    </div>
  );
}
