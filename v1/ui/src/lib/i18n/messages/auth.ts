import type { TranslationsByLanguage } from '../types';

export const authMessages: TranslationsByLanguage = {
  es: {
    'auth.login.localTitle': 'Ingreso local',
    'auth.login.localDescription': 'Clerk deshabilitado. Usá una clave API para consumir la API desde el frontend.',
    'auth.signup.localTitle': 'Registro local',
    'auth.signup.localDescription': 'Clerk deshabilitado en este ambiente.',
    'auth.goPanel': 'Ir al panel',
  },
  en: {
    'auth.login.localTitle': 'Local sign in',
    'auth.login.localDescription': 'Clerk is disabled. Use an API key to consume the API from the frontend.',
    'auth.signup.localTitle': 'Local sign up',
    'auth.signup.localDescription': 'Clerk is disabled in this environment.',
    'auth.goPanel': 'Go to dashboard',
  },
};
