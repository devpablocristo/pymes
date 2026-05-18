import { useI18n } from '../lib/i18n';

type AdminAppearanceSectionProps = {
  uiTheme: string;
  onToggle: () => void;
};

export function AdminAppearanceSection({ uiTheme, onToggle }: AdminAppearanceSectionProps) {
  const { t } = useI18n();
  return (
    <div className="card">
      <div className="card-header">
        <h2>{t('profile.admin.appearanceTitle')}</h2>
      </div>
      <p className="text-secondary">{t('profile.admin.appearanceLead')}</p>
      <div className="actions-row u-mt-sm">
        <button
          type="button"
          className="btn-secondary"
          onClick={onToggle}
          title={uiTheme === 'dark' ? t('shell.theme.light') : t('shell.theme.dark')}
        >
          {uiTheme === 'dark' ? t('shell.theme.light') : t('shell.theme.dark')}
        </button>
      </div>
    </div>
  );
}
