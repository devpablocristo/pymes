/* eslint-disable react-refresh/only-export-components -- provider + re-exports desde core */
import { createI18nProvider, mergeMessages } from '@devpablocristo/core-browser/i18n';
import { vocab } from '../vocabulary';
import { apiKeysMessages } from './messages/apiKeys';
import { aiMessages } from './messages/ai';
import { authMessages } from './messages/auth';
import { billingMessages } from './messages/billing';
import { calendarMessages } from './messages/calendar';
import { commonMessages } from './messages/common';
import { crudMessages } from './messages/crud';
import { dashboardMessages } from './messages/dashboard';
import { moduleMessages } from './messages/module';
import { onboardingMessages } from './messages/onboarding';
import { profileMessages } from './messages/profile';
import { shellMessages } from './messages/shell';

const messages = mergeMessages(
  commonMessages,
  aiMessages,
  shellMessages,
  crudMessages,
  moduleMessages,
  onboardingMessages,
  profileMessages,
  billingMessages,
  authMessages,
  calendarMessages,
  apiKeysMessages,
  dashboardMessages,
);

const i18n = createI18nProvider({
  namespace: 'pymes-ui',
  storageKey: 'pymes:language',
  defaultLanguage: 'es',
  messages,
  localizeText: vocab,
});

export const LanguageProvider = i18n.Provider;
export const useI18n = i18n.useI18n;
export { toSentenceCase } from '@devpablocristo/core-browser/i18n';
export type { LanguageCode } from '@devpablocristo/core-browser/i18n';
