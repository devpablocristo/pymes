import { createVerticalRequest } from './verticalApi';

function translateWorkshopsError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de talleres.';
    case 'organization not found':
      return 'No se encontro la organizacion.';
    case 'invalid org':
    case 'invalid org identifier':
      return 'No hay una empresa válida en la sesión para Talleres. Con Clerk: completá el onboarding (al final se crea la organización), recargá la página o cerrá sesión y volvé a entrar para renovar el token.';
    default:
      return trimmed;
  }
}

export const workshopsRequest = createVerticalRequest({
  envVar: 'VITE_WORKSHOPS_API_URL',
  fallbackPorts: [8282, 8082],
  translateError: translateWorkshopsError,
  timeoutMs: 60_000,
  timeoutMessage:
    'El backend de talleres no respondió a tiempo. Levantá work-backend (puerto 8282), revisá VITE_WORKSHOPS_API_URL y que las migraciones estén aplicadas.',
});
