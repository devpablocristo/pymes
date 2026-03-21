import type { TranslationsByLanguage } from '../types';

export const apiKeysMessages: TranslationsByLanguage = {
  es: {
    'apiKeys.adminOnly.title': 'Solo administradores',
    'apiKeys.adminOnly.body':
      'Gestionar claves API es una operación sensible. Pedile a un administrador del espacio que cree o revoque claves.',
    'apiKeys.loading': 'Cargando…',
  },
  en: {
    'apiKeys.adminOnly.title': 'Admins only',
    'apiKeys.adminOnly.body':
      'Managing API keys is sensitive. Ask a workspace admin to create or revoke keys.',
    'apiKeys.loading': 'Loading…',
  },
};
