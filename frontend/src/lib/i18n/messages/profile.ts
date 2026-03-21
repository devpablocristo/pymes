import type { TranslationsByLanguage } from '../types';

export const profileMessages: TranslationsByLanguage = {
  es: {
    'profile.apiMode.title': 'Sesión en este entorno',
    'profile.apiMode.lead':
      'Clerk no está activo: la consola se identifica con JWT o con clave API. Acá ves el contexto que resolvió el backend.',
    'profile.apiMode.keysCta': 'Gestionar claves API',
    'profile.section.account': 'Cuenta',
    'profile.section.session': 'Identidad y permisos',
    'profile.labels.org': 'Organización',
    'profile.labels.actor': 'Actor',
    'profile.labels.roleRaw': 'Rol (token)',
    'profile.labels.productRole': 'Rol en consola',
    'profile.labels.authMethod': 'Método de auth',
    'profile.labels.scopes': 'Scopes',
    'profile.labels.email': 'Email',
    'profile.labels.name': 'Nombre',
    'profile.accountPlaceholder':
      'No hay usuario sincronizado para este actor (típico con solo clave API).',
    'profile.authMethod.jwt': 'JWT',
    'profile.authMethod.api_key': 'Clave API',
    'profile.authMethod.other': 'Otro',
    'profile.error.unreachable':
      'No se pudo conectar con la API. Levantá el backend (`make cp-run` usa puerto 8100, alineado con VITE_API_URL). Si exportaste PORT=8080, poné VITE_API_URL=http://localhost:8080. Con Docker: `docker compose up cp-backend` y VITE_API_URL=http://localhost:8100.',
    'profile.error.meUnreachable':
      'No se pudo cargar /v1/users/me; la tabla de cuenta puede estar vacía. El resto de la sesión puede seguir mostrándose.',
    'profile.actions.retry': 'Reintentar',
  },
  en: {
    'profile.apiMode.title': 'Session in this environment',
    'profile.apiMode.lead':
      'Clerk is off: the console uses JWT or an API key. Below is what the backend resolved.',
    'profile.apiMode.keysCta': 'Manage API keys',
    'profile.section.account': 'Account',
    'profile.section.session': 'Identity and permissions',
    'profile.labels.org': 'Organization',
    'profile.labels.actor': 'Actor',
    'profile.labels.roleRaw': 'Role (token)',
    'profile.labels.productRole': 'Console role',
    'profile.labels.authMethod': 'Auth method',
    'profile.labels.scopes': 'Scopes',
    'profile.labels.email': 'Email',
    'profile.labels.name': 'Name',
    'profile.accountPlaceholder': 'No synced user for this actor (common with API key only).',
    'profile.authMethod.jwt': 'JWT',
    'profile.authMethod.api_key': 'API key',
    'profile.authMethod.other': 'Other',
    'profile.error.unreachable':
      'Could not reach the API. Start the backend (`make cp-run` listens on 8100, matching VITE_API_URL). If you use PORT=8080, set VITE_API_URL=http://localhost:8080. With Docker: docker compose up cp-backend and VITE_API_URL=http://localhost:8100.',
    'profile.error.meUnreachable':
      'Could not load /v1/users/me; account fields may be empty. Session details may still show below.',
    'profile.actions.retry': 'Retry',
  },
};
