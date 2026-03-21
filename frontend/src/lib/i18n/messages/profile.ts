import type { TranslationsByLanguage } from '../types';

export const profileMessages: TranslationsByLanguage = {
  es: {
    'profile.page.title': 'Perfil',
    'profile.page.subtitle': 'Gestioná tu cuenta y preferencias',
    'profile.apiMode.title': 'Sesión en este entorno',
    'profile.apiMode.badge': 'Modo consola · clave API',
    'profile.apiMode.lead':
      'Sin sesión Clerk en el navegador: la consola usa JWT o clave API contra el backend. Es el flujo habitual en desarrollo (ver docs/AUTH.md). Abajo tenés el contexto que resolvió el API.',
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
    'profile.account.empty.title': 'Sin perfil de usuario en este modo',
    'profile.account.empty.body':
      'Con solo clave API no hay persona vinculada: es esperado. Para nombre, email y preferencias sincronizadas, usá Clerk en el entorno que corresponda.',
    'profile.account.empty.clerk':
      'Todavía no hay usuario sincronizado en el backend para esta sesión. Verificá el webhook de Clerk o que el usuario exista en la org.',
    'profile.clerk.hintSecurity':
      'Contraseña, 2FA y cuentas conectadas: usá el menú de usuario en la barra inferior (Clerk). Acá solo mostramos datos que expone la API de Pymes.',
    'profile.account.unavailable':
      'No se pudo cargar la cuenta; revisá el aviso de arriba o reintentá.',
    'profile.authMethod.jwt': 'JWT',
    'profile.authMethod.api_key': 'Clave API',
    'profile.authMethod.other': 'Otro',
    'profile.error.unreachable':
      'No se pudo conectar con la API. Levantá el stack con contenedores (`make up` o `docker compose up -d --build`), revisá `docker compose ps` y `VITE_API_URL` (p. ej. http://localhost:8100). Si cambiaste el mapeo de puertos, alineá la URL al host/puerto del control plane.',
    'profile.error.meUnreachable':
      'No se pudo cargar /v1/users/me; la tabla de cuenta puede estar vacía. El resto de la sesión puede seguir mostrándose.',
    'profile.actions.retry': 'Reintentar',
  },
  en: {
    'profile.page.title': 'Profile',
    'profile.page.subtitle': 'Manage your account and preferences',
    'profile.apiMode.title': 'Session in this environment',
    'profile.apiMode.badge': 'Console mode · API key',
    'profile.apiMode.lead':
      'No Clerk session in the browser: the console uses a JWT or API key against the backend. This is the usual dev flow (see docs/AUTH.md). Below is the context the API resolved.',
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
    'profile.account.empty.title': 'No user profile in this mode',
    'profile.account.empty.body':
      'With an API key only there is no linked person: that is expected. For name, email and synced preferences, use Clerk in the right environment.',
    'profile.account.empty.clerk':
      'No user synced in the backend for this session yet. Check the Clerk webhook or org membership.',
    'profile.clerk.hintSecurity':
      'Password, 2FA and connected accounts: use the user menu in the bottom bar (Clerk). This page only shows data from the Pymes API.',
    'profile.account.unavailable': 'Could not load account; see the notice above or retry.',
    'profile.authMethod.jwt': 'JWT',
    'profile.authMethod.api_key': 'API key',
    'profile.authMethod.other': 'Other',
    'profile.error.unreachable':
      'Could not reach the API. Start the stack (`make up` or `docker compose up -d --build`), check `docker compose ps` and VITE_API_URL (e.g. http://localhost:8100). If you changed port mappings, point the URL at the control plane host/port.',
    'profile.error.meUnreachable':
      'Could not load /v1/users/me; account fields may be empty. Session details may still show below.',
    'profile.actions.retry': 'Retry',
  },
};
