import { useQuery } from '@tanstack/react-query';
import { AccountPlanSection } from '../components/AccountPlanSection';
import { PageLayout } from '../components/PageLayout';
import { clerkEnabled } from '../lib/auth';
import { useI18n } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import { getSessionWithTimeout } from './SettingsPage.data';
import { SettingsProfileBody } from './SettingsProfileBody';

type SettingsPageProps = {
  embedded?: boolean;
};

export function SettingsPage({ embedded = false }: SettingsPageProps = {}) {
  const { t } = useI18n();
  const body = <SettingsProfileBody clerkMode={clerkEnabled} />;

  if (embedded) {
    return <>{body}</>;
  }

  return (
    <PageLayout className="profile-page" title={t('profile.page.title')} lead={t('profile.page.subtitle')}>
      {body}
    </PageLayout>
  );
}

/** Sección de facturación standalone para usar en tabs de ajustes. */
export function BillingSettingsSection() {
  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: getSessionWithTimeout,
    retry: false,
  });
  if (!sessionQuery.data) return <div className="spinner" />;
  return <AccountPlanSection session={sessionQuery.data} />;
}
