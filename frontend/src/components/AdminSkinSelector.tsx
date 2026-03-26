import { useEffect, useState } from 'react';
import { type AdminSkinId, getAdminSkin, setAdminSkin } from '../lib/adminSkin';
import { useI18n } from '../lib/i18n';

export function AdminSkinSelector({ className }: { className?: string }) {
  const { t } = useI18n();
  const [skin, setSkin] = useState<AdminSkinId>(() => getAdminSkin());

  useEffect(() => {
    setSkin(getAdminSkin());
  }, []);

  return (
    <div className={className}>
      <label className="profile-field-label" htmlFor="admin-skin-select">
        {t('profile.consoleSkin.label')}
      </label>
      <select
        id="admin-skin-select"
        className="profile-input"
        value={skin}
        onChange={(e) => {
          const next = e.target.value as AdminSkinId;
          setAdminSkin(next);
          setSkin(next);
        }}
      >
        <option value="wowdash">{t('profile.consoleSkin.wowdash')}</option>
        <option value="classic">{t('profile.consoleSkin.classic')}</option>
      </select>
      <p className="text-muted profile-field-hint">{t('profile.consoleSkin.hint')}</p>
    </div>
  );
}
