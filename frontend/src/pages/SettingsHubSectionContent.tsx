import { lazy, Suspense } from 'react';
import { Link } from 'react-router-dom';
import { AdminSkinSelector } from '../components/AdminSkinSelector';
import { LanguageSelector } from '../components/LanguageSelector';
import type { SettingsSection } from './SettingsHubPage.model';
import {
  AlertChannelsTab,
  AutomationHubTab,
  BranchesTab,
  CompanyTab,
  CurrenciesTab,
  FirebaseNotifTab,
  GatewayTab,
  LanguagesTab,
  ThemeTab,
} from './SettingsHubTabs';

const AdminPage = lazy(() => import('./AdminPage').then((m) => ({ default: m.AdminPage })));
const BillingSection = lazy(() => import('./SettingsPage').then((m) => ({ default: m.BillingSettingsSection })));
const ProfilePage = lazy(() => import('./SettingsPage').then((m) => ({ default: m.SettingsPage })));
const NotificationPreferencesPage = lazy(() =>
  import('./NotificationPreferencesPage').then((m) => ({ default: m.NotificationPreferencesPage })),
);

type SettingsHubSectionContentProps = {
  section: SettingsSection;
  isAccountAdmin: boolean;
};

function SettingsSpinner() {
  return <div className="spinner" />;
}

export function SettingsHubSectionContent({ section, isAccountAdmin }: SettingsHubSectionContentProps) {
  return (
    <>
      {section === 'profile' && (
        <Suspense fallback={<SettingsSpinner />}>
          <ProfilePage embedded />
        </Suspense>
      )}
      {section === 'branches' && <BranchesTab />}
      {section === 'notifications' && (
        <>
          <div className="card stg__card-mb">
            <p className="text-secondary u-m-0 u-text-base">
              La bandeja de avisos y aprobaciones está en el menú <strong>Base → Notificaciones</strong> (
              <Link to="/notifications">abrir centro</Link>
              ).
            </p>
          </div>
          <Suspense fallback={<SettingsSpinner />}>
            <NotificationPreferencesPage embedded />
          </Suspense>
          <AlertChannelsTab />
        </>
      )}
      {section === 'automation' && <AutomationHubTab />}
      {section === 'company' && <CompanyTab />}
      {section === 'firebaseNotif' && <FirebaseNotifTab />}
      {section === 'currencies' && <CurrenciesTab />}
      {section === 'gateway' && (
        <>
          <Suspense fallback={<SettingsSpinner />}>
            <BillingSection />
          </Suspense>
          <GatewayTab />
        </>
      )}
      {section === 'appearance' && (
        <>
          <Suspense fallback={<SettingsSpinner />}>
            <AdminPage section="appearance" embedded />
          </Suspense>
          <div className="card">
            <AdminSkinSelector />
          </div>
          <ThemeTab />
        </>
      )}
      {section === 'language' && (
        <>
          <div className="card">
            <LanguageSelector />
          </div>
          <LanguagesTab />
        </>
      )}
      {section === 'workspace' && (
        <Suspense fallback={<SettingsSpinner />}>
          <AdminPage section="workspace" embedded />
        </Suspense>
      )}
      {section === 'rbac' && isAccountAdmin && (
        <Suspense fallback={<SettingsSpinner />}>
          <AdminPage section="rbac" embedded />
        </Suspense>
      )}
      {section === 'audit' && isAccountAdmin && (
        <Suspense fallback={<SettingsSpinner />}>
          <AdminPage section="audit" embedded />
        </Suspense>
      )}
    </>
  );
}
