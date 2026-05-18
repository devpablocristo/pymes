import type { TranslationsByLanguage } from '../types';

export const apiKeysMessages: TranslationsByLanguage = {
  es: {
    'apiKeys.adminOnly.title': 'Solo administradores',
    'apiKeys.adminOnly.body':
      'Gestionar claves API es una operación sensible. Pedile a un administrador del espacio que cree o revoque claves.',
    'apiKeys.loading': 'Cargando…',
    'apiKeys.error.unreachable': 'No pudimos conectar con el servidor. Verificá tu red.',
    'apiKeys.scopes.section': 'Permisos',
    'apiKeys.scopes.consoleRead.label': 'Consola y API: solo lectura',
    'apiKeys.scopes.consoleRead.hint':
      'Consultar datos y configuración; no crear ni modificar (p. ej. reportes, integraciones de solo lectura).',
    'apiKeys.scopes.consoleWrite.label': 'Consola y API: lectura y escritura',
    'apiKeys.scopes.consoleWrite.hint':
      'Operaciones completas en la API y módulos de administración que requieran cambios.',
    'apiKeys.scopes.needOne': 'Elegí al menos un permiso.',
  },
  en: {
    'apiKeys.adminOnly.title': 'Admins only',
    'apiKeys.adminOnly.body': 'Managing API keys is sensitive. Ask a workspace admin to create or revoke keys.',
    'apiKeys.loading': 'Loading…',
    'apiKeys.error.unreachable': 'Could not reach the server. Check your network.',
    'apiKeys.scopes.section': 'Permissions',
    'apiKeys.scopes.consoleRead.label': 'Console & API: read-only',
    'apiKeys.scopes.consoleRead.hint':
      'View data and settings; no create/update (e.g. reporting, read-only integrations).',
    'apiKeys.scopes.consoleWrite.label': 'Console & API: read and write',
    'apiKeys.scopes.consoleWrite.hint': 'Full API usage including changes and admin actions.',
    'apiKeys.scopes.needOne': 'Select at least one permission.',
  },
};
