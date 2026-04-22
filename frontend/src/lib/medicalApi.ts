import { createVerticalRequest } from './verticalApi';

function translateMedicalError(message: string): string {
  const trimmed = message.trim();
  switch (trimmed) {
    case '404 page not found':
      return 'La ruta no existe en el backend de medical.';
    case 'organization not found':
      return 'No se encontró la organización.';
    case 'invalid org':
    case 'invalid org identifier':
      return 'No hay una empresa válida en la sesión para Medicina laboral. Con Clerk: completá el onboarding, recargá la página o cerrá sesión y volvé a entrar para renovar el token.';
    default:
      return trimmed;
  }
}

export const medicalRequest = createVerticalRequest({
  envVar: 'VITE_MEDICAL_API_URL',
  fallbackPorts: [8585, 8085],
  translateError: translateMedicalError,
  timeoutMs: 60_000,
  timeoutMessage:
    'El backend de medical no respondió a tiempo. Levantá medical-backend (puerto 8585), revisá VITE_MEDICAL_API_URL y que las migraciones estén aplicadas.',
});
