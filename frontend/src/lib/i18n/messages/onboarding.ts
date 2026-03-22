import type { TranslationsByLanguage } from '../types';

/** Textos del onboarding ligados a Clerk / sesión (el resto del wizard sigue en la página por ahora). */
export const onboardingMessages: TranslationsByLanguage = {
  es: {
    'onboarding.clerk.sessionNotReady':
      'Todavía se está cargando tu sesión. Esperá un momento y volvé a intentar.',
    'onboarding.clerk.organizationFailed':
      'No se pudo crear o activar la organización. Reintentá o volvé a iniciar sesión.',
  },
  en: {
    'onboarding.clerk.sessionNotReady':
      'Your session is still loading. Wait a moment and try again.',
    'onboarding.clerk.organizationFailed':
      'Could not create or activate the organization. Try again or sign in again.',
  },
};
